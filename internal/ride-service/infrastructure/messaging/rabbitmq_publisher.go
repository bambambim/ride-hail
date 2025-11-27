package messaging

import (
	"context"
	"encoding/json"
	"fmt"

	"ride-hail/internal/ride-service/domain"
	"ride-hail/pkg/logger"
	"ride-hail/pkg/rabbitmq"
)

// RabbitMQEventPublisher implements EventPublisher interface
type RabbitMQEventPublisher struct {
	rabbit *rabbitmq.Connection
	logger logger.Logger
}

// NewRabbitMQEventPublisher creates a new RabbitMQ event publisher
func NewRabbitMQEventPublisher(rabbit *rabbitmq.Connection, logger logger.Logger) *RabbitMQEventPublisher {
	return &RabbitMQEventPublisher{
		rabbit: rabbit,
		logger: logger,
	}
}

// Publish publishes a domain event to RabbitMQ
func (p *RabbitMQEventPublisher) Publish(ctx context.Context, event domain.DomainEvent) error {
	// Convert event to message
	message, routingKey := p.eventToMessage(event)
	if message == nil {
		return fmt.Errorf("unsupported event type: %s", event.EventType())
	}

	// Marshal to JSON
	body, err := json.Marshal(message)
	if err != nil {
		return fmt.Errorf("marshal event: %w", err)
	}

	// Publish to ride_topic exchange
	if err := p.rabbit.Publish(ctx, "ride_topic", routingKey, body); err != nil {
		return fmt.Errorf("publish to rabbitmq: %w", err)
	}

	p.logger.WithFields(logger.LogFields{
		"event_type":  event.EventType(),
		"routing_key": routingKey,
	}).Info("event_published", "Domain event published to RabbitMQ")

	return nil
}

// eventToMessage converts domain event to RabbitMQ message
func (p *RabbitMQEventPublisher) eventToMessage(event domain.DomainEvent) (interface{}, string) {
	switch e := event.(type) {
	case domain.RideRequestedEvent:
		return map[string]interface{}{
			"ride_id":      e.RideID,
			"passenger_id": e.PassengerID,
			"pickup_location": map[string]interface{}{
				"latitude":  e.Pickup.Latitude(),
				"longitude": e.Pickup.Longitude(),
				"address":   e.Pickup.Address(),
			},
			"destination_location": map[string]interface{}{
				"latitude":  e.Destination.Latitude(),
				"longitude": e.Destination.Longitude(),
				"address":   e.Destination.Address(),
			},
			"ride_type":      e.RideType.String(),
			"estimated_fare": e.Fare,
			"requested_at":   e.RequestedAt,
		}, fmt.Sprintf("ride.request.%s", e.RideType.String())

	case domain.RideCancelledEvent:
		return map[string]interface{}{
			"ride_id":      e.RideID,
			"passenger_id": e.PassengerID,
			"driver_id":    e.DriverID,
			"status":       "CANCELLED",
			"reason":       e.Reason,
			"cancelled_at": e.CancelledAt,
		}, fmt.Sprintf("ride.cancelled.%s", e.RideID)

	case domain.RideMatchedEvent:
		return map[string]interface{}{
			"ride_id":      e.RideID,
			"passenger_id": e.PassengerID,
			"driver_id":    e.DriverID,
			"status":       "MATCHED",
			"matched_at":   e.MatchedAt,
		}, fmt.Sprintf("ride.matched.%s", e.RideID)

	case domain.RideCompletedEvent:
		return map[string]interface{}{
			"ride_id":      e.RideID,
			"passenger_id": e.PassengerID,
			"driver_id":    e.DriverID,
			"status":       "COMPLETED",
			"final_fare":   e.FinalFare,
			"completed_at": e.CompletedAt,
		}, fmt.Sprintf("ride.completed.%s", e.RideID)

	default:
		return nil, ""
	}
}
