// Package logger provides structured console/file logging and request context propagation.
package logger

import (
	"context"
	"os"
	"path/filepath"
	"strings"
	"time"
	_ "time/tzdata"

	"go.uber.org/zap"
	"go.uber.org/zap/zapcore"
)

var Log = zap.NewNop()
var file *os.File

type ctxLoggerKey struct{}

// Init runs after dotenv is loaded. Logging failures never prevent cloud startup.
func Init() {
	if loc, err := time.LoadLocation("Asia/Jakarta"); err == nil {
		time.Local = loc
	}
	jsonConfig := zap.NewProductionEncoderConfig()
	jsonConfig.TimeKey, jsonConfig.LevelKey = "time", "level"
	jsonConfig.MessageKey, jsonConfig.CallerKey = "msg", "caller"
	jsonConfig.StacktraceKey = "stacktrace"
	jsonConfig.EncodeTime = zapcore.ISO8601TimeEncoder
	jsonConfig.EncodeCaller = zapcore.ShortCallerEncoder

	consoleConfig := zap.NewDevelopmentEncoderConfig()
	consoleConfig.EncodeTime = zapcore.TimeEncoderOfLayout("2006-01-02 15:04:05.000 Z07:00")
	consoleConfig.EncodeLevel = zapcore.CapitalColorLevelEncoder
	consoleConfig.EncodeCaller = zapcore.ShortCallerEncoder
	consoleConfig.TimeKey, consoleConfig.LevelKey = "time", "level"
	consoleConfig.MessageKey, consoleConfig.CallerKey = "msg", "caller"
	consoleConfig.StacktraceKey = "stacktrace"
	appEnv := strings.ToLower(strings.TrimSpace(os.Getenv("APP_ENV")))
	var stdout zapcore.Core
	if appEnv == "" || appEnv == "development" || appEnv == "local" {
		stdout = zapcore.NewCore(zapcore.NewConsoleEncoder(consoleConfig), zapcore.Lock(os.Stdout), zap.DebugLevel)
	} else {
		stdout = zapcore.NewCore(zapcore.NewJSONEncoder(jsonConfig), zapcore.Lock(os.Stdout), zap.InfoLevel)
	}
	cores := []zapcore.Core{stdout}
	dir := os.Getenv("LOG_DIR")
	if dir == "" {
		dir = "./logs"
	}
	if err := os.MkdirAll(dir, 0700); err == nil {
		file, _ = os.OpenFile(filepath.Join(dir, "app.log"), os.O_CREATE|os.O_APPEND|os.O_WRONLY, 0600)
		if file != nil {
			cores = append(cores, zapcore.NewCore(zapcore.NewJSONEncoder(jsonConfig), zapcore.Lock(file), zap.InfoLevel))
		}
	}
	service := os.Getenv("SERVICE_NAME")
	if service == "" {
		service = "cloud-app"
	}
	hostname, _ := os.Hostname()
	Log = zap.New(NewRedactCore(zapcore.NewTee(cores...)), zap.AddCaller(), zap.AddStacktrace(zap.ErrorLevel),
		zap.Fields(zap.String("service", service), zap.String("hostname", hostname)))
	if file == nil {
		Warn("Log file unavailable; continuing with stdout logging")
	}
}

// Sync flushes sinks and closes the owned file after application shutdown.
func Sync() {
	_ = Log.Sync()
	if file != nil {
		_ = file.Close()
		file = nil
	}
}

func WithRequestID(ctx context.Context, requestID string) context.Context {
	return WithContext(ctx, zap.String("requestId", requestID))
}

func WithContext(ctx context.Context, fields ...zap.Field) context.Context {
	return context.WithValue(ctx, ctxLoggerKey{}, FromContext(ctx).With(fields...))
}

func FromContext(ctx context.Context) *zap.Logger {
	if ctx != nil {
		if log, ok := ctx.Value(ctxLoggerKey{}).(*zap.Logger); ok {
			return log
		}
	}
	return Log
}

func Debug(msg string, fields ...zap.Field) {
	Log.WithOptions(zap.AddCallerSkip(1)).Debug(msg, fields...)
}
func Info(msg string, fields ...zap.Field) {
	Log.WithOptions(zap.AddCallerSkip(1)).Info(msg, fields...)
}
func Warn(msg string, fields ...zap.Field) {
	Log.WithOptions(zap.AddCallerSkip(1)).Warn(msg, fields...)
}
func Error(msg string, fields ...zap.Field) {
	Log.WithOptions(zap.AddCallerSkip(1)).Error(msg, fields...)
}
