// 工作流域实体。
package model

import (
	"time"

	"gorm.io/datatypes"
)

// Workflow 工作流定义（DSL 为 YAML 文本）。
type Workflow struct {
	ID          int64     `gorm:"primaryKey" json:"id"`
	Name        string    `gorm:"size:128" json:"name"`
	Description string    `gorm:"type:text" json:"description"`
	DSL         string    `gorm:"column:dsl;type:text" json:"dsl"`
	Version     int       `gorm:"default:1" json:"version"`
	IsActive    bool      `json:"is_active"`
	CreatorID   int64     `json:"creator_id"`
	Status      int16     `gorm:"default:1" json:"status"` // default:1 启用——零值 0 会被误当成"停用"，未显式传 status 时落库取 DB 默认 1
	CreatedAt   time.Time `json:"created_at"`
	UpdatedAt   time.Time `json:"updated_at"`
}

// WorkflowRun 工作流执行记录。
type WorkflowRun struct {
	ID          int64          `gorm:"primaryKey" json:"id"`
	WorkflowID  int64          `gorm:"index" json:"workflow_id"`
	TriggerType string         `gorm:"size:20;default:manual" json:"trigger_type"`
	Status      string         `gorm:"size:24;index;default:running" json:"status"`
	Input       datatypes.JSON `gorm:"type:jsonb;default:'{}'" json:"input"`
	Output      datatypes.JSON `gorm:"type:jsonb" json:"output,omitempty"`
	Error       string         `gorm:"size:512" json:"error"`
	TraceID     string         `gorm:"size:64" json:"trace_id"`
	StartedAt   *time.Time     `json:"started_at"`
	FinishedAt  *time.Time     `json:"finished_at"`
	CreatedAt   time.Time      `json:"created_at"`

	// 非库字段：步骤轨迹
	Steps []WorkflowStepRun `gorm:"foreignKey:RunID" json:"steps,omitempty"`
}

// 工作流执行状态。
const (
	RunStatusRunning   = "running"
	RunStatusWaiting   = "waiting_approval" // HITL 等待审批
	RunStatusSucceeded = "succeeded"
	RunStatusFailed    = "failed"
	RunStatusCanceled  = "canceled"
)

// WorkflowStepRun 工作流节点级执行快照（断点恢复依据）。
type WorkflowStepRun struct {
	ID         int64          `gorm:"primaryKey" json:"id"`
	RunID      int64          `gorm:"index:idx_step_runs_run,priority:1" json:"run_id"`
	NodeKey    string         `gorm:"size:64" json:"node_key"`
	NodeType   string         `gorm:"size:24" json:"node_type"` // llm/tool/kb/condition/parallel/human/subflow
	Status     string         `gorm:"size:20;default:running" json:"status"`
	Input      datatypes.JSON `gorm:"type:jsonb" json:"input,omitempty"`
	Output     datatypes.JSON `gorm:"type:jsonb" json:"output,omitempty"`
	Error      string         `gorm:"size:512" json:"error"`
	Tokens     int            `json:"tokens"`
	StartedAt  *time.Time     `json:"started_at"`
	FinishedAt *time.Time     `json:"finished_at"`
}

// 节点类型。
const (
	NodeLLM       = "llm"
	NodeTool      = "tool"
	NodeKB        = "kb"
	NodeCondition = "condition"
	NodeParallel  = "parallel"
	NodeHuman     = "human"
	NodeSubflow   = "subflow"
)
