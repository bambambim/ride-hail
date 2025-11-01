package service

import (
	"context"
	"fmt"
	"time"

	"ride-hail/pkg/logger"
	"ride-hail/services/driver_location_service/internal/domain"
	"ride-hail/services/driver_location_service/internal/ports"
)

// driverService implements the DriverService interface
type driverService struct {
	repo          ports.Repository
	messageBroker ports.MessageBroker
	wsHub         ports.WebSocketHub
	rateLimiter   ports.RateLimiter
	logger        logger.Logger
}

// NewDriverService creates a new driver service
func NewDriverService(
	repo ports.Repository,
	messageBroker ports.MessageBroker,
	wsHub ports.WebSocketHub,
	rateLimiter ports.RateLimiter,
	log logger.Logger,
) ports.DriverService {
	return &driverService{
		repo:          repo,
		messageBroker: messageBroker,
		wsHub:         wsHub,
		rateLimiter:   rateLimiter,
		logger:        log,
	}
}

// GoOnline sets a driver's status to available and creates a session
func (s *driverService) GoOnline(ctx context.Context, driverID string, req *domain.OnlineRequest) (*domain.OnlineResponse, error) {
	s.logger.Info("driver.go_online", fmt.Sprintf("Driver %s going online", driverID))

	// Get driver to verify they exist
	_, err := s.repo.GetByID(ctx, driverID)
	if err != nil {
		s.logger.Error("driver.go_online.get_driver", err)
		return nil, fmt.Errorf("failed to get driver: %w", err)
	}

	// Check if driver already has an active session
	activeSession, err := s.repo.GetActiveSession(ctx, driverID)
	if err != nil {
		s.logger.Error("driver.go_online.get_active_session", err)
		return nil, fmt.Errorf("failed to check active session: %w", err)
	}

	if activeSession != nil {
		return &domain.OnlineResponse{
			Status:    domain.DriverStatusAvailable,
			SessionID: activeSession.ID,
			Message:   "You are already online",
		}, nil
	}

	// Create new session
	session, err := s.repo.CreateSession(ctx, driverID)
	if err != nil {
		s.logger.Error("driver.go_online.create_session", err)
		return nil, fmt.Errorf("failed to create session: %w", err)
	}

	// Update driver status to AVAILABLE
	if err := s.repo.UpdateStatus(ctx, driverID, domain.DriverStatusAvailable); err != nil {
		s.logger.Error("driver.go_online.update_status", err)
		return nil, fmt.Errorf("failed to update driver status: %w", err)
	}

	// Update driver location
	locationUpdate := &domain.LocationUpdate{
		DriverID:  driverID,
		Latitude:  req.Latitude,
		Longitude: req.Longitude,
		Timestamp: time.Now(),
	}

	_, err = s.repo.UpdateLocation(ctx, locationUpdate)
	if err != nil {
		s.logger.Error("driver.go_online.update_location", err)
		return nil, fmt.Errorf("failed to update location: %w", err)
	}

	// Publish driver status update
	statusUpdate := &domain.DriverStatusUpdate{
		DriverID:  driverID,
		Status:    domain.DriverStatusAvailable,
		Timestamp: time.Now(),
	}

	if err := s.messageBroker.PublishDriverStatusUpdate(ctx, statusUpdate); err != nil {
		s.logger.Error("driver.go_online.publish_status", err)
		// Don't fail the request if publishing fails
	}

	s.logger.Info("driver.go_online.success", fmt.Sprintf("Driver %s is now online with session %s", driverID, session.ID))

	return &domain.OnlineResponse{
		Status:    domain.DriverStatusAvailable,
		SessionID: session.ID,
		Message:   "You are now online and ready to accept rides",
	}, nil
}

// GoOffline sets a driver's status to offline and ends the session
func (s *driverService) GoOffline(ctx context.Context, driverID string) (*domain.OfflineResponse, error) {
	s.logger.Info("driver.go_offline", fmt.Sprintf("Driver %s going offline", driverID))

	// Get driver to verify they exist
	_, err := s.repo.GetByID(ctx, driverID)
	if err != nil {
		s.logger.Error("driver.go_offline.get_driver", err)
		return nil, fmt.Errorf("failed to get driver: %w", err)
	}

	// Get active session
	activeSession, err := s.repo.GetActiveSession(ctx, driverID)
	if err != nil {
		s.logger.Error("driver.go_offline.get_active_session", err)
		return nil, fmt.Errorf("failed to get active session: %w", err)
	}

	if activeSession == nil {
		return nil, fmt.Errorf("no active session found")
	}

	// End the session
	endedSession, err := s.repo.EndSession(ctx, activeSession.ID, activeSession.TotalRides, activeSession.TotalEarnings)
	if err != nil {
		s.logger.Error("driver.go_offline.end_session", err)
		return nil, fmt.Errorf("failed to end session: %w", err)
	}

	// Update driver status to OFFLINE
	if err := s.repo.UpdateStatus(ctx, driverID, domain.DriverStatusOffline); err != nil {
		s.logger.Error("driver.go_offline.update_status", err)
		return nil, fmt.Errorf("failed to update driver status: %w", err)
	}

	// Calculate session summary
	duration := endedSession.EndedAt.Sub(endedSession.StartedAt)
	summary := domain.SessionSummary{
		DurationHours:  duration.Hours(),
		RidesCompleted: endedSession.TotalRides,
		Earnings:       endedSession.TotalEarnings,
	}

	// Publish driver status update
	statusUpdate := &domain.DriverStatusUpdate{
		DriverID:  driverID,
		Status:    domain.DriverStatusOffline,
		Timestamp: time.Now(),
	}

	if err := s.messageBroker.PublishDriverStatusUpdate(ctx, statusUpdate); err != nil {
		s.logger.Error("driver.go_offline.publish_status", err)
	}

	s.logger.Info("driver.go_offline.success", fmt.Sprintf("Driver %s is now offline", driverID))

	return &domain.OfflineResponse{
		Status:         domain.DriverStatusOffline,
		SessionID:      endedSession.ID,
		SessionSummary: summary,
		Message:        "You are now offline",
	}, nil
}

// UpdateLocation updates a driver's current location
func (s *driverService) UpdateLocation(ctx context.Context, driverID string, req *domain.UpdateLocationRequest) (*domain.UpdateLocationResponse, error) {
	// Rate limit check: max 1 update per 3 seconds
	rateLimitKey := fmt.Sprintf("location_update:%s", driverID)
	allowed, err := s.rateLimiter.Allow(ctx, rateLimitKey)
	if err != nil {
		s.logger.Error("driver.update_location.rate_limit", err)
		return nil, fmt.Errorf("rate limit check failed: %w", err)
	}

	if !allowed {
		return nil, fmt.Errorf("rate limit exceeded: maximum 1 update per 3 seconds")
	}

	// Get driver to check current status
	driver, err := s.repo.GetByID(ctx, driverID)
	if err != nil {
		s.logger.Error("driver.update_location.get_driver", err)
		return nil, fmt.Errorf("failed to get driver: %w", err)
	}

	// Only allow location updates if driver is online
	if driver.Status == domain.DriverStatusOffline {
		return nil, fmt.Errorf("cannot update location while offline")
	}

	// Create location update
	locationUpdate := &domain.LocationUpdate{
		DriverID:       driverID,
		Latitude:       req.Latitude,
		Longitude:      req.Longitude,
		AccuracyMeters: req.AccuracyMeters,
		SpeedKmh:       req.SpeedKmh,
		HeadingDegrees: req.HeadingDegrees,
		Timestamp:      time.Now(),
	}

	// Update location in database
	coord, err := s.repo.UpdateLocation(ctx, locationUpdate)
	if err != nil {
		s.logger.Error("driver.update_location.update", err)
		return nil, fmt.Errorf("failed to update location: %w", err)
	}

	// Broadcast location update via fanout exchange
	broadcast := &domain.LocationBroadcast{
		DriverID: driverID,
		Location: domain.Location{
			Latitude:  req.Latitude,
			Longitude: req.Longitude,
		},
		SpeedKmh:       req.SpeedKmh,
		HeadingDegrees: req.HeadingDegrees,
		Timestamp:      time.Now(),
	}

	if err := s.messageBroker.PublishLocationUpdate(ctx, broadcast); err != nil {
		s.logger.Error("driver.update_location.broadcast", err)
		// Don't fail the request if broadcasting fails
	}

	return &domain.UpdateLocationResponse{
		CoordinateID: coord.ID,
		UpdatedAt:    coord.UpdatedAt,
	}, nil
}

// StartRide marks a driver as busy and starts tracking a ride
func (s *driverService) StartRide(ctx context.Context, driverID string, req *domain.StartRideRequest) (*domain.StartRideResponse, error) {
	s.logger.Info("driver.start_ride", fmt.Sprintf("Driver %s starting ride %s", driverID, req.RideID))

	// Get driver
	driver, err := s.repo.GetByID(ctx, driverID)
	if err != nil {
		s.logger.Error("driver.start_ride.get_driver", err)
		return nil, fmt.Errorf("failed to get driver: %w", err)
	}

	// Verify driver is available
	if driver.Status != domain.DriverStatusAvailable && driver.Status != domain.DriverStatusEnRoute {
		return nil, fmt.Errorf("driver must be available or en route to start a ride")
	}

	// Update driver status to BUSY
	if err := s.repo.UpdateStatus(ctx, driverID, domain.DriverStatusBusy); err != nil {
		s.logger.Error("driver.start_ride.update_status", err)
		return nil, fmt.Errorf("failed to update driver status: %w", err)
	}

	// Update driver location
	locationUpdate := &domain.LocationUpdate{
		DriverID:  driverID,
		Latitude:  req.DriverLocation.Latitude,
		Longitude: req.DriverLocation.Longitude,
		RideID:    &req.RideID,
		Timestamp: time.Now(),
	}

	_, err = s.repo.UpdateLocation(ctx, locationUpdate)
	if err != nil {
		s.logger.Error("driver.start_ride.update_location", err)
		return nil, fmt.Errorf("failed to update location: %w", err)
	}

	// Publish driver status update
	statusUpdate := &domain.DriverStatusUpdate{
		DriverID:  driverID,
		Status:    domain.DriverStatusBusy,
		RideID:    &req.RideID,
		Timestamp: time.Now(),
	}

	if err := s.messageBroker.PublishDriverStatusUpdate(ctx, statusUpdate); err != nil {
		s.logger.Error("driver.start_ride.publish_status", err)
	}

	s.logger.Info("driver.start_ride.success", fmt.Sprintf("Driver %s started ride %s", driverID, req.RideID))

	return &domain.StartRideResponse{
		RideID:    req.RideID,
		Status:    domain.DriverStatusBusy,
		StartedAt: time.Now(),
		Message:   "Ride started successfully",
	}, nil
}

// CompleteRide marks a ride as complete and updates driver stats
func (s *driverService) CompleteRide(ctx context.Context, driverID string, req *domain.CompleteRideRequest) (*domain.CompleteRideResponse, error) {
	s.logger.Info("driver.complete_ride", fmt.Sprintf("Driver %s completing ride %s", driverID, req.RideID))

	// Get driver
	driver, err := s.repo.GetByID(ctx, driverID)
	if err != nil {
		s.logger.Error("driver.complete_ride.get_driver", err)
		return nil, fmt.Errorf("failed to get driver: %w", err)
	}

	// Verify driver is busy
	if driver.Status != domain.DriverStatusBusy {
		return nil, fmt.Errorf("driver must be busy to complete a ride")
	}

	// Calculate driver earnings (80% of estimated fare, simplified)
	// In a real system, this would come from the ride service
	driverEarnings := req.ActualDistanceKm * 200.0 // Simplified calculation

	// Update driver status to AVAILABLE
	if err := s.repo.UpdateStatus(ctx, driverID, domain.DriverStatusAvailable); err != nil {
		s.logger.Error("driver.complete_ride.update_status", err)
		return nil, fmt.Errorf("failed to update driver status: %w", err)
	}

	// Update driver stats
	newTotalRides := driver.TotalRides + 1
	newTotalEarnings := driver.TotalEarnings + driverEarnings

	if err := s.repo.UpdateStats(ctx, driverID, newTotalRides, newTotalEarnings); err != nil {
		s.logger.Error("driver.complete_ride.update_stats", err)
		return nil, fmt.Errorf("failed to update driver stats: %w", err)
	}

	// Update session stats
	activeSession, err := s.repo.GetActiveSession(ctx, driverID)
	if err != nil {
		s.logger.Error("driver.complete_ride.get_session", err)
	} else if activeSession != nil {
		sessionRides := activeSession.TotalRides + 1
		sessionEarnings := activeSession.TotalEarnings + driverEarnings

		if err := s.repo.UpdateSessionStats(ctx, activeSession.ID, sessionRides, sessionEarnings); err != nil {
			s.logger.Error("driver.complete_ride.update_session_stats", err)
		}
	}

	// Update final location
	locationUpdate := &domain.LocationUpdate{
		DriverID:  driverID,
		Latitude:  req.FinalLocation.Latitude,
		Longitude: req.FinalLocation.Longitude,
		RideID:    &req.RideID,
		Timestamp: time.Now(),
	}

	_, err = s.repo.UpdateLocation(ctx, locationUpdate)
	if err != nil {
		s.logger.Error("driver.complete_ride.update_location", err)
	}

	// Publish driver status update
	statusUpdate := &domain.DriverStatusUpdate{
		DriverID:  driverID,
		Status:    domain.DriverStatusAvailable,
		Timestamp: time.Now(),
	}

	if err := s.messageBroker.PublishDriverStatusUpdate(ctx, statusUpdate); err != nil {
		s.logger.Error("driver.complete_ride.publish_status", err)
	}

	s.logger.Info("driver.complete_ride.success", fmt.Sprintf("Driver %s completed ride %s", driverID, req.RideID))

	return &domain.CompleteRideResponse{
		RideID:         req.RideID,
		Status:         domain.DriverStatusAvailable,
		CompletedAt:    time.Now(),
		DriverEarnings: driverEarnings,
		Message:        "Ride completed successfully",
	}, nil
}

// GetDriver retrieves driver information
func (s *driverService) GetDriver(ctx context.Context, driverID string) (*domain.Driver, error) {
	driver, err := s.repo.GetByID(ctx, driverID)
	if err != nil {
		s.logger.Error("driver.get_driver", err)
		return nil, fmt.Errorf("failed to get driver: %w", err)
	}

	// Get active session if exists
	activeSession, err := s.repo.GetActiveSession(ctx, driverID)
	if err != nil {
		s.logger.Error("driver.get_driver.get_session", err)
	} else {
		driver.CurrentSession = activeSession
	}

	return driver, nil
}

// GetNearbyDrivers finds available drivers near a location
func (s *driverService) GetNearbyDrivers(ctx context.Context, lat, lng float64, radiusKm float64, vehicleType string, limit int) ([]*domain.NearbyDriver, error) {
	s.logger.Info("driver.get_nearby", fmt.Sprintf("Finding drivers near lat=%f, lng=%f", lat, lng))

	drivers, err := s.repo.GetNearbyDrivers(ctx, lat, lng, radiusKm, vehicleType, limit)
	if err != nil {
		s.logger.Error("driver.get_nearby", err)
		return nil, fmt.Errorf("failed to get nearby drivers: %w", err)
	}

	s.logger.Info("driver.get_nearby.success", fmt.Sprintf("Found %d nearby drivers", len(drivers)))

	return drivers, nil
}

// HandleRideRequest processes incoming ride requests and finds drivers
func (s *driverService) HandleRideRequest(ctx context.Context, request *domain.RideRequest) error {
	s.logger.Info("driver.handle_ride_request", fmt.Sprintf("Processing ride request %s", request.RideID))

	// Find nearby drivers
	drivers, err := s.repo.GetNearbyDrivers(
		ctx,
		request.PickupLocation.Latitude,
		request.PickupLocation.Longitude,
		request.MaxDistanceKm,
		request.RideType,
		10, // Max 10 drivers
	)

	if err != nil {
		s.logger.Error("driver.handle_ride_request.get_nearby", err)
		return fmt.Errorf("failed to find nearby drivers: %w", err)
	}

	if len(drivers) == 0 {
		s.logger.Info("driver.handle_ride_request.no_drivers", "No available drivers found")
		// Send response indicating no drivers available
		response := &domain.DriverMatchResponse{
			RideID:        request.RideID,
			Accepted:      false,
			CorrelationID: request.CorrelationID,
		}
		return s.messageBroker.PublishDriverResponse(ctx, response)
	}

	// Send ride offers to drivers via WebSocket
	timeout := time.Duration(request.TimeoutSeconds) * time.Second
	offerExpiry := time.Now().Add(timeout)

	for _, driver := range drivers {
		// Check if driver is connected via WebSocket
		if !s.wsHub.IsDriverConnected(driver.DriverID) {
			continue
		}

		// Calculate driver earnings (80% of fare)
		driverEarnings := request.EstimatedFare * 0.8

		// Estimate ride duration (simplified: distance / average speed)
		estimatedDuration := int(request.MaxDistanceKm / 40.0 * 60) // 40 km/h average

		offer := &domain.RideOffer{
			Type:                         "ride_offer",
			OfferID:                      fmt.Sprintf("offer_%s_%s", request.RideID, driver.DriverID),
			RideID:                       request.RideID,
			RideNumber:                   request.RideNumber,
			PickupLocation:               request.PickupLocation,
			DestinationLocation:          request.DestinationLocation,
			EstimatedFare:                request.EstimatedFare,
			DriverEarnings:               driverEarnings,
			DistanceToPickupKm:           driver.DistanceKm,
			EstimatedRideDurationMinutes: estimatedDuration,
			ExpiresAt:                    offerExpiry,
		}

		if err := s.wsHub.SendRideOffer(driver.DriverID, offer); err != nil {
			s.logger.Error("driver.handle_ride_request.send_offer", err)
			continue
		}

		s.logger.Info("driver.handle_ride_request.offer_sent", fmt.Sprintf("Sent offer to driver %s", driver.DriverID))
	}

	return nil
}

// HandleRideStatusUpdate processes ride status updates
func (s *driverService) HandleRideStatusUpdate(ctx context.Context, update *domain.RideStatusUpdate) error {
	s.logger.Info("driver.handle_ride_status", fmt.Sprintf("Processing ride status update: %s - %s", update.RideID, update.Status))

	// Handle different status updates
	switch update.Status {
	case "COMPLETED":
		// Update can be handled by the complete ride endpoint
		s.logger.Info("driver.handle_ride_status.completed", fmt.Sprintf("Ride %s completed", update.RideID))

	case "CANCELLED":
		// Mark driver as available again
		s.logger.Info("driver.handle_ride_status.cancelled", fmt.Sprintf("Ride %s cancelled", update.RideID))

	default:
		s.logger.Debug("driver.handle_ride_status.unknown", fmt.Sprintf("Unknown status: %s", update.Status))
	}

	return nil
}
