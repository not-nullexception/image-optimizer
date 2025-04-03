package rabbitmq

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"time"

	"github.com/not-nullexception/image-optimizer/config"
	"github.com/not-nullexception/image-optimizer/internal/logger"
	rabbitmq "github.com/not-nullexception/image-optimizer/internal/queue"
	amqp "github.com/rabbitmq/amqp091-go"
	"github.com/rs/zerolog"
)

type RabbitMQClient struct {
	conn         *amqp.Connection
	channel      *amqp.Channel
	queueName    string
	exchangeName string
	routingKey   string
	consumerTag  string
	logger       zerolog.Logger
}

const (
	TaskTypeResizeImage = "resize_image"
)

func NewClient(cfg *config.RabbitMQConfig) (rabbitmq.Client, error) {
	log := logger.GetLogger("rabbitmq-client")

	// Connect to RabbitMQ
	conn, err := connect(cfg, log)
	if err != nil {
		return nil, err
	}

	// Create a channel
	channel, err := conn.Channel()
	if err != nil {
		conn.Close()
		return nil, fmt.Errorf("error creating channel: %w", err)
	}

	// Declare exchange
	err = channel.ExchangeDeclare(
		cfg.Exchange, //name
		"direct",     // type
		true,         // durable
		false,        // auto-deleted
		false,        // internal
		false,        // no-wait
		nil,          // arguments
	)
	if err != nil {
		channel.Close()
		conn.Close()
		return nil, fmt.Errorf("error declaring exchange: %w", err)
	}

	// Declare queue
	_, err = channel.QueueDeclare(
		cfg.Queue, // name
		true,      // durable
		false,     // delete when unused
		false,     // exclusive
		false,     // no-wait
		nil,       // arguments
	)
	if err != nil {
		channel.Close()
		conn.Close()
		return nil, fmt.Errorf("error declaring queue: %w", err)
	}

	// Bind queue to exchange
	err = channel.QueueBind(
		cfg.Queue,      // queue name
		cfg.RoutingKey, // routing key
		cfg.Exchange,   // exchange name
		false,          // no-wait
		nil,            // arguments
	)
	if err != nil {
		channel.Close()
		conn.Close()
		return nil, fmt.Errorf("error binding queue: %w", err)
	}

	// Set QoS
	err = channel.Qos(
		1,     // prefetch count
		0,     // prefetch size
		false, // global
	)
	if err != nil {
		channel.Close()
		conn.Close()
		return nil, fmt.Errorf("error setting QoS: %w", err)
	}

	log.Info().
		Str("exchange", cfg.Exchange).
		Str("queue", cfg.Queue).
		Str("routing_key", cfg.RoutingKey).
		Msg("RabbitMQ client initialized")

	return &RabbitMQClient{
		conn:         conn,
		channel:      channel,
		queueName:    cfg.Queue,
		exchangeName: cfg.Exchange,
		routingKey:   cfg.RoutingKey,
		consumerTag:  cfg.ConsumerTag,
		logger:       log,
	}, nil
}

func connect(cfg *config.RabbitMQConfig, log zerolog.Logger) (*amqp.Connection, error) {
	var conn *amqp.Connection
	var err error

	maxRetries := 5
	retryDelay := time.Second

	for i := 0; i < maxRetries; i++ {
		log.Info().
			Str("host", cfg.Host).
			Int("port", cfg.Port).
			Int("attempt", i+1).
			Int("max_attempts", maxRetries).
			Msg("Connecting to RabbitMQ")

		conn, err = amqp.Dial(cfg.RabbitMQURL())
		if err != nil {
			log.Info().Msg("Connected to RabbitMQ")
			return conn, nil
		}

		log.Warn().
			Err(err).
			Int("attempt", i+1).
			Dur("retry_delay", retryDelay).
			Msg("Failed to connect to RabbitMQ, retrying...")

		time.Sleep(retryDelay)
		retryDelay *= 2 // Exponential backoff
	}

	return nil, fmt.Errorf("failed to connect to RabbitMQ after %d attempts: %w", maxRetries, err)
}

// Publish publishes a task to the queue
func (c *RabbitMQClient) Publish(ctx context.Context, task rabbitmq.Task) error {
	body, err := json.Marshal(task)
	if err != nil {
		return fmt.Errorf("error marshaling task: %w", err)
	}

	err = c.channel.PublishWithContext(
		ctx,
		c.exchangeName, // exchange
		c.routingKey,   // routing key
		false,          // mandatory
		false,          // immediate
		amqp.Publishing{
			ContentType:  "application/json",
			DeliveryMode: amqp.Persistent,
			Body:         body,
		},
	)
	if err != nil {
		return fmt.Errorf("error publishing message: %w", err)
	}

	c.logger.Debug().
		Str("task_id", task.ID).
		Str("task_type", string(task.Type)).
		Msg("Task published")

	return nil
}

// Consume starts consuming tasks from the queue
func (c *RabbitMQClient) Consume(ctx context.Context, processFunc rabbitmq.ProcessFunc) error {
	messages, err := c.channel.Consume(
		c.queueName,   // queue
		c.consumerTag, // consumer
		false,         // auto-ack
		false,         // exclusive
		false,         // no-local
		false,         // no-wait
		nil,           // args
	)
	if err != nil {
		return fmt.Errorf("error consuming from queue: %w", err)
	}

	c.logger.Info().
		Str("queue", c.queueName).
		Str("consumer_tag", c.consumerTag).
		Msg("Started consuming messages")

	// Process messages in a separate goroutine
	go func() {
		for {
			select {
			case msg, ok := <-messages:
				if !ok {
					c.logger.Warn().Msg("RabbitMQ channel closed")
					return
				}

				c.logger.Debug().
					Str("delivery_tag", fmt.Sprintf("%d", msg.DeliveryTag)).
					Msg("Received message")

				// Process the message
				err := c.processMessage(ctx, msg, processFunc)
				if err != nil {
					c.logger.Error().
						Err(err).
						Str("delivery_tag", fmt.Sprintf("%d", msg.DeliveryTag)).
						Msg("Error processing message")

					// Reject the message and requeue
					err = msg.Nack(false, true)
					if err != nil {
						c.logger.Error().
							Err(err).
							Str("delivery_tag", fmt.Sprintf("%d", msg.DeliveryTag)).
							Msg("Error negatively acknowledging message")
					}
				} else {
					// Acknowledge the message
					err = msg.Ack(false)
					if err != nil {
						c.logger.Error().
							Err(err).
							Str("delivery_tag", fmt.Sprintf("%d", msg.DeliveryTag)).
							Msg("Error acknowledging message")
					}
				}

			case <-ctx.Done():
				c.logger.Info().Msg("Stopping consumer due to context cancellation")
				return
			}
		}
	}()

	return nil
}

func (c *RabbitMQClient) processMessage(ctx context.Context, msg amqp.Delivery, processFunc rabbitmq.ProcessFunc) error {
	var task rabbitmq.Task
	err := json.Unmarshal(msg.Body, &task)
	if err != nil {
		return fmt.Errorf("error unmarshaling message: %w", err)
	}

	c.logger.Debug().
		Str("task_id", task.ID).
		Str("task_type", string(task.Type)).
		Msg("Processing task")

	err = processFunc(ctx, task)
	if err != nil {
		return fmt.Errorf("error processing task: %w", err)
	}

	c.logger.Debug().
		Str("task_id", task.ID).
		Str("task_type", string(task.Type)).
		Msg("Task processed successfully")

	return nil
}

// Close closes the RabbitMQ connection
func (c *RabbitMQClient) Close() error {
	var err error
	var channelErr, connErr error

	if c.channel != nil {
		channelErr = c.channel.Close()
	}

	if c.conn != nil {
		connErr = c.conn.Close()
	}

	// Return the first non-nil error
	if channelErr != nil {
		err = errors.Join(err, fmt.Errorf("error closing channel: %w", channelErr))
	}
	if connErr != nil {
		err = errors.Join(err, fmt.Errorf("error closing connection: %w", connErr))
	}

	if err != nil {
		return err
	}

	c.logger.Info().Msg("RabbitMQ client closed")
	return nil
}
