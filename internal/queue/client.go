package rabbitmq

import (
	"context"
)

type Task struct {
	ID   string         `json:"id"`
	Type string         `json:"type"`
	Data map[string]any `json:"data"`
}

// ProcessFunc is a function that processes a task
type ProcessFunc func(ctx context.Context, task Task) error

// Client defines the interface for RabbitMQ operations
type Client interface {
	Publish(ctx context.Context, task Task) error
	Consume(ctx context.Context, processFunc ProcessFunc) error

	// Close closes the RabbitMQ connection
	Close() error
}
