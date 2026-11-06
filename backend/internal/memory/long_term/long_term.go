// Package long_term 长期记忆（mem0 思路：抽取 → 去重合并 → 召回注入）。
//
// 流程：
//
//	写入: 每 N 条消息 → LLM 抽取事实(JSON 数组) → 向量化
//	      → 相似查询(阈值) → LLM 判定 ADD/UPDATE/DELETE/NOOP → 落库+向量
//	召回: 对话前 top-k 相似记忆注入 system
package long_term

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"
	"time"

	"agent/internal/llm"
	"agent/internal/model"
	"agent/internal/vector"
)

// MemoryRepo 记忆元数据访问。
type MemoryRepo interface {
	Create(ctx context.Context, m *model.Memory) error
	UpdateContent(ctx context.Context, id int64, content string) error
	Archive(ctx context.Context, id int64) error
	ListActive(ctx context.Context, userID int64, agentID *int64, limit int) ([]model.Memory, error)
}

// Config 长期记忆参数。
type Config struct {
	SimilarityThreshold float32 // 相似去重阈值
	RecallTopK          int     // 召回条数
	ExtractBatch        int     // 每次抽取的消息条数
}

// DefaultConfig 默认参数。
func DefaultConfig() Config {
	return Config{SimilarityThreshold: 0.88, RecallTopK: 3, ExtractBatch: 6}
}

// Manager 长期记忆管理器。
type Manager struct {
	gateway    *llm.Gateway
	extractMdl func() *llm.ModelDescriptor // 抽取用低成本模型（实时解析取值函数）
	embedMdl   func() *llm.ModelDescriptor
	repo       MemoryRepo
	store      vector.Store
	collection string
	cfg        Config
}

// NewManager 构造（传入 service 层 Default*() 实时解析函数，改配置无需重启）。
func NewManager(gateway *llm.Gateway, extractMdl, embedMdl func() *llm.ModelDescriptor, repo MemoryRepo, store vector.Store) *Manager {
	return &Manager{
		gateway: gateway, extractMdl: extractMdl, embedMdl: embedMdl,
		repo: repo, store: store, collection: "memories", cfg: DefaultConfig(),
	}
}

// extractPrompt 事实抽取提示词。
const extractPrompt = `从对话中提取关于用户的长期有用事实（偏好、背景、约束、进行中的事项）。
规则：1. 每条事实一句独立陈述；2. 只输出 JSON 数组，无其他文字；3. 无可提取内容输出 []。
示例：["用户偏好使用 Go 语言","用户正在准备系统设计面试"]`

// opsPrompt 记忆操作判定提示词。
const opsPrompt = `基于新记忆与已有记忆列表，判定每条新记忆的操作：
ADD(新增)、UPDATE(id,更新为)、DELETE(id,过时删除)、NOOP(重复忽略)。
只输出 JSON 数组：[{"op":"ADD","content":"..."},{"op":"UPDATE","id":1,"content":"..."},{"op":"DELETE","id":2}]`

// ExtractAndStore 从消息中抽取事实并写入记忆。
func (m *Manager) ExtractAndStore(ctx context.Context, userID int64, agentID *int64, msgs []llm.Message) error {
	// ① LLM 抽取事实
	var b strings.Builder
	for _, msg := range msgs {
		b.WriteString(msg.Role + ": " + msg.Content + "\n")
	}
	resp, _, err := m.gateway.Chat(ctx, m.extractMdl(), nil, &llm.ChatRequest{
		Messages: []llm.Message{
			{Role: llm.RoleSystem, Content: extractPrompt},
			{Role: llm.RoleUser, Content: b.String()},
		},
		Temperature: 0.1,
	})
	if err != nil {
		return err
	}

	var facts []string
	if err := json.Unmarshal([]byte(extractJSON(resp.Content)), &facts); err != nil || len(facts) == 0 {
		return nil // 无事实或解析失败静默跳过
	}

	// ② 向量化事实
	factVecs, err := m.gateway.Embed(ctx, m.embedMdl(), facts)
	if err != nil {
		return fmt.Errorf("embed facts: %w", err)
	}

	// ③ 相似查询已有记忆
	existing, err := m.store.Search(ctx, m.collection, factVecs[0], 10)
	if err != nil {
		existing = nil // 检索失败不阻塞写入
	}
	candidates := make([]string, 0, len(existing))
	for _, h := range existing {
		if mm, e := m.repo.ListActive(ctx, userID, agentID, 50); e == nil {
			for _, item := range mm {
				if item.ID == h.ID {
					candidates = append(candidates, fmt.Sprintf(`{"id":%d,"content":%q}`, item.ID, item.Content))
				}
			}
		}
	}

	// ④ LLM 判定操作
	opsMsg := fmt.Sprintf("已有记忆: [%s]\n新事实: %q", strings.Join(candidates, ","), facts)
	opsResp, _, err := m.gateway.Chat(ctx, m.extractMdl(), nil, &llm.ChatRequest{
		Messages: []llm.Message{
			{Role: llm.RoleSystem, Content: opsPrompt},
			{Role: llm.RoleUser, Content: opsMsg},
		},
		Temperature: 0.1,
	})
	if err != nil {
		return err
	}

	var ops []struct {
		Op      string `json:"op"`
		ID      int64  `json:"id"`
		Content string `json:"content"`
	}
	_ = json.Unmarshal([]byte(extractJSON(opsResp.Content)), &ops)

	// ⑤ 执行操作
	for i, op := range ops {
		switch op.Op {
		case "ADD", "":
			if i >= len(factVecs) {
				continue
			}
			mem := &model.Memory{UserID: userID, AgentID: agentID, Content: op.Content, Scope: "user"}
			if err := m.repo.Create(ctx, mem); err != nil {
				continue
			}
			agentIDVal := int64(0)
			if agentID != nil {
				agentIDVal = *agentID
			}
			_ = m.store.Upsert(ctx, m.collection, []vector.Record{{
				ID: mem.ID, Vector: factVecs[i],
				Metadata: map[string]any{"user_id": userID, "agent_id": agentIDVal},
			}})
		case "UPDATE":
			_ = m.repo.UpdateContent(ctx, op.ID, op.Content)
		case "DELETE":
			_ = m.repo.Archive(ctx, op.ID)
		}
	}
	return nil
}

// Recall 召回相关记忆（注入 system 的文本）。
func (m *Manager) Recall(ctx context.Context, userID int64, agentID *int64, query string) ([]model.Memory, error) {
	vec, err := m.gateway.Embed(ctx, m.embedMdl(), []string{query})
	if err != nil {
		return nil, err
	}
	hits, err := m.store.Search(ctx, m.collection, vec[0], m.cfg.RecallTopK)
	if err != nil {
		return nil, err
	}
	if len(hits) == 0 {
		return nil, nil
	}

	all, err := m.repo.ListActive(ctx, userID, agentID, 200)
	if err != nil {
		return nil, err
	}
	byID := map[int64]model.Memory{}
	for _, mm := range all {
		byID[mm.ID] = mm
	}

	out := make([]model.Memory, 0, len(hits))
	now := time.Now()
	for _, h := range hits {
		if mm, ok := byID[h.ID]; ok {
			mm.HitCount++
			t := now
			mm.LastHitAt = &t
			out = append(out, mm)
		}
	}
	return out, nil
}

// extractJSON 提取 JSON 数组。
func extractJSON(s string) string {
	if i := strings.Index(s, "["); i >= 0 {
		if j := strings.LastIndex(s, "]"); j > i {
			return s[i : j+1]
		}
	}
	return "[]"
}
