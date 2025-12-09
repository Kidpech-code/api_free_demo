package logging

import (
	"go.uber.org/zap"
	"go.uber.org/zap/zapcore"
)

// New builds a zap logger according to env.
func New(env string) (*zap.Logger, error) {
	config := zap.NewProductionConfig()
	if env == "development" {
		config = zap.NewDevelopmentConfig()
	}
	config.OutputPaths = []string{"stdout"}
	config.ErrorOutputPaths = []string{"stderr"}
	config.EncoderConfig.TimeKey = "timestamp"
	config.EncoderConfig.EncodeTime = zapcore.ISO8601TimeEncoder
	return config.Build()
}

// WithRequestID attaches request context to logger.
func WithRequestID(logger *zap.Logger, requestID string) *zap.Logger {
	if logger == nil {
		return zap.NewNop()
	}
	return logger.With(zap.String("request_id", requestID))
}

// ReplaceGlobals ensures go-kit libs use same logger.
func ReplaceGlobals(logger *zap.Logger) {
	zap.ReplaceGlobals(logger)
}

// Sync flushes logger.
func Sync(logger *zap.Logger) {
	if logger == nil {
		return
	}
	_ = logger.Sync()
}
