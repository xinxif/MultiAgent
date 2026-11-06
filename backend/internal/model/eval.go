// 评估域实体。
package model

import (
	"time"

	"gorm.io/datatypes"
)

// EvalDataset 评估数据集。
type EvalDataset struct {
	ID          int64     `gorm:"primaryKey" json:"id"`
	Name        string    `gorm:"size:128" json:"name"`
	Description string    `gorm:"type:text" json:"description"`
	Scene       string    `gorm:"size:16;default:agent" json:"scene"` // agent/rag/e2e
	CaseCount   int       `json:"case_count"`
	CreatedAt   time.Time `json:"created_at"`
	UpdatedAt   time.Time `json:"updated_at"`
}

// EvalCase 评估用例。
type EvalCase struct {
	ID               int64          `gorm:"primaryKey" json:"id"`
	DatasetID        int64          `gorm:"index" json:"dataset_id"`
	Question         string         `gorm:"type:text" json:"question"`
	ExpectedAnswer   string         `gorm:"type:text" json:"expected_answer,omitempty"`
	ExpectedTools    datatypes.JSON `gorm:"type:jsonb;default:'[]'" json:"expected_tools"`
	ExpectedChunkIDs datatypes.JSON `gorm:"type:jsonb;default:'[]'" json:"expected_chunk_ids"`
	Tags             string         `gorm:"size:255" json:"tags"`
	Source           string         `gorm:"size:16;default:manual" json:"source"` // manual/import/feedback
	CreatedAt        time.Time      `json:"created_at"`
}

// EvalRun 评估执行。
type EvalRun struct {
	ID         int64          `gorm:"primaryKey" json:"id"`
	DatasetID  int64          `gorm:"index" json:"dataset_id"`
	AgentID    *int64         `json:"agent_id"`
	Config     datatypes.JSON `gorm:"type:jsonb;default:'{}'" json:"config"`
	Status     string         `gorm:"size:16;default:pending" json:"status"`
	Total      int            `json:"total"`
	Passed     int            `json:"passed"`
	Metrics    datatypes.JSON `gorm:"type:jsonb" json:"metrics,omitempty"`
	StartedAt  *time.Time     `json:"started_at"`
	FinishedAt *time.Time     `json:"finished_at"`
	CreatedAt  time.Time      `json:"created_at"`
}

// EvalMetrics 评估指标（写入 EvalRun.Metrics）。
type EvalMetrics struct {
	CompletionRate    float64         `json:"completion_rate"`     // 任务完成率
	Accuracy          float64         `json:"accuracy"`            // 准确性（judge LLM）
	ToolReasonableness float64        `json:"tool_reasonableness"` // 工具调用合理性
	P50LatencyMs      int64           `json:"p50_latency_ms"`
	P95LatencyMs      int64           `json:"p95_latency_ms"`
	TokenEfficiency   float64         `json:"token_efficiency"` // tokens/任务
}

// EvalResult 单用例评估结果。
type EvalResult struct {
	ID           int64          `gorm:"primaryKey" json:"id"`
	RunID        int64          `gorm:"index" json:"run_id"`
	CaseID       int64          `json:"case_id"`
	ActualAnswer string         `gorm:"type:text" json:"actual_answer"`
	ToolTrace    datatypes.JSON `gorm:"type:jsonb" json:"tool_trace,omitempty"`
	Scores       datatypes.JSON `gorm:"type:jsonb;default:'{}'" json:"scores"`
	Passed       bool           `json:"passed"`
	LatencyMs    int            `json:"latency_ms"`
	Tokens       int            `json:"tokens"`
	Error        string         `gorm:"size:512" json:"error"`
}

// Prompt Prompt 模板版本（P2 实验室）。
type Prompt struct {
	ID        int64     `gorm:"primaryKey" json:"id"`
	AgentID   *int64    `json:"agent_id"`
	Name      string    `gorm:"size:128" json:"name"`
	Content   string    `gorm:"type:text" json:"content"`
	Variables datatypes.JSON `gorm:"type:jsonb;default:'[]'" json:"variables"`
	Version   int       `gorm:"default:1" json:"version"`
	IsActive  bool      `json:"is_active"`
	Remark    string    `gorm:"size:255" json:"remark"`
	CreatedAt time.Time `json:"created_at"`
	UpdatedAt time.Time `json:"updated_at"`
}
