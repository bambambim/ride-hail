package service

import (
	"context"
	"encoding/json"
	"fmt"
	"time"

	"ride-hail/internal/domain"
	"ride-hail/pkg/logger"
	"ride-hail/pkg/rabbitmq"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
)

// NOTE: We assume your pkg/db and pkg/rabbitmq clients are correctly implemented.

// RideService orchestrates the ride lifecycle.
type RideService struct {
	DB  *pgxpool.Pool
	MQ  *rabbitmq.Connection
	Log logger.Logger
}

func NewService(db *pgxpool.Pool, mq *rabbitmq.Connection, log logger.Logger) *RideService {
	return &RideService{
		DB:  db,
		MQ:  mq,
		Log: log,
	}
}

// CreateRide handles the entire ride initiation process.
func (s *RideService) CreateRide(ctx context.Context, req domain.RideRequest) (domain.UUID, error) {
	estimatedFare := s.calculateFare(req.RequestedRideType)
	requestedAt := time.Now()

	// 1. Generate the new UUID for the ride itself
	rideID := domain.MustNewV4() // Uses our custom UUID generator

	// 2. Database Transaction (Atomicity is key!)
	var pickupID domain.UUID
	var destID domain.UUID

	tx, err := s.DB.BeginTx(ctx, pgx.TxOptions{})
	if err != nil {
		s.Log.Error("transaction_begin_failed", err)
		return domain.Nil, err
	}
	defer tx.Rollback(ctx)

	// 2a. Insert Coordinates (generate IDs for coordinates)
	pickupID, err = s.insertCoordinate(ctx, tx, req.PickupLat, req.PickupLon, req.PickupAddress)
	if err != nil {
		s.Log.Error("insert_pickup_coordinates_failed", err)
		return domain.Nil, fmt.Errorf("failed to insert pickup coordinates: %w", err)
	}

	destID, err = s.insertCoordinate(ctx, tx, req.DestinationLat, req.DestinationLon, req.DestinationAddress)
	if err != nil {
		s.Log.Error("insert_destination_coordinates_failed", err)
		return domain.Nil, fmt.Errorf("failed to insert destination coordinates: %w", err)
	}

	// 2b. Insert Ride
	err = s.insertRide(ctx, tx, rideID, req, pickupID, destID, estimatedFare, requestedAt)
	if err != nil {
		s.Log.Error("insert_ride_failed", err)
		return domain.Nil, fmt.Errorf("failed to insert ride: %w", err)
	}

	// Commit transaction
	if err = tx.Commit(ctx); err != nil {
		s.Log.Error("transaction_commit_failed", err)
		return domain.Nil, err
	}

	// 3. Publish Message (After successful commit)
	err = s.publishMatchRequest(ctx, req, rideID)
	if err != nil {
		s.Log.Error("publish_match_request_failed", err)
		// CRITICAL: Logging the failure, but the ride is still REQUESTED in DB.
	}

	// 4. Start Timeout Logic
	go s.startTimeoutTimer(rideID, 2*time.Minute)

	return rideID, nil
}

// --- Helper Methods ---

func (s *RideService) calculateFare(rideType domain.RideType) float64 {
	// ... (Simplistic calculation logic remains the same)
	baseFare := 5.00
	switch rideType {
	case domain.Premium:
		baseFare = 7.50
	case domain.XL:
		baseFare = 10.00
	}
	return baseFare + 20.00
}

// insertCoordinate generates a UUID and takes the address
func (s *RideService) insertCoordinate(ctx context.Context, tx pgx.Tx, lat, lon float64, address string) (domain.UUID, error) {
	newID := domain.MustNewV4()
	query := `INSERT INTO coordinates (id, lat, lon, address) VALUES ($1, $2, $3, $4)`
	val, err := newID.Value()
	if err != nil {
		return domain.Nil, err
	}
	_, err = tx.Exec(ctx, query, val, lat, lon, address)
	if err != nil {
		return domain.Nil, err
	}
	return newID, nil
}

// insertRide inserts the ride record
func (s *RideService) insertRide(ctx context.Context, tx pgx.Tx, id domain.UUID, req domain.RideRequest, pickupID, destID domain.UUID, estimatedFare float64, requestedAt time.Time) error {
	query := `INSERT INTO rides (id, passenger_id, status, requested_at, estimated_fare, ride_type, pickup_coordinate_id, destination_coordinate_id) 
	          VALUES ($1, $2, $3, $4, $5, $6, $7, $8)`

	idVal, _ := id.Value()
	passengerVal, _ := req.PassengerID.Value()
	pickupVal, _ := pickupID.Value()
	destVal, _ := destID.Value()

	_, err := tx.Exec(ctx, query,
		idVal,
		passengerVal,
		"REQUESTED",
		requestedAt,
		estimatedFare,
		req.RequestedRideType,
		pickupVal,
		destVal,
	)
	return err
}

func (s *RideService) publishMatchRequest(ctx context.Context, req domain.RideRequest, rideID domain.UUID) error {
	matchReq := domain.DriverMatchRequest{
		RideID:      rideID,
		RideType:    req.RequestedRideType,
		PickupLat:   req.PickupLat,
		PickupLon:   req.PickupLon,
		PassengerID: req.PassengerID,
	}

	body, err := json.Marshal(matchReq)
	if err != nil {
		return fmt.Errorf("failed to marshal match request: %w", err)
	}

	routingKey := fmt.Sprintf("ride.request.%s", req.RequestedRideType)
	return s.MQ.Publish(ctx, "ride_topic", routingKey, body)
}

func (s *RideService) startTimeoutTimer(rideID domain.UUID, duration time.Duration) {
	// ... (Logic remains the same)
	time.Sleep(duration)
	s.Log.Info("ride_timeout_check", fmt.Sprintf("Timeout check for ride %s", rideID.String()))
}
