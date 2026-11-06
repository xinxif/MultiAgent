// Package observability 可观测性：OpenTelemetry 追踪初始化。
// 全链路：HTTP 中间件 → service → agent 每一步决策 → LLM/工具调用 都产生 span，
// 导出 OTLP → Jaeger 可视化。
package observability

import (
	"context"
	"net/url"
	"strings"

	"go.opentelemetry.io/otel"
	"go.opentelemetry.io/otel/attribute"
	"go.opentelemetry.io/otel/codes"
	"go.opentelemetry.io/otel/exporters/otlp/otlptrace/otlptracehttp"
	"go.opentelemetry.io/otel/propagation"
	"go.opentelemetry.io/otel/sdk/resource"
	sdktrace "go.opentelemetry.io/otel/sdk/trace"
	semconv "go.opentelemetry.io/otel/semconv/v1.26.0"
	"go.opentelemetry.io/otel/trace"
)

// instrumentationName 埋点库标识，会写进 span 的 instrumentation_scope。
const instrumentationName = "agent"

// InitTracer 初始化 OTel TracerProvider。
// endpoint 形如 http://127.0.0.1:4318（Jaeger OTLP HTTP 端口）。
func InitTracer(ctx context.Context, endpoint string) (func(context.Context) error, error) {
	if strings.TrimSpace(endpoint) == "" {
		return func(context.Context) error { return nil }, nil
	}

	// 拆出 host:port 供 otlptracehttp 使用（它不接受 scheme）
	u, err := url.Parse(endpoint)
	if err != nil {
		return nil, err
	}

	exp, err := otlptracehttp.New(ctx,
		otlptracehttp.WithEndpoint(u.Host),
		otlptracehttp.WithInsecure(), // 本地部署不走 TLS
	)
	if err != nil {
		return nil, err
	}

	res, err := resource.New(ctx,
		resource.WithAttributes(semconv.ServiceName("agent")),
	)
	if err != nil {
		return nil, err
	}

	tp := sdktrace.NewTracerProvider(
		sdktrace.WithBatcher(exp),
		sdktrace.WithResource(res),
	)
	otel.SetTracerProvider(tp)
	// W3C traceparent 传播（跨进程演示用）
	otel.SetTextMapPropagator(propagation.NewCompositeTextMapPropagator(
		propagation.TraceContext{}, propagation.Baggage{},
	))
	return tp.Shutdown, nil
}

// ---- 埋点辅助 ----
//
// 这里刻意不返回 error、也不暴露 Provider：InitTracer 未启用时
// otel.Tracer 拿到的是 no-op 实现，StartSpan 返回的也是 no-op span，
// 所以所有业务埋点代码都无需判空、无需加 if enabled。

// Tracer 返回全局 Tracer。
func Tracer() trace.Tracer { return otel.Tracer(instrumentationName) }

// StartSpan 开启一个 span 并返回携带它的新 ctx。
// 追踪未启用时返回 no-op span，调用方照常 defer span.End() 即可。
func StartSpan(ctx context.Context, name string, attrs ...attribute.KeyValue) (context.Context, trace.Span) {
	return Tracer().Start(ctx, name, trace.WithAttributes(attrs...))
}

// StartServerSpan 开启服务端 span（HTTP 根 span 用，Jaeger 里会标成 server 类型）。
func StartServerSpan(ctx context.Context, name string, attrs ...attribute.KeyValue) (context.Context, trace.Span) {
	return Tracer().Start(ctx, name,
		trace.WithSpanKind(trace.SpanKindServer),
		trace.WithAttributes(attrs...),
	)
}

// MarkError 把失败记录进 span（只打标记，不结束 span——结束交给调用方的 defer）。
func MarkError(span trace.Span, err error) {
	if err == nil {
		return
	}
	span.RecordError(err)
	span.SetStatus(codes.Error, err.Error())
}
