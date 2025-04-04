FROM golang:1.21-alpine AS builder

WORKDIR /app

# Instalar dependências e o migrate tool
RUN apk add --no-cache ca-certificates git
RUN go install -tags 'postgres' github.com/golang-migrate/migrate/v4/cmd/migrate@latest

# Copiar arquivos de módulos e baixar dependências
COPY go.mod go.sum ./
RUN go mod download

# Copiar o código-fonte e construir a aplicação
COPY . .
RUN CGO_ENABLED=0 GOOS=linux go build -o /app/api ./cmd/api

# Imagem final minimalista
FROM alpine:latest

WORKDIR /app

RUN apk --no-cache add ca-certificates tzdata

# Copiar binário da API e o migrate tool da etapa builder
COPY --from=builder /app/api /app/api
COPY --from=builder /go/bin/migrate /usr/local/bin/migrate

# Copiar os arquivos de migração e os scripts (entrypoint e wait-for-it)
COPY migrations /app/migrations
COPY scripts/wait-for-it.sh /app/scripts/wait-for-it.sh
COPY scripts/entrypoint.sh /app/entrypoint.sh
RUN chmod +x /app/entrypoint.sh /app/scripts/wait-for-it.sh

# Definir o entrypoint
ENTRYPOINT ["/app/entrypoint.sh"]
