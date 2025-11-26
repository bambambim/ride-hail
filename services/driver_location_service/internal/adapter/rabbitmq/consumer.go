package rabbitmq

import (
	"context"
	"encoding/json"
	"ride-hail/pkg/logger"
	"ride-hail/pkg/rabbitmq"
	"ride-hail/services/driver_location_service/internal/domain"
	"time"

	amqp "github.com/rabbitmq/amqp091-go"
)

type DriverLocationConsumer struct {
	conn *rabbitmq.Connection
	repo domain.DriverLocationRepository
	log  logger.Logger
}

func NewDriverLocationConsumer(conn *rabbitmq.Connection, repo domain.DriverLocationRepository, log logger.Logger) *DriverLocationConsumer {
	return &DriverLocationConsumer{
		conn: conn,
		repo: repo,
		log:  log,
	}
}

func (c *DriverLocationConsumer) ConsumeDriverMatching(ctx context.Context) error {
	// might use context for cancellation or timeout
	return c.conn.Consume("driver_matching", driverMatchingHandler(c))
}

type DriverMatchingRequest struct {
	RideID              string   `json:"ride_id"`
	RideNumber          string   `json:"ride_number"`
	PickupLocation      Location `json:"pickup_location"`
	DestinationLocation Location `json:"destination_location"`
	RideType            string   `json:"ride_type"`
	EstimatedFare       int64    `json:"estimated_fare"`
	MaxDistanceKM       int64    `json:"max_distance_km"`
	TimeoutSeconds      int64    `json:"timeout_seconds"`
	CorrelationID       string   `json:"correlation_id"`
}

type Location struct {
	Lat     float64 `json:"lat"`
	Lng     float64 `json:"lng"`
	Address string  `json:"address"`
}

func driverMatchingHandler(c *DriverLocationConsumer) func(amqp.Delivery) {
	return func(d amqp.Delivery) {
		var req DriverMatchingRequest

		if err := json.Unmarshal(d.Body, &req); err != nil {
			d.Nack(false, false)
			return
		}

		// add business logic here
		// service.MatchDriver(req)

		d.Ack(false)
	}
}

func (c *DriverLocationConsumer) ConsumeRideStatus(ctx context.Context) error {
	// might use context for cancellation or timeout
	return c.conn.Consume("ride_status", rideStatusHandler(c))
}

type RideStatusRequest struct {
	RideID        string    `json:"ride_id"`
	Status        string    `json:"status"`
	Timestamp     time.Time `json:"timestamp"`
	FinalFare     int64     `json:"final_fare"`
	CorrelationID string    `json:"correlation_id"`
}

func rideStatusHandler(c *DriverLocationConsumer) func(amqp.Delivery) {
	return func(d amqp.Delivery) {
		var req RideStatusRequest

		if err := json.Unmarshal(d.Body, &req); err != nil {
			d.Nack(false, false)
			return
		}

		// add business logic here
		// service.MatchDriver(req)

		d.Ack(false)
	}
}