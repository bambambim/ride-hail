package service

import (
	"context"
	"crypto/rand"
	"fmt"
	"time"

	"ride-hail/internal/ride-service/domain"
	"ride-hail/pkg/logger"
)

// generateUUID generates a UUID v4 string using crypto/rand
func generateUUID() string {
	b := make([]byte, 16)
	rand.Read(b)
	b[6] = (b[6] & 0x0f) | 0x40 // Version 4
	b[8] = (b[8] & 0x3f) | 0x80 // Variant RFC4122
	return fmt.Sprintf("%x-%x-%x-%x-%x", b[0:4], b[4:6], b[6:8], b[8:10], b[10:])
}

// CreateRideCommand represents the input for creating a ride
type CreateRideCommand struct {
	PassengerID          string
	PickupLatitude       float64
	PickupLongitude      float64
	PickupAddress        string
	DestinationLatitude  float64
	DestinationLongitude float64
	DestinationAddress   string
	RideType             string
}

// RideDTO represents the output data transfer object
type RideDTO struct {
	ID            string  `json:"id"`
	RideNumber    string  `json:"ride_number"`
	PassengerID   string  `json:"passenger_id"`
	Status        string  `json:"status"`
	RideType      string  `json:"ride_type"`
	EstimatedFare float64 `json:"estimated_fare"`
	RequestedAt   string  `json:"requested_at"`
}

// EventPublisher is the interface for publishing domain events
type EventPublisher interface {
	Publish(ctx context.Context, event domain.DomainEvent) error
}

// CreateRideUseCase handles the business workflow for creating a ride
type CreateRideUseCase struct {
	rideRepo       domain.RideRepository
	eventPublisher EventPublisher
	fareCalculator *domain.FareCalculator
	logger         logger.Logger
}

// NewCreateRideUseCase creates a new use case instance
func NewCreateRideUseCase(
	rideRepo domain.RideRepository,
	eventPublisher EventPublisher,
	fareCalculator *domain.FareCalculator,
	logger logger.Logger,
) *CreateRideUseCase {
	return &CreateRideUseCase{
		rideRepo:       rideRepo,
		eventPublisher: eventPublisher,
		fareCalculator: fareCalculator,
		logger:         logger,
	}
}

// Execute runs the use case
func (uc *CreateRideUseCase) Execute(ctx context.Context, cmd CreateRideCommand) (*RideDTO, error) {
	// 1. Validate and create pickup coordinate
	pickup, err := domain.NewCoordinate(
		cmd.PickupLatitude,
		cmd.PickupLongitude,
		cmd.PickupAddress,
	)
	if err != nil {
		uc.logger.Error("invalid_pickup_coordinate", err)
		return nil, fmt.Errorf("invalid pickup location: %w", err)
	}

	// 2. Validate and create destination coordinate
	dest, err := domain.NewCoordinate(
		cmd.DestinationLatitude,
		cmd.DestinationLongitude,
		cmd.DestinationAddress,
	)
	if err != nil {
		uc.logger.Error("invalid_destination_coordinate", err)
		return nil, fmt.Errorf("invalid destination location: %w", err)
	}

	// 3. Validate ride type
	rideType := domain.RideType(cmd.RideType)
	if !rideType.IsValid() {
		uc.logger.Error("invalid_ride_type", domain.ErrInvalidRideType)
		return nil, domain.ErrInvalidRideType
	}

	// 4. Calculate estimated fare using domain service
	estimatedFare := uc.fareCalculator.Calculate(pickup, dest, rideType)

	uc.logger.WithFields(logger.LogFields{
		"passenger_id":   cmd.PassengerID,
		"ride_type":      cmd.RideType,
		"estimated_fare": estimatedFare,
	}).Info("fare_calculated", "Estimated fare calculated")

	// 5. Create ride domain entity
	ride, err := domain.NewRide(
		cmd.PassengerID,
		pickup,
		dest,
		rideType,
		estimatedFare,
	)
	if err != nil {
		uc.logger.Error("create_ride_entity_failed", err)
		return nil, fmt.Errorf("failed to create ride: %w", err)
	}

	// 6. Generate and set ride ID
	rideID := generateUUID()
	ride.SetID(rideID)

	uc.logger.WithFields(logger.LogFields{
		"ride_id":      rideID,
		"passenger_id": cmd.PassengerID,
	}).Info("ride_entity_created", "Ride entity created")

	// 7. Persist ride (infrastructure layer)
	if err := uc.rideRepo.Save(ctx, ride); err != nil {
		uc.logger.Error("save_ride_failed", err)
		return nil, fmt.Errorf("failed to save ride: %w", err)
	}

	uc.logger.WithFields(logger.LogFields{
		"ride_id": rideID,
	}).Info("ride_persisted", "Ride saved to database")

	// 8. Publish domain event (for async processing)
	event := domain.RideRequestedEvent{
		RideID:      ride.ID(),
		PassengerID: ride.PassengerID(),
		Pickup:      ride.PickupLocation(),
		Destination: ride.DestLocation(),
		RideType:    ride.RideTypeValue(),
		Fare:        ride.EstimatedFare(),
		RequestedAt: ride.RequestedAt(),
	}

	if err := uc.eventPublisher.Publish(ctx, event); err != nil {
		// Log error but don't fail the request - ride is already saved
		uc.logger.WithFields(logger.LogFields{
			"ride_id": rideID,
			"error":   err.Error(),
		}).Error("publish_event_failed", err)
	} else {
		uc.logger.WithFields(logger.LogFields{
			"ride_id":    rideID,
			"event_type": event.EventType(),
		}).Info("event_published", "Domain event published")
	}

	// 9. Return DTO
	return toRideDTO(ride), nil
}

// toRideDTO converts domain entity to DTO
func toRideDTO(ride *domain.Ride) *RideDTO {
	return &RideDTO{
		ID:            ride.ID(),
		RideNumber:    ride.RideNumber(),
		PassengerID:   ride.PassengerID(),
		Status:        ride.Status().String(),
		RideType:      ride.RideTypeValue().String(),
		EstimatedFare: ride.EstimatedFare(),
		RequestedAt:   ride.RequestedAt().Format(time.RFC3339),
	}
}
