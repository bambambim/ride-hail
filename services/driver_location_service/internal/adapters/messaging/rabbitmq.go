package messaging

import (
	"context"
	"encoding/json"
	"fmt"
	"time"

	"ride-hail/pkg/logger"
	"ride-hail/services/driver_location_service/internal/domain"
	"ride-hail/services/driver_location_service/internal/ports"

	amqp "github.com/rabbitmq/amqp091-go"
)

// RabbitMQBroker implements the MessageBroker interface using RabbitMQ
type RabbitMQBroker struct {
	conn    *amqp.Connection
	channel *amqp.Channel
	logger  logger.Logger
}

// RabbitMQConfig holds configuration for RabbitMQ connection
type RabbitMQConfig struct {
	URL        string
	Logger     logger.Logger
	Exchanges  ExchangeConfig
	Queues     QueueConfig
	MaxRetries int
	RetryDelay time.Duration
}

// ExchangeConfig holds exchange configuration
type ExchangeConfig struct {
	RideTopic      string
	DriverTopic    string
	LocationFanout string
}

// QueueConfig holds queue configuration
type QueueConfig struct {
	DriverMatching   string
	RideStatusUpdate string
}

// NewRabbitMQBroker creates a new RabbitMQ message broker
func NewRabbitMQBroker(config RabbitMQConfig) (ports.MessageBroker, error) {
	// Connect to RabbitMQ
	conn, err := amqp.Dial(config.URL)
	if err != nil {
		return nil, fmt.Errorf("failed to connect to RabbitMQ: %w", err)
	}

	// Create a channel
	channel, err := conn.Channel()
	if err != nil {
		conn.Close()
		return nil, fmt.Errorf("failed to open channel: %w", err)
	}

	broker := &RabbitMQBroker{
		conn:    conn,
		channel: channel,
		logger:  config.Logger,
	}

	// Declare exchanges and queues
	if err := broker.declareTopology(config); err != nil {
		broker.Close()
		return nil, fmt.Errorf("failed to declare topology: %w", err)
	}

	return broker, nil
}

// declareTopology declares all necessary exchanges and queues
func (b *RabbitMQBroker) declareTopology(config RabbitMQConfig) error {
	// Declare ride_topic exchange (topic)
	if err := b.channel.ExchangeDeclare(
		config.Exchanges.RideTopic, // name
		"topic",                    // type
		true,                       // durable
		false,                      // auto-deleted
		false,                      // internal
		false,                      // no-wait
		nil,                        // arguments
	); err != nil {
		return fmt.Errorf("failed to declare ride_topic exchange: %w", err)
	}

	// Declare driver_topic exchange (topic)
	if err := b.channel.ExchangeDeclare(
		config.Exchanges.DriverTopic, // name
		"topic",                      // type
		true,                         // durable
		false,                        // auto-deleted
		false,                        // internal
		false,                        // no-wait
		nil,                          // arguments
	); err != nil {
		return fmt.Errorf("failed to declare driver_topic exchange: %w", err)
	}

	// Declare location_fanout exchange (fanout)
	if err := b.channel.ExchangeDeclare(
		config.Exchanges.LocationFanout, // name
		"fanout",                        // type
		true,                            // durable
		false,                           // auto-deleted
		false,                           // internal
		false,                           // no-wait
		nil,                             // arguments
	); err != nil {
		return fmt.Errorf("failed to declare location_fanout exchange: %w", err)
	}

	// Declare driver_matching queue
	if _, err := b.channel.QueueDeclare(
		config.Queues.DriverMatching, // name
		true,                         // durable
		false,                        // delete when unused
		false,                        // exclusive
		false,                        // no-wait
		nil,                          // arguments
	); err != nil {
		return fmt.Errorf("failed to declare driver_matching queue: %w", err)
	}

	// Bind driver_matching queue to ride_topic exchange
	if err := b.channel.QueueBind(
		config.Queues.DriverMatching, // queue name
		"ride.request.*",             // routing key
		config.Exchanges.RideTopic,   // exchange
		false,
		nil,
	); err != nil {
		return fmt.Errorf("failed to bind driver_matching queue: %w", err)
	}

	// Declare ride_status_update queue
	if _, err := b.channel.QueueDeclare(
		config.Queues.RideStatusUpdate, // name
		true,                           // durable
		false,                          // delete when unused
		false,                          // exclusive
		false,                          // no-wait
		nil,                            // arguments
	); err != nil {
		return fmt.Errorf("failed to declare ride_status_update queue: %w", err)
	}

	// Bind ride_status_update queue to ride_topic exchange
	if err := b.channel.QueueBind(
		config.Queues.RideStatusUpdate, // queue name
		"ride.status.*",                // routing key
		config.Exchanges.RideTopic,     // exchange
		false,
		nil,
	); err != nil {
		return fmt.Errorf("failed to bind ride_status_update queue: %w", err)
	}

	return nil
}

// PublishDriverResponse publishes a driver's response to a ride offer
func (b *RabbitMQBroker) PublishDriverResponse(ctx context.Context, response *domain.DriverMatchResponse) error {
	body, err := json.Marshal(response)
	if err != nil {
		return fmt.Errorf("failed to marshal driver response: %w", err)
	}

	routingKey := fmt.Sprintf("driver.response.%s", response.RideID)

	err = b.channel.PublishWithContext(
		ctx,
		"driver_topic", // exchange
		routingKey,     // routing key
		false,          // mandatory
		false,          // immediate
		amqp.Publishing{
			ContentType:  "application/json",
			Body:         body,
			DeliveryMode: amqp.Persistent,
			Timestamp:    time.Now(),
			MessageId:    response.CorrelationID,
		},
	)

	if err != nil {
		b.logger.Error("messaging.publish_driver_response", err)
		return fmt.Errorf("failed to publish driver response: %w", err)
	}

	b.logger.Info("messaging.publish_driver_response", fmt.Sprintf("Published driver response for ride %s", response.RideID))
	return nil
}

// PublishDriverStatusUpdate publishes driver status changes
func (b *RabbitMQBroker) PublishDriverStatusUpdate(ctx context.Context, update *domain.DriverStatusUpdate) error {
	body, err := json.Marshal(update)
	if err != nil {
		return fmt.Errorf("failed to marshal driver status update: %w", err)
	}

	routingKey := fmt.Sprintf("driver.status.%s", update.DriverID)

	err = b.channel.PublishWithContext(
		ctx,
		"driver_topic", // exchange
		routingKey,     // routing key
		false,          // mandatory
		false,          // immediate
		amqp.Publishing{
			ContentType:  "application/json",
			Body:         body,
			DeliveryMode: amqp.Persistent,
			Timestamp:    time.Now(),
		},
	)

	if err != nil {
		b.logger.Error("messaging.publish_driver_status", err)
		return fmt.Errorf("failed to publish driver status update: %w", err)
	}

	b.logger.Info("messaging.publish_driver_status", fmt.Sprintf("Published status update for driver %s: %s", update.DriverID, update.Status))
	return nil
}

// PublishLocationUpdate publishes location updates to fanout exchange
func (b *RabbitMQBroker) PublishLocationUpdate(ctx context.Context, broadcast *domain.LocationBroadcast) error {
	body, err := json.Marshal(broadcast)
	if err != nil {
		return fmt.Errorf("failed to marshal location broadcast: %w", err)
	}

	err = b.channel.PublishWithContext(
		ctx,
		"location_fanout", // exchange
		"",                // routing key (empty for fanout)
		false,             // mandatory
		false,             // immediate
		amqp.Publishing{
			ContentType:  "application/json",
			Body:         body,
			DeliveryMode: amqp.Transient, // Don't persist location updates
			Timestamp:    time.Now(),
		},
	)

	if err != nil {
		b.logger.Error("messaging.publish_location", err)
		return fmt.Errorf("failed to publish location update: %w", err)
	}

	return nil
}

// ConsumeRideRequests starts consuming ride request messages
func (b *RabbitMQBroker) ConsumeRideRequests(ctx context.Context, handler func(context.Context, *domain.RideRequest) error) error {
	msgs, err := b.channel.Consume(
		"driver_matching", // queue
		"",                // consumer
		false,             // auto-ack (we'll manually ack)
		false,             // exclusive
		false,             // no-local
		false,             // no-wait
		nil,               // args
	)
	if err != nil {
		return fmt.Errorf("failed to register consumer: %w", err)
	}

	b.logger.Info("messaging.consume_ride_requests", "Started consuming ride requests")

	// Process messages in a goroutine
	go func() {
		for {
			select {
			case <-ctx.Done():
				b.logger.Info("messaging.consume_ride_requests", "Stopping ride request consumer")
				return

			case msg, ok := <-msgs:
				if !ok {
					b.logger.Info("messaging.consume_ride_requests", "Message channel closed")
					return
				}

				b.handleRideRequest(ctx, msg, handler)
			}
		}
	}()

	return nil
}

// handleRideRequest processes a single ride request message
func (b *RabbitMQBroker) handleRideRequest(ctx context.Context, msg amqp.Delivery, handler func(context.Context, *domain.RideRequest) error) {
	var request domain.RideRequest
	if err := json.Unmarshal(msg.Body, &request); err != nil {
		b.logger.Error("messaging.handle_ride_request.unmarshal", err)
		msg.Nack(false, false) // Don't requeue
		return
	}

	b.logger.Info("messaging.handle_ride_request", fmt.Sprintf("Processing ride request %s", request.RideID))

	// Call handler
	if err := handler(ctx, &request); err != nil {
		b.logger.Error("messaging.handle_ride_request.handler", err)
		// Requeue the message for retry
		msg.Nack(false, true)
		return
	}

	// Acknowledge the message
	if err := msg.Ack(false); err != nil {
		b.logger.Error("messaging.handle_ride_request.ack", err)
	}
}

// ConsumeRideStatusUpdates starts consuming ride status update messages
func (b *RabbitMQBroker) ConsumeRideStatusUpdates(ctx context.Context, handler func(context.Context, *domain.RideStatusUpdate) error) error {
	msgs, err := b.channel.Consume(
		"ride_status_update", // queue
		"",                   // consumer
		false,                // auto-ack
		false,                // exclusive
		false,                // no-local
		false,                // no-wait
		nil,                  // args
	)
	if err != nil {
		return fmt.Errorf("failed to register consumer: %w", err)
	}

	b.logger.Info("messaging.consume_ride_status", "Started consuming ride status updates")

	// Process messages in a goroutine
	go func() {
		for {
			select {
			case <-ctx.Done():
				b.logger.Info("messaging.consume_ride_status", "Stopping ride status consumer")
				return

			case msg, ok := <-msgs:
				if !ok {
					b.logger.Info("messaging.consume_ride_status", "Message channel closed")
					return
				}

				b.handleRideStatusUpdate(ctx, msg, handler)
			}
		}
	}()

	return nil
}

// handleRideStatusUpdate processes a single ride status update message
func (b *RabbitMQBroker) handleRideStatusUpdate(ctx context.Context, msg amqp.Delivery, handler func(context.Context, *domain.RideStatusUpdate) error) {
	var update domain.RideStatusUpdate
	if err := json.Unmarshal(msg.Body, &update); err != nil {
		b.logger.Error("messaging.handle_ride_status.unmarshal", err)
		msg.Nack(false, false) // Don't requeue
		return
	}

	b.logger.Info("messaging.handle_ride_status", fmt.Sprintf("Processing ride status update %s: %s", update.RideID, update.Status))

	// Call handler
	if err := handler(ctx, &update); err != nil {
		b.logger.Error("messaging.handle_ride_status.handler", err)
		// Requeue the message for retry
		msg.Nack(false, true)
		return
	}

	// Acknowledge the message
	if err := msg.Ack(false); err != nil {
		b.logger.Error("messaging.handle_ride_status.ack", err)
	}
}

// Close closes the RabbitMQ connection
func (b *RabbitMQBroker) Close() error {
	if b.channel != nil {
		if err := b.channel.Close(); err != nil {
			b.logger.Error("messaging.close_channel", err)
		}
	}

	if b.conn != nil {
		if err := b.conn.Close(); err != nil {
			b.logger.Error("messaging.close_connection", err)
			return err
		}
	}

	b.logger.Info("messaging.close", "RabbitMQ connection closed")
	return nil
}

// IsConnected checks if the broker is connected
func (b *RabbitMQBroker) IsConnected() bool {
	return b.conn != nil && !b.conn.IsClosed()
}

// Reconnect attempts to reconnect to RabbitMQ
func (b *RabbitMQBroker) Reconnect(config RabbitMQConfig) error {
	b.logger.Info("messaging.reconnect", "Attempting to reconnect to RabbitMQ")

	// Close existing connection
	if err := b.Close(); err != nil {
		b.logger.Error("messaging.reconnect.close", err)
	}

	// Reconnect
	conn, err := amqp.Dial(config.URL)
	if err != nil {
		return fmt.Errorf("failed to reconnect to RabbitMQ: %w", err)
	}

	channel, err := conn.Channel()
	if err != nil {
		conn.Close()
		return fmt.Errorf("failed to open channel: %w", err)
	}

	b.conn = conn
	b.channel = channel

	// Redeclare topology
	if err := b.declareTopology(config); err != nil {
		return fmt.Errorf("failed to redeclare topology: %w", err)
	}

	b.logger.Info("messaging.reconnect.success", "Successfully reconnected to RabbitMQ")
	return nil
}
