// Runner 辅助函数与成本计算。
package runtime

import (
	"encoding/json"
	"fmt"
)

// jsonUnmarshal 包装（便于测试替换）。
var jsonUnmarshal = json.Unmarshal

// ModelPrice 模型单价（元/1M tokens）——由 service 层注入更佳；
// 当前先内置计算逻辑，后续可重构为网关计量的统一出口。
type ModelPrice struct {
	InputPerMillion  float64
	OutputPerMillion float64
}

// CalcCost 按单价计算成本字符串。
func CalcCost(p ModelPrice, promptTokens, completionTokens int) string {
	cost := float64(promptTokens)/1e6*p.InputPerMillion +
		float64(completionTokens)/1e6*p.OutputPerMillion
	return fmt.Sprintf("%.4f", cost)
}

// fmtCost 默认成本占位（价格未知时返回 0.0000）。
func fmtCost(prompt, completion int) string {
	return CalcCost(ModelPrice{}, prompt, completion)
}
