package domain

import (
	"errors"
	"math"
)

// Coordinate errors
var (
	ErrInvalidLatitude  = errors.New("latitude must be between -90 and 90")
	ErrInvalidLongitude = errors.New("longitude must be between -180 and 180")
	ErrZeroCoordinates  = errors.New("coordinates cannot be zero")
)

// Coordinate is a value object representing a geographic location
// Value objects are immutable
type Coordinate struct {
	latitude  float64
	longitude float64
	address   string
}

// NewCoordinate creates a new coordinate with validation
func NewCoordinate(lat, lng float64, address string) (Coordinate, error) {
	if lat < -90 || lat > 90 {
		return Coordinate{}, ErrInvalidLatitude
	}
	if lng < -180 || lng > 180 {
		return Coordinate{}, ErrInvalidLongitude
	}
	if lat == 0 && lng == 0 {
		return Coordinate{}, ErrZeroCoordinates
	}

	return Coordinate{
		latitude:  lat,
		longitude: lng,
		address:   address,
	}, nil
}

// Validate checks if the coordinate is valid
func (c Coordinate) Validate() error {
	if c.latitude < -90 || c.latitude > 90 {
		return ErrInvalidLatitude
	}
	if c.longitude < -180 || c.longitude > 180 {
		return ErrInvalidLongitude
	}
	if c.latitude == 0 && c.longitude == 0 {
		return ErrZeroCoordinates
	}
	return nil
}

// DistanceTo calculates the distance to another coordinate in kilometers
// Uses the Haversine formula
func (c Coordinate) DistanceTo(other Coordinate) float64 {
	return haversineDistance(c.latitude, c.longitude, other.latitude, other.longitude)
}

// Getters (encapsulation - coordinates are immutable)
func (c Coordinate) Latitude() float64  { return c.latitude }
func (c Coordinate) Longitude() float64 { return c.longitude }
func (c Coordinate) Address() string    { return c.address }

// haversineDistance calculates the distance between two points on Earth
// Returns distance in kilometers
func haversineDistance(lat1, lng1, lat2, lng2 float64) float64 {
	const earthRadius = 6371.0 // Earth's radius in kilometers

	// Convert degrees to radians
	dLat := toRadians(lat2 - lat1)
	dLng := toRadians(lng2 - lng1)

	lat1Rad := toRadians(lat1)
	lat2Rad := toRadians(lat2)

	// Haversine formula
	a := math.Sin(dLat/2)*math.Sin(dLat/2) +
		math.Cos(lat1Rad)*math.Cos(lat2Rad)*
			math.Sin(dLng/2)*math.Sin(dLng/2)

	c := 2 * math.Atan2(math.Sqrt(a), math.Sqrt(1-a))

	distance := earthRadius * c
	return distance
}

// toRadians converts degrees to radians
func toRadians(degrees float64) float64 {
	return degrees * math.Pi / 180
}

// ValidateCoordinates is a helper function for validation
func ValidateCoordinates(lat, lng float64) error {
	if lat < -90 || lat > 90 {
		return ErrInvalidLatitude
	}
	if lng < -180 || lng > 180 {
		return ErrInvalidLongitude
	}
	if lat == 0 && lng == 0 {
		return ErrZeroCoordinates
	}
	return nil
}
