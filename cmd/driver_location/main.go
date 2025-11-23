package main

import (
	// "ride-hail/pkg/config"
	"ride-hail/pkg/logger"
)

// TODO:
// WebSocket
// RabbitMQ
// HTTP Server
// PostgreSQL
// Implement driver location service main function

func main() {
	log := logger.NewLogger("driver_location_service")
	log.Info("service_start", "Driver location service is starting")

	// cfg, err := config.LoadConfig(".env")
	// if err != nil {
	// 	panic(err)
	// }
}