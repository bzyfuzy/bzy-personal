package logging

import (
	"go.uber.org/zap"
	"go.uber.org/zap/zapcore"
)

// New builds a production-ready zap logger.
func New(level, format, serviceName string) (*zap.Logger, error) {
	lvl, err := zapcore.ParseLevel(level)
	if err != nil {
		lvl = zapcore.InfoLevel
	}

	var cfg zap.Config
	if format == "console" {
		cfg = zap.NewDevelopmentConfig()
	} else {
		cfg = zap.NewProductionConfig()
	}
	cfg.Level = zap.NewAtomicLevelAt(lvl)

	logger, err := cfg.Build(
		zap.AddCaller(),
		zap.AddStacktrace(zapcore.ErrorLevel),
		zap.Fields(zap.String("service", serviceName)),
	)
	if err != nil {
		return nil, err
	}
	return logger, nil
}

// MustNew panics if logger construction fails — use only in main().
func MustNew(level, format, serviceName string) *zap.Logger {
	l, err := New(level, format, serviceName)
	if err != nil {
		panic(err)
	}
	return l
}
