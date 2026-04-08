package logger

import (
	"fmt"
	"os"
	"strings"
	"sync"

	"github.com/JekYUlll/Dipole/internal/config"
	"go.uber.org/zap"
	"go.uber.org/zap/zapcore"
)

var (
	mu          sync.RWMutex
	global      = zap.NewNop()
	globalSugar = global.Sugar()
)

func Init() error {
	builtLogger, err := build(config.LogConfig())
	if err != nil {
		return fmt.Errorf("build zap logger: %w", err)
	}

	mu.Lock()
	oldLogger := global
	oldSugar := globalSugar
	global = builtLogger
	globalSugar = builtLogger.Sugar()
	mu.Unlock()

	_ = oldSugar.Sync()
	_ = oldLogger.Sync()
	zap.ReplaceGlobals(builtLogger)

	return nil
}

func L() *zap.Logger {
	mu.RLock()
	defer mu.RUnlock()

	return global
}

func Named(name string) *zap.Logger {
	return L().Named(name)
}

func With(fields ...zap.Field) *zap.Logger {
	return L().With(fields...)
}

func S() *zap.SugaredLogger {
	mu.RLock()
	defer mu.RUnlock()

	return globalSugar
}

func Debug(msg string, fields ...zap.Field) {
	L().Debug(msg, fields...)
}

func Info(msg string, fields ...zap.Field) {
	L().Info(msg, fields...)
}

func Warn(msg string, fields ...zap.Field) {
	L().Warn(msg, fields...)
}

func Error(msg string, fields ...zap.Field) {
	L().Error(msg, fields...)
}

func Fatal(msg string, fields ...zap.Field) {
	L().Fatal(msg, fields...)
}

func Sync() error {
	return L().Sync()
}

func build(cfg config.Log) (*zap.Logger, error) {
	level, err := parseLevel(cfg.Level)
	if err != nil {
		return nil, err
	}

	encoderConfig := zap.NewProductionEncoderConfig()
	encoderConfig.TimeKey = "time"
	encoderConfig.EncodeTime = zapcore.ISO8601TimeEncoder
	encoderConfig.EncodeDuration = zapcore.StringDurationEncoder

	var encoder zapcore.Encoder
	switch strings.ToLower(strings.TrimSpace(cfg.Format)) {
	case "", "console":
		encoderConfig.EncodeLevel = zapcore.CapitalLevelEncoder
		encoder = zapcore.NewConsoleEncoder(encoderConfig)
	case "json":
		encoderConfig.EncodeLevel = zapcore.LowercaseLevelEncoder
		encoder = zapcore.NewJSONEncoder(encoderConfig)
	default:
		return nil, fmt.Errorf("unsupported log format: %s", cfg.Format)
	}

	core := zapcore.NewCore(encoder, zapcore.AddSync(os.Stdout), level)
	options := []zap.Option{
		zap.AddCaller(),
		zap.AddStacktrace(zap.ErrorLevel),
	}
	if cfg.Development {
		options = append(options, zap.Development())
	}

	return zap.New(core, options...).Named("dipole"), nil
}

func parseLevel(raw string) (zapcore.LevelEnabler, error) {
	level := zapcore.InfoLevel
	if raw == "" {
		return level, nil
	}

	if err := level.Set(strings.ToLower(strings.TrimSpace(raw))); err != nil {
		return nil, fmt.Errorf("parse log level: %w", err)
	}

	return level, nil
}
