// 内置工具：计算器与时间日期（最小工具实现示例）。
package tool

import (
	"context"
	"fmt"
	"math"
	"strconv"
	"time"

	"github.com/expr-lang/expr"
)

// RegisterSimple 注册 calculator 与 datetime。
func RegisterSimple(r Registry) {
	r.Register("calculator", runCalculator)
	r.Register("datetime", runDatetime)
}

// runCalculator 表达式求值（expr-lang 沙箱化求值，不使用 eval）。
func runCalculator(_ context.Context, args map[string]any, _ string) (string, error) {
	expression, _ := args["expression"].(string)
	if expression == "" {
		return "", fmt.Errorf("expression is required")
	}

	program, err := expr.Compile(expression, expr.AsFloat64(), expr.DisableAllBuiltins())
	if err != nil {
		return "", fmt.Errorf("invalid expression: %v", err)
	}
	out, err := expr.Run(program, nil)
	if err != nil {
		return "", fmt.Errorf("evaluate: %v", err)
	}

	val, ok := out.(float64)
	if !ok {
		return "", fmt.Errorf("expression must be numeric")
	}
	// 整数值去掉小数点（1.0 -> 1）
	if val == math.Trunc(val) {
		return strconv.FormatFloat(val, 'f', -1, 64), nil
	}
	return strconv.FormatFloat(val, 'f', 6, 64), nil
}

// runDatetime 时间日期操作。
func runDatetime(_ context.Context, args map[string]any, _ string) (string, error) {
	operation, _ := args["operation"].(string)
	tz := "Asia/Shanghai"
	if s, ok := args["tz"].(string); ok && s != "" {
		tz = s
	}
	loc, err := time.LoadLocation(tz)
	if err != nil {
		return "", fmt.Errorf("bad timezone: %v", err)
	}
	now := time.Now().In(loc)

	switch operation {
	case "now":
		return now.Format(time.RFC3339), nil
	case "add_days":
		days := 0
		switch v := args["days"].(type) {
		case float64:
			days = int(v)
		case int:
			days = v
		}
		return now.AddDate(0, 0, days).Format(time.RFC3339), nil
	case "diff":
		// 返回今天到年底的天数（可扩展为自定义两日期）
		yearEnd := time.Date(now.Year()+1, 1, 1, 0, 0, 0, 0, loc)
		return fmt.Sprintf("%d days until %d-01-01",
			int(yearEnd.Sub(now).Hours()/24), now.Year()+1), nil
	default:
		return "", fmt.Errorf("operation must be now/add_days/diff")
	}
}
