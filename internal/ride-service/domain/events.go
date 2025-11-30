package domain

import "time"

// DomainEvent is the interface for all domain events
type DomainEvent interface {
	EventType() string
	OccurredAt() time.Time
}

// RideRequestedEvent is raised when a new ride is requested
type RideRequestedEvent struct {
	RideID      string
	PassengerID string
	Pickup      Coordinate
	Destination Coordinate
	RideType    RideType
	Fare        float64
	RequestedAt time.Time
}

func (e RideRequestedEvent) EventType() string {
	return "ride.requested"
}

func (e RideRequestedEvent) OccurredAt() time.Time {
	return e.RequestedAt
}

// RideMatchedEvent is raised when a driver is assigned to a ride
type RideMatchedEvent struct {
	RideID      string
	PassengerID string
	DriverID    string
	MatchedAt   time.Time
}

func (e RideMatchedEvent) EventType() string {
	return "ride.matched"
}

func (e RideMatchedEvent) OccurredAt() time.Time {
	return e.MatchedAt
}

// RideCancelledEvent is raised when a ride is cancelled
type RideCancelledEvent struct {
	RideID      string
	PassengerID string
	DriverID    *string
	Reason      string
	CancelledAt time.Time
}

func (e RideCancelledEvent) EventType() string {
	return "ride.cancelled"
}

func (e RideCancelledEvent) OccurredAt() time.Time {
	return e.CancelledAt
}

// RideCompletedEvent is raised when a ride is completed
type RideCompletedEvent struct {
	RideID      string
	PassengerID string
	DriverID    string
	FinalFare   float64
	CompletedAt time.Time
}

func (e RideCompletedEvent) EventType() string {
	return "ride.completed"
}

func (e RideCompletedEvent) OccurredAt() time.Time {
	return e.CompletedAt
}

// RideStatusChangedEvent is raised when ride status changes
type RideStatusChangedEvent struct {
	RideID    string
	OldStatus RideStatus
	NewStatus RideStatus
	ChangedAt time.Time
}

func (e RideStatusChangedEvent) EventType() string {
	return "ride.status.changed"
}

func (e RideStatusChangedEvent) OccurredAt() time.Time {
	return e.ChangedAt
}
