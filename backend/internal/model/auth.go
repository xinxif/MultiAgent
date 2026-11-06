// Package model 数据库实体定义。
// 认证与用户域。
package model

import (
	"time"

	"gorm.io/datatypes"
)

// User 用户（P0 仅 admin/user 两级角色，不做 RBAC）。
type User struct {
	ID           int64     `gorm:"primaryKey" json:"id"`
	Username     string    `gorm:"size:64;uniqueIndex" json:"username"`
	Email        string    `gorm:"size:128;uniqueIndex" json:"email"`
	PasswordHash string    `gorm:"size:256" json:"-"` // 绝不序列化
	Role         string    `gorm:"size:20;default:user" json:"role"`
	AvatarURL    string    `gorm:"size:512" json:"avatar_url"`
	Status       int16     `gorm:"default:1" json:"status"`
	CreatedAt    time.Time `json:"created_at"`
	UpdatedAt    time.Time `json:"updated_at"`
}

// IsAdmin 是否管理员。
func (u *User) IsAdmin() bool { return u.Role == "admin" }

// Session 对话会话（对外 ID 用 UUID 防遍历）。
type Session struct {
	ID           string         `gorm:"type:uuid;primaryKey;default:gen_random_uuid()" json:"id"`
	UserID       int64          `gorm:"index" json:"user_id"`
	AgentID      *int64         `json:"agent_id"`
	Title        string         `gorm:"size:256;default:新对话" json:"title"`
	Status       string         `gorm:"size:20;default:active" json:"status"` // active/archived
	Pinned       bool           `json:"pinned"`
	MessageCount int            `json:"message_count"`
	TokenUsage   int64          `json:"token_usage"`
	LastMsgAt    *time.Time     `json:"last_msg_at"`
	Metadata     datatypes.JSON `gorm:"type:jsonb;default:'{}'" json:"metadata"`
	CreatedAt    time.Time      `json:"created_at"`
	UpdatedAt    time.Time      `json:"updated_at"`
}

// Message 消息。role: user/assistant/system/tool
type Message struct {
	ID               int64          `gorm:"primaryKey" json:"id"`
	SessionID        string         `gorm:"type:uuid;index:idx_messages_session,priority:1" json:"session_id"`
	Role             string         `gorm:"size:20" json:"role"`
	Content          string         `gorm:"type:text" json:"content"`
	ReasoningContent string         `gorm:"type:text" json:"reasoning_content,omitempty"` // 推理模型思考过程
	ToolCalls        datatypes.JSON `gorm:"type:jsonb" json:"tool_calls,omitempty"`       // 模型工具调用原始结构
	ToolCallID       string         `gorm:"size:128" json:"tool_call_id,omitempty"`
	Refs             datatypes.JSON `gorm:"type:jsonb" json:"refs,omitempty"` // RAG 引用
	ModelName        string         `gorm:"size:64" json:"model_name,omitempty"`
	PromptTokens     int            `json:"prompt_tokens"`
	CompletionTokens int            `json:"completion_tokens"`
	FirstTokenMs     int            `json:"first_token_ms"`
	LatencyMs        int            `json:"latency_ms"`
	Status           int16          `gorm:"default:1" json:"status"` // 1完成 2中断 3失败
	TraceID          string         `gorm:"size:64;default:" json:"trace_id"`
	Metadata         datatypes.JSON `gorm:"type:jsonb;default:'{}'" json:"metadata"` // 含 agent_trace 决策链
	CreatedAt        time.Time      `gorm:"index:idx_messages_session,priority:2" json:"created_at"`
}

// 消息状态。
const (
	MsgStatusDone    int16 = 1
	MsgStatusStopped int16 = 2 // 用户中断
	MsgStatusFailed  int16 = 3
)

// Feedback 用户反馈（坏例回流闭环入口）。
type Feedback struct {
	ID          int64          `gorm:"primaryKey" json:"id"`
	MessageID   int64          `json:"message_id"`
	SessionID   string         `gorm:"type:uuid" json:"session_id"`
	UserID      int64          `json:"user_id"`
	Rating      int16          `json:"rating"` // 1赞 2踩
	Reason      string         `gorm:"size:64" json:"reason"`
	Comment     string         `gorm:"size:512" json:"comment"`
	Resolved    bool           `json:"resolved"`
	EvalCaseID  *int64         `json:"eval_case_id,omitempty"`
	CreatedAt   time.Time      `json:"created_at"`
}

// 反馈评分。
const (
	RatingLike  int16 = 1
	RatingDislike int16 = 2
)
