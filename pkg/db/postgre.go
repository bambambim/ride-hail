package db

import (
	"context"
	"fmt"
	"github.com/jackc/pgx/v5/pgxpool"
	"ride-hail/pkg/config"
	"ride-hail/pkg/logger"
	"time"
)

const (
	maxRetries    = 5
	retryInterval = 3 * time.Second
)

func NewConnection(cfg *config.Config, log logger.Logger) (*pgxpool.Pool, error) {
	dsn := fmt.Sprintf("postgres://%s:%s@%s:%d/%s?sslmode=disable",
		cfg.DB.User,
		cfg.DB.Password,
		cfg.DB.Host,
		cfg.DB.Port,
		cfg.DB.Database,
	)
	var pool *pgxpool.Pool
	var err error

	log.Info("db_connect", "Connecting to database...")

	for i := 0; i < maxRetries; i++ {
		pool, err = pgxpool.New(context.Background(), dsn)
		if err != nil {
			log.Error("db_connect_failed", fmt.Errorf("failed to connect to database(attempt %d/%d): %w ", i+1, maxRetries, err))
			time.Sleep(retryInterval)
			continue
		}
		err = pool.Ping(context.Background())
		if err == nil {
			log.Info("db_connected_success", "Successfully connected to database")
			return pool, nil
		}

		log.Error("db_ping_failed", fmt.Errorf("failed to connect to database(attempt %d/%d): %w", i+1, maxRetries, err))
		pool.Close()
		time.Sleep(retryInterval)
	}

	return nil, fmt.Errorf("failed to connect to database after %d attempts: %w", maxRetries, err)

}
