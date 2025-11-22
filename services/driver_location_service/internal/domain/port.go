package domain

import (
	"context"

	amqp "github.com/rabbitmq/amqp091-go"
)

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

type DriverLocationPublisher interface {
	PublishDriverResponse(ctx context.Context, exchange, routingKey string, body []byte) error
	PublishDriverStatus(ctx context.Context, exchange, routingKey string, body []byte) error
	PublishLocationUpdate(ctx context.Context, exchange, routingKey string, body []byte) error
}

type DriverLocationSubscriber interface {
	ConsumeDriverMatching(ctx context.Context, handler func(amqp.Delivery)) error
	ConsumeRideStatus(ctx context.Context, handler func(amqp.Delivery)) error
}

type DriverLocationRealtime interface {
	// WebsocketConnect(driverID string) error
}