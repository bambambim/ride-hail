package app

import (
	"context"
	"encoding/json"
	"fmt"
	"sync"
	"time"

	"ride-hail/internal/driver_location_service/domain"
	"ride-hail/pkg/logger"
)

// DriverLocationService is the application service handling driver location business logic
type DriverLocationService struct {
	log       logger.Logger
	repo      domain.DriverLocationRepository
	publisher domain.DriverLocationPublisher
	wsMgr     domain.WebSocketManager

	// Track pending ride offers with timeouts
	pendingOffers   map[string]*RideOffer // offerID -> RideOffer
	offerMu         sync.RWMutex
	locationLimiter map[string]time.Time // driverID -> last update time
	limiterMu       sync.RWMutex
}

// RideOffer represents a pending ride offer to a driver
type RideOffer struct {
	OfferID     string
	RideID      string
	DriverID    string
	RideRequest *domain.RideMatchingRequest
	ExpiresAt   time.Time
	Cancelled   bool
}

func NewDriverLocationService(
	log logger.Logger,
	repo domain.DriverLocationRepository,
	publisher domain.DriverLocationPublisher,
	wsMgr domain.WebSocketManager,
) *DriverLocationService {
	return &DriverLocationService{
		log:             log,
		repo:            repo,
		publisher:       publisher,
		wsMgr:           wsMgr,
		pendingOffers:   make(map[string]*RideOffer),
		locationLimiter: make(map[string]time.Time),
	}
}

// DriverGoOnline handles driver going online
func (s *DriverLocationService) DriverGoOnline(ctx context.Context, driverID string, latitude, longitude float64, address string) (string, error) {
	log := s.log.WithFields(logger.LogFields{"driver_id": driverID})
	log.Info("driver_going_online", "Driver attempting to go online")

	// Validate driver exists
	_, err := s.repo.GetDriver(ctx, driverID)
	if err != nil {
		log.Error("get_driver_failed", err)
		return "", fmt.Errorf("failed to get driver: %w", err)
	}

	// if !driver.IsVerified {
	// 	return "", fmt.Errorf("driver not verified")
	// }

	// Create new session
	sessionID, err := s.repo.CreateDriverSession(ctx, driverID)
	if err != nil {
		log.Error("create_session_failed", err)
		return "", fmt.Errorf("failed to create session: %w", err)
	}

	// Save initial location
	_, err = s.repo.SaveDriverLocation(ctx, driverID, latitude, longitude, address)
	if err != nil {
		log.Error("save_location_failed", err)
		return "", fmt.Errorf("failed to save location: %w", err)
	}

	// Update driver status to AVAILABLE
	err = s.repo.UpdateDriverStatus(ctx, driverID, domain.DriverStatusAvailable)
	if err != nil {
		log.Error("update_status_failed", err)
		return "", fmt.Errorf("failed to update status: %w", err)
	}

	// Publish driver status update
	statusUpdate := map[string]interface{}{
		"driver_id": driverID,
		"status":    domain.DriverStatusAvailable,
		"timestamp": time.Now().Format(time.RFC3339),
	}
	statusData, _ := json.Marshal(statusUpdate)
	if err := s.publisher.PublishDriverStatus(ctx, "driver_topic", fmt.Sprintf("driver.status.%s", driverID), statusData); err != nil {
		log.Error("publish_driver_status_failed", err)
	}

	log.Info("driver_online_success", fmt.Sprintf("Driver online with session %s", sessionID))
	return sessionID, nil
}

// DriverGoOffline handles driver going offline
func (s *DriverLocationService) DriverGoOffline(ctx context.Context, driverID string) (*domain.DriverSession, error) {
	log := s.log.WithFields(logger.LogFields{"driver_id": driverID})
	log.Info("driver_going_offline", "Driver attempting to go offline")

	// Get active session
	session, err := s.repo.GetActiveSession(ctx, driverID)
	if err != nil {
		log.Error("get_session_failed", err)
		return nil, fmt.Errorf("failed to get session: %w", err)
	}

	if session == nil {
		return nil, fmt.Errorf("no active session found")
	}

	// End session
	endedSession, err := s.repo.EndDriverSession(ctx, session.ID)
	if err != nil {
		log.Error("end_session_failed", err)
		return nil, fmt.Errorf("failed to end session: %w", err)
	}

	// Update driver status to OFFLINE
	err = s.repo.UpdateDriverStatus(ctx, driverID, domain.DriverStatusOffline)
	if err != nil {
		log.Error("update_status_failed", err)
		return nil, fmt.Errorf("failed to update status: %w", err)
	}

	// Publish driver status update
	statusUpdate := map[string]interface{}{
		"driver_id": driverID,
		"status":    domain.DriverStatusOffline,
		"timestamp": time.Now().Format(time.RFC3339),
	}
	statusData, _ := json.Marshal(statusUpdate)
	if err := s.publisher.PublishDriverStatus(ctx, "driver_topic", fmt.Sprintf("driver.status.%s", driverID), statusData); err != nil {
		log.Error("publish_driver_status_failed", err)
	}

	log.Info("driver_offline_success", "Driver now offline")
	return endedSession, nil
}

// UpdateDriverLocation updates driver's current location with rate limiting
func (s *DriverLocationService) UpdateDriverLocation(ctx context.Context, driverID string, latitude, longitude, accuracy, speed, heading float64, address string) (string, error) {
	log := s.log.WithFields(logger.LogFields{"driver_id": driverID})

	// Rate limit: max 1 update per 3 seconds
	s.limiterMu.Lock()
	lastUpdate, exists := s.locationLimiter[driverID]
	if exists && time.Since(lastUpdate) < 3*time.Second {
		s.limiterMu.Unlock()
		return "", fmt.Errorf("rate limit exceeded: max 1 update per 3 seconds")
	}
	s.locationLimiter[driverID] = time.Now()
	s.limiterMu.Unlock()

	// Save location as current
	coordinateID, err := s.repo.SaveDriverLocation(ctx, driverID, latitude, longitude, address)
	if err != nil {
		log.Error("save_location_failed", err)
		return "", fmt.Errorf("failed to save location: %w", err)
	}

	// Archive to location history with metrics
	// Get current ride ID if driver is busy
	rideID := "" // Could be retrieved from driver state
	err = s.repo.ArchiveLocation(ctx, driverID, latitude, longitude, accuracy, speed, heading, rideID)
	if err != nil {
		log.Error("archive_location_failed", err)
		// Don't fail the request if archiving fails
	}

	// Publish location update to fanout exchange
	locationUpdate := map[string]interface{}{
		"driver_id":       driverID,
		"ride_id":         rideID,
		"location":        map[string]float64{"latitude": latitude, "longitude": longitude},
		"speed_kmh":       speed,
		"heading_degrees": heading,
		"timestamp":       time.Now().Format(time.RFC3339),
	}
	updateData, _ := json.Marshal(locationUpdate)
	if err := s.publisher.PublishLocationUpdate(ctx, "location_fanout", updateData); err != nil {
		log.Error("publish_location_failed", err)
	}

	log.Debug("location_updated", "Location update successful")
	return coordinateID, nil
}

// HandleRideMatchingRequest processes incoming ride requests for matching
func (s *DriverLocationService) HandleRideMatchingRequest(ctx context.Context, req *domain.RideMatchingRequest) error {
	log := s.log.WithFields(logger.LogFields{
		"ride_id":        req.RideID,
		"correlation_id": req.CorrelationID,
	})
	log.Info("ride_matching_request", "Processing ride matching request")

	// Find nearby available drivers
	radiusMeters := 5000.0 // 5km default
	if req.MaxDistanceKM > 0 {
		radiusMeters = req.MaxDistanceKM * 1000
	}

	nearbyDrivers, err := s.repo.FindNearbyDrivers(ctx, req.PickupLocation.Lat, req.PickupLocation.Lng, req.RideType, radiusMeters, 10)
	if err != nil {
		log.Error("find_drivers_failed", err)
		return fmt.Errorf("failed to find nearby drivers: %w", err)
	}

	if len(nearbyDrivers) == 0 {
		log.Info("no_drivers_available", "No drivers found for matching")
		// Send rejection response
		s.sendDriverResponse(ctx, req.RideID, "", false, "No drivers available", req.CorrelationID)
		return nil
	}

	log.Info("drivers_found", fmt.Sprintf("Found %d nearby drivers", len(nearbyDrivers)))

	// Send ride offers to drivers (with timeout)
	timeout := 30 * time.Second
	if req.TimeoutSeconds > 0 {
		timeout = time.Duration(req.TimeoutSeconds) * time.Second
	}

	for _, driver := range nearbyDrivers {
		// Check if driver is WebSocket connected
		if !s.wsMgr.IsDriverConnected(driver.DriverID) {
			log.Debug("driver_not_connected", fmt.Sprintf("Driver %s not connected", driver.DriverID))
			continue
		}

		// Create offer
		offerID := fmt.Sprintf("offer_%s_%s", req.RideID, driver.DriverID)
		offer := &RideOffer{
			OfferID:     offerID,
			RideID:      req.RideID,
			DriverID:    driver.DriverID,
			RideRequest: req,
			ExpiresAt:   time.Now().Add(timeout),
		}

		// Store pending offer
		s.offerMu.Lock()
		s.pendingOffers[offerID] = offer
		s.offerMu.Unlock()

		// Send offer via WebSocket
		offerMsg := map[string]interface{}{
			"offer_id":                        offerID,
			"ride_id":                         req.RideID,
			"ride_number":                     req.RideNumber,
			"pickup_location":                 req.PickupLocation,
			"destination_location":            req.DestinationLocation,
			"estimated_fare":                  req.EstimatedFare,
			"driver_earnings":                 req.EstimatedFare * 0.8, // 80% for driver
			"distance_to_pickup_km":           driver.DistanceKm,
			"estimated_ride_duration_minutes": 15, // Placeholder
			"expires_at":                      offer.ExpiresAt.Format(time.RFC3339),
		}

		err = s.wsMgr.SendRideOffer(driver.DriverID, offerMsg)
		if err != nil {
			log.Error("send_offer_failed", err)
			continue
		}

		log.Info("offer_sent", fmt.Sprintf("Ride offer sent to driver %s", driver.DriverID))

		// Set timeout to cancel offer
		go s.handleOfferTimeout(offer)
	}

	return nil
}

// handleOfferTimeout cancels offer if not accepted within timeout
func (s *DriverLocationService) handleOfferTimeout(offer *RideOffer) {
	time.Sleep(time.Until(offer.ExpiresAt))

	s.offerMu.Lock()
	defer s.offerMu.Unlock()

	// Check if offer still pending
	if existingOffer, exists := s.pendingOffers[offer.OfferID]; exists && !existingOffer.Cancelled {
		existingOffer.Cancelled = true
		delete(s.pendingOffers, offer.OfferID)
		s.log.Info("offer_expired", fmt.Sprintf("Offer %s expired", offer.OfferID))
	}
}

// HandleDriverRideResponse processes driver's acceptance/rejection
func (s *DriverLocationService) HandleDriverRideResponse(ctx context.Context, driverID string, offerID string, rideID string, accepted bool) error {
	log := s.log.WithFields(logger.LogFields{
		"driver_id": driverID,
		"ride_id":   rideID,
		"offer_id":  offerID,
	})

	// Get offer
	s.offerMu.Lock()
	offer, exists := s.pendingOffers[offerID]
	if !exists || offer.Cancelled {
		s.offerMu.Unlock()
		log.Info("offer_not_found", "Offer not found or expired")
		return fmt.Errorf("offer not found or expired")
	}

	// Mark as handled
	delete(s.pendingOffers, offerID)
	s.offerMu.Unlock()

	if !accepted {
		log.Info("driver_rejected", "Driver rejected ride offer")
		// Could try next driver in the list
		return nil
	}

	log.Info("driver_accepted", "Driver accepted ride offer")

	// Update driver status to EN_ROUTE
	err := s.repo.UpdateDriverStatus(ctx, driverID, domain.DriverStatusEnRoute)
	if err != nil {
		log.Error("update_status_failed", err)
		return fmt.Errorf("failed to update driver status: %w", err)
	}

	// Set current ride
	err = s.repo.SetDriverCurrentRide(ctx, driverID, rideID)
	if err != nil {
		log.Error("set_ride_failed", err)
	}

	// Get driver info
	driver, err := s.repo.GetDriver(ctx, driverID)
	if err != nil {
		log.Error("get_driver_failed", err)
		return fmt.Errorf("failed to get driver: %w", err)
	}

	// Send driver response to ride service
	s.sendDriverResponse(ctx, rideID, driverID, true, "", offer.RideRequest.CorrelationID)

	// Send ride details back to driver via WebSocket
	rideDetails := map[string]interface{}{
		"ride_id":         rideID,
		"passenger_name":  "Passenger", // Would fetch from ride service
		"pickup_location": offer.RideRequest.PickupLocation,
	}
	if driver != nil {
		rideDetails["driver_info"] = map[string]interface{}{
			"email":   driver.Email,
			"rating":  driver.Rating,
			"vehicle": driver.VehicleAttrs,
		}
	}
	s.wsMgr.SendRideDetails(driverID, rideDetails)

	// Publish driver status
	statusUpdate := map[string]interface{}{
		"driver_id": driverID,
		"status":    domain.DriverStatusEnRoute,
		"ride_id":   rideID,
		"timestamp": time.Now().Format(time.RFC3339),
	}
	statusData, _ := json.Marshal(statusUpdate)
	if err := s.publisher.PublishDriverStatus(ctx, "driver_topic", fmt.Sprintf("driver.status.%s", driverID), statusData); err != nil {
		log.Error("publish_driver_status_failed", err)
	}

	log.Info("ride_matched", "Ride successfully matched to driver")

	return nil
}

// sendDriverResponse sends the driver match response back to ride service
func (s *DriverLocationService) sendDriverResponse(ctx context.Context, rideID, driverID string, accepted bool, reason string, correlationID string) {
	response := map[string]interface{}{
		"ride_id":        rideID,
		"driver_id":      driverID,
		"accepted":       accepted,
		"correlation_id": correlationID,
		"timestamp":      time.Now().Format(time.RFC3339),
	}

	if !accepted {
		response["reason"] = reason
	} else {
		// Add driver info
		driver, err := s.repo.GetDriver(ctx, driverID)
		if err == nil {
			response["driver_info"] = map[string]interface{}{
				"name":    driver.Email, // Would use actual name from attrs
				"rating":  driver.Rating,
				"vehicle": driver.VehicleAttrs,
			}

			// Add location
			location, _ := s.repo.GetCurrentLocation(ctx, driverID)
			if location != nil {
				response["driver_location"] = map[string]float64{
					"latitude":  location.Latitude,
					"longitude": location.Longitude,
				}
				response["estimated_arrival_minutes"] = 3 // Placeholder
			}
		}
	}

	responseData, _ := json.Marshal(response)
	if err := s.publisher.PublishDriverResponse(ctx, "driver_topic", fmt.Sprintf("driver.response.%s", rideID), responseData); err != nil {
		s.log.Error("publish_driver_response_failed", err)
	}
}

// StartRide handles driver starting the ride
func (s *DriverLocationService) StartRide(ctx context.Context, driverID string, rideID string) error {
	log := s.log.WithFields(logger.LogFields{"driver_id": driverID, "ride_id": rideID})
	log.Info("ride_starting", "Driver starting ride")

	// Update driver status to BUSY (in progress)
	err := s.repo.UpdateDriverStatus(ctx, driverID, domain.DriverStatusBusy)
	if err != nil {
		log.Error("update_status_failed", err)
		return fmt.Errorf("failed to update status: %w", err)
	}

	// Publish status update
	statusUpdate := map[string]interface{}{
		"driver_id": driverID,
		"ride_id":   rideID,
		"status":    "IN_PROGRESS",
		"timestamp": time.Now().Format(time.RFC3339),
	}
	statusData, _ := json.Marshal(statusUpdate)
	if err := s.publisher.PublishDriverStatus(ctx, "driver_topic", fmt.Sprintf("driver.status.%s", driverID), statusData); err != nil {
		log.Error("publish_driver_status_failed", err)
	}

	log.Info("ride_started", "Ride started successfully")
	return nil
}

// CompleteRide handles driver completing the ride
func (s *DriverLocationService) CompleteRide(ctx context.Context, driverID string, rideID string, actualDistanceKM float64, actualDurationMin int) (float64, error) {
	log := s.log.WithFields(logger.LogFields{"driver_id": driverID, "ride_id": rideID})
	log.Info("ride_completing", "Driver completing ride")

	// Calculate earnings (80% of fare)
	// In real implementation, would fetch actual fare from ride service
	// earnings := 1216.0 // Placeholder
	earnings, err := s.repo.GetEstimatedFare(ctx, rideID)
	if err != nil {
		log.Error("get_fare_failed", err)
		return 0, fmt.Errorf("failed to get fare: %w", err)
	}
	earnings = earnings * 0.8

	// Update session stats
	err = s.repo.UpdateDriverSessionStats(ctx, driverID, 1, earnings)
	if err != nil {
		log.Error("update_stats_failed", err)
	}

	// Clear current ride and set back to AVAILABLE
	err = s.repo.ClearDriverCurrentRide(ctx, driverID)
	if err != nil {
		log.Error("clear_ride_failed", err)
		return 0, fmt.Errorf("failed to clear ride: %w", err)
	}

	// Publish status update
	statusUpdate := map[string]interface{}{
		"driver_id": driverID,
		"ride_id":   rideID,
		"status":    "COMPLETED",
		"timestamp": time.Now().Format(time.RFC3339),
	}
	statusData, _ := json.Marshal(statusUpdate)
	if err := s.publisher.PublishDriverStatus(ctx, "driver_topic", fmt.Sprintf("driver.status.%s", driverID), statusData); err != nil {
		log.Error("publish_driver_status_failed", err)
	}

	log.Info("ride_completed", fmt.Sprintf("Ride completed, driver earned %.2f", earnings))
	return earnings, nil
}

// HandleRideStatusUpdate processes ride status updates from ride service
func (s *DriverLocationService) HandleRideStatusUpdate(ctx context.Context, rideID string, driverID string, status string, finalFare float64) error {
	log := s.log.WithFields(logger.LogFields{"ride_id": rideID, "driver_id": driverID})
	log.Info("ride_status_update", fmt.Sprintf("Ride status changed to %s", status))

	// Handle different statuses
	switch status {
	case "CANCELLED":
		log.Info("ride_cancelled", "Ride was cancelled")

		// Only act if we have a valid driver ID
		if driverID != "" {
			// 1. Update driver status back to AVAILABLE
			if err := s.repo.UpdateDriverStatus(ctx, driverID, domain.DriverStatusAvailable); err != nil {
				log.Error("cancel_update_status_failed", err)
			}

			// 2. Clear current ride assignment
			if err := s.repo.ClearDriverCurrentRide(ctx, driverID); err != nil {
				log.Error("cancel_clear_ride_failed", err)
			}

			// 3. Notify driver via WebSocket
			if s.wsMgr.IsDriverConnected(driverID) {
				if err := s.wsMgr.SendRideCancelled(driverID, rideID); err != nil {
					log.Error("send_cancel_notification_failed", err)
				}
			}

			// 4. Publish driver status update (Available)
			statusUpdate := map[string]interface{}{
				"driver_id": driverID,
				"status":    domain.DriverStatusAvailable,
				"timestamp": time.Now().Format(time.RFC3339),
			}
			statusData, _ := json.Marshal(statusUpdate)
			if err := s.publisher.PublishDriverStatus(ctx, "driver_topic", fmt.Sprintf("driver.status.%s", driverID), statusData); err != nil {
				log.Error("publish_driver_status_failed", err)
			}
		}

	case "COMPLETED":
		// Update earnings if needed
		log.Info("ride_completed_confirmed", fmt.Sprintf("Ride completed with fare %.2f", finalFare))
	}

	return nil
}
