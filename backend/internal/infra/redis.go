// 基础设施：Redis。
package infra

import (
	"context"
	"fmt"
	"time"

	"github.com/redis/go-redis/v9"

	"agent/internal/config"
)

// NewRedis 初始化 Redis 客户端。
// Redis 在本项目承担四种角色：缓存（会话窗口）、限流（令牌桶）、
// 锁（Worker 抢占）、消息队列（Streams，文档 ETL / 评估任务）。
func NewRedis(ctx context.Context, cfg *config.Redis) (*redis.Client, error) {
	cli := redis.NewClient(&redis.Options{
		Addr:     cfg.Addr,
		Password: cfg.Password,
		DB:       cfg.DB,
	})

	pingCtx, cancel := context.WithTimeout(ctx, 5*time.Second)
	defer cancel()
	if err := cli.Ping(pingCtx).Err(); err != nil {
		return nil, fmt.Errorf("ping redis: %w", err)
	}
	return cli, nil
}

// Stream 相关 Key 常量。
const (
	StreamDocETL = "stream:doc-etl" // 文档 ETL 任务流
	StreamEval   = "stream:eval"    // 评估任务流
	StreamDLQ    = "stream:dlq"     // 死信队列
)

// Consumer Group 常量。
const (
	GroupDocETL = "doc-etl-workers"
	GroupEval   = "eval-workers"
)
