// Package repo 数据访问层（GORM）。
//
// 职责边界：
//   - 只做数据读写与查询编排，不含业务规则（业务在 service）；
//   - 统一错误：gorm.ErrRecordNotFound 上抛，由 service 转换为 errcode；
//   - 事务：聚合根内多表写用 db.Transaction；跨聚合由 service 组合多个 repo 方法。
package repo

import (
	"gorm.io/gorm"
)

// Repos 全量数据访问的聚合入口（依赖注入到 service 层）。
type Repos struct {
	DB *gorm.DB

	User     *UserRepo
	Session  *SessionRepo
	Agent    *AgentRepo
	Tool     *ToolRepo
	LLM      *LLMRepo
	KB       *KBRepo
	Memory   *MemoryRepo
	Workflow *WorkflowRepo
	MCP      *MCPRepo
	A2A      *A2ARepo
	Usage    *UsageRepo
	Eval     *EvalRepo
}

// New 构造全部 repo（共享同一 gorm.DB 连接池）。
func New(db *gorm.DB) *Repos {
	return &Repos{
		DB:       db,
		User:     &UserRepo{db: db},
		Session:  &SessionRepo{db: db},
		Agent:    &AgentRepo{db: db},
		Tool:     &ToolRepo{db: db},
		LLM:      &LLMRepo{db: db},
		KB:       &KBRepo{db: db},
		Memory:   &MemoryRepo{db: db},
		Workflow: &WorkflowRepo{db: db},
		MCP:      &MCPRepo{db: db},
		A2A:      &A2ARepo{db: db},
		Usage:    &UsageRepo{db: db},
		Eval:     &EvalRepo{db: db},
	}
}
