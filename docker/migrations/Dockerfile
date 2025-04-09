FROM golang:1.24.2-alpine

WORKDIR /app

# Install migrate tool
RUN go install -tags 'postgres' github.com/golang-migrate/migrate/v4/cmd/migrate@latest

RUN apk --no-cache add bash
# Copy migration files
COPY migrations /app/migrations
COPY scripts/wait-for-it.sh /app/scripts/wait-for-it.sh
COPY scripts/entrypoint-migrations.sh /app/entrypoint-migrations.sh
RUN chmod +x /app/scripts/wait-for-it.sh /app/entrypoint-migrations.sh

# Define entry point
ENTRYPOINT ["/app/entrypoint-migrations.sh"]
