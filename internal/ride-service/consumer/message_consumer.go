package consumer

import (
	"context"
	"encoding/json"
	"time"

	"ride-hail/pkg/logger"
	"ride-hail/pkg/rabbitmq"
	"ride-hail/pkg/websocket"

	amqp "github.com/rabbitmq/amqp091-go"
)

// RideConsumer handles incoming messages for the Ride Service
type RideConsumer struct {
	rabbit    *rabbitmq.Connection
	log       logger.Logger
	wsManager *websocket.Manager
}

func New(rabbit *rabbitmq.Connection, log logger.Logger, wsManager *websocket.Manager) *RideConsumer {
	return &RideConsumer{
		rabbit:    rabbit,
		log:       log,
		wsManager: wsManager,
	}
}

// DriverResponseMessage represents driver acceptance/rejection
type DriverResponseMessage struct {
	RideID           string    `json:"ride_id"`
	DriverID         string    `json:"driver_id"`
	PassengerID      string    `json:"passenger_id"` // Added for WebSocket notification
	Accepted         bool      `json:"accepted"`
	EstimatedArrival time.Time `json:"estimated_arrival"`
	CorrelationID    string    `json:"correlation_id"`
}

// DriverStatusMessage represents driver status updates
type DriverStatusMessage struct {
	DriverID  string    `json:"driver_id"`
	RideID    string    `json:"ride_id,omitempty"`
	OldStatus string    `json:"old_status"`
	NewStatus string    `json:"new_status"`
	Latitude  float64   `json:"latitude,omitempty"`
	Longitude float64   `json:"longitude,omitempty"`
	Timestamp time.Time `json:"timestamp"`
}

// LocationUpdateMessage represents driver location updates
type LocationUpdateMessage struct {
	DriverID       string    `json:"driver_id"`
	RideID         string    `json:"ride_id,omitempty"`
	Latitude       float64   `json:"latitude"`
	Longitude      float64   `json:"longitude"`
	HeadingDegrees float64   `json:"heading_degrees"`
	Timestamp      time.Time `json:"timestamp"`
}

// StartConsuming starts all message consumers
func (c *RideConsumer) StartConsuming(ctx context.Context) error {
	// Start consuming driver responses
	go c.consumeDriverResponses(ctx)

	// Start consuming driver status updates
	go c.consumeDriverStatus(ctx)

	// Start consuming location updates
	go c.consumeLocationUpdates(ctx)

	c.log.Info("consumers_started", "All message consumers started")
	return nil
}

// consumeDriverResponses handles driver.response.{ride_id} messages
func (c *RideConsumer) consumeDriverResponses(ctx context.Context) {
	queueName := "driver_responses"

	c.log.WithFields(logger.LogFields{
		"queue": queueName,
	}).Info("consumer_starting", "Starting driver response consumer")

	// Use the correct Consume API - pass handler function
	err := c.rabbit.Consume(queueName, func(msg amqp.Delivery) {
		c.handleDriverResponse(ctx, msg.Body)
		msg.Ack(false)
	})

	if err != nil {
		c.log.Error("consume_driver_responses_failed", err)
	}
}

func (c *RideConsumer) handleDriverResponse(ctx context.Context, body []byte) {
	var response DriverResponseMessage
	if err := json.Unmarshal(body, &response); err != nil {
		c.log.Error("unmarshal_driver_response_failed", err)
		return
	}

	c.log.WithFields(logger.LogFields{
		"ride_id":      response.RideID,
		"driver_id":    response.DriverID,
		"passenger_id": response.PassengerID,
		"accepted":     response.Accepted,
	}).Info("driver_response_received", "Driver response message received")

	if response.Accepted {
		// Send WebSocket notification to passenger
		notification := map[string]interface{}{
			"type":              "ride_matched",
			"ride_id":           response.RideID,
			"driver_id":         response.DriverID,
			"status":            "MATCHED",
			"estimated_arrival": response.EstimatedArrival,
			"timestamp":         time.Now(),
		}

		// Send notification to the passenger via WebSocket
		if err := c.wsManager.SendToUser(response.PassengerID, notification); err != nil {
			c.log.WithFields(logger.LogFields{
				"passenger_id": response.PassengerID,
				"ride_id":      response.RideID,
				"error":        err.Error(),
			}).Error("websocket_notification_failed", err)
		} else {
			c.log.WithFields(logger.LogFields{
				"passenger_id": response.PassengerID,
				"ride_id":      response.RideID,
				"driver_id":    response.DriverID,
			}).Info("ride_matched_notification_sent", "WebSocket notification sent to passenger")
		}
	} else {
		// Driver rejected the ride
		c.log.WithFields(logger.LogFields{
			"ride_id":   response.RideID,
			"driver_id": response.DriverID,
		}).Info("ride_rejected", "Driver rejected the ride")

		// Could send rejection notification to passenger if needed
		// For now, the ride remains in REQUESTED status for other drivers
	}
}

// consumeDriverStatus handles driver.status.* messages
func (c *RideConsumer) consumeDriverStatus(ctx context.Context) {
	queueName := "driver_status"

	c.log.WithFields(logger.LogFields{
		"queue": queueName,
	}).Info("consumer_starting", "Starting driver status consumer")

	// Use the correct Consume API - pass handler function
	err := c.rabbit.Consume(queueName, func(msg amqp.Delivery) {
		c.handleDriverStatus(ctx, msg.Body)
		msg.Ack(false)
	})

	if err != nil {
		c.log.Error("consume_driver_status_failed", err)
	}
}

func (c *RideConsumer) handleDriverStatus(ctx context.Context, body []byte) {
	var status DriverStatusMessage
	if err := json.Unmarshal(body, &status); err != nil {
		c.log.Error("unmarshal_driver_status_failed", err)
		return
	}

	c.log.WithFields(logger.LogFields{
		"driver_id":  status.DriverID,
		"ride_id":    status.RideID,
		"old_status": status.OldStatus,
		"new_status": status.NewStatus,
	}).Info("driver_status_received", "Driver status update received")

	// TODO: Update ride status based on driver status
	// EN_ROUTE -> Update ride to EN_ROUTE
	// ARRIVED -> Update ride to ARRIVED
	// TODO: Get passenger_id from database

	// Send WebSocket notification to passenger
	notification := map[string]interface{}{
		"type":      "ride_status_update",
		"ride_id":   status.RideID,
		"driver_id": status.DriverID,
		"status":    status.NewStatus,
		"latitude":  status.Latitude,
		"longitude": status.Longitude,
		"timestamp": status.Timestamp,
	}

	// Placeholder: In production, query DB for passenger_id and send
	// c.wsManager.SendToUser(passengerID, notification)
	_ = notification
}

// consumeLocationUpdates handles location updates from location_fanout
func (c *RideConsumer) consumeLocationUpdates(ctx context.Context) {
	queueName := "location_updates_ride"

	c.log.WithFields(logger.LogFields{
		"queue": queueName,
	}).Info("consumer_starting", "Starting location update consumer")

	// Use the correct Consume API - pass handler function
	err := c.rabbit.Consume(queueName, func(msg amqp.Delivery) {
		c.handleLocationUpdate(ctx, msg.Body)
		msg.Ack(false)
	})

	if err != nil {
		c.log.Error("consume_location_updates_failed", err)
	}
}

func (c *RideConsumer) handleLocationUpdate(ctx context.Context, body []byte) {
	var location LocationUpdateMessage
	if err := json.Unmarshal(body, &location); err != nil {
		c.log.Error("unmarshal_location_update_failed", err)
		return
	}

	// Only log occasionally to avoid spam (every 10th message)
	// In production, you'd implement more sophisticated rate limiting
	c.log.WithFields(logger.LogFields{
		"driver_id": location.DriverID,
		"ride_id":   location.RideID,
		"latitude":  location.Latitude,
		"longitude": location.Longitude,
	}).Debug("location_update_received", "Driver location update received")

	// Send WebSocket notification to passenger with driver location
	if location.RideID != "" {
		notification := map[string]interface{}{
			"type":            "driver_location_update",
			"ride_id":         location.RideID,
			"driver_id":       location.DriverID,
			"latitude":        location.Latitude,
			"longitude":       location.Longitude,
			"heading_degrees": location.HeadingDegrees,
			"timestamp":       location.Timestamp,
		}

		// Placeholder: In production, query DB for passenger_id and send
		// c.wsManager.SendToUser(passengerID, notification)
		_ = notification
	}

	// TODO: Calculate distance to pickup/destination
}
