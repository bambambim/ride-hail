BEGIN;

-----------------------------------------------------
-- USERS
-----------------------------------------------------

INSERT INTO users (id, email, role, status, password_hash, attrs)
VALUES
    ('11111111-1111-1111-1111-111111111111', 'passenger1@example.com', 'PASSENGER', 'ACTIVE', 'hash_passenger1', '{"name": "John Passenger"}'),
    ('22222222-2222-2222-2222-222222222222', 'driver1@example.com', 'DRIVER', 'ACTIVE', 'hash_driver1', '{"name": "Mike Driver"}'),
    ('33333333-3333-3333-3333-333333333333', 'admin@example.com', 'ADMIN', 'ACTIVE', 'hash_admin', '{"name": "Root Admin"}');

-----------------------------------------------------
-- DRIVERS TABLE
-----------------------------------------------------

INSERT INTO drivers (id, license_number, vehicle_type, vehicle_attrs, rating, total_rides, total_earnings, status, is_verified)
VALUES
(
    '22222222-2222-2222-2222-222222222222', 
    'KZ-DRIVER-001',
    'ECONOMY',
    '{"vehicle_make": "Toyota", "vehicle_model": "Camry", "vehicle_color": "White", "vehicle_plate": "KZ 123 ABC", "vehicle_year": 2020}',
    4.85,
    120,
    250000,
    'AVAILABLE',
    true
);

-----------------------------------------------------
-- COORDINATES (current driver location + pickup + destination)
-----------------------------------------------------

-- Driver current location
INSERT INTO coordinates (id, entity_id, entity_type, address, latitude, longitude, is_current)
VALUES
(
    '44444444-4444-4444-4444-444444444444',
    '22222222-2222-2222-2222-222222222222',
    'driver',
    'Almaty, Abay Ave 50',
    43.238949,
    76.889709,
    true
);

-- Passenger pickup point
INSERT INTO coordinates (id, entity_id, entity_type, address, latitude, longitude, is_current)
VALUES
(
    '55555555-5555-5555-5555-555555555555',
    '11111111-1111-1111-1111-111111111111',
    'passenger',
    'Almaty, Dostyk Ave 100',
    43.240200,
    76.915500,
    true
);

-- Passenger destination point
INSERT INTO coordinates (id, entity_id, entity_type, address, latitude, longitude, is_current)
VALUES
(
    '66666666-6666-6666-6666-666666666666',
    '11111111-1111-1111-1111-111111111111',
    'passenger',
    'Almaty, Mega Center',
    43.235600,
    76.885000,
    false
);

-----------------------------------------------------
-- RIDES
-----------------------------------------------------

INSERT INTO rides (
    id, ride_number, passenger_id, driver_id, vehicle_type, status, priority,
    estimated_fare, final_fare,
    pickup_coordinate_id,
    destination_coordinate_id,
    requested_at, matched_at, arrived_at, started_at
)
VALUES
(
    '77777777-7777-7777-7777-777777777777',
    'RIDE-0001',
    '11111111-1111-1111-1111-111111111111',
    '22222222-2222-2222-2222-222222222222',
    'ECONOMY',
    'IN_PROGRESS',
    1,
    1500,
    NULL,
    '55555555-5555-5555-5555-555555555555',
    '66666666-6666-6666-6666-666666666666',
    NOW() - INTERVAL '15 minutes',
    NOW() - INTERVAL '10 minutes',
    NOW() - INTERVAL '5 minutes',
    NOW() - INTERVAL '3 minutes'
);

-----------------------------------------------------
-- DRIVER SESSIONS
-----------------------------------------------------

INSERT INTO driver_sessions (id, driver_id, started_at, total_rides, total_earnings)
VALUES
(
    '88888888-8888-8888-8888-888888888888',
    '22222222-2222-2222-2222-222222222222',
    NOW() - INTERVAL '2 hours',
    3,
    4500
);

-----------------------------------------------------
-- LOCATION HISTORY
-----------------------------------------------------

INSERT INTO location_history (id, coordinate_id, driver_id, latitude, longitude, accuracy_meters, speed_kmh, heading_degrees, recorded_at, ride_id)
VALUES
(
    '99999999-9999-9999-9999-999999999999',
    '44444444-4444-4444-4444-444444444444',
    '22222222-2222-2222-2222-222222222222',
    43.239100,
    76.889900,
    4.5,
    32.0,
    180,
    NOW() - INTERVAL '2 minutes',
    '77777777-7777-7777-7777-777777777777'
);

-----------------------------------------------------
-- RIDE EVENTS
-----------------------------------------------------

INSERT INTO ride_events (id, ride_id, event_type, event_data)
VALUES
(
    'aaaaaaa1-aaaa-aaaa-aaaa-aaaaaaaaaaa1',
    '77777777-7777-7777-7777-777777777777',
    'RIDE_REQUESTED',
    '{"passenger_id": "11111111-1111-1111-1111-111111111111"}'
),
(
    'aaaaaaa2-aaaa-aaaa-aaaa-aaaaaaaaaaa2',
    '77777777-7777-7777-7777-777777777777',
    'DRIVER_MATCHED',
    '{"driver_id": "22222222-2222-2222-2222-222222222222"}'
),
(
    'aaaaaaa3-aaaa-aaaa-aaaa-aaaaaaaaaaa3',
    '77777777-7777-7777-7777-777777777777',
    'LOCATION_UPDATED',
    '{"lat": 43.239100, "lng": 76.889900, "speed": 32}'
);

COMMIT;
