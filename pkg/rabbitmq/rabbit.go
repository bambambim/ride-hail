package rabbitmq

import (
	"context"
	"fmt"
	"sync"
	"time"

	"ride-hail/pkg/config"
	"ride-hail/pkg/logger"

	amqp "github.com/rabbitmq/amqp091-go"
)

const (
	maxRetries    = 10
	retryInterval = 3 * time.Second
)

// Connection is a wrapper around the amqp.Connection that handles auto-reconnection.
type Connection struct {
	logger      logger.Logger
	config      *config.Config
	dsn         string
	conn        *amqp.Connection
	pubChannel  *amqp.Channel // A dedicated channel for publishing
	mu          sync.RWMutex  // Protects conn and pubChannel during reconnects
	isConnected bool
	notifyClose chan *amqp.Error
	done        chan bool // Signals graceful shutdown
}

func NewConnection(cfg *config.Config, log logger.Logger) (*Connection, error) {
	dsn := fmt.Sprintf("amqp://%s:%s@%s:%d/",
		cfg.RabbitMQ.User,
		cfg.RabbitMQ.Password,
		cfg.RabbitMQ.Host,
		cfg.RabbitMQ.Port,
	)
	c := &Connection{
		logger: log,
		config: cfg,
		dsn:    dsn,
		done:   make(chan bool),
	}
	var err error
	for i := 0; i < maxRetries; i++ {
		err = c.connect()
		if err != nil {
			log.Error("rabbitmq_connect_retry", fmt.Errorf("failed to connect to RabbitMQ (attempt %d/%d): %w", i+1, maxRetries, err))
			time.Sleep(retryInterval)
			continue
		}
		log.Info("rabbitmq_connect", "Initial RabbitMQ connection established")
		if setupErr := c.SetupTopology(); setupErr != nil {
			c.Close()
			return nil, fmt.Errorf("failed to setup RabbitMQ topology: %w", setupErr)
		}
		go c.reconnectLoop()
		return c, nil
	}
	return nil, fmt.Errorf("failed to connect to RabbitMQ after %d retries: %w", maxRetries, err)
}

func (c *Connection) connect() error {
	c.mu.Lock()
	defer c.mu.Unlock()

	var err error
	c.conn, err = amqp.Dial(c.dsn)
	if err != nil {
		return fmt.Errorf("failed to dial: %w", err)
	}
	c.pubChannel, err = c.conn.Channel()
	if err != nil {
		c.conn.Close()
		return fmt.Errorf("failed to open publisher channel: %w", err)
	}

	c.isConnected = true
	c.notifyClose = make(chan *amqp.Error, 1)
	c.conn.NotifyClose(c.notifyClose)

	c.logger.Info("rabbitmq_connect_internal", "Connection and publisher channel established")
	return nil
}

func (c *Connection) reconnectLoop() {
	c.logger.Info("rabbitmq_reconnect_loop", "Starting reconnection loop")
	for {
		select {
		case <-c.done:
			c.logger.Info("rabbitmq_reconnect_loop", "Shutting down reconnection loop")
			return
		case err := <-c.notifyClose:
			if err != nil {
				c.logger.Error("rabbitmq_disconnect", fmt.Errorf("RabbitMQ connection lost: %w", err))
				c.mu.Lock()
				c.isConnected = false
				c.mu.Unlock()

				backoff := time.Second
				for {
					c.logger.Info("rabbitmq_reconnect_attempt", fmt.Sprintf("Attempting to reconnect in %s...", backoff))
					time.Sleep(backoff)

					if err := c.connect(); err != nil {
						c.logger.Error("rabbitmq_reconnect_failed", fmt.Errorf("failed to reconnect to RabbitMQ: %w", err))
						backoff = time.Duration(float64(backoff) * 1.5)
						if backoff > 30*time.Second {
							backoff = 30 * time.Second
						}
						continue
					}

					if setupErr := c.SetupTopology(); setupErr != nil {
						c.logger.Error("rabbitmq_reconnect_setup_failed", fmt.Errorf("failed to re-declare topology to RabbitMQ: %w", setupErr))
						continue
					}
					c.logger.Info("rabbitmq_reconnect_success", "RabbitMQ connection established")
					break
				}
			} else {
				c.logger.Info("rabbitmq_reconnect_loop", "Connection closed gracefully")
				return // Graceful close
			}

		}
	}
}

// SetupTopology declares all required topology.
func (c *Connection) SetupTopology() error {
	c.mu.RLock()
	if !c.isConnected {
		c.mu.RUnlock()
		return fmt.Errorf("RabbitMQ does not connected")
	}
	ch, err := c.conn.Channel()
	if err != nil {
		c.mu.RUnlock()
		return fmt.Errorf("failed to open setup channel: %w", err)
	}
	defer ch.Close()
	c.mu.RUnlock()

	c.logger.Info("rabbitmq_setup", "Declaring RabbitMQ topology")

	exchanges := []struct {
		Name string
		Type string
	}{
		{Name: "ride_topic", Type: "topic"},
		{Name: "driver_topic", Type: "topic"},
		{Name: "location_fanout", Type: "fanout"},
	}
	for _, ex := range exchanges {
		if err := ch.ExchangeDeclare(ex.Name, ex.Type, true, false, false, false, nil); err != nil {
			return fmt.Errorf("failed to declare exchange %s: %w", ex.Name, err)
		}
	}

	queues := []string{
		"ride_requests",
		"ride_status",
		"driver_matching",
		"driver_responses",
		"driver_status",
		"location_updates_ride",
	}
	for _, queue := range queues {
		if _, err := ch.QueueDeclare(queue, true, false, false, false, nil); err != nil {
			return fmt.Errorf("failed to declare queue %s: %w", queue, err)
		}
	}
	bindings := []struct {
		Queue      string
		RoutingKey string
		Exchange   string
	}{
		{"ride_requests", "ride.request.*", "ride_topic"},
		{"ride_status", "ride.status.*", "ride_topic"},
		{"driver_matching", "ride.request.*", "ride_topic"},
		{"driver_responses", "driver.response.*", "driver_topic"},
		{"driver_status", "driver.status.*", "driver_topic"},
		{"location_updates_ride", "", "location_fanout"}, // No routing key for fanout
	}
	for _, b := range bindings {
		if err := ch.QueueBind(b.Queue, b.RoutingKey, b.Exchange, false, nil); err != nil {
			return fmt.Errorf("failed to bind queue %s to %s: %w", b.Queue, b.Exchange, err)
		}
	}
	c.logger.Info("rabbitmq_setup_success", "Successfully declared RabbitMQ topology")
	return nil
}

// Publish sends a message to an exchange. It is goroutine-safe.
func (c *Connection) Publish(ctx context.Context, exchange, routingkey string, body []byte) error {
	c.mu.RLock()
	defer c.mu.RUnlock()

	if !c.isConnected {
		return fmt.Errorf("RabbitMQ does not connected")
	}
	msg := amqp.Publishing{
		ContentType:  "application/json",
		Body:         body,
		DeliveryMode: amqp.Persistent,
		Timestamp:    time.Now(),
	}
	return c.pubChannel.Publish(exchange, routingkey, false, false, msg)
}

// Consume starts a consumer on a specific queue.
// The handler function is executed for each message.
// This method handles its own reconnection loop for the consumer.
func (c *Connection) Consume(queueName string, handler func(amqp.Delivery)) error {
	log := c.logger.WithFields(logger.LogFields{"queue": queueName})
	log.Info("consumer_start", "Starting consumer goroutine")

	go func() {
		for {
			c.mu.RLock() // Read lock to check connection status
			if !c.isConnected {
				c.mu.RUnlock()
				log.Info("consumer_wait", "Not connected, waiting to restart consumer...")
				time.Sleep(retryInterval)
				continue
			}

			// Create a new channel for this consumer
			ch, err := c.conn.Channel()
			if err != nil {
				c.mu.RUnlock()
				log.Error("consumer_channel_fail", fmt.Errorf("failed to open consumer channel: %w", err))
				time.Sleep(retryInterval)
				continue
			}
			c.mu.RUnlock() // Unlock after getting channel

			// Start consuming
			msgs, err := ch.Consume(
				queueName,
				"",    // consumer tag
				false, // auto-ack (false = manual ack)
				false, // exclusive
				false, // no-local
				false, // no-wait
				nil,   // args
			)
			if err != nil {
				log.Error("consumer_consume_fail", fmt.Errorf("failed to start consuming: %w", err))
				ch.Close()
				time.Sleep(retryInterval)
				continue
			}

			log.Info("consumer_running", "Consumer started and waiting for messages")

			// Create listeners for channel closure or service shutdown
			notifyChanClose := ch.NotifyClose(make(chan *amqp.Error, 1))

			// Run the consumer loop
		consumerLoop:
			for {
				select {
				case <-c.done:
					log.Info("consumer_shutdown", "Service shutting down, stopping consumer")
					ch.Close()
					return // Exit goroutine

				case err := <-notifyChanClose:
					log.Error("consumer_channel_closed", fmt.Errorf("consumer channel closed: %v", err))
					break consumerLoop // Exit loop to reconnect

				case msg, ok := <-msgs:
					if !ok {
						log.Error("consumer_delivery_closed", fmt.Errorf("delivery channel closed"))
						break consumerLoop // Exit loop to reconnect
					}
					// Run the handler in a new goroutine so one slow message
					// doesn't block all other messages on this channel.
					go handler(msg)
				}
			}
		}
	}()
	return nil
}

// Close gracefully shuts down the connection and the reconnect loop.
func (c *Connection) Close() {
	c.mu.Lock()
	defer c.mu.Unlock()
	if !c.isConnected {
		return
	}
	c.logger.Info("rabbitmq_close", "Closing RabbitMQ connection")
	c.isConnected = false
	close(c.done)

	if c.pubChannel != nil {
		c.pubChannel.Close()
	}
	if c.conn != nil {
		c.conn.Close()
	}
}
