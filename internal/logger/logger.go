package logger

import (
	"context"
	"strings"
	"time"

	"github.com/not-nullexception/image-optimizer/config"
	"github.com/rs/zerolog"
	"github.com/rs/zerolog/log"
	"go.opentelemetry.io/otel/trace"
)

func Setup(cfg *config.LogConfig) {
	zerolog.TimeFieldFormat = time.RFC3339

	level := getLogLevel(cfg.Level)
	zerolog.SetGlobalLevel(level)

	log.Info().Str("level", level.String()).Msg("Logger initialized")
}

// getLogLevel converts a string log level to zerolog.Level
func getLogLevel(level string) zerolog.Level {
	switch strings.ToLower(level) {
	case "debug":
		return zerolog.DebugLevel
	case "info":
		return zerolog.InfoLevel
	case "warn":
		return zerolog.WarnLevel
	case "error":
		return zerolog.ErrorLevel
	case "fatal":
		return zerolog.FatalLevel
	case "panic":
		return zerolog.PanicLevel
	default:
		return zerolog.InfoLevel
	}
}

// GetLogger returns a configured logger with the given component
func GetLogger(component string) zerolog.Logger {
	return log.With().Str("component", component).Logger()
}

// TODO - Study best way to migrate to use GetLoggerWithContext
// GetLoggerWithContext returns a configured logger with the given component and context
func GetLoggerWithContext(component string, ctx context.Context) zerolog.Logger {
	logger := log.With().Str("component", component).Logger()
	span := trace.SpanFromContext(ctx)
	if span.SpanContext().IsValid() {
		logger = logger.With().
			Str("trace_id", span.SpanContext().TraceID().String()).
			Str("span_id", span.SpanContext().SpanID().String()).
			Logger()
	}
	logger.Info().Msg("Logger configured with tracing information")
	return logger
}
