package rabbitmq

import "ride-hail/pkg/rabbitmq"

type DriverLocationConsumer struct {
	conn *rabbitmq.Connection
}

func NewDriverLocationConsumer(conn *rabbitmq.Connection) *DriverLocationConsumer {
	return &DriverLocationConsumer{
		conn: conn,
	}
}

// TODO: write own handler methods for consuming messages
// func (c *DriverLocationConsumer) ConsumeDriverMatching(ctx context.Context, handler func(delivery rabbitmq.Delivery)) error {
// 	return c.conn.Consume(ctx, "driver_matching_queue", handler)
// }

// func (c *DriverLocationConsumer) ConsumeRideStatus(ctx context.Context, handler func(delivery rabbitmq.Delivery)) error {
// 	return c.conn.Consume(ctx, "ride_status_queue", handler)
// }
