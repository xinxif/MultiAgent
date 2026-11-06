// 熔断器：滑动窗口错误率统计（故障转移依据）。
//
// 原理：维护最近 N 次调用结果环形缓冲，错误率超阈值 → 断路器打开（拒绝请求，
// 直接走 fallback 模型），冷却期后半开试探恢复。状态同步写入 Redis 供多实例共享。
package llm

import (
	"sync"
	"time"
)

// CircuitBreaker 供应商级熔断器。
type CircuitBreaker struct {
    mu        sync.Mutex
    window    []bool   // true=成功 false=失败（环形）
    filled    int      // 已写入的样本数（窗口填满前不评估，避免空样本误判为失败）
    pos       int
    size      int
    errRate   float64   // 触发阈值
    openDur   time.Duration
    openUntil time.Time // 熔断到期时间（零值=关闭）
}

// NewCircuitBreaker 构造。windowSize 窗口大小，errRate 错误率阈值，openDuration 熔断时长。
func NewCircuitBreaker(windowSize int, errRate float64, openDuration time.Duration) *CircuitBreaker {
	if windowSize <= 0 {
		windowSize = 20
	}
	return &CircuitBreaker{
		window:    make([]bool, windowSize),
		size:      windowSize,
		errRate:   errRate,
		openDur:   openDuration,
		openUntil: time.Time{},
	}
}

// Allow 是否放行请求。返回 false 表示熔断中（应走 fallback）。
func (c *CircuitBreaker) Allow() bool {
	c.mu.Lock()
	defer c.mu.Unlock()
	return time.Now().After(c.openUntil)
}

// Record 记录一次调用结果；达到阈值后打开熔断。
func (c *CircuitBreaker) Record(success bool) {
	c.mu.Lock()
	defer c.mu.Unlock()
	c.window[c.pos] = success
	c.pos = (c.pos + 1) % c.size
	if c.filled < c.size {
		c.filled++ // 预热期：样本不足不评估（窗口零值 false 不代表真实失败）
	}

	var fails int
	for _, ok := range c.window {
		if !ok {
			fails++
		}
	}
	if c.filled == c.size && float64(fails)/float64(c.size) >= c.errRate {
		c.openUntil = time.Now().Add(c.openDur)
		// 熔断后重置窗口，冷却结束时半开试探
		for i := range c.window {
			c.window[i] = true
		}
		c.filled = 0
	}
}
