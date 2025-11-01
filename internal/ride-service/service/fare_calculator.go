package service

import (
	"math"
)

// FareRates defines the pricing structure for each ride type
type FareRates struct {
	BaseFare   float64
	RatePerKm  float64
	RatePerMin float64
}

var rateTable = map[string]FareRates{
	"ECONOMY": {BaseFare: 500, RatePerKm: 100, RatePerMin: 50},
	"PREMIUM": {BaseFare: 800, RatePerKm: 120, RatePerMin: 60},
	"XL":      {BaseFare: 1000, RatePerKm: 150, RatePerMin: 75},
}

// CalculateFare calculates the estimated fare for a ride using dynamic pricing
// Formula: base_fare + (distance_km * rate_per_km) + (duration_min * rate_per_min)
func CalculateFare(pickupLat, pickupLng, destLat, destLng float64, rideType string) float64 {
	// Calculate distance using Haversine formula
	distanceKm := haversineDistance(pickupLat, pickupLng, destLat, destLng)

	// Estimate duration based on average speed of 40 km/h in city
	estimatedDurationMin := (distanceKm / 40.0) * 60.0

	return CalculateFareWithDuration(distanceKm, estimatedDurationMin, rideType)
}

// CalculateFareWithDuration calculates fare when duration is known
func CalculateFareWithDuration(distanceKm, durationMin float64, rideType string) float64 {
	rates, exists := rateTable[rideType]
	if !exists {
		rates = rateTable["ECONOMY"] // default to ECONOMY
	}

	// Dynamic pricing formula
	fare := rates.BaseFare + (distanceKm * rates.RatePerKm) + (durationMin * rates.RatePerMin)

	return fare
}

// haversineDistance calculates distance between two coordinates in km
func haversineDistance(lat1, lon1, lat2, lon2 float64) float64 {
	const earthRadius = 6371 // km

	dLat := toRadians(lat2 - lat1)
	dLon := toRadians(lon2 - lon1)

	a := math.Sin(dLat/2)*math.Sin(dLat/2) +
		math.Cos(toRadians(lat1))*math.Cos(toRadians(lat2))*
			math.Sin(dLon/2)*math.Sin(dLon/2)

	c := 2 * math.Atan2(math.Sqrt(a), math.Sqrt(1-a))

	return earthRadius * c
}

func toRadians(deg float64) float64 {
	return deg * math.Pi / 180
}
