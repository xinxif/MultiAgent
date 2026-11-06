// 内置工具注册表。
// 注册表模式：新增内置工具 = 写一个 handler + Register 一行，
// 工具描述（description/JSON Schema）质量直接决定模型调用准确率。
package tool

import (
	"sort"
	"sync"
)

// registry 内置工具注册表实现。
type registry struct {
	mu       sync.RWMutex
	handlers map[string]BuiltinHandler
}

// NewRegistry 构造空注册表。
func NewRegistry() Registry {
	return &registry{handlers: map[string]BuiltinHandler{}}
}

// Register 注册（同 code 覆盖，便于测试替换）。
func (r *registry) Register(code string, h BuiltinHandler) {
	r.mu.Lock()
	defer r.mu.Unlock()
	r.handlers[code] = h
}

// Get 查找。
func (r *registry) Get(code string) (BuiltinHandler, bool) {
	r.mu.RLock()
	defer r.mu.RUnlock()
	h, ok := r.handlers[code]
	return h, ok
}

// Codes 列出全部已注册 code（排序，稳定输出）。
func (r *registry) Codes() []string {
	r.mu.RLock()
	defer r.mu.RUnlock()
	codes := make([]string, 0, len(r.handlers))
	for c := range r.handlers {
		codes = append(codes, c)
	}
	sort.Strings(codes)
	return codes
}
