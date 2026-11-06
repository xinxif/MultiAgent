// 模型管理服务：供应商/模型配置 CRUD 与"配置 → 运行时描述符"解析。
//
// 网关不查库 —— model_configs 是"声明态"（存在 DB、
// api_key 加密），ModelDescriptor 是"运行态"（明文、可直接发起请求）。
// 桥接两者（查库/解密/组装）正是 service 层的职责。
package service

import (
	"context"

	"agent/internal/llm"
	"agent/internal/model"
	"agent/internal/pkg/errcode"
	"agent/internal/repo"
)

// 模型类型（与 model 包常量对齐的内部简写）。
const (
	modelTypeChat    = "chat"
	modelTypeEmbed   = "embedding"
	modelTypeRerank  = "rerank"
)

// ModelService 模型管理。
type ModelService struct {
	repos     *repo.Repos
	encryptor Encryptor
}

// Encryptor 加密器接口（便于测试桩注入）。
type Encryptor interface {
	Encrypt(plain string) (string, error)
	Decrypt(cipher string) (string, error)
}

// NewModelService 构造。
func NewModelService(repos *repo.Repos, enc Encryptor) *ModelService {
	return &ModelService{repos: repos, encryptor: enc}
}

// ---- 供应商 CRUD ----

// ListProviders 供应商列表（API Key 打码回显：前2位+****+后4位，"" 表示未配置）。
func (s *ModelService) ListProviders(ctx context.Context) ([]model.ProviderView, error) {
	list, err := s.repos.LLM.ListProviders(ctx)
	if err != nil {
		return nil, err
	}
	out := make([]model.ProviderView, 0, len(list))
	for i := range list {
		p := &list[i]
		// 密文解密后打码；解密失败按未配置处理（不阻塞列表）
		masked := ""
		if plain, e := s.encryptor.Decrypt(p.APIKey); e == nil {
			masked = model.MaskAPIKey(plain)
		}
		out = append(out, model.ProviderView{
			ID: p.ID, Code: p.Code, Name: p.Name, BaseURL: p.BaseURL,
			APIKeyMasked: masked, Status: p.Status,
			CreatedAt: p.CreatedAt, UpdatedAt: p.UpdatedAt,
		})
	}
	return out, nil
}

// CreateProvider 创建供应商（api_key 加密落库，status 缺省启用）。
func (s *ModelService) CreateProvider(ctx context.Context, req *model.ProviderUpsert) error {
	if req.Code == nil || req.Name == nil || req.BaseURL == nil {
		return errcode.ErrMissingParam
	}
	enc, err := s.encryptor.Encrypt(derefStr(req.APIKey))
	if err != nil {
		return errcode.ErrInternal.WithCause(err)
	}
	p := &model.ModelProvider{
		Code:    *req.Code,
		Name:    *req.Name,
		BaseURL: *req.BaseURL,
		APIKey:  enc,
		Status:  1, // 默认启用；显式传 status 可覆盖
	}
	if req.Status != nil {
		p.Status = *req.Status
	}
	return s.repos.LLM.CreateProvider(ctx, p)
}

// UpdateProvider 更新供应商（读-改-写合并：req 中 nil 字段保留原值）。
// 修复历史 bug：此前整包 Save 绑定零值，未传 api_key 会清空密钥、未传 status 会停用。
func (s *ModelService) UpdateProvider(ctx context.Context, id int64, req *model.ProviderUpsert) error {
	p, err := s.repos.LLM.GetProvider(ctx, id)
	if err != nil {
		return errcode.ErrProviderNotFound.WithCause(err)
	}
	if req.Code != nil {
		p.Code = *req.Code
	}
	if req.Name != nil {
		p.Name = *req.Name
	}
	if req.BaseURL != nil {
		p.BaseURL = *req.BaseURL
	}
	// 约定：api_key 留空/未传 = 不修改（前端表单不回显密钥）
	if req.APIKey != nil && *req.APIKey != "" {
		enc, err := s.encryptor.Encrypt(*req.APIKey)
		if err != nil {
			return errcode.ErrInternal.WithCause(err)
		}
		p.APIKey = enc
	}
	if req.Status != nil {
		p.Status = *req.Status
	}
	return s.repos.LLM.UpdateProvider(ctx, p)
}

// DeleteProvider 删除供应商。
func (s *ModelService) DeleteProvider(ctx context.Context, id int64) error {
	return s.repos.LLM.DeleteProvider(ctx, id)
}

// ---- 模型配置 CRUD ----

// ListModels 模型列表。
func (s *ModelService) ListModels(ctx context.Context, typ string) ([]model.ModelConfig, error) {
	return s.repos.LLM.ListModels(ctx, typ)
}

// ListChatModels 对话模型选择列表（对话页下拉用，普通用户可访问）。
// 只返回"模型与供应商均已启用"的 chat 模型，字段精简到选择器所需。
func (s *ModelService) ListChatModels(ctx context.Context) ([]ChatModelItem, error) {
	all, err := s.repos.LLM.ListModels(ctx, modelTypeChat)
	if err != nil {
		return nil, err
	}
	out := make([]ChatModelItem, 0, len(all))
	for i := range all {
		m := &all[i]
		if m.Status != 1 || m.Provider == nil || m.Provider.Status != 1 {
			continue // 停用的模型/供应商不出现在选择器
		}
		label := m.Alias
		if label == "" {
			label = m.ModelName
		}
		out = append(out, ChatModelItem{
			ID: m.ID, Label: label, ModelName: m.ModelName,
			Provider: m.Provider.Name, IsDefault: m.IsDefault,
		})
	}
	return out, nil
}

// ChatModelItem 对话页模型选择器条目。
type ChatModelItem struct {
	ID        int64  `json:"id"`
	Label     string `json:"label"`      // 展示名（alias 优先）
	ModelName string `json:"model_name"` // 上游模型名
	Provider  string `json:"provider"`   // 供应商展示名
	IsDefault bool   `json:"is_default"`
}

// CreateModel 创建模型配置（类型等关键字段必填，其余取默认值）。
func (s *ModelService) CreateModel(ctx context.Context, req *model.ModelUpsert) error {
	if req.ProviderID == nil || req.ModelName == nil || req.Type == nil {
		return errcode.ErrMissingParam
	}
	m := &model.ModelConfig{
		ProviderID:    *req.ProviderID,
		ModelName:     *req.ModelName,
		Alias:         derefStr(req.Alias),
		Type:          *req.Type,
		ContextWindow: 8192,
		MaxOutput:     4096,
		Priority:      100,
		Capabilities:  model.CapabilitiesJSON(derefCaps(req.Capabilities)),
		Status:        1,
	}
	if req.ContextWindow != nil {
		m.ContextWindow = *req.ContextWindow
	}
	if req.MaxOutput != nil {
		m.MaxOutput = *req.MaxOutput
	}
	if req.InputPrice != nil {
		m.InputPrice = *req.InputPrice
	}
	if req.OutputPrice != nil {
		m.OutputPrice = *req.OutputPrice
	}
	if req.Priority != nil {
		m.Priority = *req.Priority
	}
	if req.IsDefault != nil {
		m.IsDefault = *req.IsDefault
	}
	if req.Status != nil {
		m.Status = *req.Status
	}
	return s.repos.LLM.CreateModel(ctx, m)
}

// UpdateModel 更新模型配置（读-改-写合并，语义同 UpdateProvider）。
func (s *ModelService) UpdateModel(ctx context.Context, id int64, req *model.ModelUpsert) error {
	m, err := s.repos.LLM.GetModel(ctx, id)
	if err != nil {
		return errcode.ErrProviderNotFound.WithCause(err)
	}
	if req.ProviderID != nil {
		m.ProviderID = *req.ProviderID
	}
	if req.ModelName != nil {
		m.ModelName = *req.ModelName
	}
	if req.Alias != nil {
		m.Alias = *req.Alias
	}
	if req.Type != nil {
		m.Type = *req.Type
	}
	if req.ContextWindow != nil {
		m.ContextWindow = *req.ContextWindow
	}
	if req.MaxOutput != nil {
		m.MaxOutput = *req.MaxOutput
	}
	if req.InputPrice != nil {
		m.InputPrice = *req.InputPrice
	}
	if req.OutputPrice != nil {
		m.OutputPrice = *req.OutputPrice
	}
	if req.Priority != nil {
		m.Priority = *req.Priority
	}
	if req.Capabilities != nil {
		m.Capabilities = model.CapabilitiesJSON(*req.Capabilities)
	}
	if req.IsDefault != nil {
		m.IsDefault = *req.IsDefault
	}
	if req.Status != nil {
		m.Status = *req.Status
	}
	return s.repos.LLM.UpdateModel(ctx, m)
}

// DeleteModel 删除模型配置。
func (s *ModelService) DeleteModel(ctx context.Context, id int64) error {
	return s.repos.LLM.DeleteModel(ctx, id)
}

// ---- 请求体辅助（nil 解引用默认值） ----

func derefStr(p *string) string {
	if p == nil {
		return ""
	}
	return *p
}

func derefCaps(p *[]string) []string {
	if p == nil {
		return nil
	}
	return *p
}

// ---- 描述符解析 ----

// ResolveModel 把 model_config_id 解析为运行时描述符（每次实时查库+解密，
// 保证管理页改配置后无需重启即生效；返回值为本请求独享，可安全修改采样参数）。
// fallbackID 为 Agent 配置的备用模型（0 表示无备用）。
func (s *Services) ResolveModel(ctx context.Context, modelConfigID, fallbackID int64) (*llm.ModelDescriptor, *llm.ModelDescriptor, error) {
	if modelConfigID == 0 {
		return s.DefaultChat(), nil, nil
	}
	m, err := s.Deps.Repos.LLM.GetModel(ctx, modelConfigID)
	if err != nil {
		return nil, nil, errcode.ErrProviderNotFound.WithCause(err)
	}
	// 采样参数以模型配置为准（Agent 级覆盖在 ChatService 组装时完成）
	primary, err := s.descriptorOf(m, 0, 0)
	if err != nil {
		return nil, nil, err
	}

	var fallback *llm.ModelDescriptor
	if fallbackID > 0 {
		if fb, err := s.Deps.Repos.LLM.GetModel(ctx, fallbackID); err == nil {
			fallback, _ = s.descriptorOf(fb, 0, 0)
		}
	}
	return primary, fallback, nil
}

// resolveByName 按别名解析（alias → 配置 → 描述符）。
func (s *Services) resolveByName(alias, typ string) (*llm.ModelDescriptor, error) {
	m, err := s.Deps.Repos.LLM.DefaultModel(context.Background(), alias, typ)
	if err != nil {
		return nil, errcode.ErrProviderNotFound.WithMsg("默认模型 %q 未配置", alias).WithCause(err)
	}
	return s.descriptorOf(m, 0, 0)
}

// descriptorOf 单条配置 → 描述符（解密 api_key）。
// temp/topP ≤0 时取模型配置缺省（0.7/0.9）。
func (s *Services) descriptorOf(m *model.ModelConfig, temp, topP float32) (*llm.ModelDescriptor, error) {
	if m.Provider == nil {
		return nil, errcode.ErrProviderNotFound.WithMsg("模型 %q 未关联供应商", m.Alias)
	}
	apiKey, err := s.Deps.Encryptor.Decrypt(m.Provider.APIKey)
	if err != nil {
		return nil, errcode.ErrInternal.WithCause(err)
	}
	if temp <= 0 {
		temp = 0.7
	}
	if topP <= 0 {
		topP = 0.9
	}
	return &llm.ModelDescriptor{
		ProviderCode: m.Provider.Code,
		BaseURL:      m.Provider.BaseURL,
		APIKey:       apiKey,
		ModelName:    m.ModelName,
		Temperature:  temp,
		TopP:         topP,
		MaxTokens:    m.MaxOutput,
	}, nil
}
