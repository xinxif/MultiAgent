// 并发控制：供应商级信号量。
// 超载时排队等待（带 ctx 超时），防止打爆上游配额。
package llm

import (
	"context"
	"fmt"
)

// Semaphore 朴素信号量。
type Semaphore struct {
	ch chan struct{}
}

// NewSemaphore 构造容量为 n 的信号量。
func NewSemaphore(n int) *Semaphore {
	if n <= 0 {
		n = 16
	}
	return &Semaphore{ch: make(chan struct{}, n)}
}

// Acquire 获取配额；ctx 超时/取消时返回错误。
func (s *Semaphore) Acquire(ctx context.Context) error {
	select {
	case s.ch <- struct{}{}:
		return nil
	case <-ctx.Done():
		return fmt.Errorf("acquire semaphore: %w", ctx.Err())
	}
}

// Release 释放配额。
func (s *Semaphore) Release() {
	select {
	case <-s.ch:
	default:
	}
}
