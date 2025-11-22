package rabbitmq

import (
	"context"
	"ride-hail/pkg/rabbitmq"
)

type DriverLocationPublisher struct {
	conn *rabbitmq.Connection
}

func NewDriverLocationPublisher(conn *rabbitmq.Connection) *DriverLocationPublisher {
	return &DriverLocationPublisher{
		conn: conn,
	}
}

func (p *DriverLocationPublisher) PublishDriverResponse(ctx context.Context, exchange, routingKey string, body []byte) error {
	return p.conn.Publish(ctx, exchange, routingKey, body)
}

func (p *DriverLocationPublisher) PublishDriverStatus(ctx context.Context, exchange, routingKey string, body []byte) error {
	return p.conn.Publish(ctx, exchange, routingKey, body)
}

func (p *DriverLocationPublisher) PublishLocationUpdate(ctx context.Context, exchange, routingKey string, body []byte) error {
	return p.conn.Publish(ctx, exchange, routingKey, body)
}
