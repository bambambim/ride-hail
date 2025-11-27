package db

import (
	"context"
	"fmt"
	"ride-hail/pkg/config"
	"ride-hail/pkg/db"
	"ride-hail/pkg/logger"

	"github.com/jackc/pgx/v5/pgxpool"
)

type PostgresDriverLocationRepository struct {
	log  logger.Logger
	cfg  *config.Config
	pool *pgxpool.Pool
}

func NewPostgresDriverLocationRepository(log logger.Logger, cfg *config.Config) *PostgresDriverLocationRepository {
	pool, err := db.NewConnection(cfg, log)
	if err != nil {
		log.Error("db_connection_failed", err)
	}
	return &PostgresDriverLocationRepository{
		log:  log,
		cfg:  cfg,
		pool: pool,
	}
}

func (r *PostgresDriverLocationRepository) SaveDriverLocation(ctx context.Context, driverID string, latitude, longitude float64) error {

	return nil
}

func (r *PostgresDriverLocationRepository) SaveDriverSession(ctx context.Context, driverID string, sessionID string) error {
	// Implementation here
	return nil
}

func (r *PostgresDriverLocationRepository) UpdateDriverStatus(ctx context.Context, driverID, status string) error {
	if status != "OFFLINE" && status != "BUSY" && status != "AVAILABLE" && status != "EN_ROUTE" {
		return fmt.Errorf("invalid status value")
	}
	query := `UPDATE drivers SET status=$1 WHERE id=$2`
	_, err := r.pool.Exec(ctx, query, status, driverID)
	if err != nil {
		return fmt.Errorf("failed to update driver status: %w", err)
	}
	return nil
}

func (r *PostgresDriverLocationRepository) FindNearbyDrivers(ctx context.Context, latitude, longitude float64, radiusMeters float64) ([]string, error) {
	return nil, nil
}
