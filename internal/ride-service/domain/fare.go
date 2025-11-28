package domain

// FareCalculator is a domain service for calculating ride fares
// Domain services contain business logic that doesn't naturally fit in an entity
type FareCalculator struct {
	baseFares  map[RideType]float64
	perKmRates map[RideType]float64
}

// NewFareCalculator creates a new fare calculator with default rates
func NewFareCalculator() *FareCalculator {
	return &FareCalculator{
		baseFares: map[RideType]float64{
			RideTypeEconomy: 100.0, // Base fare in currency units
			RideTypePremium: 150.0,
			RideTypeLuxury:  250.0,
		},
		perKmRates: map[RideType]float64{
			RideTypeEconomy: 15.0, // Per kilometer rate
			RideTypePremium: 25.0,
			RideTypeLuxury:  40.0,
		},
	}
}

// NewFareCalculatorWithRates creates a fare calculator with custom rates
func NewFareCalculatorWithRates(baseFares, perKmRates map[RideType]float64) *FareCalculator {
	return &FareCalculator{
		baseFares:  baseFares,
		perKmRates: perKmRates,
	}
}

// Calculate calculates the estimated fare for a ride
func (fc *FareCalculator) Calculate(pickup, dest Coordinate, rideType RideType) float64 {
	distance := pickup.DistanceTo(dest)
	return fc.CalculateByDistance(distance, rideType)
}

// CalculateByDistance calculates fare based on distance and ride type
func (fc *FareCalculator) CalculateByDistance(distanceKm float64, rideType RideType) float64 {
	baseFare, ok := fc.baseFares[rideType]
	if !ok {
		baseFare = fc.baseFares[RideTypeEconomy] // Default to economy
	}

	perKm, ok := fc.perKmRates[rideType]
	if !ok {
		perKm = fc.perKmRates[RideTypeEconomy] // Default to economy
	}

	totalFare := baseFare + (distanceKm * perKm)
	return totalFare
}

// GetBaseFare returns the base fare for a ride type
func (fc *FareCalculator) GetBaseFare(rideType RideType) float64 {
	if fare, ok := fc.baseFares[rideType]; ok {
		return fare
	}
	return fc.baseFares[RideTypeEconomy]
}

// GetPerKmRate returns the per kilometer rate for a ride type
func (fc *FareCalculator) GetPerKmRate(rideType RideType) float64 {
	if rate, ok := fc.perKmRates[rideType]; ok {
		return rate
	}
	return fc.perKmRates[RideTypeEconomy]
}

// CalculateFare is a convenience function for calculating fare
// Can be used directly without creating a FareCalculator instance
func CalculateFare(pickupLat, pickupLng, destLat, destLng float64, rideType RideType) float64 {
	pickup, err := NewCoordinate(pickupLat, pickupLng, "")
	if err != nil {
		return 0
	}

	dest, err := NewCoordinate(destLat, destLng, "")
	if err != nil {
		return 0
	}

	calculator := NewFareCalculator()
	return calculator.Calculate(pickup, dest, rideType)
}
