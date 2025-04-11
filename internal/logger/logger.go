// Arquivo: internal/logger/logger.go
package logger

import (
	"context"
	"strings"
	"time"

	// Verifique se o path do config está correto para seu projeto
	"github.com/not-nullexception/image-optimizer/config"
	"github.com/rs/zerolog"
	"github.com/rs/zerolog/log" // Logger global zerolog
	"go.opentelemetry.io/otel/trace"
)

// contextKey define um tipo privado para chaves de contexto para evitar colisões.
type contextKey string

// loggerKey é a chave usada para armazenar/recuperar o logger do context.Context.
const loggerKey = contextKey("logger")

// baseLogger fornece uma instância base do logger.
// Usar log.With().Logger() cria uma instância separada, mais segura para futuras
// modificações (como hooks) do que usar diretamente log.Logger global.
var baseLogger = log.With().Logger()

// Setup inicializa as configurações globais do zerolog e reconfigura nosso baseLogger.
func Setup(cfg *config.LogConfig) {
	zerolog.TimeFieldFormat = time.RFC3339
	level := getLogLevel(cfg.Level)
	zerolog.SetGlobalLevel(level) // Define o nível globalmente

	// Atualiza nosso baseLogger para refletir as configurações globais atuais
	// (caso mude o output writer global, por exemplo).
	baseLogger = log.With().Logger()

	// Log inicial usa a instância global zerolog.log
	log.Info().Str("level", level.String()).Msg("Global logger initialized")
}

// getLogLevel (Permanece igual)
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

// GetLogger retorna um logger básico com apenas o componente.
// Útil para logs fora do contexto de uma requisição (ex: inicialização, tarefas em background).
func GetLogger(component string) zerolog.Logger {
	return baseLogger.With().Str("component", component).Logger()
}

// GetLoggerWithContext retorna um logger enriquecido com o nome do componente
// e IDs de trace/span (se disponíveis no contexto).
func GetLoggerWithContext(ctx context.Context, component string) zerolog.Logger {
	// Começa com o logger base
	loggerWithComponent := baseLogger.With().Str("component", component).Logger()

	span := trace.SpanFromContext(ctx)
	if spanCtx := span.SpanContext(); spanCtx.IsValid() {
		// Retorna uma NOVA instância de logger com os IDs adicionados
		return loggerWithComponent.With().
			Str("trace_id", spanCtx.TraceID().String()).
			Str("span_id", spanCtx.SpanID().String()).
			Logger()
	}
	// Retorna o logger apenas com o componente se não houver span válido
	return loggerWithComponent
}

// ToContext anexa o logger fornecido ao context.Context.
func ToContext(ctx context.Context, logger zerolog.Logger) context.Context {
	return context.WithValue(ctx, loggerKey, logger)
}

// FromContext recupera o logger do context.Context.
// Retorna um logger de fallback (sem trace IDs) se nenhum logger for encontrado.
func FromContext(ctx context.Context) zerolog.Logger {
	if logger, ok := ctx.Value(loggerKey).(zerolog.Logger); ok {
		return logger // Retorna o logger encontrado no contexto
	}

	// Fallback: Se nenhum logger for encontrado, retorna um logger básico.
	// Isso evita erros, mas os logs não terão trace IDs.
	// Você pode querer logar um aviso aqui se isso não for esperado.
	fallbackLogger := GetLogger("context-fallback")
	fallbackLogger.Warn().Msg("Logger not found in context, using fallback logger. Trace information will be missing.")
	return fallbackLogger // Componente genérico para o fallback
}
