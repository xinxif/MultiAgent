// api 包日志快捷入口。
package api

import (
	"fmt"

	"agent/internal/observability"
)

// logInfo 包装 observability.LogInfo。
func logInfo(msg string, kv ...string) { observability.LogInfo(msg, kv...) }

// logError 包装 observability.LogError。
func logError(msg string, kv ...string) { observability.LogError(msg, kv...) }

// toString 任意值转字符串（日志用）。
func toString(v any) string {
	if err, ok := v.(error); ok {
		return err.Error()
	}
	return fmt.Sprintf("%v", v)
}
