// Agent 与工具数据访问（实现 tool.ToolRepo 接口）。
package repo

import (
	"context"

	"gorm.io/gorm"

	"agent/internal/model"
)

// AgentRepo Agent 表。
type AgentRepo struct {
	db *gorm.DB
}

// Create 创建 Agent。
func (r *AgentRepo) Create(ctx context.Context, a *model.Agent) error {
	return r.db.WithContext(ctx).Create(a).Error
}

// Update 更新。
func (r *AgentRepo) Update(ctx context.Context, a *model.Agent) error {
	return r.db.WithContext(ctx).Save(a).Error
}

// Delete 删除。
func (r *AgentRepo) Delete(ctx context.Context, id int64) error {
	return r.db.WithContext(ctx).Delete(&model.Agent{}, id).Error
}

// GetByID 详情（含工具绑定）。
func (r *AgentRepo) GetByID(ctx context.Context, id int64) (*model.Agent, error) {
	var a model.Agent
	if err := r.db.WithContext(ctx).First(&a, id).Error; err != nil {
		return nil, err
	}
	return &a, nil
}

// List 列表（keyword 匹配名称/描述）。
func (r *AgentRepo) List(ctx context.Context, keyword string, offset, limit int) ([]model.Agent, int64, error) {
	var (
		list  []model.Agent
		total int64
	)
	q := r.db.WithContext(ctx).Model(&model.Agent{})
	if keyword != "" {
		q = q.Where("name ILIKE ? OR description ILIKE ?", "%"+keyword+"%", "%"+keyword+"%")
	}
	if err := q.Count(&total).Error; err != nil {
		return nil, 0, err
	}
	err := q.Order("id ASC").Offset(offset).Limit(limit).Find(&list).Error
	return list, total, err
}

// ListHosts 列出全部 host 类型 Agent。
func (r *AgentRepo) ListHosts(ctx context.Context) ([]model.Agent, error) {
	var list []model.Agent
	err := r.db.WithContext(ctx).Where("type = ?", model.AgentTypeHost).Find(&list).Error
	return list, err
}

// ListExpertsByHost 根据 host 的 config.experts JSON 找专家：
// 简化实现：列出全部 expert，由 service 过滤（配置驱动路由）。
func (r *AgentRepo) ListExperts(ctx context.Context) ([]model.Agent, error) {
	var list []model.Agent
	err := r.db.WithContext(ctx).Where("type = ? AND status = 1", model.AgentTypeExpert).Find(&list).Error
	return list, err
}

// ---- Tool ----

// ToolRepo 工具表（实现 tool.ToolRepo）。
type ToolRepo struct {
	db *gorm.DB
}

// Create 创建。
func (r *ToolRepo) Create(ctx context.Context, t *model.Tool) error {
	return r.db.WithContext(ctx).Create(t).Error
}

// Update 更新。
func (r *ToolRepo) Update(ctx context.Context, t *model.Tool) error {
	return r.db.WithContext(ctx).Save(t).Error
}

// Delete 删除。
func (r *ToolRepo) Delete(ctx context.Context, id int64) error {
	return r.db.WithContext(ctx).Delete(&model.Tool{}, id).Error
}

// GetByID 详情。
func (r *ToolRepo) GetByID(ctx context.Context, id int64) (*model.Tool, error) {
	var t model.Tool
	if err := r.db.WithContext(ctx).First(&t, id).Error; err != nil {
		return nil, err
	}
	return &t, nil
}

// GetByCode 按 code 查询（tool.ToolRepo 接口）。
func (r *ToolRepo) GetByCode(ctx context.Context, code string) (*model.Tool, error) {
	var t model.Tool
	if err := r.db.WithContext(ctx).Where("code = ?", code).First(&t).Error; err != nil {
		return nil, err
	}
	return &t, nil
}

// ListByCodes 按 code 批量查询（tool.ToolRepo 接口）。
func (r *ToolRepo) ListByCodes(ctx context.Context, codes []string) ([]model.Tool, error) {
	var list []model.Tool
	err := r.db.WithContext(ctx).Where("code IN ? AND status = 1", codes).Find(&list).Error
	return list, err
}

// ListByIDs 按 ID 批量查询（Agent 绑定只带 tool_id 时回填定义用）。
func (r *ToolRepo) ListByIDs(ctx context.Context, ids []int64) ([]model.Tool, error) {
	var list []model.Tool
	err := r.db.WithContext(ctx).Where("id IN ? AND status = 1", ids).Find(&list).Error
	return list, err
}

// List 工具列表。
func (r *ToolRepo) List(ctx context.Context, typ string, offset, limit int) ([]model.Tool, int64, error) {
	var (
		list  []model.Tool
		total int64
	)
	q := r.db.WithContext(ctx).Model(&model.Tool{})
	if typ != "" {
		q = q.Where("type = ?", typ)
	}
	if err := q.Count(&total).Error; err != nil {
		return nil, 0, err
	}
	err := q.Order("id ASC").Offset(offset).Limit(limit).Find(&list).Error
	return list, total, err
}
