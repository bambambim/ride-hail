package consumer

import (
	"context"
	"encoding/json"
	"time"

	"ride-hail/internal/ride-service/domain"
	"ride-hail/internal/ride-service/infrastructure/repository"
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
	repo      *repository.PostgresRideRepository
}

func New(rabbit *rabbitmq.Connection, log logger.Logger, wsManager *websocket.Manager, repo *repository.PostgresRideRepository) *RideConsumer {
	return &RideConsumer{
		rabbit:    rabbit,
		log:       log,
		wsManager: wsManager,
		repo:      repo,
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
	DriverID    string    `json:"driver_id"`
	RideID      string    `json:"ride_id,omitempty"`
	PassengerID string    `json:"passenger_id,omitempty"` // Added for WebSocket notification
	OldStatus   string    `json:"old_status"`
	NewStatus   string    `json:"new_status"`
	Latitude    float64   `json:"latitude,omitempty"`
	Longitude   float64   `json:"longitude,omitempty"`
	Timestamp   time.Time `json:"timestamp"`
}

// LocationUpdateMessage represents driver location updates
type LocationUpdateMessage struct {
	DriverID       string    `json:"driver_id"`
	RideID         string    `json:"ride_id,omitempty"`
	PassengerID    string    `json:"passenger_id"` // Added for WebSocket notification
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
		// Update ride status to MATCHED in database
		if err := c.repo.UpdateRideStatus(ctx, response.RideID, "MATCHED"); err != nil {
			c.log.WithFields(logger.LogFields{
				"ride_id": response.RideID,
				"error":   err.Error(),
			}).Error("update_ride_status_failed", err)
			// Continue with WebSocket notification even if DB update fails
		}

		// Assign driver to the ride
		if err := c.repo.AssignDriver(ctx, response.RideID, response.DriverID); err != nil {
			c.log.WithFields(logger.LogFields{
				"ride_id":   response.RideID,
				"driver_id": response.DriverID,
				"error":     err.Error(),
			}).Error("assign_driver_failed", err)
			// Continue with WebSocket notification even if assignment fails
		}

		// Save DRIVER_MATCHED event to ride_events table
		matchedEvent := domain.RideMatchedEvent{
			RideID:      response.RideID,
			PassengerID: response.PassengerID,
			DriverID:    response.DriverID,
			MatchedAt:   time.Now(),
		}
		if err := c.repo.SaveEvent(ctx, response.RideID, matchedEvent); err != nil {
			c.log.WithFields(logger.LogFields{
				"ride_id": response.RideID,
				"error":   err.Error(),
			}).Error("save_matched_event_failed", err)
		} else {
			c.log.WithFields(logger.LogFields{
				"ride_id":    response.RideID,
				"event_type": "DRIVER_MATCHED",
			}).Info("event_saved", "DRIVER_MATCHED event saved to ride_events")
		}

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
		"driver_id":    status.DriverID,
		"passenger_id": status.PassengerID,
		"ride_id":      status.RideID,
		"old_status":   status.OldStatus,
		"new_status":   status.NewStatus,
	}).Info("driver_status_received", "Driver status update received")

	// Update ride status in database based on driver status
	var rideStatus string
	switch status.NewStatus {
	case "EN_ROUTE":
		rideStatus = "EN_ROUTE"
	case "ARRIVED":
		rideStatus = "ARRIVED"
	case "STARTED", "IN_PROGRESS":
		rideStatus = "IN_PROGRESS"
	case "COMPLETED":
		rideStatus = "COMPLETED"
	case "MATCHED":
		rideStatus = "MATCHED"
	case "REQUESTED":
		rideStatus = "REQUESTED"
	case "CANCELLED":
		rideStatus = "CANCELLED"
	default:
		// Unknown or empty status, log and skip DB update
		c.log.WithFields(logger.LogFields{
			"status": status.NewStatus,
		}).Info("unknown_driver_status", "Unknown or empty driver status received, skipping DB update")
		// Don't update the database with invalid status
		rideStatus = ""
	}

	// Update ride status in database if we have a valid ride_id and status
	if status.RideID != "" && rideStatus != "" {
		if err := c.repo.UpdateRideStatus(ctx, status.RideID, rideStatus); err != nil {
			c.log.WithFields(logger.LogFields{
				"ride_id": status.RideID,
				"status":  rideStatus,
				"error":   err.Error(),
			}).Error("update_ride_status_failed", err)
			// Continue with WebSocket notification even if DB update fails
		}

		// Save status change event to ride_events table
		statusEvent := domain.RideStatusChangedEvent{
			RideID:    status.RideID,
			OldStatus: domain.RideStatus(status.OldStatus),
			NewStatus: domain.RideStatus(rideStatus),
			ChangedAt: time.Now(),
		}
		if err := c.repo.SaveEvent(ctx, status.RideID, statusEvent); err != nil {
			c.log.WithFields(logger.LogFields{
				"ride_id": status.RideID,
				"error":   err.Error(),
			}).Error("save_status_event_failed", err)
		} else {
			c.log.WithFields(logger.LogFields{
				"ride_id":    status.RideID,
				"old_status": status.OldStatus,
				"new_status": rideStatus,
			}).Info("event_saved", "STATUS_CHANGED event saved to ride_events")
		}

		// If COMPLETED, also save RideCompletedEvent
		if rideStatus == "COMPLETED" {
			completedEvent := domain.RideCompletedEvent{
				RideID:      status.RideID,
				PassengerID: status.PassengerID,
				DriverID:    status.DriverID,
				FinalFare:   0, // TODO: Get final fare from message or calculate
				CompletedAt: time.Now(),
			}
			if err := c.repo.SaveEvent(ctx, status.RideID, completedEvent); err != nil {
				c.log.WithFields(logger.LogFields{
					"ride_id": status.RideID,
					"error":   err.Error(),
				}).Error("save_completed_event_failed", err)
			}
		}
	}

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

	// Send notification to passenger via WebSocket
	if status.PassengerID != "" {
		if err := c.wsManager.SendToUser(status.PassengerID, notification); err != nil {
			c.log.WithFields(logger.LogFields{
				"passenger_id": status.PassengerID,
				"ride_id":      status.RideID,
				"error":        err.Error(),
			}).Error("websocket_status_notification_failed", err)
		} else {
			c.log.WithFields(logger.LogFields{
				"passenger_id": status.PassengerID,
				"ride_id":      status.RideID,
				"status":       status.NewStatus,
			}).Info("status_notification_sent", "Status update sent via WebSocket")
		}
	}
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

	// Only log occasionally to avoid spam (debug level)
	c.log.WithFields(logger.LogFields{
		"driver_id":    location.DriverID,
		"passenger_id": location.PassengerID,
		"ride_id":      location.RideID,
		"latitude":     location.Latitude,
		"longitude":    location.Longitude,
	}).Debug("location_update_received", "Driver location update received")

	// Send WebSocket notification to passenger with driver location
	if location.RideID != "" && location.PassengerID != "" {
		notification := map[string]interface{}{
			"type":            "driver_location_update",
			"ride_id":         location.RideID,
			"driver_id":       location.DriverID,
			"latitude":        location.Latitude,
			"longitude":       location.Longitude,
			"heading_degrees": location.HeadingDegrees,
			"timestamp":       location.Timestamp,
		}

		// Send notification to passenger via WebSocket
		if err := c.wsManager.SendToUser(location.PassengerID, notification); err != nil {
			c.log.WithFields(logger.LogFields{
				"passenger_id": location.PassengerID,
				"ride_id":      location.RideID,
				"error":        err.Error(),
			}).Debug("websocket_location_notification_failed", "Failed to send location update") // Debug to avoid spam
		}
		// No success log for location updates to avoid spam
	}

	// TODO: Calculate distance to pickup/destination and send arrival estimates
}
