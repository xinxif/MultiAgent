// Prometheus 指标定义（Grafana 看板数据源）。
package observability

import (
	"github.com/prometheus/client_golang/prometheus"
	"github.com/prometheus/client_golang/prometheus/promauto"
)

// Metrics 全局指标集合。
type Metrics struct {
	ChatFirstToken   *prometheus.HistogramVec // 首 token 延迟
	LLMTokens        *prometheus.CounterVec   // Token 用量（model 方向）
	LLMCost          *prometheus.CounterVec   // 成本（模型）
	LLMRequests      *prometheus.CounterVec   // 模型请求计数（status）
	LLMLatency       *prometheus.HistogramVec // 模型调用延迟
	ToolCalls        *prometheus.CounterVec   // 工具调用计数（tool,status）
	ToolLatency      *prometheus.HistogramVec // 工具调用延迟
	RAGSearchSeconds *prometheus.HistogramVec // 检索延迟（stage: dense/bm25/fused/rerank）
	AgentIterations  prometheus.Histogram     // ReAct 迭代次数分布
	ActiveStreams    prometheus.Gauge         // 活跃 SSE 流数
}

var m *Metrics

// MetricsInstance 返回全局指标（未启用时返回 nil，调用方判空）。
func MetricsInstance() *Metrics { return m }

// InitMetrics 初始化全局指标注册。
func InitMetrics() {
	m = &Metrics{
		ChatFirstToken: promauto.NewHistogramVec(prometheus.HistogramOpts{
			Name: "chat_first_token_seconds",
			Help: "首个 token 延迟（秒）",
			Buckets: prometheus.ExponentialBuckets(0.1, 2, 10),
		}, []string{"model"}),
		LLMTokens: promauto.NewCounterVec(prometheus.CounterOpts{
			Name: "llm_tokens_total", Help: "LLM Token 用量",
		}, []string{"model", "direction"}), // direction: prompt/completion
		LLMCost: promauto.NewCounterVec(prometheus.CounterOpts{
			Name: "llm_cost_total", Help: "LLM 调用成本（元）",
		}, []string{"model"}),
		LLMRequests: promauto.NewCounterVec(prometheus.CounterOpts{
			Name: "llm_requests_total", Help: "LLM 请求数",
		}, []string{"model", "status"}),
		LLMLatency: promauto.NewHistogramVec(prometheus.HistogramOpts{
			Name: "llm_latency_seconds", Help: "LLM 调用延迟",
			Buckets: prometheus.ExponentialBuckets(0.1, 2, 12),
		}, []string{"model"}),
		ToolCalls: promauto.NewCounterVec(prometheus.CounterOpts{
			Name: "tool_calls_total", Help: "工具调用计数",
		}, []string{"tool", "status"}),
		ToolLatency: promauto.NewHistogramVec(prometheus.HistogramOpts{
			Name: "tool_latency_seconds", Help: "工具调用延迟",
			Buckets: prometheus.ExponentialBuckets(0.01, 2, 12),
		}, []string{"tool"}),
		RAGSearchSeconds: promauto.NewHistogramVec(prometheus.HistogramOpts{
			Name: "rag_search_seconds", Help: "RAG 各阶段检索延迟",
			Buckets: prometheus.ExponentialBuckets(0.005, 2, 12),
		}, []string{"stage"}),
		AgentIterations: promauto.NewHistogram(prometheus.HistogramOpts{
			Name: "agent_iterations", Help: "ReAct 循环迭代次数分布",
			Buckets: []float64{1, 2, 3, 5, 8, 10, 15, 20},
		}),
		ActiveStreams: promauto.NewGauge(prometheus.GaugeOpts{
			Name: "stream_active_sessions", Help: "活跃 SSE 流数量",
		}),
	}
}
