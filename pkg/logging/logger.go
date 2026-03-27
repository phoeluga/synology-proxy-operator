package logging

import (
	"fmt"
	"strings"
	
	"go.uber.org/zap"
	"go.uber.org/zap/zapcore"
)

// Logger is the interface for structured logging
type Logger interface {
	Info(msg string, keysAndValues ...interface{})
	Error(msg string, err error, keysAndValues ...interface{})
	Debug(msg string, keysAndValues ...interface{})
	Warn(msg string, keysAndValues ...interface{})
	WithValues(keysAndValues ...interface{}) Logger
	WithName(name string) Logger
}

// zapLogger wraps zap.Logger with sensitive data filtering
type zapLogger struct {
	logger *zap.Logger
}

// NewLogger creates a new logger with the given configuration
func NewLogger(logLevel, logFormat string) (Logger, error) {
	var zapConfig zap.Config
	
	if logFormat == "json" {
		zapConfig = zap.NewProductionConfig()
	} else {
		zapConfig = zap.NewDevelopmentConfig()
	}
	
	// Set log level
	level, err := parseLogLevel(logLevel)
	if err != nil {
		return nil, err
	}
	zapConfig.Level = zap.NewAtomicLevelAt(level)
	
	// Build logger
	zapLog, err := zapConfig.Build()
	if err != nil {
		return nil, fmt.Errorf("failed to build logger: %w", err)
	}
	
	return &zapLogger{logger: zapLog}, nil
}

// Info logs an informational message
func (l *zapLogger) Info(msg string, keysAndValues ...interface{}) {
	fields := sanitizeFields(keysAndValues)
	l.logger.Info(msg, fields...)
}

// Error logs an error message
func (l *zapLogger) Error(msg string, err error, keysAndValues ...interface{}) {
	fields := sanitizeFields(keysAndValues)
	fields = append(fields, zap.Error(err))
	l.logger.Error(msg, fields...)
}

// Debug logs a debug message
func (l *zapLogger) Debug(msg string, keysAndValues ...interface{}) {
	fields := sanitizeFields(keysAndValues)
	l.logger.Debug(msg, fields...)
}

// Warn logs a warning message
func (l *zapLogger) Warn(msg string, keysAndValues ...interface{}) {
	fields := sanitizeFields(keysAndValues)
	l.logger.Warn(msg, fields...)
}

// WithValues returns a logger with additional context
func (l *zapLogger) WithValues(keysAndValues ...interface{}) Logger {
	fields := sanitizeFields(keysAndValues)
	return &zapLogger{logger: l.logger.With(fields...)}
}

// WithName returns a logger with a name prefix
func (l *zapLogger) WithName(name string) Logger {
	return &zapLogger{logger: l.logger.Named(name)}
}

// parseLogLevel converts string log level to zapcore.Level
func parseLogLevel(level string) (zapcore.Level, error) {
	switch strings.ToLower(level) {
	case "debug":
		return zapcore.DebugLevel, nil
	case "info":
		return zapcore.InfoLevel, nil
	case "warn":
		return zapcore.WarnLevel, nil
	case "error":
		return zapcore.ErrorLevel, nil
	default:
		return zapcore.InfoLevel, fmt.Errorf("invalid log level: %s", level)
	}
}

// sanitizeFields filters sensitive data from log fields
func sanitizeFields(keysAndValues []interface{}) []zap.Field {
	// Use the centralized filter function
	filtered := FilterKeyValues(keysAndValues...)
	
	fields := make([]zap.Field, 0, len(filtered)/2)
	
	for i := 0; i < len(filtered); i += 2 {
		if i+1 >= len(filtered) {
			break
		}
		
		key, ok := filtered[i].(string)
		if !ok {
			continue
		}
		
		value := filtered[i+1]
		fields = append(fields, zap.Any(key, value))
	}
	
	return fields
}

// isSensitiveField checks if a field name contains sensitive data
// Deprecated: Use isSensitiveKey from filter.go instead
func isSensitiveField(key string) bool {
	return isSensitiveKey(key)
}
