// Package logger 封装 Zap 日志，提供结构化日志入口。
package logger

import (
	"os"
	"time"

	"go.uber.org/zap"
	"go.uber.org/zap/zapcore"
)

// Logger 是 zap.SugaredLogger 的别名，支持键值对形式的结构化日志。
// 用法：log.Infow("msg", "key", value, "key2", value2)
type Logger = zap.SugaredLogger

// New 根据 level 与 format 创建日志实例。
func New(level, format string) *Logger {
	var lvl zapcore.Level
	switch level {
	case "debug":
		lvl = zapcore.DebugLevel
	case "warn":
		lvl = zapcore.WarnLevel
	case "error":
		lvl = zapcore.ErrorLevel
	default:
		lvl = zapcore.InfoLevel
	}

	encoder := newEncoder(format, lvl)
	core := zapcore.NewCore(encoder, zapcore.AddSync(os.Stdout), lvl)

	zl := zap.New(core,
		zap.AddCaller(),
		zap.AddCallerSkip(1),
		zap.AddStacktrace(zapcore.ErrorLevel),
	)
	return zl.Sugar()
}

// newEncoder 创建编码器。
func newEncoder(format string, _ zapcore.Level) zapcore.Encoder {
	encoderCfg := zapcore.EncoderConfig{
		TimeKey:       "time",
		LevelKey:      "level",
		CallerKey:     "caller",
		MessageKey:    "msg",
		StacktraceKey: "stack",
		LineEnding:    zapcore.DefaultLineEnding,
		EncodeTime:    iso8601TimeEncoder,
		EncodeLevel:   zapcore.CapitalLevelEncoder,
		EncodeCaller:  zapcore.ShortCallerEncoder,
	}

	if format == "json" {
		return zapcore.NewJSONEncoder(encoderCfg)
	}

	// 控制台格式：带颜色的级别 + 空格分隔符
	encoderCfg.EncodeLevel = zapcore.CapitalColorLevelEncoder
	encoderCfg.ConsoleSeparator = " "
	return zapcore.NewConsoleEncoder(encoderCfg)
}

// iso8601TimeEncoder 使用 Asia/Shanghai 时区的 ISO8601 时间格式。
func iso8601TimeEncoder(t time.Time, enc zapcore.PrimitiveArrayEncoder) {
	cst := t.In(time.FixedZone("CST", 8*3600))
	enc.AppendString(cst.Format("2006-01-02 15:04:05.000"))
}
