package logger

import (
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"time"

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

	writeSyncer, cleanup, err := buildWriteSyncer(cfg)
	if err != nil {
		return nil, err
	}
	if cleanup != nil {
		defer cleanup()
	}

	core := zapcore.NewCore(encoder, writeSyncer, level)
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

func buildWriteSyncer(cfg config.Log) (zapcore.WriteSyncer, func(), error) {
	writers := []io.Writer{os.Stdout}
	var closers []io.Closer
	if cfg.FileEnabled {
		path := strings.TrimSpace(cfg.FilePath)
		if path == "" {
			return nil, nil, fmt.Errorf("log file path is empty")
		}

		if cfg.FileRotateDaily {
			rotatingWriter, err := newDailyRotatingWriter(path, time.Now)
			if err != nil {
				return nil, nil, err
			}
			writers = append(writers, rotatingWriter)
			closers = append(closers, rotatingWriter)
		} else {
			if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
				return nil, nil, fmt.Errorf("create log directory: %w", err)
			}

			file, err := os.OpenFile(path, os.O_CREATE|os.O_APPEND|os.O_WRONLY, 0o644)
			if err != nil {
				return nil, nil, fmt.Errorf("open log file: %w", err)
			}
			writers = append(writers, file)
			closers = append(closers, file)
		}
	}

	cleanup := func() {
		for _, closer := range closers {
			_ = closer.Close()
		}
	}

	return zapcore.AddSync(io.MultiWriter(writers...)), cleanup, nil
}

type dailyRotatingWriter struct {
	mu       sync.Mutex
	basePath string
	now      func() time.Time

	currentDate string
	currentFile *os.File
}

func newDailyRotatingWriter(basePath string, now func() time.Time) (*dailyRotatingWriter, error) {
	writer := &dailyRotatingWriter{
		basePath: strings.TrimSpace(basePath),
		now:      now,
	}
	if writer.now == nil {
		writer.now = time.Now
	}

	if err := writer.rotateIfNeeded(); err != nil {
		return nil, err
	}

	return writer, nil
}

func (w *dailyRotatingWriter) Write(p []byte) (int, error) {
	w.mu.Lock()
	defer w.mu.Unlock()

	if err := w.rotateIfNeeded(); err != nil {
		return 0, err
	}

	return w.currentFile.Write(p)
}

func (w *dailyRotatingWriter) Sync() error {
	w.mu.Lock()
	defer w.mu.Unlock()

	if w.currentFile == nil {
		return nil
	}

	return w.currentFile.Sync()
}

func (w *dailyRotatingWriter) Close() error {
	w.mu.Lock()
	defer w.mu.Unlock()

	if w.currentFile == nil {
		return nil
	}

	err := w.currentFile.Close()
	w.currentFile = nil
	return err
}

func (w *dailyRotatingWriter) rotateIfNeeded() error {
	currentTime := w.now()
	date := currentTime.Format("2006-01-02")
	if w.currentFile != nil && w.currentDate == date {
		return nil
	}

	path := datedLogPath(w.basePath, currentTime)
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		return fmt.Errorf("create log directory: %w", err)
	}

	file, err := os.OpenFile(path, os.O_CREATE|os.O_APPEND|os.O_WRONLY, 0o644)
	if err != nil {
		return fmt.Errorf("open log file: %w", err)
	}

	if w.currentFile != nil {
		_ = w.currentFile.Close()
	}

	w.currentDate = date
	w.currentFile = file
	return nil
}

func datedLogPath(basePath string, currentTime time.Time) string {
	ext := filepath.Ext(basePath)
	trimmed := strings.TrimSuffix(basePath, ext)
	return fmt.Sprintf("%s-%s%s", trimmed, currentTime.Format("2006-01-02"), ext)
}
