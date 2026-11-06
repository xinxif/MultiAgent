// Package infra 基础设施连接初始化：PostgreSQL。
package infra

import (
	"context"
	"fmt"
	"time"

	"gorm.io/driver/postgres"
	"gorm.io/gorm"
	gormlogger "gorm.io/gorm/logger"

	"agent/internal/config"
	"agent/internal/observability/logger"
)

// NewPG 初始化 GORM PostgreSQL 连接。
func NewPG(ctx context.Context, cfg *config.PG) (*gorm.DB, error) {
	logLevel := gormlogger.Warn
	if cfg.DSN == "" {
		return nil, fmt.Errorf("pg dsn is empty")
	}

	db, err := gorm.Open(postgres.Open(cfg.DSN), &gorm.Config{
		// 全局禁用外键迁移（表结构由 migrations 管理，GORM 只做读写）
		DisableForeignKeyConstraintWhenMigrating: true,
		Logger: gormlogger.New(
			gormWriter{},
			gormlogger.Config{
				SlowThreshold:             200 * time.Millisecond,
				LogLevel:                  logLevel,
				IgnoreRecordNotFoundError: true,
			},
		),
	})
	if err != nil {
		return nil, fmt.Errorf("open postgres: %w", err)
	}

	sqlDB, err := db.DB()
	if err != nil {
		return nil, fmt.Errorf("get sql.DB: %w", err)
	}
	sqlDB.SetMaxOpenConns(cfg.MaxOpenConns)
	sqlDB.SetMaxIdleConns(cfg.MaxIdleConns)
	sqlDB.SetConnMaxLifetime(cfg.ConnMaxLifetime)

	// 启动时探活
	pingCtx, cancel := context.WithTimeout(ctx, 5*time.Second)
	defer cancel()
	if err := sqlDB.PingContext(pingCtx); err != nil {
		return nil, fmt.Errorf("ping postgres: %w", err)
	}

	logger.L().Info("postgres connected", logger.TraceID("infra"))
	return db, nil
}

// gormWriter 把 GORM 日志转发到 zap，保持日志出口统一。
type gormWriter struct{}

func (gormWriter) Printf(format string, args ...any) {
	logger.L().Sugar().Infof("[gorm] "+format, args...)
}
