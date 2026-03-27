package logging

import (
	"context"

	"github.com/go-logr/logr"
)

type contextKey string

const loggerKey contextKey = "logger"

// WithLogger adds a logger to the context
func WithLogger(ctx context.Context, logger Logger) context.Context {
	return context.WithValue(ctx, loggerKey, logger)
}

// FromContext retrieves a logger from the context
// Returns a no-op logger if no logger is found in context
func FromContext(ctx context.Context) Logger {
	if logger, ok := ctx.Value(loggerKey).(Logger); ok {
		return logger
	}
	// Return no-op logger if not found
	return &noOpLogger{}
}

// WithValues adds key-value pairs to the logger in context
func WithValues(ctx context.Context, keysAndValues ...interface{}) context.Context {
	logger := FromContext(ctx)
	return WithLogger(ctx, logger.WithValues(keysAndValues...))
}

// WithName adds a name to the logger in context
func WithName(ctx context.Context, name string) context.Context {
	logger := FromContext(ctx)
	return WithLogger(ctx, logger.WithName(name))
}

// noOpLogger is a logger that does nothing
type noOpLogger struct{}

func (l *noOpLogger) Debug(msg string, keysAndValues ...interface{}) {}
func (l *noOpLogger) Info(msg string, keysAndValues ...interface{})  {}
func (l *noOpLogger) Warn(msg string, keysAndValues ...interface{})  {}
func (l *noOpLogger) Error(msg string, err error, keysAndValues ...interface{}) {}
func (l *noOpLogger) WithValues(keysAndValues ...interface{}) Logger {
	return l
}
func (l *noOpLogger) WithName(name string) Logger {
	return l
}

// ToLogr converts our Logger interface to logr.Logger
func ToLogr(logger Logger) logr.Logger {
	if logrLogger, ok := logger.(interface{ GetLogr() logr.Logger }); ok {
		return logrLogger.GetLogr()
	}
	return logr.Discard()
}

// FromLogr converts logr.Logger to our Logger interface
func FromLogr(logrLogger logr.Logger) Logger {
	return &logrAdapter{logger: logrLogger}
}

// logrAdapter adapts logr.Logger to our Logger interface
type logrAdapter struct {
	logger logr.Logger
}

func (l *logrAdapter) Debug(msg string, keysAndValues ...interface{}) {
	l.logger.V(1).Info(msg, keysAndValues...)
}

func (l *logrAdapter) Info(msg string, keysAndValues ...interface{}) {
	l.logger.Info(msg, keysAndValues...)
}

func (l *logrAdapter) Warn(msg string, keysAndValues ...interface{}) {
	l.logger.Info(msg, append([]interface{}{"level", "warn"}, keysAndValues...)...)
}

func (l *logrAdapter) Error(msg string, err error, keysAndValues ...interface{}) {
	if err != nil {
		keysAndValues = append(keysAndValues, "error", err.Error())
	}
	l.logger.Error(err, msg, keysAndValues...)
}

func (l *logrAdapter) WithValues(keysAndValues ...interface{}) Logger {
	return &logrAdapter{logger: l.logger.WithValues(keysAndValues...)}
}

func (l *logrAdapter) WithName(name string) Logger {
	return &logrAdapter{logger: l.logger.WithName(name)}
}

func (l *logrAdapter) GetLogr() logr.Logger {
	return l.logger
}
