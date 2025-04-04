#!/bin/sh
set -e

echo "Aguardando PostgreSQL..."
/app/scripts/wait-for-it.sh postgres:5432 -t 30

echo "Aguardando MinIO..."
/app/scripts/wait-for-it.sh minio:9000 -t 30

echo "Aguardando RabbitMQ..."
/app/scripts/wait-for-it.sh rabbitmq:5672 -t 30

echo "Iniciando o Worker..."
exec /app/worker
