// LLM 供应商与模型配置数据访问。
package repo

import (
	"context"

	"gorm.io/gorm"

	"agent/internal/model"
)

// LLMRepo 供应商/模型配置表。
type LLMRepo struct {
	db *gorm.DB
}

// ---- Provider ----

// CreateProvider 创建供应商。
func (r *LLMRepo) CreateProvider(ctx context.Context, p *model.ModelProvider) error {
	return r.db.WithContext(ctx).Create(p).Error
}

// UpdateProvider 更新供应商。
func (r *LLMRepo) UpdateProvider(ctx context.Context, p *model.ModelProvider) error {
	return r.db.WithContext(ctx).Save(p).Error
}

// DeleteProvider 删除供应商（级联约束在 DDL 层）。
func (r *LLMRepo) DeleteProvider(ctx context.Context, id int64) error {
	return r.db.WithContext(ctx).Delete(&model.ModelProvider{}, id).Error
}

// ListProviders 供应商列表。
func (r *LLMRepo) ListProviders(ctx context.Context) ([]model.ModelProvider, error) {
	var list []model.ModelProvider
	err := r.db.WithContext(ctx).Order("id ASC").Find(&list).Error
	return list, err
}

// GetProvider 按 ID 查询供应商（解密 API Key 前置步骤）。
func (r *LLMRepo) GetProvider(ctx context.Context, id int64) (*model.ModelProvider, error) {
	var p model.ModelProvider
	if err := r.db.WithContext(ctx).First(&p, id).Error; err != nil {
		return nil, err
	}
	return &p, nil
}

// ---- ModelConfig ----

// CreateModel 创建模型配置。
func (r *LLMRepo) CreateModel(ctx context.Context, m *model.ModelConfig) error {
	return r.db.WithContext(ctx).Create(m).Error
}

// UpdateModel 更新模型配置。
func (r *LLMRepo) UpdateModel(ctx context.Context, m *model.ModelConfig) error {
	return r.db.WithContext(ctx).Save(m).Error
}

// DeleteModel 删除模型配置。
func (r *LLMRepo) DeleteModel(ctx context.Context, id int64) error {
	return r.db.WithContext(ctx).Delete(&model.ModelConfig{}, id).Error
}

// ListModels 模型列表（可按类型过滤，含供应商关联）。
func (r *LLMRepo) ListModels(ctx context.Context, typ string) ([]model.ModelConfig, error) {
	var list []model.ModelConfig
	q := r.db.WithContext(ctx).Preload("Provider").Order("priority ASC, id ASC")
	if typ != "" {
		q = q.Where("type = ?", typ)
	}
	err := q.Find(&list).Error
	return list, err
}

// GetModel 按 ID 查询模型配置（含供应商）。
func (r *LLMRepo) GetModel(ctx context.Context, id int64) (*model.ModelConfig, error) {
	var m model.ModelConfig
	if err := r.db.WithContext(ctx).Preload("Provider").First(&m, id).Error; err != nil {
		return nil, err
	}
	return &m, nil
}

// DefaultModel 按别名取默认模型（config.llm.default_chat 指向 alias）。
func (r *LLMRepo) DefaultModel(ctx context.Context, alias string, typ string) (*model.ModelConfig, error) {
	var m model.ModelConfig
	q := r.db.WithContext(ctx).Preload("Provider")
	err := q.Where("alias = ? AND type = ? AND status = 1", alias, typ).First(&m).Error
	if err == nil {
		return &m, nil
	}
	if err != gorm.ErrRecordNotFound {
		return nil, err
	}
	// 别名不存在时回退：该类型的 is_default 或 priority 最小者
	err = q.Where("type = ? AND status = 1", typ).Order("is_default DESC, priority ASC").First(&m).Error
	return &m, err
}
