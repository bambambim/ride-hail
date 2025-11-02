begin;

-- ============================================================================
-- MOCK DATA SEED SCRIPT
-- ============================================================================

-- Clear existing data (in reverse order of dependencies)
truncate table ride_events cascade;
truncate table location_history cascade;
truncate table driver_sessions cascade;
truncate table rides cascade;
truncate table coordinates cascade;
truncate table drivers cascade;
truncate table users cascade;

-- ============================================================================
-- USERS (Passengers, Drivers, Admins)
-- ============================================================================

-- Passengers (10 users)
insert into users (id, email, role, status, password_hash, attrs) values
('11111111-1111-1111-1111-111111111111', 'john.doe@example.com', 'PASSENGER', 'ACTIVE', '$2a$10$abcdefghijklmnopqrstuvwxyz', '{"first_name": "John", "last_name": "Doe", "phone": "+77771234567"}'::jsonb),
('11111111-1111-1111-1111-111111111112', 'jane.smith@example.com', 'PASSENGER', 'ACTIVE', '$2a$10$abcdefghijklmnopqrstuvwxyz', '{"first_name": "Jane", "last_name": "Smith", "phone": "+77771234568"}'::jsonb),
('11111111-1111-1111-1111-111111111113', 'mike.johnson@example.com', 'PASSENGER', 'ACTIVE', '$2a$10$abcdefghijklmnopqrstuvwxyz', '{"first_name": "Mike", "last_name": "Johnson", "phone": "+77771234569"}'::jsonb),
('11111111-1111-1111-1111-111111111114', 'sarah.williams@example.com', 'PASSENGER', 'ACTIVE', '$2a$10$abcdefghijklmnopqrstuvwxyz', '{"first_name": "Sarah", "last_name": "Williams", "phone": "+77771234570"}'::jsonb),
('11111111-1111-1111-1111-111111111115', 'david.brown@example.com', 'PASSENGER', 'ACTIVE', '$2a$10$abcdefghijklmnopqrstuvwxyz', '{"first_name": "David", "last_name": "Brown", "phone": "+77771234571"}'::jsonb),
('11111111-1111-1111-1111-111111111116', 'emily.jones@example.com', 'PASSENGER', 'ACTIVE', '$2a$10$abcdefghijklmnopqrstuvwxyz', '{"first_name": "Emily", "last_name": "Jones", "phone": "+77771234572"}'::jsonb),
('11111111-1111-1111-1111-111111111117', 'chris.miller@example.com', 'PASSENGER', 'ACTIVE', '$2a$10$abcdefghijklmnopqrstuvwxyz', '{"first_name": "Chris", "last_name": "Miller", "phone": "+77771234573"}'::jsonb),
('11111111-1111-1111-1111-111111111118', 'lisa.davis@example.com', 'PASSENGER', 'ACTIVE', '$2a$10$abcdefghijklmnopqrstuvwxyz', '{"first_name": "Lisa", "last_name": "Davis", "phone": "+77771234574"}'::jsonb),
('11111111-1111-1111-1111-111111111119', 'robert.garcia@example.com', 'PASSENGER', 'INACTIVE', '$2a$10$abcdefghijklmnopqrstuvwxyz', '{"first_name": "Robert", "last_name": "Garcia", "phone": "+77771234575"}'::jsonb),
('11111111-1111-1111-1111-111111111120', 'maria.rodriguez@example.com', 'PASSENGER', 'ACTIVE', '$2a$10$abcdefghijklmnopqrstuvwxyz', '{"first_name": "Maria", "last_name": "Rodriguez", "phone": "+77771234576"}'::jsonb);

-- Drivers (8 users)
insert into users (id, email, role, status, password_hash, attrs) values
('22222222-2222-2222-2222-222222222221', 'driver1@example.com', 'DRIVER', 'ACTIVE', '$2a$10$abcdefghijklmnopqrstuvwxyz', '{"first_name": "Aibek", "last_name": "Nurlan", "phone": "+77051234567"}'::jsonb),
('22222222-2222-2222-2222-222222222222', 'driver2@example.com', 'DRIVER', 'ACTIVE', '$2a$10$abcdefghijklmnopqrstuvwxyz', '{"first_name": "Nurlan", "last_name": "Serik", "phone": "+77051234568"}'::jsonb),
('22222222-2222-2222-2222-222222222223', 'driver3@example.com', 'DRIVER', 'ACTIVE', '$2a$10$abcdefghijklmnopqrstuvwxyz', '{"first_name": "Arman", "last_name": "Kuanysh", "phone": "+77051234569"}'::jsonb),
('22222222-2222-2222-2222-222222222224', 'driver4@example.com', 'DRIVER', 'ACTIVE', '$2a$10$abcdefghijklmnopqrstuvwxyz', '{"first_name": "Serik", "last_name": "Askar", "phone": "+77051234570"}'::jsonb),
('22222222-2222-2222-2222-222222222225', 'driver5@example.com', 'DRIVER', 'ACTIVE', '$2a$10$abcdefghijklmnopqrstuvwxyz', '{"first_name": "Daniyar", "last_name": "Murat", "phone": "+77051234571"}'::jsonb),
('22222222-2222-2222-2222-222222222226', 'driver6@example.com', 'DRIVER', 'ACTIVE', '$2a$10$abcdefghijklmnopqrstuvwxyz', '{"first_name": "Yerlan", "last_name": "Bauyrzhan", "phone": "+77051234572"}'::jsonb),
('22222222-2222-2222-2222-222222222227', 'driver7@example.com', 'DRIVER', 'INACTIVE', '$2a$10$abcdefghijklmnopqrstuvwxyz', '{"first_name": "Timur", "last_name": "Dias", "phone": "+77051234573"}'::jsonb),
('22222222-2222-2222-2222-222222222228', 'driver8@example.com', 'DRIVER', 'ACTIVE', '$2a$10$abcdefghijklmnopqrstuvwxyz', '{"first_name": "Kanat", "last_name": "Azamat", "phone": "+77051234574"}'::jsonb);

-- Admin (1 user)
insert into users (id, email, role, status, password_hash, attrs) values
('33333333-3333-3333-3333-333333333331', 'admin@example.com', 'ADMIN', 'ACTIVE', '$2a$10$abcdefghijklmnopqrstuvwxyz', '{"first_name": "Admin", "last_name": "User", "phone": "+77071234567"}'::jsonb);

-- ============================================================================
-- DRIVERS
-- ============================================================================

insert into drivers (id, license_number, vehicle_type, vehicle_attrs, rating, total_rides, total_earnings, status, is_verified) values
('22222222-2222-2222-2222-222222222221', 'KZ-ALM-000001', 'ECONOMY', '{"vehicle_make": "Toyota", "vehicle_model": "Camry", "vehicle_color": "White", "vehicle_plate": "KZ 123 ABC", "vehicle_year": 2020}'::jsonb, 4.85, 245, 98000.00, 'AVAILABLE', true),
('22222222-2222-2222-2222-222222222222', 'KZ-ALM-000002', 'ECONOMY', '{"vehicle_make": "Hyundai", "vehicle_model": "Elantra", "vehicle_color": "Black", "vehicle_plate": "KZ 456 DEF", "vehicle_year": 2021}'::jsonb, 4.92, 312, 124800.00, 'AVAILABLE', true),
('22222222-2222-2222-2222-222222222223', 'KZ-ALM-000003', 'PREMIUM', '{"vehicle_make": "BMW", "vehicle_model": "5 Series", "vehicle_color": "Silver", "vehicle_plate": "KZ 789 GHI", "vehicle_year": 2022}'::jsonb, 4.95, 189, 113400.00, 'BUSY', true),
('22222222-2222-2222-2222-222222222224', 'KZ-ALM-000004', 'ECONOMY', '{"vehicle_make": "Kia", "vehicle_model": "Rio", "vehicle_color": "Red", "vehicle_plate": "KZ 101 JKL", "vehicle_year": 2019}'::jsonb, 4.78, 567, 226800.00, 'EN_ROUTE', true),
('22222222-2222-2222-2222-222222222225', 'KZ-ALM-000005', 'XL', '{"vehicle_make": "Toyota", "vehicle_model": "Alphard", "vehicle_color": "Black", "vehicle_plate": "KZ 202 MNO", "vehicle_year": 2021}'::jsonb, 4.88, 134, 80400.00, 'AVAILABLE', true),
('22222222-2222-2222-2222-222222222226', 'KZ-ALM-000006', 'PREMIUM', '{"vehicle_make": "Mercedes-Benz", "vehicle_model": "E-Class", "vehicle_color": "Blue", "vehicle_plate": "KZ 303 PQR", "vehicle_year": 2023}'::jsonb, 4.97, 98, 73500.00, 'OFFLINE', true),
('22222222-2222-2222-2222-222222222227', 'KZ-ALM-000007', 'ECONOMY', '{"vehicle_make": "Volkswagen", "vehicle_model": "Polo", "vehicle_color": "Gray", "vehicle_plate": "KZ 404 STU", "vehicle_year": 2018}'::jsonb, 4.65, 412, 164800.00, 'OFFLINE', false),
('22222222-2222-2222-2222-222222222228', 'KZ-ALM-000008', 'XL', '{"vehicle_make": "Hyundai", "vehicle_model": "Starex", "vehicle_color": "White", "vehicle_plate": "KZ 505 VWX", "vehicle_year": 2020}'::jsonb, 4.89, 201, 120600.00, 'AVAILABLE', true);

-- ============================================================================
-- COORDINATES (Pickup and Destination locations in Almaty)
-- ============================================================================

insert into coordinates (id, entity_id, entity_type, address, latitude, longitude, fare_amount, distance_km, duration_minutes, is_current) values
-- Pickup locations
('c0000000-0000-0000-0000-000000000001', '11111111-1111-1111-1111-111111111111', 'passenger', 'Abay Ave 52, Almaty', 43.238949, 76.889709, null, null, null, false),
('c0000000-0000-0000-0000-000000000002', '11111111-1111-1111-1111-111111111112', 'passenger', 'Dostyk Ave 162, Almaty', 43.232150, 76.943214, null, null, null, false),
('c0000000-0000-0000-0000-000000000003', '11111111-1111-1111-1111-111111111113', 'passenger', 'Al-Farabi Ave 77/8, Almaty', 43.215389, 76.851234, null, null, null, false),
('c0000000-0000-0000-0000-000000000004', '11111111-1111-1111-1111-111111111114', 'passenger', 'Nauryzbay Batyr St 55, Almaty', 43.265432, 76.950123, null, null, null, false),
('c0000000-0000-0000-0000-000000000005', '11111111-1111-1111-1111-111111111115', 'passenger', 'Tole Bi St 285, Almaty', 43.253678, 76.912345, null, null, null, false),
-- Destination locations
('c0000000-0000-0000-0000-000000000011', '11111111-1111-1111-1111-111111111111', 'passenger', 'Almaty International Airport', 43.352108, 77.040539, 2500.00, 12.5, 25, false),
('c0000000-0000-0000-0000-000000000012', '11111111-1111-1111-1111-111111111112', 'passenger', 'Mega Park Mall, Almaty', 43.207845, 76.668932, 3200.00, 18.3, 32, false),
('c0000000-0000-0000-0000-000000000013', '11111111-1111-1111-1111-111111111113', 'passenger', 'KBTU University, Almaty', 43.217234, 76.851987, 1500.00, 7.2, 15, false),
('c0000000-0000-0000-0000-000000000014', '11111111-1111-1111-1111-111111111114', 'passenger', 'Kok-Tobe Hill, Almaty', 43.225678, 76.955432, 1800.00, 8.5, 18, false),
('c0000000-0000-0000-0000-000000000015', '11111111-1111-1111-1111-111111111115', 'passenger', 'Central Stadium, Almaty', 43.238912, 76.945678, 1200.00, 5.3, 12, false),
-- Current driver locations
('c0000000-0000-0000-0000-000000000021', '22222222-2222-2222-2222-222222222221', 'driver', 'Satpaev St 90, Almaty', 43.236789, 76.892345, null, null, null, true),
('c0000000-0000-0000-0000-000000000022', '22222222-2222-2222-2222-222222222222', 'driver', 'Furmanov St 175, Almaty', 43.245678, 76.945123, null, null, null, true),
('c0000000-0000-0000-0000-000000000023', '22222222-2222-2222-2222-222222222223', 'driver', 'Rozybakiev St 247, Almaty', 43.212345, 76.887654, null, null, null, true),
('c0000000-0000-0000-0000-000000000024', '22222222-2222-2222-2222-222222222224', 'driver', 'Ablay Khan Ave 112, Almaty', 43.258901, 76.932456, null, null, null, true),
('c0000000-0000-0000-0000-000000000025', '22222222-2222-2222-2222-222222222225', 'driver', 'Zhandosov St 98, Almaty', 43.267890, 76.898765, null, null, null, true);

-- ============================================================================
-- RIDES
-- ============================================================================

-- Completed rides
insert into rides (id, ride_number, passenger_id, driver_id, vehicle_type, status, requested_at, matched_at, arrived_at, started_at, completed_at, estimated_fare, final_fare, pickup_coordinate_id, destination_coordinate_id) values
('r0000000-0000-0000-0000-000000000001', 'RIDE-2024-001', '11111111-1111-1111-1111-111111111111', '22222222-2222-2222-2222-222222222221', 'ECONOMY', 'COMPLETED', now() - interval '2 hours', now() - interval '1 hour 55 minutes', now() - interval '1 hour 50 minutes', now() - interval '1 hour 45 minutes', now() - interval '1 hour 20 minutes', 2500.00, 2500.00, 'c0000000-0000-0000-0000-000000000001', 'c0000000-0000-0000-0000-000000000011'),
('r0000000-0000-0000-0000-000000000002', 'RIDE-2024-002', '11111111-1111-1111-1111-111111111112', '22222222-2222-2222-2222-222222222222', 'ECONOMY', 'COMPLETED', now() - interval '3 hours', now() - interval '2 hours 55 minutes', now() - interval '2 hours 50 minutes', now() - interval '2 hours 45 minutes', now() - interval '2 hours 13 minutes', 3200.00, 3200.00, 'c0000000-0000-0000-0000-000000000002', 'c0000000-0000-0000-0000-000000000012'),
('r0000000-0000-0000-0000-000000000003', 'RIDE-2024-003', '11111111-1111-1111-1111-111111111113', '22222222-2222-2222-2222-222222222223', 'PREMIUM', 'COMPLETED', now() - interval '4 hours', now() - interval '3 hours 58 minutes', now() - interval '3 hours 50 minutes', now() - interval '3 hours 45 minutes', now() - interval '3 hours 30 minutes', 2100.00, 2100.00, 'c0000000-0000-0000-0000-000000000003', 'c0000000-0000-0000-0000-000000000013');

-- In-progress ride
insert into rides (id, ride_number, passenger_id, driver_id, vehicle_type, status, requested_at, matched_at, arrived_at, started_at, estimated_fare, pickup_coordinate_id, destination_coordinate_id) values
('r0000000-0000-0000-0000-000000000004', 'RIDE-2024-004', '11111111-1111-1111-1111-111111111114', '22222222-2222-2222-2222-222222222223', 'PREMIUM', 'IN_PROGRESS', now() - interval '15 minutes', now() - interval '12 minutes', now() - interval '8 minutes', now() - interval '5 minutes', 1800.00, 'c0000000-0000-0000-0000-000000000004', 'c0000000-0000-0000-0000-000000000014');

-- Driver en route
insert into rides (id, ride_number, passenger_id, driver_id, vehicle_type, status, requested_at, matched_at, estimated_fare, pickup_coordinate_id, destination_coordinate_id) values
('r0000000-0000-0000-0000-000000000005', 'RIDE-2024-005', '11111111-1111-1111-1111-111111111115', '22222222-2222-2222-2222-222222222224', 'ECONOMY', 'EN_ROUTE', now() - interval '8 minutes', now() - interval '6 minutes', 1200.00, 'c0000000-0000-0000-0000-000000000005', 'c0000000-0000-0000-0000-000000000015');

-- Cancelled ride
insert into rides (id, ride_number, passenger_id, driver_id, vehicle_type, status, requested_at, matched_at, cancelled_at, cancellation_reason, estimated_fare, pickup_coordinate_id, destination_coordinate_id) values
('r0000000-0000-0000-0000-000000000006', 'RIDE-2024-006', '11111111-1111-1111-1111-111111111116', '22222222-2222-2222-2222-222222222225', 'XL', 'CANCELLED', now() - interval '30 minutes', now() - interval '28 minutes', now() - interval '25 minutes', 'Passenger changed plans', 2800.00, 'c0000000-0000-0000-0000-000000000002', 'c0000000-0000-0000-0000-000000000011');

-- ============================================================================
-- RIDE EVENTS
-- ============================================================================

-- Events for completed ride 1
insert into ride_events (ride_id, event_type, event_data, created_at) values
('r0000000-0000-0000-0000-000000000001', 'RIDE_REQUESTED', '{"passenger_id": "11111111-1111-1111-1111-111111111111", "pickup": "Abay Ave 52, Almaty", "destination": "Almaty International Airport", "vehicle_type": "ECONOMY"}'::jsonb, now() - interval '2 hours'),
('r0000000-0000-0000-0000-000000000001', 'DRIVER_MATCHED', '{"driver_id": "22222222-2222-2222-2222-222222222221", "driver_name": "Aibek Nurlan", "vehicle": "Toyota Camry", "eta_minutes": 5}'::jsonb, now() - interval '1 hour 55 minutes'),
('r0000000-0000-0000-0000-000000000001', 'DRIVER_ARRIVED', '{"location": {"lat": 43.238949, "lng": 76.889709}}'::jsonb, now() - interval '1 hour 50 minutes'),
('r0000000-0000-0000-0000-000000000001', 'RIDE_STARTED', '{"start_time": "2024-12-16T10:15:00Z", "odometer": 125430}'::jsonb, now() - interval '1 hour 45 minutes'),
('r0000000-0000-0000-0000-000000000001', 'RIDE_COMPLETED', '{"end_time": "2024-12-16T10:40:00Z", "distance_km": 12.5, "duration_minutes": 25, "final_fare": 2500.00, "odometer": 125442}'::jsonb, now() - interval '1 hour 20 minutes');

-- Events for in-progress ride
insert into ride_events (ride_id, event_type, event_data, created_at) values
('r0000000-0000-0000-0000-000000000004', 'RIDE_REQUESTED', '{"passenger_id": "11111111-1111-1111-1111-111111111114", "pickup": "Nauryzbay Batyr St 55, Almaty", "destination": "Kok-Tobe Hill, Almaty", "vehicle_type": "PREMIUM"}'::jsonb, now() - interval '15 minutes'),
('r0000000-0000-0000-0000-000000000004', 'DRIVER_MATCHED', '{"driver_id": "22222222-2222-2222-2222-222222222223", "driver_name": "Arman Kuanysh", "vehicle": "BMW 5 Series", "eta_minutes": 3}'::jsonb, now() - interval '12 minutes'),
('r0000000-0000-0000-0000-000000000004', 'DRIVER_ARRIVED', '{"location": {"lat": 43.265432, "lng": 76.950123}}'::jsonb, now() - interval '8 minutes'),
('r0000000-0000-0000-0000-000000000004', 'RIDE_STARTED', '{"start_time": "2024-12-16T11:55:00Z", "odometer": 89234}'::jsonb, now() - interval '5 minutes');

-- Events for cancelled ride
insert into ride_events (ride_id, event_type, event_data, created_at) values
('r0000000-0000-0000-0000-000000000006', 'RIDE_REQUESTED', '{"passenger_id": "11111111-1111-1111-1111-111111111116", "pickup": "Dostyk Ave 162, Almaty", "destination": "Almaty International Airport", "vehicle_type": "XL"}'::jsonb, now() - interval '30 minutes'),
('r0000000-0000-0000-0000-000000000006', 'DRIVER_MATCHED', '{"driver_id": "22222222-2222-2222-2222-222222222225", "driver_name": "Daniyar Murat", "vehicle": "Toyota Alphard", "eta_minutes": 4}'::jsonb, now() - interval '28 minutes'),
('r0000000-0000-0000-0000-000000000006', 'RIDE_CANCELLED', '{"cancelled_by": "passenger", "reason": "Passenger changed plans", "cancellation_fee": 0}'::jsonb, now() - interval '25 minutes');

-- ============================================================================
-- DRIVER SESSIONS
-- ============================================================================

-- Active sessions
insert into driver_sessions (id, driver_id, started_at, total_rides, total_earnings) values
('s0000000-0000-0000-0000-000000000001', '22222222-2222-2222-2222-222222222221', now() - interval '6 hours', 3, 8200.00),
('s0000000-0000-0000-0000-000000000002', '22222222-2222-2222-2222-222222222222', now() - interval '5 hours', 4, 11500.00),
('s0000000-0000-0000-0000-000000000003', '22222222-2222-2222-2222-222222222223', now() - interval '4 hours', 2, 5600.00);

-- Completed sessions
insert into driver_sessions (id, driver_id, started_at, ended_at, total_rides, total_earnings) values
('s0000000-0000-0000-0000-000000000004', '22222222-2222-2222-2222-222222222224', now() - interval '1 day', now() - interval '1 day' + interval '8 hours', 12, 32400.00),
('s0000000-0000-0000-0000-000000000005', '22222222-2222-2222-2222-222222222225', now() - interval '2 days', now() - interval '2 days' + interval '7 hours', 9, 24300.00);

-- ============================================================================
-- LOCATION HISTORY
-- ============================================================================

-- Location history for completed ride
insert into location_history (coordinate_id, driver_id, latitude, longitude, accuracy_meters, speed_kmh, heading_degrees, recorded_at, ride_id) values
('c0000000-0000-0000-0000-000000000021', '22222222-2222-2222-2222-222222222221', 43.238949, 76.889709, 5.2, 0, 90, now() - interval '1 hour 45 minutes', 'r0000000-0000-0000-0000-000000000001'),
('c0000000-0000-0000-0000-000000000021', '22222222-2222-2222-2222-222222222221', 43.245678, 76.915432, 4.8, 45, 85, now() - interval '1 hour 40 minutes', 'r0000000-0000-0000-0000-000000000001'),
('c0000000-0000-0000-0000-000000000021', '22222222-2222-2222-2222-222222222221', 43.267890, 76.945123, 5.1, 52, 75, now() - interval '1 hour 35 minutes', 'r0000000-0000-0000-0000-000000000001'),
('c0000000-0000-0000-0000-000000000021', '22222222-2222-2222-2222-222222222221', 43.298765, 76.985678, 4.9, 58, 70, now() - interval '1 hour 30 minutes', 'r0000000-0000-0000-0000-000000000001'),
('c0000000-0000-0000-0000-000000000021', '22222222-2222-2222-2222-222222222221', 43.352108, 77.040539, 5.0, 0, 70, now() - interval '1 hour 20 minutes', 'r0000000-0000-0000-0000-000000000001');

-- Location history for in-progress ride
insert into location_history (coordinate_id, driver_id, latitude, longitude, accuracy_meters, speed_kmh, heading_degrees, recorded_at, ride_id) values
('c0000000-0000-0000-0000-000000000023', '22222222-2222-2222-2222-222222222223', 43.265432, 76.950123, 4.5, 0, 180, now() - interval '5 minutes', 'r0000000-0000-0000-0000-000000000004'),
('c0000000-0000-0000-0000-000000000023', '22222222-2222-2222-2222-222222222223', 43.255123, 76.952345, 4.8, 38, 175, now() - interval '3 minutes', 'r0000000-0000-0000-0000-000000000004'),
('c0000000-0000-0000-0000-000000000023', '22222222-2222-2222-2222-222222222223', 43.245678, 76.954567, 5.1, 42, 170, now() - interval '1 minute', 'r0000000-0000-0000-0000-000000000004');

commit;

-- ============================================================================
-- SUMMARY
-- ============================================================================
-- Users: 10 passengers, 8 drivers, 1 admin
-- Drivers: 8 drivers with various statuses and vehicle types
-- Rides: 3 completed, 1 in-progress, 1 en-route, 1 cancelled
-- Coordinates: Multiple pickup/destination locations in Almaty
-- Events: Full audit trail for rides
-- Sessions: Both active and completed driver sessions
-- Location History: GPS tracking data for rides
-- ============================================================================
