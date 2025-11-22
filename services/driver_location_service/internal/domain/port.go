package domain

type DriverLocationService interface {
	OnlineDriver(driverID string, latitude, longitude float64) error
	OfflineDriver(driverID string) error
	UpdateDriverLocation(driverID string, latitude, longitude float64) error
	StartRide(driverID string) error
	EndRide(driverID string) error
	WebsocketConnect(driverID string) error
}

type DriverLocationRepository interface {

}