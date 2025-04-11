// Arquivo: internal/api/middleware/logger.go
package middleware

import (
	"strings"

	"github.com/gin-gonic/gin"
	"github.com/not-nullexception/image-optimizer/internal/logger" // Ajuste o path se necessário
)

// ContextualLogger cria um middleware Gin que:
// 1. Obtém um logger contextualizado (com trace/span IDs, se disponíveis).
// 2. Injeta esse logger no contexto da requisição.
func ContextualLogger(defaultComponent string) gin.HandlerFunc {
	return func(c *gin.Context) {
		component := defaultComponent
		// Tentativa opcional de obter um nome de componente mais específico da rota Gin
		routePath := c.FullPath() // Ex: "/images/:id"
		if routePath != "" {
			// Exemplo simples de limpeza do path para usar como componente
			component = strings.Trim(strings.ReplaceAll(routePath, "/", "-"), "-")
			if component == "" {
				component = "root"
			} // Caso seja a rota "/"
		}

		// Cria o logger usando o contexto original da requisição (que já deve ter trace info do middleware OpenTelemetry)
		requestLogger := logger.GetLoggerWithContext(c.Request.Context(), component)

		// Cria um *novo* contexto derivado do original, mas com o logger anexado
		newCtx := logger.ToContext(c.Request.Context(), requestLogger)

		// Substitui o contexto da requisição pelo novo contexto com o logger
		c.Request = c.Request.WithContext(newCtx)

		// Chama o próximo middleware ou handler na cadeia
		c.Next()
	}
}
