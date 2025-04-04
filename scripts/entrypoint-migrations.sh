#!/bin/sh
set -e

echo "Aguardando PostgreSQL..."
/app/scripts/wait-for-it.sh postgres:5432 -t 30

echo "Executando migrações..."
exec migrate -path=/app/migrations -database=postgres://${DB_USER}:${DB_PASSWORD}@${DB_HOST}:${DB_PORT}/${DB_NAME}?sslmode=${DB_SSL_MODE} up
