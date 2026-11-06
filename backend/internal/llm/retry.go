// 重试：指数退避。
// 仅对可重试错误（429/5xx/网络）重试；流式请求首字节后不再重试（由调用方控制）。
package llm

import (
	"context"
	"math/rand"
	"time"
)

// RetryConfig 重试参数。
type RetryConfig struct {
	MaxAttempts int           // 最大尝试次数（含首次）
	BaseDelay   time.Duration // 基础退避
	MaxDelay    time.Duration // 退避上限
}

// DefaultRetryConfig 默认配置：3 次，100ms 起，退避上限 2s。
func DefaultRetryConfig() RetryConfig {
	return RetryConfig{MaxAttempts: 3, BaseDelay: 100 * time.Millisecond, MaxDelay: 2 * time.Second}
}

// Do 执行 fn，可重试错误按指数退避重试，其余错误立即返回。
func RetryDo(ctx context.Context, cfg RetryConfig, fn func(ctx context.Context) error) error {
	var lastErr error
	for attempt := 0; attempt < cfg.MaxAttempts; attempt++ {
		if err := ctx.Err(); err != nil {
			return err
		}
		lastErr = fn(ctx)
		if lastErr == nil {
			return nil
		}
		if !IsRetryable(lastErr) {
			return lastErr
		}
		// 指数退避 + 抖动：100ms, 200ms, 400ms...
		delay := cfg.BaseDelay << attempt
		if delay > cfg.MaxDelay {
			delay = cfg.MaxDelay
		}
		delay += time.Duration(rand.Int63n(int64(delay / 4)))
		select {
		case <-time.After(delay):
		case <-ctx.Done():
			return ctx.Err()
		}
	}
	return lastErr
}
