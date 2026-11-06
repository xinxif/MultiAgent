// Package service 业务编排层。
//
// 职责：组合 repo/llm 网关/RAG 引擎/记忆/工作流等组件，实现面向用例的业务逻辑。
// 事务边界、错误码转换、权限校验都发生在这一层；handler 只做协议适配。
package service

import (
	"context"

	"github.com/redis/go-redis/v9"

	"agent/internal/config"
	"agent/internal/llm"
	"agent/internal/rag/engine"
	"agent/internal/repo"
	"agent/internal/security"
	"agent/internal/tool"
	"agent/internal/vector"
)

// Deps service 层依赖全集（main 装配）。
type Deps struct {
	Cfg      *config.Config
	Repos    *repo.Repos
	Gateway  *llm.Gateway
	Encryptor *security.Encryptor
	JWT      *security.JWTManager
	RDB      *redis.Client
	Vector   vector.Store
	Executor tool.Executor
	Registry tool.Registry // 内置工具注册表（MCP 同步工具时注册执行 handler）
	Streams  StreamPublisher // 异步任务投递（worker 实现，可 nil）
}

// StreamPublisher 异步任务投递接口（internal/worker 实现，接口在此隔离依赖方向）。
type StreamPublisher interface {
	PublishDocETL(ctx context.Context, docID int64) error
	PublishEvalRun(ctx context.Context, runID int64) error
}

// Services 业务服务集合。
type Services struct {
	Deps

	// 默认模型描述符（启动时解析一次，仅作实时解析失败时的降级兜底；
	// 正常路径一律通过 Default*() 实时查库，保证改配置后无需重启）
	defaultChat      *llm.ModelDescriptor
	defaultEmbedding *llm.ModelDescriptor
	defaultRerank    *llm.ModelDescriptor

	ragEngine *engine.Engine // RAG 引擎（main 构造后注入）

	Auth     *AuthService
	Models   *ModelService
	Chat     *ChatService
	Agent    *AgentService
	KB       *KBService
	Workflow *WorkflowService
	MCP      *MCPService
	A2A      *A2AService
	Memory   *MemoryService
}

// New 装配全部业务服务。
func New(d Deps) (*Services, error) {
	s := &Services{Deps: d}

	// ① 默认模型解析（网关保持纯净：查库+解密在 service 完成）
	if err := s.reloadDefaultModels(); err != nil {
		return nil, err
	}

	// ② 各业务服务
	s.Auth = NewAuthService(d.Repos, d.JWT, d.RDB)
	s.Models = NewModelService(d.Repos, d.Encryptor)
	s.Chat = NewChatService(s)
	s.Agent = NewAgentService(d.Repos, d.Cfg)
	s.KB = NewKBService(s)
	s.Workflow = NewWorkflowService(s)
	s.MCP = NewMCPService(s)
	s.A2A = NewA2AService(s)
	s.Memory = NewMemoryService(s)
	return s, nil
}

// DefaultChat 默认对话模型（实时查库解析；失败降级用启动缓存）。
func (s *Services) DefaultChat() *llm.ModelDescriptor {
	if d, err := s.resolveByName(s.Deps.Cfg.LLM.DefaultChat, modelTypeChat); err == nil {
		return d
	}
	return s.defaultChat
}

// DefaultEmbedding 默认向量化模型（实时解析，语义同 DefaultChat）。
func (s *Services) DefaultEmbedding() *llm.ModelDescriptor {
	if d, err := s.resolveByName(s.Deps.Cfg.LLM.DefaultEmbedding, modelTypeEmbed); err == nil {
		return d
	}
	return s.defaultEmbedding
}

// DefaultRerank 默认重排序模型（未配置为 nil）。
func (s *Services) DefaultRerank() *llm.ModelDescriptor { return s.defaultRerank }

// ResolveModelByAlias 按别名解析模型（工作流节点用）。
// alias 为空返回平台默认 chat 模型（实时解析，改配置即时生效）。
func (s *Services) ResolveModelByAlias(ctx context.Context, alias string) (*llm.ModelDescriptor, *llm.ModelDescriptor, error) {
	if alias == "" {
		return s.DefaultChat(), nil, nil
	}
	m, err := s.resolveByName(alias, modelTypeChat)
	if err != nil {
		return nil, nil, err
	}
	return m, nil, nil
}

// SetRAGEngine 注入 RAG 引擎（engine 依赖 gateway+repo，构造顺序在 service 之后）。
func (s *Services) SetRAGEngine(e *engine.Engine) { s.ragEngine = e }

// reloadDefaultModels 解析平台默认模型。
func (s *Services) reloadDefaultModels() error {
	var err error
	if s.defaultChat, err = s.resolveByName(s.Deps.Cfg.LLM.DefaultChat, modelTypeChat); err != nil {
		return err
	}
	if s.defaultEmbedding, err = s.resolveByName(s.Deps.Cfg.LLM.DefaultEmbedding, modelTypeEmbed); err != nil {
		return err
	}
	// rerank 可选：解析失败静默跳过（检索管线自动降级为融合排序）
	s.defaultRerank, _ = s.resolveByName("bge-reranker", modelTypeRerank)
	return nil
}
