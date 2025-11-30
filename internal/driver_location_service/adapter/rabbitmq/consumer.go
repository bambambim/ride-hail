package rabbitmq

import (
	"context"
	"encoding/json"
	"fmt"
	"time"

	"ride-hail/internal/driver_location_service/domain"
	"ride-hail/pkg/logger"
	"ride-hail/pkg/rabbitmq"

	amqp "github.com/rabbitmq/amqp091-go"
)

type DriverLocationConsumer struct {
	conn *rabbitmq.Connection
	svc  domain.DriverLocationService
	log  logger.Logger
}

// NewDriverLocationConsumer wires a driver-location service to the RabbitMQ connection.
func NewDriverLocationConsumer(conn *rabbitmq.Connection, svc domain.DriverLocationService, log logger.Logger) *DriverLocationConsumer {
	return &DriverLocationConsumer{
		conn: conn,
		svc:  svc,
		log:  log,
	}
}

// ConsumeDriverMatching listens for ride matching requests and forwards them to the application service.
func (c *DriverLocationConsumer) ConsumeDriverMatching(ctx context.Context) error {
	return c.conn.Consume("driver_matching", c.driverMatchingHandler(ctx))
}

func (c *DriverLocationConsumer) driverMatchingHandler(ctx context.Context) func(amqp.Delivery) {
	return func(d amqp.Delivery) {
		handlerCtx := c.baseCtx(ctx)

		var req domain.RideMatchingRequest
		if err := json.Unmarshal(d.Body, &req); err != nil {
			c.log.Error("driver_matching_unmarshal_failed", err)
			d.Nack(false, false)
			return
		}
		fmt.Println(req)
		if err := c.svc.HandleRideMatchingRequest(handlerCtx, &req); err != nil {
			c.log.Error("driver_matching_handle_failed", err)
			d.Nack(false, true)
			return
		}

		d.Ack(false)
	}
}

// ConsumeRideStatus listens for ride status updates published by the ride service.
func (c *DriverLocationConsumer) ConsumeRideStatus(ctx context.Context) error {
	return c.conn.Consume("ride_status", c.rideStatusHandler(ctx))
}

type rideStatusMessage struct {
	RideID        string    `json:"ride_id"`
	Status        string    `json:"status"`
	Timestamp     time.Time `json:"timestamp"`
	FinalFare     float64   `json:"final_fare"`
	CorrelationID string    `json:"correlation_id"`
	DriverID      string    `json:"driver_id"`
}

func (c *DriverLocationConsumer) rideStatusHandler(ctx context.Context) func(amqp.Delivery) {
	return func(d amqp.Delivery) {
		handlerCtx := c.baseCtx(ctx)

		var msg rideStatusMessage
		if err := json.Unmarshal(d.Body, &msg); err != nil {
			c.log.Error("ride_status_unmarshal_failed", err)
			d.Nack(false, false)
			return
		}

		if err := c.svc.HandleRideStatusUpdate(handlerCtx, msg.RideID, msg.DriverID, msg.Status, msg.FinalFare); err != nil {
			c.log.Error("ride_status_handle_failed", err)
			d.Nack(false, true)
			return
		}

		d.Ack(false)
	}
}

func (c *DriverLocationConsumer) baseCtx(ctx context.Context) context.Context {
	if ctx != nil {
		return ctx
	}
	return context.Background()
}
