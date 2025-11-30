package domain

import (
	"errors"
	"fmt"
	"time"
)

// Domain errors
var (
	ErrInvalidCoordinates        = errors.New("invalid coordinates")
	ErrCannotAssignDriver        = errors.New("cannot assign driver to ride")
	ErrCannotCancelRide          = errors.New("cannot cancel ride")
	ErrCannotCancelCompletedRide = errors.New("cannot cancel completed ride")
	ErrRideAlreadyMatched        = errors.New("ride already matched with driver")
	ErrInvalidRideType           = errors.New("invalid ride type")
)

// RideStatus represents the state of a ride
type RideStatus string

const (
	StatusRequested  RideStatus = "REQUESTED"
	StatusMatched    RideStatus = "MATCHED"
	StatusEnRoute    RideStatus = "EN_ROUTE"
	StatusArrived    RideStatus = "ARRIVED"
	StatusInProgress RideStatus = "IN_PROGRESS"
	StatusCompleted  RideStatus = "COMPLETED"
	StatusCancelled  RideStatus = "CANCELLED"
)

// String returns string representation of status
func (s RideStatus) String() string {
	return string(s)
}

// IsValid checks if status is valid
func (s RideStatus) IsValid() bool {
	switch s {
	case StatusRequested, StatusMatched, StatusEnRoute, StatusArrived,
		StatusInProgress, StatusCompleted, StatusCancelled:
		return true
	}
	return false
}

// RideType represents the vehicle category
type RideType string

const (
	RideTypeEconomy RideType = "ECONOMY"
	RideTypePremium RideType = "PREMIUM"
	RideTypeLuxury  RideType = "LUXURY"
)

// String returns string representation of ride type
func (rt RideType) String() string {
	return string(rt)
}

// IsValid checks if ride type is valid
func (rt RideType) IsValid() bool {
	switch rt {
	case RideTypeEconomy, RideTypePremium, RideTypeLuxury:
		return true
	}
	return false
}

// Ride is the core domain entity
type Ride struct {
	id             string
	rideNumber     string
	passengerID    string
	driverID       *string
	status         RideStatus
	rideType       RideType
	pickupLocation Coordinate
	destLocation   Coordinate
	estimatedFare  float64
	finalFare      *float64
	requestedAt    time.Time
	matchedAt      *time.Time
	startedAt      *time.Time
	completedAt    *time.Time
	cancelledAt    *time.Time
	cancelReason   string
}

// NewRide creates a new ride with validation
func NewRide(
	passengerID string,
	pickup Coordinate,
	dest Coordinate,
	rideType RideType,
	estimatedFare float64,
	todayRideCount int,
) (*Ride, error) {
	// Validate ride type
	if !rideType.IsValid() {
		return nil, ErrInvalidRideType
	}

	// Validate locations
	if err := pickup.Validate(); err != nil {
		return nil, fmt.Errorf("invalid pickup location: %w", err)
	}
	if err := dest.Validate(); err != nil {
		return nil, fmt.Errorf("invalid destination location: %w", err)
	}

	// Generate ride number
	rideNumber := generateRideNumber(todayRideCount)

	return &Ride{
		passengerID:    passengerID,
		rideNumber:     rideNumber,
		status:         StatusRequested,
		rideType:       rideType,
		pickupLocation: pickup,
		destLocation:   dest,
		estimatedFare:  estimatedFare,
		requestedAt:    time.Now(),
	}, nil
}

// ReconstructRide reconstructs a ride from persistence (used by repository)
func ReconstructRide(
	id string,
	rideNumber string,
	passengerID string,
	driverID *string,
	status RideStatus,
	rideType RideType,
	pickup Coordinate,
	dest Coordinate,
	estimatedFare float64,
	finalFare *float64,
	requestedAt time.Time,
	matchedAt *time.Time,
	startedAt *time.Time,
	completedAt *time.Time,
	cancelledAt *time.Time,
	cancelReason string,
) *Ride {
	return &Ride{
		id:             id,
		rideNumber:     rideNumber,
		passengerID:    passengerID,
		driverID:       driverID,
		status:         status,
		rideType:       rideType,
		pickupLocation: pickup,
		destLocation:   dest,
		estimatedFare:  estimatedFare,
		finalFare:      finalFare,
		requestedAt:    requestedAt,
		matchedAt:      matchedAt,
		startedAt:      startedAt,
		completedAt:    completedAt,
		cancelledAt:    cancelledAt,
		cancelReason:   cancelReason,
	}
}

// Business methods

// AssignDriver assigns a driver to the ride
func (r *Ride) AssignDriver(driverID string) error {
	if r.status != StatusRequested {
		return ErrCannotAssignDriver
	}

	r.driverID = &driverID
	r.status = StatusMatched
	now := time.Now()
	r.matchedAt = &now

	return nil
}

// StartTrip marks the ride as in progress
func (r *Ride) StartTrip() error {
	if r.status != StatusArrived {
		return errors.New("ride must be in ARRIVED status to start")
	}

	r.status = StatusInProgress
	now := time.Now()
	r.startedAt = &now

	return nil
}

// CompleteTrip marks the ride as completed
func (r *Ride) CompleteTrip(finalFare float64) error {
	if r.status != StatusInProgress {
		return errors.New("ride must be in progress to complete")
	}

	r.status = StatusCompleted
	r.finalFare = &finalFare
	now := time.Now()
	r.completedAt = &now

	return nil
}

// Cancel cancels the ride with a reason
func (r *Ride) Cancel(reason string) error {
	if !r.CanBeCancelled() {
		return ErrCannotCancelRide
	}

	r.status = StatusCancelled
	r.cancelReason = reason
	now := time.Now()
	r.cancelledAt = &now

	return nil
}

// UpdateStatus updates the ride status
func (r *Ride) UpdateStatus(newStatus RideStatus) error {
	if !newStatus.IsValid() {
		return errors.New("invalid status")
	}

	r.status = newStatus

	// Set timestamps based on status
	now := time.Now()
	switch newStatus {
	case StatusMatched:
		if r.matchedAt == nil {
			r.matchedAt = &now
		}
	case StatusInProgress:
		if r.startedAt == nil {
			r.startedAt = &now
		}
	case StatusCompleted:
		if r.completedAt == nil {
			r.completedAt = &now
		}
	case StatusCancelled:
		if r.cancelledAt == nil {
			r.cancelledAt = &now
		}
	}

	return nil
}

// Query methods

// CanBeCancelled checks if the ride can be cancelled
func (r *Ride) CanBeCancelled() bool {
	return r.status != StatusCompleted && r.status != StatusCancelled
}

// IsCompleted checks if the ride is completed
func (r *Ride) IsCompleted() bool {
	return r.status == StatusCompleted
}

// IsCancelled checks if the ride is cancelled
func (r *Ride) IsCancelled() bool {
	return r.status == StatusCancelled
}

// IsActive checks if the ride is currently active
func (r *Ride) IsActive() bool {
	return !r.IsCompleted() && !r.IsCancelled()
}

// HasDriver checks if a driver is assigned
func (r *Ride) HasDriver() bool {
	return r.driverID != nil
}

// Getters (encapsulation)

func (r *Ride) ID() string                 { return r.id }
func (r *Ride) RideNumber() string         { return r.rideNumber }
func (r *Ride) PassengerID() string        { return r.passengerID }
func (r *Ride) DriverID() *string          { return r.driverID }
func (r *Ride) Status() RideStatus         { return r.status }
func (r *Ride) RideTypeValue() RideType    { return r.rideType }
func (r *Ride) PickupLocation() Coordinate { return r.pickupLocation }
func (r *Ride) DestLocation() Coordinate   { return r.destLocation }
func (r *Ride) EstimatedFare() float64     { return r.estimatedFare }
func (r *Ride) FinalFare() *float64        { return r.finalFare }
func (r *Ride) RequestedAt() time.Time     { return r.requestedAt }
func (r *Ride) MatchedAt() *time.Time      { return r.matchedAt }
func (r *Ride) StartedAt() *time.Time      { return r.startedAt }
func (r *Ride) CompletedAt() *time.Time    { return r.completedAt }
func (r *Ride) CancelledAt() *time.Time    { return r.cancelledAt }
func (r *Ride) CancelReason() string       { return r.cancelReason }

// SetID sets the ride ID (used after persistence)
func (r *Ride) SetID(id string) {
	r.id = id
}

// Helper functions

// generateRideNumber generates a unique ride number in format RIDE_YYYYMMDD_XXX
func generateRideNumber(todayRideCount int) string {
	today := time.Now().Format("20060102")
	counter := todayRideCount + 1
	return fmt.Sprintf("RIDE_%s_%03d", today, counter)
}

// ValidateRideType validates a ride type string
func ValidateRideType(rideType string) error {
	rt := RideType(rideType)
	if !rt.IsValid() {
		return ErrInvalidRideType
	}
	return nil
}

// CanCancelRide checks if a ride with given status can be cancelled
func CanCancelRide(status RideStatus) bool {
	return status != StatusCompleted && status != StatusCancelled
}
