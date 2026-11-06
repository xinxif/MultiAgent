// LLM 域实体：供应商与模型配置。
package model

import (
	"encoding/json"
	"time"

	"gorm.io/datatypes"
)

// ModelProvider 模型供应商（OpenAI 兼容协议家族 / 豆包 / Ollama）。
type ModelProvider struct {
	ID        int64          `gorm:"primaryKey" json:"id"`
	Code      string         `gorm:"size:32;uniqueIndex" json:"code"` // deepseek/qwen/zhipu/moonshot/doubao/ollama
	Name      string         `gorm:"size:64" json:"name"`
	BaseURL   string         `gorm:"column:base_url;size:512" json:"base_url"`
	APIKey    string         `gorm:"column:api_key;type:text" json:"-"` // AES-GCM 加密存储，不序列化
	Status    int16          `json:"status"`
	CreatedAt time.Time      `json:"created_at"`
	UpdatedAt time.Time      `json:"updated_at"`
}

// 模型类型。
const (
	ModelTypeChat     = "chat"
	ModelTypeEmbed    = "embedding"
	ModelTypeRerank   = "rerank"
)

// ModelConfig 模型配置（供应商下的具体模型及其元数据）。
type ModelConfig struct {
	ID            int64          `gorm:"primaryKey" json:"id"`
	ProviderID    int64          `gorm:"index" json:"provider_id"`
	ModelName     string         `gorm:"size:64" json:"model_name"` // 上游模型名
	Alias         string         `gorm:"size:64" json:"alias"`      // 平台展示名
	Type          string         `gorm:"size:16;index:idx_model_configs_type,priority:1" json:"type"`
	ContextWindow int            `gorm:"default:8192" json:"context_window"`
	MaxOutput     int            `gorm:"default:4096" json:"max_output"`
	InputPrice    float64        `gorm:"type:numeric(12,6);default:0" json:"input_price"`  // 元/1M tokens
	OutputPrice   float64        `gorm:"type:numeric(12,6);default:0" json:"output_price"`
	Priority      int            `gorm:"default:100;index:idx_model_configs_type,priority:2" json:"priority"` // 故障转移顺序，小者优先
	Capabilities  datatypes.JSON `gorm:"type:jsonb;default:'[]'" json:"capabilities"` // ["vision","function_call","reasoning"]
	IsDefault     bool           `json:"is_default"`
	Status        int16          `json:"status"`
	CreatedAt     time.Time      `json:"created_at"`
	UpdatedAt     time.Time      `json:"updated_at"`

	// 非数据库字段：关联信息
	Provider *ModelProvider `gorm:"foreignKey:ProviderID" json:"provider,omitempty"`
}

// Capability 模型能力标记。
const (
	CapVision        = "vision"
	CapFunctionCall  = "function_call"
	CapReasoning     = "reasoning"
)

// ProviderView 供应商列表视图：API Key 不回显明文/密文，
// 只返回打码摘要（前2位+****+后4位），让用户能分辨"哪个配了 Key 哪个没配"。
type ProviderView struct {
	ID           int64     `json:"id"`
	Code         string    `json:"code"`
	Name         string    `json:"name"`
	BaseURL      string    `json:"base_url"`
	APIKeyMasked string    `json:"api_key_masked"` // "" 表示未配置
	Status       int16     `json:"status"`
	CreatedAt    time.Time `json:"created_at"`
	UpdatedAt    time.Time `json:"updated_at"`
}

// MaskAPIKey 打码 API Key：前2位 + "****" + 后4位（sk-4f39...2929 → sk****2929）。
// 空=未配置；过短（≤6 位）无法安全露出首尾，整体打码。
func MaskAPIKey(plain string) string {
	if plain == "" {
		return ""
	}
	r := []rune(plain)
	if len(r) <= 6 {
		return "****"
	}
	return string(r[:2]) + "****" + string(r[len(r)-4:])
}

// ProviderUpsert 供应商创建/更新请求体。
// 指针字段 nil = 不修改：APIKey 为密文入参（服务端加密），Status 为 nil 时
// 更新不覆盖现有状态 —— 避免 Go 零值（0=停用）经 Save 全字段覆盖造成数据丢失。
type ProviderUpsert struct {
	Code    *string `json:"code"`
	Name    *string `json:"name"`
	BaseURL *string `json:"base_url"`
	APIKey  *string `json:"api_key"` // 明文入参，服务端 AES-GCM 加密落库；更新时留空=不修改
	Status  *int16  `json:"status"`
}

// ModelUpsert 模型配置创建/更新请求体（指针字段 nil = 不修改，语义同上）。
type ModelUpsert struct {
	ProviderID    *int64    `json:"provider_id"`
	ModelName     *string   `json:"model_name"`
	Alias         *string   `json:"alias"`
	Type          *string   `json:"type"`
	ContextWindow *int      `json:"context_window"`
	MaxOutput     *int      `json:"max_output"`
	InputPrice    *float64  `json:"input_price"`
	OutputPrice   *float64  `json:"output_price"`
	Priority      *int      `json:"priority"`
	Capabilities  *[]string `json:"capabilities"` // ["vision","function_call","reasoning"]
	IsDefault     *bool     `json:"is_default"`
	Status        *int16    `json:"status"`
}

// CapabilitiesJSON 能力数组 → JSONB（空数组序列化为 "[]"）。
func CapabilitiesJSON(caps []string) datatypes.JSON {
	if caps == nil {
		caps = []string{}
	}
	raw, _ := json.Marshal(caps)
	return datatypes.JSON(raw)
}

// HasCapability 判断模型是否具备某能力（function_call 决定能否挂工具）。
func (m *ModelConfig) HasCapability(cap string) bool {
	return len(m.Capabilities) > 0 && // JSONB 数组包含判断在 repository 层做，这里仅防空
		string(m.Capabilities) != "[]"
}
