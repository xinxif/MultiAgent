// 运维域实体：用量统计与沙箱执行记录。
package model

import (
	"time"

	"gorm.io/datatypes"
)

// UsageLog Token/成本明细（成本面板数据源）。
type UsageLog struct {
	ID               int64          `gorm:"primaryKey" json:"id"`
	UserID           int64          `gorm:"default:0" json:"user_id"`
	AgentID          int64          `gorm:"default:0" json:"agent_id"`
	SessionID        *string        `gorm:"type:uuid" json:"session_id,omitempty"`
	MessageID        int64          `gorm:"default:0" json:"message_id"`
	Scene            string         `gorm:"size:16;default:chat" json:"scene"` // chat/eval/etl
	Model            string         `gorm:"size:64" json:"model"`
	PromptTokens     int            `json:"prompt_tokens"`
	CompletionTokens int            `json:"completion_tokens"`
	CostAmount       float64        `gorm:"type:numeric(12,6);default:0" json:"cost_amount"`
	CreatedAt        time.Time      `gorm:"index:idx_usage_time" json:"created_at"`
}

// 场景标记。
const (
	SceneChat = "chat"
	SceneEval = "eval"
	SceneETL  = "etl"
)

// SandboxExecution 沙箱执行记录（代码只存 hash，不存原文）。
type SandboxExecution struct {
	ID          int64          `gorm:"primaryKey" json:"id"`
	UserID      int64          `json:"user_id"`
	ToolCallID  string         `gorm:"size:64" json:"tool_call_id"`
	Language    string         `gorm:"size:16" json:"language"`
	CodeHash    string         `gorm:"size:64" json:"code_hash"` // sha256
	ExitCode    int            `gorm:"default:-1" json:"exit_code"`
	DurationMs  int            `json:"duration_ms"`
	StdoutHead  string         `gorm:"type:text" json:"stdout_head"`
	StderrHead  string         `gorm:"type:text" json:"stderr_head"`
	Metadata    datatypes.JSON `gorm:"type:jsonb" json:"-"`
	CreatedAt   time.Time      `json:"created_at"`
}
