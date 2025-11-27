package domain

import "time"

// Driver represents a driver in the system
type Driver struct {
	ID             string
	Email          string
	LicenseNumber  string
	VehicleType    string
	VehicleAttrs   map[string]interface{}
	Rating         float64
	TotalRides     int
	TotalEarnings  float64
	Status         string
	IsVerified     bool
	CurrentRideID  string
	CurrentSession *DriverSession
}

// DriverSession tracks online/offline times
type DriverSession struct {
	ID            string
	DriverID      string
	StartedAt     time.Time
	EndedAt       *time.Time
	TotalRides    int
	TotalEarnings float64
}

// Coordinate represents a location point
type Coordinate struct {
	ID              string
	EntityID        string
	EntityType      string
	Address         string
	Latitude        float64
	Longitude       float64
	FareAmount      *float64
	DistanceKM      *float64
	DurationMinutes *int
	IsCurrent       bool
	CreatedAt       time.Time
	UpdatedAt       time.Time
}

// Location represents a latitude/longitude pair with optional address metadata.
type Location struct {
	Lat     float64 `json:"lat"`
	Lng     float64 `json:"lng"`
	Address string  `json:"address,omitempty"`
}

// RideMatchingRequest is the payload emitted by the ride service when a driver
// match must be performed.
type RideMatchingRequest struct {
	RideID              string   `json:"ride_id"`
	RideNumber          string   `json:"ride_number"`
	PickupLocation      Location `json:"pickup_location"`
	DestinationLocation Location `json:"destination_location"`
	RideType            string   `json:"ride_type"`
	EstimatedFare       float64  `json:"estimated_fare"`
	MaxDistanceKM       float64  `json:"max_distance_km"`
	TimeoutSeconds      int      `json:"timeout_seconds"`
	CorrelationID       string   `json:"correlation_id"`
}

// LocationUpdate represents a real-time location update from driver
type LocationUpdate struct {
	DriverID       string
	RideID         string
	Latitude       float64
	Longitude      float64
	AccuracyMeters float64
	SpeedKmh       float64
	HeadingDegrees float64
	Timestamp      time.Time
}

// LocationHistory archives past location data
type LocationHistory struct {
	ID             string
	CoordinateID   string
	DriverID       string
	Latitude       float64
	Longitude      float64
	AccuracyMeters float64
	SpeedKmh       float64
	HeadingDegrees float64
	RecordedAt     time.Time
	RideID         string
}

// NearbyDriver represents a driver found near a location
type NearbyDriver struct {
	DriverID    string
	Email       string
	Rating      float64
	VehicleType string
	Latitude    float64
	Longitude   float64
	DistanceKm  float64
	VehicleInfo map[string]interface{}
}

// Driver status constants
const (
	DriverStatusOffline   = "OFFLINE"
	DriverStatusAvailable = "AVAILABLE"
	DriverStatusBusy      = "BUSY"
	DriverStatusEnRoute   = "EN_ROUTE"
)

// Vehicle type constants
const (
	VehicleTypeEconomy = "ECONOMY"
	VehicleTypePremium = "PREMIUM"
	VehicleTypeXL      = "XL"
)
