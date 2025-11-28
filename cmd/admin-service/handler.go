package adminservice

import (
	"context"
	"database/sql"
	"net/http"
	"ride-hail/pkg/logger"
	"time"

	"github.com/jackc/pgx/v5/pgxpool"
)

type AdminHandler struct {
	log  logger.Logger
	pool *pgxpool.Pool
}

type OverviewMetrics struct {
	ActiveRides         int `json:"active_rides"`
	AvailableDrivers    int `json:"available_drivers"`
	BusyDrivers         int `json:"busy_drivers"`
	TotalRidesToday     int `json:"total_rides_today"`
	TotalRevenueToday   int `json:"total_revenue_today"`
	AverageWaitTime     int `json:"average_wait_time_minutes"`
	AverageRideDuration int `json:"average_ride_duration_minutes"`
}
type ActiveRide struct {
	RideID             string    `json:"ride_id"`
	RideNumber         string    `json:"ride_number"`
	Status             string    `json:"status"`
	PassengerID        string    `json:"passenger_id"`
	DriverID           string    `json:"driver_id"`
	PickupAddress      string    `json:"pickup_address"`
	DestinationAddress string    `json:"destination_address"`
	StartedAt          time.Time `json:"started_at"`
}

type ActiveRidesResponse struct {
	Rides      []ActiveRide `json:"rides"`
	TotalCount int          `json:"total_count"`
	Page       int          `json:"page"`
	PageSize   int          `json:"page_size"`
}

func NewAdminHandler(log logger.Logger, pool *pgxpool.Pool) *AdminHandler {
	return &AdminHandler{
		log:  log,
		pool: pool,
	}
}

func (h *AdminHandler) getOverviewMetrics(w http.ResponseWriter, r *http.Request) {
	ctx, cancel := context.WithTimeout(r.Context(), time.Second*10)
	defer cancel()

	var metrics OverviewMetrics
	tx, err := h.pool.Begin(ctx)
	if err != nil {
		h.log.Error("get_overview_metrics: ", err)
		writeError(w, http.StatusInternalServerError, "Database error")
		return
	}

	defer tx.Rollback(ctx)

	err = tx.QueryRow(ctx, `
	SELECT COUNT(*) FROM rides
	WHERE status IN ('REQUESTED', 'MATCHED', 'EN_ROUTE', 'ARRIVED', 'IN PROGRESS')
	`).Scan(&metrics.ActiveRides)
	if err != nil {
		h.log.Error("get_overview_query_active_rides: ", err)
		writeError(w, http.StatusInternalServerError, "Database error")
		return
	}

	err = tx.QueryRow(ctx, `
	SELECT COUNT(*) FROM drivers
	WHERE status = 'AVAILABLE'
	`).Scan(&metrics.AvailableDrivers)
	if err != nil {
		h.log.Error("get_overview_query_available_drivers: ", err)
		writeError(w, http.StatusInternalServerError, "Database error")
		return
	}

	err = tx.QueryRow(ctx, `
	SELECT COUNT(*) FROM drivers 
	WHERE status IN ('BUSY', 'EN_ROUTE')
	`).Scan(&metrics.BusyDrivers)
	if err != nil {
		h.log.Error("get_overview_query_busy_drivers: ", err)
		writeError(w, http.StatusInternalServerError, "Database error")
		return
	}

	err = tx.QueryRow(ctx, `
	SELECT COUNT(*) FROM rides
	WHERE completed_at >= current_date
	`).Scan(&metrics.TotalRidesToday)
	if err != nil {
		h.log.Error("get_overview_query_total_rides_today: ", err)
		writeError(w, http.StatusInternalServerError, "Database error")
		return
	}

	err = tx.QueryRow(ctx, `
	SELECT COALESCE(SUM(final_fare * 100), 0) FROM rides
	WHERE completed_at >= current_date AND status = 'COMPLETED'
	`).Scan(&metrics.TotalRevenueToday)
	if err != nil {
		h.log.Error("get_overview_query_total_revenue_today: ", err)
		writeError(w, http.StatusInternalServerError, "Database error")
		return
	}

	err = tx.QueryRow(ctx, `
	SELECT COALESCE(AVG(EXTRACT(EPOCH FROM (matched_at - requested_at))) / 60 ,0)
	FROM rides
	WHERE matched_at IS NOT NULL AND requested_at >= current_date
	`).Scan(&metrics.AverageWaitTime)
	if err != nil {
		h.log.Error("get_overview_query_avg_wait_time_minutes: ", err)
		writeError(w, http.StatusInternalServerError, "Database error")
		return
	}

	err = tx.QueryRow(ctx, `
	SELECT COALESCE(AVG(EXTRACT(EPOCH FROM (completed_at - started_at))) / 60 ,0)
	FROM rides
	WHERE status = 'COMPLETED' AND completed_at >= current_date
	`).Scan(&metrics.AverageRideDuration)
	if err != nil {
		h.log.Error("get_overview_query_avg_rides_today: ", err)
		writeError(w, http.StatusInternalServerError, "Database error")
		return
	}

	if err := tx.Commit(ctx); err != nil {
		h.log.Error("get_overview_commit_tx: ", err)
		writeError(w, http.StatusInternalServerError, "Database error")
		return
	}

	writeJSON(w, http.StatusOK, metrics)
}

func (h *AdminHandler) getActiveRides(w http.ResponseWriter, r *http.Request) {
	ctx, cancel := context.WithTimeout(r.Context(), time.Second*10)
	defer cancel()

	page, pageSize := parsePagination(r)
	offset := (page - 1) * pageSize

	var response ActiveRidesResponse
	response.Rides = make([]ActiveRide, 0)
	response.Page = page
	response.PageSize = pageSize

	tx, err := h.pool.Begin(ctx)
	if err != nil {
		h.log.Error("get_active_rides: ", err)
		writeError(w, http.StatusInternalServerError, "Database error")
		return
	}
	defer tx.Rollback(ctx)

	err = tx.QueryRow(ctx, `
	SELECT COUNT(*) FROM rides
	WHERE status IN ('REQUESTED', 'MATCHED', 'EN_ROUTE', 'ARRIVED', 'IN_PROGRES')
	`).Scan(&response.TotalCount)
	if err != nil {
		h.log.Error("get_active_rides_total_count: ", err)
		writeError(w, http.StatusInternalServerError, "Database error")
		return
	}

	if response.TotalCount == 0 {
		if err := tx.Commit(ctx); err != nil {
			h.log.Error("get_active_rides_commit_tx: ", err)
			writeError(w, http.StatusInternalServerError, "Database error")
			return
		}
		writeJSON(w, http.StatusOK, response)
		return
	}

	query := `
		SELECT 
			r.id, r.ride_number, r.status, r.passenger_id, r.driver_id,
			COALESCE(pickup.address, 'N/A') as pickup_address,
			COALESCE(destination.address, 'N/A') as destination_address,
			r.started_at
		FROM rides AS r
		LEFT JOIN coordinates pickup ON r.pickup_coordinate_id = pickup.id
		LEFT JOIN coordinates destination ON r.destination_coordinate_id = destination.id
		WHERE r.status IN ('REQUESTED', 'MATCHED', 'EN_ROUTE', 'ARRIVED', 'IN_PROGRES')
		ORDER BY r.requested_at DESC
		LIMIT $1 OFFSET $2
		`

	rows, err := tx.Query(ctx, query, pageSize, offset)
	if err != nil {
		h.log.Error("get_active_rides_rows: ", err)
		writeError(w, http.StatusInternalServerError, "Database error")
		return
	}
	defer rows.Close()

	for rows.Next() {
		var ride ActiveRide
		var driverID sql.NullString
		var startedAt sql.NullTime

		err := rows.Scan(
			&ride.RideID,
			&ride.RideNumber,
			&ride.Status,
			&ride.PassengerID,
			&driverID,
			&ride.PickupAddress,
			&ride.DestinationAddress,
			&startedAt,
		)
		if err != nil {
			h.log.Error("get_active_rides_rows: ", err)
			writeError(w, http.StatusInternalServerError, "Database error")
			return
		}

		if driverID.Valid {
			ride.DriverID = driverID.String
		}
		if startedAt.Valid {
			ride.StartedAt = startedAt.Time
		}

		response.Rides = append(response.Rides, ride)
	}
	if err := rows.Err(); err != nil {
		h.log.Error("get_active_rides_rows: ", err)
		writeError(w, http.StatusInternalServerError, "Database error")
		return
	}

	if err := tx.Commit(ctx); err != nil {
		h.log.Error("get_active_rides_commit_tx: ", err)
		writeError(w, http.StatusInternalServerError, "Database error")
		return
	}
	writeJSON(w, http.StatusOK, response)
}
