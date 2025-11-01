package domain

import (
	"time"
)

// DriverStatus represents the current status of a driver
type DriverStatus string

const (
	DriverStatusOffline   DriverStatus = "OFFLINE"
	DriverStatusAvailable DriverStatus = "AVAILABLE"
	DriverStatusBusy      DriverStatus = "BUSY"
	DriverStatusEnRoute   DriverStatus = "EN_ROUTE"
)

// Driver represents a driver in the system
type Driver struct {
	ID             string         `json:"id"`
	CreatedAt      time.Time      `json:"created_at"`
	UpdatedAt      time.Time      `json:"updated_at"`
	LicenseNumber  string         `json:"license_number"`
	VehicleType    string         `json:"vehicle_type"`
	VehicleAttrs   VehicleAttrs   `json:"vehicle_attrs"`
	Rating         float64        `json:"rating"`
	TotalRides     int            `json:"total_rides"`
	TotalEarnings  float64        `json:"total_earnings"`
	Status         DriverStatus   `json:"status"`
	IsVerified     bool           `json:"is_verified"`
	Email          string         `json:"email,omitempty"`
	CurrentSession *DriverSession `json:"current_session,omitempty"`
}

// VehicleAttrs contains vehicle information
type VehicleAttrs struct {
	Make  string `json:"vehicle_make"`
	Model string `json:"vehicle_model"`
	Color string `json:"vehicle_color"`
	Plate string `json:"vehicle_plate"`
	Year  int    `json:"vehicle_year"`
}

// DriverSession tracks a driver's online session
type DriverSession struct {
	ID            string     `json:"id"`
	DriverID      string     `json:"driver_id"`
	StartedAt     time.Time  `json:"started_at"`
	EndedAt       *time.Time `json:"ended_at,omitempty"`
	TotalRides    int        `json:"total_rides"`
	TotalEarnings float64    `json:"total_earnings"`
}

// Location represents a geographic location
type Location struct {
	Latitude  float64 `json:"latitude"`
	Longitude float64 `json:"longitude"`
	Address   string  `json:"address,omitempty"`
}

// Coordinate represents a coordinate entry in the database
type Coordinate struct {
	ID         string    `json:"id"`
	CreatedAt  time.Time `json:"created_at"`
	UpdatedAt  time.Time `json:"updated_at"`
	EntityID   string    `json:"entity_id"`
	EntityType string    `json:"entity_type"`
	Address    string    `json:"address"`
	Latitude   float64   `json:"latitude"`
	Longitude  float64   `json:"longitude"`
	FareAmount *float64  `json:"fare_amount,omitempty"`
	DistanceKm *float64  `json:"distance_km,omitempty"`
	IsCurrent  bool      `json:"is_current"`
}

// LocationUpdate represents a location update request
type LocationUpdate struct {
	DriverID       string    `json:"driver_id"`
	Latitude       float64   `json:"latitude"`
	Longitude      float64   `json:"longitude"`
	AccuracyMeters *float64  `json:"accuracy_meters,omitempty"`
	SpeedKmh       *float64  `json:"speed_kmh,omitempty"`
	HeadingDegrees *float64  `json:"heading_degrees,omitempty"`
	RideID         *string   `json:"ride_id,omitempty"`
	Timestamp      time.Time `json:"timestamp"`
}

// LocationHistory represents historical location data
type LocationHistory struct {
	ID             string    `json:"id"`
	CoordinateID   *string   `json:"coordinate_id,omitempty"`
	DriverID       string    `json:"driver_id"`
	Latitude       float64   `json:"latitude"`
	Longitude      float64   `json:"longitude"`
	AccuracyMeters *float64  `json:"accuracy_meters,omitempty"`
	SpeedKmh       *float64  `json:"speed_kmh,omitempty"`
	HeadingDegrees *float64  `json:"heading_degrees,omitempty"`
	RecordedAt     time.Time `json:"recorded_at"`
	RideID         *string   `json:"ride_id,omitempty"`
}

// NearbyDriver represents a driver found near a location
type NearbyDriver struct {
	DriverID   string       `json:"driver_id"`
	Email      string       `json:"email"`
	Rating     float64      `json:"rating"`
	Location   Location     `json:"location"`
	DistanceKm float64      `json:"distance_km"`
	Vehicle    VehicleAttrs `json:"vehicle"`
}

// RideRequest represents a ride request from the matching service
type RideRequest struct {
	RideID              string   `json:"ride_id"`
	RideNumber          string   `json:"ride_number"`
	PickupLocation      Location `json:"pickup_location"`
	DestinationLocation Location `json:"destination_location"`
	RideType            string   `json:"ride_type"`
	EstimatedFare       float64  `json:"estimated_fare"`
	MaxDistanceKm       float64  `json:"max_distance_km"`
	TimeoutSeconds      int      `json:"timeout_seconds"`
	CorrelationID       string   `json:"correlation_id"`
}

// RideOffer represents a ride offer sent to a driver
type RideOffer struct {
	Type                         string    `json:"type"`
	OfferID                      string    `json:"offer_id"`
	RideID                       string    `json:"ride_id"`
	RideNumber                   string    `json:"ride_number"`
	PickupLocation               Location  `json:"pickup_location"`
	DestinationLocation          Location  `json:"destination_location"`
	EstimatedFare                float64   `json:"estimated_fare"`
	DriverEarnings               float64   `json:"driver_earnings"`
	DistanceToPickupKm           float64   `json:"distance_to_pickup_km"`
	EstimatedRideDurationMinutes int       `json:"estimated_ride_duration_minutes"`
	ExpiresAt                    time.Time `json:"expires_at"`
}

// RideResponse represents a driver's response to a ride offer
type RideResponse struct {
	Type            string   `json:"type"`
	OfferID         string   `json:"offer_id"`
	RideID          string   `json:"ride_id"`
	Accepted        bool     `json:"accepted"`
	CurrentLocation Location `json:"current_location"`
}

// DriverMatchResponse represents a response to the ride matching service
type DriverMatchResponse struct {
	RideID                  string     `json:"ride_id"`
	DriverID                string     `json:"driver_id"`
	Accepted                bool       `json:"accepted"`
	EstimatedArrivalMinutes int        `json:"estimated_arrival_minutes"`
	DriverLocation          Location   `json:"driver_location"`
	DriverInfo              DriverInfo `json:"driver_info"`
	CorrelationID           string     `json:"correlation_id"`
}

// DriverInfo contains public driver information
type DriverInfo struct {
	Name    string       `json:"name"`
	Rating  float64      `json:"rating"`
	Vehicle VehicleAttrs `json:"vehicle"`
}

// DriverStatusUpdate represents a status update message
type DriverStatusUpdate struct {
	DriverID  string       `json:"driver_id"`
	Status    DriverStatus `json:"status"`
	RideID    *string      `json:"ride_id,omitempty"`
	Timestamp time.Time    `json:"timestamp"`
}

// LocationBroadcast represents a location update broadcast
type LocationBroadcast struct {
	DriverID       string    `json:"driver_id"`
	RideID         *string   `json:"ride_id,omitempty"`
	Location       Location  `json:"location"`
	SpeedKmh       *float64  `json:"speed_kmh,omitempty"`
	HeadingDegrees *float64  `json:"heading_degrees,omitempty"`
	Timestamp      time.Time `json:"timestamp"`
}

// OnlineRequest represents a request to go online
type OnlineRequest struct {
	Latitude  float64 `json:"latitude" validate:"required,min=-90,max=90"`
	Longitude float64 `json:"longitude" validate:"required,min=-180,max=180"`
}

// OnlineResponse represents the response when a driver goes online
type OnlineResponse struct {
	Status    DriverStatus `json:"status"`
	SessionID string       `json:"session_id"`
	Message   string       `json:"message"`
}

// OfflineResponse represents the response when a driver goes offline
type OfflineResponse struct {
	Status         DriverStatus   `json:"status"`
	SessionID      string         `json:"session_id"`
	SessionSummary SessionSummary `json:"session_summary"`
	Message        string         `json:"message"`
}

// SessionSummary contains summary of a driver's session
type SessionSummary struct {
	DurationHours  float64 `json:"duration_hours"`
	RidesCompleted int     `json:"rides_completed"`
	Earnings       float64 `json:"earnings"`
}

// UpdateLocationRequest represents a location update request
type UpdateLocationRequest struct {
	Latitude       float64  `json:"latitude" validate:"required,min=-90,max=90"`
	Longitude      float64  `json:"longitude" validate:"required,min=-180,max=180"`
	AccuracyMeters *float64 `json:"accuracy_meters,omitempty"`
	SpeedKmh       *float64 `json:"speed_kmh,omitempty"`
	HeadingDegrees *float64 `json:"heading_degrees,omitempty"`
}

// UpdateLocationResponse represents the response to a location update
type UpdateLocationResponse struct {
	CoordinateID string    `json:"coordinate_id"`
	UpdatedAt    time.Time `json:"updated_at"`
}

// StartRideRequest represents a request to start a ride
type StartRideRequest struct {
	RideID         string   `json:"ride_id" validate:"required"`
	DriverLocation Location `json:"driver_location" validate:"required"`
}

// StartRideResponse represents the response when starting a ride
type StartRideResponse struct {
	RideID    string       `json:"ride_id"`
	Status    DriverStatus `json:"status"`
	StartedAt time.Time    `json:"started_at"`
	Message   string       `json:"message"`
}

// CompleteRideRequest represents a request to complete a ride
type CompleteRideRequest struct {
	RideID                string   `json:"ride_id" validate:"required"`
	FinalLocation         Location `json:"final_location" validate:"required"`
	ActualDistanceKm      float64  `json:"actual_distance_km" validate:"required,gt=0"`
	ActualDurationMinutes int      `json:"actual_duration_minutes" validate:"required,gt=0"`
}

// CompleteRideResponse represents the response when completing a ride
type CompleteRideResponse struct {
	RideID         string       `json:"ride_id"`
	Status         DriverStatus `json:"status"`
	CompletedAt    time.Time    `json:"completed_at"`
	DriverEarnings float64      `json:"driver_earnings"`
	Message        string       `json:"message"`
}

// RideStatusUpdate represents a ride status update from the ride service
type RideStatusUpdate struct {
	RideID        string    `json:"ride_id"`
	Status        string    `json:"status"`
	Timestamp     time.Time `json:"timestamp"`
	FinalFare     *float64  `json:"final_fare,omitempty"`
	CorrelationID string    `json:"correlation_id"`
}

// WebSocketMessage represents a generic WebSocket message
type WebSocketMessage struct {
	Type    string      `json:"type"`
	Payload interface{} `json:"payload,omitempty"`
}

// AuthMessage represents an authentication message via WebSocket
type AuthMessage struct {
	Type  string `json:"type"`
	Token string `json:"token"`
}

// RideDetails represents detailed ride information sent after acceptance
type RideDetails struct {
	Type           string   `json:"type"`
	RideID         string   `json:"ride_id"`
	PassengerName  string   `json:"passenger_name"`
	PassengerPhone string   `json:"passenger_phone"`
	PickupLocation Location `json:"pickup_location"`
	PickupNotes    string   `json:"pickup_notes,omitempty"`
}

// ErrorResponse represents an error response
type ErrorResponse struct {
	Error   string                 `json:"error"`
	Code    string                 `json:"code,omitempty"`
	Details map[string]interface{} `json:"details,omitempty"`
}
