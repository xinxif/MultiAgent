// Agent 与工具域实体。
package model

import (
	"time"

	"gorm.io/datatypes"
)

// Agent Agent 配置（host 负责路由聚合，expert 负责专项执行）。
type Agent struct {
	ID                int64          `gorm:"primaryKey" json:"id"`
	Name              string         `gorm:"size:128" json:"name"`
	Description       string         `gorm:"type:text" json:"description"`
	Type              string         `gorm:"size:20;default:expert" json:"type"` // host/expert
	SystemPrompt      string         `gorm:"type:text" json:"system_prompt"`
	ModelConfigID     int64          `json:"model_config_id"`
	FallbackModelID   *int64         `gorm:"column:fallback_model_config_id" json:"fallback_model_config_id"` // 显式列名：默认命名会错误映射为 fallback_model_id
	Temperature       float32        `gorm:"default:0.7" json:"temperature"`
	TopP              float32        `gorm:"default:0.9" json:"top_p"`
	MaxTokens         int            `gorm:"default:4096" json:"max_tokens"`
	MaxIterations     int            `gorm:"default:10" json:"max_iterations"`
	MemoryEnabled     bool           `gorm:"default:true" json:"memory_enabled"`
	MemoryWindow      int            `gorm:"default:20" json:"memory_window"`
	MemoryLongEnabled bool           `gorm:"default:false" json:"memory_long_enabled"`
	KBIDs             datatypes.JSON `gorm:"type:jsonb;default:'[]'" json:"kb_ids"`    // 绑定知识库
	Tools             datatypes.JSON `gorm:"type:jsonb;default:'[]'" json:"tools"`     // [{tool_id, config}]
	IsPreset          bool           `json:"is_preset"`
	Config            datatypes.JSON `gorm:"type:jsonb;default:'{}'" json:"config"`
	Status            int16          `gorm:"default:1" json:"status"` // default:1 启用——零值 0 会被误当成"停用"，未显式传 status 时落库取 DB 默认 1
	CreatedAt         time.Time      `json:"created_at"`
	UpdatedAt         time.Time      `json:"updated_at"`
}

// Agent 类型。
const (
	AgentTypeHost   = "host"
	AgentTypeExpert = "expert"
)

// AgentToolBinding Agent.tools JSON 数组元素。
type AgentToolBinding struct {
	ToolID int64          `json:"tool_id"`
	Config map[string]any `json:"config,omitempty"`
}

// Tool 工具（builtin / mcp / custom）。
type Tool struct {
	ID            int64          `gorm:"primaryKey" json:"id"`
	Code          string         `gorm:"size:64;uniqueIndex" json:"code"`
	Name          string         `gorm:"size:128" json:"name"`
	Type          string         `gorm:"size:20" json:"type"`
	Description   string         `gorm:"type:text" json:"description"` // 给模型看
	Parameters    datatypes.JSON `gorm:"type:jsonb" json:"parameters"` // JSON Schema
	Handler       string         `gorm:"size:64" json:"handler,omitempty"`
	MCPServerID   *int64         `json:"mcp_server_id,omitempty"`
	Endpoint      string         `gorm:"size:512" json:"endpoint,omitempty"`
	AuthConfig    datatypes.JSON `gorm:"type:jsonb" json:"auth_config,omitempty"` // 加密
	DefaultConfig datatypes.JSON `gorm:"type:jsonb;default:'{}'" json:"default_config"`
	TimeoutMs     int            `gorm:"default:30000" json:"timeout_ms"`
	IsDangerous   bool           `json:"is_dangerous"` // HITL 审批依据
	Status        int16          `json:"status"`
	CreatedAt     time.Time      `json:"created_at"`
	UpdatedAt     time.Time      `json:"updated_at"`
}

// 工具类型。
const (
	ToolTypeBuiltin = "builtin"
	ToolTypeMCP     = "mcp"
	ToolTypeCustom  = "custom"
)

// MCPServer MCP 服务端配置。
type MCPServer struct {
	ID            int64          `gorm:"primaryKey" json:"id"`
	Name          string         `gorm:"size:64" json:"name"`
	Transport     string         `gorm:"size:20" json:"transport"` // streamable_http/stdio/sse
	URL           string         `gorm:"size:512" json:"url"`
	Command       string         `gorm:"size:512" json:"command"`
	Args          datatypes.JSON `gorm:"type:jsonb;default:'[]'" json:"args"`
	Env           datatypes.JSON `gorm:"type:jsonb;default:'{}'" json:"env"`
	Headers       datatypes.JSON `gorm:"type:jsonb;default:'{}'" json:"headers"`
	Status        string         `gorm:"size:20;default:disconnected" json:"status"`
	LastHealthAt  *time.Time     `json:"last_health_at"`
	ToolsCache    datatypes.JSON `gorm:"type:jsonb;default:'[]'" json:"tools_cache"`
	CreatedAt     time.Time      `json:"created_at"`
	UpdatedAt     time.Time      `json:"updated_at"`
}

// A2AAgent A2A 远程 Agent 注册表。
type A2AAgent struct {
	ID           int64          `gorm:"primaryKey" json:"id"`
	Name         string         `gorm:"size:128" json:"name"`
	BaseURL      string         `gorm:"size:512" json:"base_url"`
	Card         datatypes.JSON `gorm:"type:jsonb" json:"card"` // Agent Card 快照
	Status       string         `gorm:"size:20;default:unknown" json:"status"`
	LastHealthAt *time.Time     `json:"last_health_at"`
        CreatedAt    time.Time      `json:"created_at"`
        UpdatedAt    time.Time      `json:"updated_at"`
}

// TableName 显式指定表名：GORM 默认命名会把 A2AAgent 复数化为 a2_a_agents（数字后强制分段），
// 与迁移建表的 a2a_agents 不一致。
func (A2AAgent) TableName() string { return "a2a_agents" }
