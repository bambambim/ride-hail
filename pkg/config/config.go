package config

import (
	"bufio"
	"fmt"
	"os"
	"strconv"
	"strings"
)

type Config struct {
	DB struct {
		Host     string
		Port     int
		User     string
		Password string
		Database string
	}
	RabbitMQ struct {
		Host     string
		Port     int
		User     string
		Password string
	}
	Websocket struct {
		Port int
	}
	Services struct {
		RideService           int
		DriverLocationService int
		AdminService          int
	}
}

func LoadConfig(filename string) (*Config, error) {
	err := loadEnvFile(filename)
	if err != nil {
		return nil, err
	}
	cfg := &Config{}
	cfg.DB.Host = getEnv("DB_HOST", "localhost")
	cfg.DB.Port = getEnvAsInt("DB_PORT", 5432)
	cfg.DB.User = getEnv("DB_USER", "ridehail_user")
	cfg.DB.Password = getEnv("DB_PASS", "ridehail_pass")
	cfg.DB.Database = getEnv("DB_NAME", "ridehail_db")
	cfg.RabbitMQ.Host = getEnv("RABBITMQ_HOST", "localhost")
	cfg.RabbitMQ.Port = getEnvAsInt("RABBITMQ_PORT", 5672)
	cfg.RabbitMQ.User = getEnv("RABBITMQ_USER", "guest")
	cfg.RabbitMQ.Password = getEnv("RABBITMQ_PASS", "guest")
	cfg.Websocket.Port = getEnvAsInt("WEBSOCKET_PORT", 8080)
	cfg.Services.RideService = getEnvAsInt("SERVICES_RIDE_SERVICE", 3000)
	cfg.Services.DriverLocationService = getEnvAsInt("DRIVER_LOCATION_SERVICE", 3001)
	cfg.Services.AdminService = getEnvAsInt("ADMIN_SERVICE", 3004)

	return cfg, nil

}

func loadEnvFile(filename string) error {
	file, err := os.Open(filename)
	if err != nil {
		return fmt.Errorf("could not open env file: %w", err)
	}
	defer file.Close()

	scanner := bufio.NewScanner(file)
	for scanner.Scan() {
		line := scanner.Text()

		// Trim spaces and ignore comments or empty lines
		line = strings.TrimSpace(line)
		if line == "" || strings.HasPrefix(line, "#") {
			continue
		}

		// Split into key=value
		parts := strings.SplitN(line, "=", 2)
		if len(parts) != 2 {
			continue // or return error if strict
		}

		key := strings.TrimSpace(parts[0])
		value := strings.TrimSpace(parts[1])

		// Remove optional surrounding quotes
		value = strings.Trim(value, `"'`)

		err := os.Setenv(key, value)
		if err != nil {
			return fmt.Errorf("could not set env var %s: %w", key, err)
		}
	}

	if err := scanner.Err(); err != nil {
		return fmt.Errorf("error reading env file: %w", err)
	}

	return nil
}

func getEnv(key, fallback string) string {
	if value, exists := os.LookupEnv(key); exists {
		return value
	}
	return fallback
}

func getEnvAsInt(key string, fallback int) int {
	valueStr := getEnv(key, "")
	if value, err := strconv.Atoi(valueStr); err == nil {
		return value
	}
	return fallback
}
