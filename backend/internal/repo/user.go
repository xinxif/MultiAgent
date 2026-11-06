// 用户数据访问。
package repo

import (
	"context"

	"gorm.io/gorm"

	"agent/internal/model"
)

// UserRepo 用户表。
type UserRepo struct {
	db *gorm.DB
}

// Create 创建用户。
func (r *UserRepo) Create(ctx context.Context, u *model.User) error {
	return r.db.WithContext(ctx).Create(u).Error
}

// GetByID 按 ID 查询。
func (r *UserRepo) GetByID(ctx context.Context, id int64) (*model.User, error) {
	var u model.User
	if err := r.db.WithContext(ctx).First(&u, id).Error; err != nil {
		return nil, err
	}
	return &u, nil
}

// GetByUsername 按用户名查询（登录）。
func (r *UserRepo) GetByUsername(ctx context.Context, username string) (*model.User, error) {
	var u model.User
	if err := r.db.WithContext(ctx).Where("username = ?", username).First(&u).Error; err != nil {
		return nil, err
	}
	return &u, nil
}

// ExistsBy 徽记查询：用户名/邮箱是否已存在。
func (r *UserRepo) ExistsBy(ctx context.Context, username, email string) (bool, error) {
	var count int64
	err := r.db.WithContext(ctx).Model(&model.User{}).
		Where("username = ? OR email = ?", username, email).
		Count(&count).Error
	return count > 0, err
}

// Update 更新用户。
func (r *UserRepo) Update(ctx context.Context, u *model.User) error {
	return r.db.WithContext(ctx).Save(u).Error
}

// Count 统计用户总数（首个注册用户提升为 admin）。
func (r *UserRepo) Count(ctx context.Context) (int64, error) {
	var n int64
	err := r.db.WithContext(ctx).Model(&model.User{}).Count(&n).Error
	return n, err
}
