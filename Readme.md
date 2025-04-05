# Image Optimizer API

A high-performance, fully observable, containerized image optimization service built with Go. This project provides a scalable solution for image size reduction while maintaining quality, using modern cloud-native technologies and patterns.

![Go](https://img.shields.io/badge/Go-1.21%2B-blue)
![Docker](https://img.shields.io/badge/Docker-Compose-blue)
![License](https://img.shields.io/badge/License-MIT-green)

## 🌟 Features

- **Asynchronous Image Processing**: Upload images and receive a transaction ID instantly - processing happens in the background
- **Resize & Optimize**: Reduce image file size while maintaining quality using Go's imaging libraries
- **RESTful API**: Clean API endpoints with Gin web framework
- **Persistent Storage**: Optimized images stored in MinIO (S3-compatible)
- **Scalable Architecture**: Process multiple images concurrently with worker pools
- **Message Queue**: RabbitMQ for reliable background processing
- **Cloud Native**: Fully containerized with Docker and ready for Kubernetes
- **Complete Observability**: OpenTelemetry for traces, Prometheus for metrics, Loki for logs

## 📋 Technology Stack

### Backend Core
- **Go 1.21+**: Modern, high-performance language for backend services
- **Gin**: Fast HTTP web framework with excellent middleware support
- **Zerolog**: Zero-allocation JSON logger for structured logging
- **Viper**: Configuration management supporting env variables and config files
- **PostgreSQL + pgx**: High-performance PostgreSQL driver and toolkit for Go
- **goroutines**: Concurrent processing with Go's lightweight threads

### Storage & Messaging
- **MinIO**: S3-compatible object storage for image files
- **PostgreSQL**: Relational database for metadata and transaction records
- **RabbitMQ**: Message broker for background processing tasks

### Image Processing
- **Imaging Library**: Go-based image resizing and manipulation
- **Concurrent Worker Pool**: Custom implementation for parallel processing

### Observability
- **OpenTelemetry**: Distributed tracing and metrics collection framework
- **Tempo**: High-scale, minimal-dependency distributed tracing backend
- **Prometheus**: Time-series metrics database
- **Loki**: Log aggregation system designed for high throughput
- **Grafana**: Visualization platform for metrics, logs, and traces
- **Promtail**: Log collection agent for Loki

### Infrastructure
- **Docker & Docker Compose**: Containerization and local orchestration
- **Makefile**: Build automation and development tooling
- **Go Modules**: Dependency management
- **Database Migrations**: Versioned database schema management

## 🏗️ Architecture

TODO - Draw on Excalidraw

The service follows a modern, distributed architecture pattern with several key components:

```
┌─────────────┐     ┌──────────────┐     ┌───────────┐     ┌─────────────┐
│   API Server│──┬──│PostgreSQL DB │     │ RabbitMQ  │     │ MinIO       │
│   (Gin)     │  │  │              │     │ Queue     │     │ Storage     │
└─────────────┘  │  └──────────────┘     └───────────┘     └─────────────┘
                 │         ▲                  ▲                   ▲
                 │         │                  │                   │
                 ├─────────┼──────────────────┼───────────────────┤
                 │         │                  │                   │
                 ▼         │                  │                   │
┌─────────────┐  │  ┌──────────────┐     ┌───────────┐      ┌────────────┐
│ Worker      │──┘  │ OpenTelemetry│     │ Prometheus│      │ Loki       │
│ Processors  │     │ Collector    │     │ Metrics   │      │ Logs       │
└─────────────┘     └──────────────┘     └───────────┘      └────────────┘
                            │                 │                   │
                            └─────────────────┼───────────────────┘
                                              ▼
                                        ┌───────────┐
                                        │ Grafana   │
                                        │ Dashboard │
                                        └───────────┘
```

### Process Flow:

1. Client uploads an image via the API
2. API stores image in MinIO and creates a database entry
3. API publishes a processing task to RabbitMQ and returns transaction ID
4. Worker services consume tasks and process images in parallel
5. Workers update the database and store optimized images
6. Client queries status via API using the transaction ID
7. All operations are traced and monitored through the observability stack

## 🚀 Getting Started

### Prerequisites

- Docker and Docker Compose
- Make (optional, for using the Makefile)
- Go 1.21+ (for local development)

### Quick Start

1. Clone the repository:
```bash
git clone https://github.com/yourusername/image-optimizer.git
cd image-optimizer
```

2. Start all services using Docker Compose:
```bash
make docker-run
# or 
docker-compose up -d
```

3. Test the API:
```bash
# Upload an image
curl -F "image=@/path/to/image.jpg" http://localhost:8080/api/images

# Check status (replace ID with the one from the response)
curl http://localhost:8080/api/images/123e4567-e89b-12d3-a456-426614174000
```

4. Access dashboards:
- Grafana: http://localhost:3000 (admin/admin)
- MinIO Console: http://localhost:9001 (minioadmin/minioadmin)
- RabbitMQ Management: http://localhost:15672 (guest/guest)

### Environment Variables

All configuration is handled via environment variables. See `.env.example` for all available options.

## 🔍 Observability

This project implements the "three pillars of observability" to provide complete visibility into the system:

### 1. Logs (Loki)
- Structured JSON logging with context fields
- Centralized log collection and indexing
- Queries by service, level, and custom attributes
- Correlation with traces via trace ID fields

### 2. Metrics (Prometheus)
- Request counts, latencies, and error rates
- Worker pool utilization and queue depths
- System resource utilization
- Custom business metrics like optimization ratios

### 3. Traces (OpenTelemetry + Tempo)
- End-to-end transaction tracking
- Detailed timing of each processing step
- Service dependencies and bottleneck identification
- Correlation with logs and metrics

### Dashboards (Grafana)
- System overview with key performance indicators
- Service-specific operational dashboards
- Log exploration and search
- Trace visualization and analysis

Access Grafana at http://localhost:3000 to view all dashboards.

## 📝 API Documentation

### Upload Image
```
POST /api/images
```
- **Request**: Multipart form with `image` field containing the file
- **Response**: 
  ```json
  {
    "id": "123e4567-e89b-12d3-a456-426614174000",
    "status": "pending"
  }
  ```

### Get Image Status
```
GET /api/images/{id}
```
- **Response**:
  ```json
  {
    "id": "123e4567-e89b-12d3-a456-426614174000",
    "original_name": "example.jpg",
    "status": "completed",
    "original_url": "https://...",
    "optimized_url": "https://...",
    "original_size": 1024000,
    "optimized_size": 512000,
    "reduction": 50.0,
    "created_at": "2023-01-01T12:00:00Z",
    "updated_at": "2023-01-01T12:01:00Z"
  }
  ```

### List Images
```
GET /api/images?limit=10&page=1
```
- **Response**:
  ```json
  {
    "images": [...],
    "total": 42
  }
  ```

### Delete Image
```
DELETE /api/images/{id}
```
- **Response**:
  ```json
  {
    "status": "success"
  }
  ```

## 🛠️ Development

### Makefile Commands

- `make build`: Build the application binaries
- `make docker-build`: Build Docker images
- `make docker-up`: Start all services via Docker Compose
- `make docker-down`: Stop all services
- `make run-api`: Run the API server locally
- `make run-worker`: Run the worker locally
- `make test`: Run all tests
- `make lint`: Run linters
- `make migrate-up`: Apply database migrations
- `make migrate-down`: Revert database migrations
- `make deps`: Install development dependencies

### Project Structure

```
image-optimizer/
├── cmd/
│   ├── api/           # API service entry point
│   └── worker/        # Worker service entry point
├── config/            # Configuration handling
├── internal/
│   ├── api/           # API implementation
│   │   ├── handlers/  # HTTP handlers
│   │   ├── middleware/# Gin middleware
│   │   └── router/    # Route definitions
│   ├── db/            # Database layer
│   │   ├── models/    # Data models
│   │   └── postgres/  # PostgreSQL implementation
│   ├── logger/        # Logging setup
│   ├── metrics/       # Metrics collection
│   ├── minio/         # MinIO client
│   ├── processor/     # Image processing logic
│   ├── queue/         # Message queue
│   │   └── rabbitmq/  # RabbitMQ implementation
│   ├── tracing/       # Distributed tracing
│   └── worker/        # Worker implementation
├── docker/            # Dockerfiles and configurations
├── migrations/        # Database migration files
└── .env.example       # Example environment variables
```

### Running Tests

```bash
# Run all tests
make test

# Run specific tests
go test ./internal/processor/...
```

## 📊 Performance

The service is designed for high throughput and scalability:

- Concurrent processing with configurable worker pools
- Efficient image optimization algorithms
- Connection pooling for database and object storage
- Horizontal scalability of all components

## 📜 License

This project is licensed under the MIT License - see the LICENSE file for details.

## 🤝 Contributing

Contributions are welcome! Please feel free to submit a Pull Request.

1. Fork the repository
2. Create your feature branch (`git checkout -b feature/amazing-feature`)
3. Commit your changes (`git commit -m 'Add some amazing feature'`)
4. Push to the branch (`git push origin feature/amazing-feature`)
5. Open a Pull Request


TODO - Add examples
TODO - Use TLS on MinIO and RabbitMQ
TODO - Add how to configure to production