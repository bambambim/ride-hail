package service

import (
	"context"
	"fmt"

	"ride-hail/internal/ride-service/domain"
	"ride-hail/pkg/logger"
)

// CancelRideCommand represents the input for cancelling a ride
type CancelRideCommand struct {
	RideID      string
	PassengerID string
	Reason      string
}

// CancelRideUseCase handles the business workflow for cancelling a ride
type CancelRideUseCase struct {
	rideRepo       domain.RideRepository
	eventPublisher EventPublisher
	logger         logger.Logger
}

// NewCancelRideUseCase creates a new use case instance
func NewCancelRideUseCase(
	rideRepo domain.RideRepository,
	eventPublisher EventPublisher,
	logger logger.Logger,
) *CancelRideUseCase {
	return &CancelRideUseCase{
		rideRepo:       rideRepo,
		eventPublisher: eventPublisher,
		logger:         logger,
	}
}

// Execute runs the use case
func (uc *CancelRideUseCase) Execute(ctx context.Context, cmd CancelRideCommand) error {
	// 1. Retrieve ride and verify ownership
	ride, err := uc.rideRepo.FindByPassenger(ctx, cmd.RideID, cmd.PassengerID)
	if err != nil {
		uc.logger.WithFields(logger.LogFields{
			"ride_id":      cmd.RideID,
			"passenger_id": cmd.PassengerID,
		}).Error("ride_not_found", err)
		return fmt.Errorf("ride not found: %w", err)
	}

	uc.logger.WithFields(logger.LogFields{
		"ride_id":      cmd.RideID,
		"passenger_id": cmd.PassengerID,
		"status":       ride.Status().String(),
	}).Info("ride_retrieved", "Ride retrieved for cancellation")

	// 2. Cancel ride (domain logic)
	if err := ride.Cancel(cmd.Reason); err != nil {
		uc.logger.WithFields(logger.LogFields{
			"ride_id": cmd.RideID,
			"status":  ride.Status().String(),
		}).Error("cancel_ride_failed", err)
		return fmt.Errorf("cannot cancel ride: %w", err)
	}

	// 3. Persist changes
	if err := uc.rideRepo.Update(ctx, ride); err != nil {
		uc.logger.Error("update_ride_failed", err)
		return fmt.Errorf("failed to update ride: %w", err)
	}

	uc.logger.WithFields(logger.LogFields{
		"ride_id": cmd.RideID,
		"reason":  cmd.Reason,
	}).Info("ride_cancelled", "Ride cancelled successfully")

	// 4. Publish cancellation event
	event := domain.RideCancelledEvent{
		RideID:      ride.ID(),
		PassengerID: ride.PassengerID(),
		DriverID:    ride.DriverID(),
		Reason:      cmd.Reason,
		CancelledAt: *ride.CancelledAt(),
	}

	if err := uc.eventPublisher.Publish(ctx, event); err != nil {
		// Log error but don't fail the request
		uc.logger.WithFields(logger.LogFields{
			"ride_id": cmd.RideID,
			"error":   err.Error(),
		}).Error("publish_cancellation_event_failed", err)
	}

	return nil
}
