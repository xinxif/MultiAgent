// Package logger 全局结构化日志（Zap）。
// 统一 JSON 输出到 stdout，由日志收集器采集；强制携带 trace_id 字段。
package logger

import (
	"os"
	"strings"

	"go.uber.org/zap"
	"go.uber.org/zap/zapcore"
)

var lg *zap.Logger

// Init 初始化全局 logger。level: debug/info/warn/error; format: json/console
func Init(level, format string) {
	lvl := zapcore.InfoLevel
	switch strings.ToLower(level) {
	case "debug":
		lvl = zapcore.DebugLevel
	case "warn":
		lvl = zapcore.WarnLevel
	case "error":
		lvl = zapcore.ErrorLevel
	}

	encCfg := zap.NewProductionEncoderConfig()
	encCfg.TimeKey = "ts"
	encCfg.EncodeTime = zapcore.ISO8601TimeEncoder

	var enc zapcore.Encoder
	if strings.EqualFold(format, "console") {
		enc = zapcore.NewConsoleEncoder(encCfg)
	} else {
		enc = zapcore.NewJSONEncoder(encCfg)
	}

	core := zapcore.NewCore(enc, zapcore.Lock(os.Stdout), lvl)
	lg = zap.New(core, zap.AddCallerSkip(1), zap.AddStacktrace(zapcore.ErrorLevel))
}

// L 返回全局 logger（未初始化时降级为 Nop，避免 panic）。
func L() *zap.Logger {
	if lg == nil {
		return zap.NewNop()
	}
	return lg
}

// 便捷方法（带 trace_id 快捷字段）。
func Debug(msg string, fields ...zap.Field)  { L().Debug(msg, fields...) }
func Info(msg string, fields ...zap.Field)   { L().Info(msg, fields...) }
func Warn(msg string, fields ...zap.Field)   { L().Warn(msg, fields...) }
func Error(msg string, fields ...zap.Field)  { L().Error(msg, fields...) }

// TraceID 构造 trace_id 字段。
func TraceID(id string) zap.Field { return zap.String("trace_id", id) }
