FROM golang:1.21-alpine AS builder

WORKDIR /app

# Install dependencies
RUN apk add --no-cache ca-certificates git

# Copy go.mod and go.sum files
COPY go.mod go.sum ./

# Download dependencies
RUN go mod download

# Copy the source code
COPY . .

# Build the application
RUN CGO_ENABLED=0 GOOS=linux go build -o /app/worker ./cmd/worker

# Create a minimal image
FROM alpine:latest

WORKDIR /app

RUN apk --no-cache add ca-certificates tzdata

# Copy binary from builder
COPY --from=builder /app/worker /app/worker

# Copiar o script de espera e o entrypoint do worker
COPY scripts/wait-for-it.sh /app/scripts/wait-for-it.sh
COPY scripts/entrypoint-worker.sh /app/entrypoint-worker.sh
RUN chmod +x /app/scripts/wait-for-it.sh /app/entrypoint-worker.sh

# Definir o entrypoint para o worker
ENTRYPOINT ["/app/entrypoint-worker.sh"]
