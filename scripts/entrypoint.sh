#!/bin/sh
set -e

echo "Aguardando PostgreSQL..."
/app/scripts/wait-for-it.sh postgres:5432 -t 30

echo "Aguardando MinIO..."
/app/scripts/wait-for-it.sh minio:9000 -t 30

echo "Aguardando RabbitMQ..."
/app/scripts/wait-for-it.sh rabbitmq:5672 -t 30

echo "Executando migrações..."
migrate -path=/app/migrations -database=postgres://${DB_USER}:${DB_PASSWORD}@${DB_HOST}:${DB_PORT}/${DB_NAME}?sslmode=${DB_SSL_MODE} up

echo "Iniciando a API..."
exec /app/api
