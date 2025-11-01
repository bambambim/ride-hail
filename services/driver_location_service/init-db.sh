#!/bin/sh
set -e

echo "Starting database initialization..."

# Wait for PostgreSQL to be ready
until pg_isready -h localhost -U postgres; do
  echo "Waiting for PostgreSQL to be ready..."
  sleep 2
done

echo "PostgreSQL is ready!"

# Enable PostGIS extension
psql -v ON_ERROR_STOP=1 --username "$POSTGRES_USER" --dbname "$POSTGRES_DB" <<-EOSQL
    -- Enable PostGIS extension
    CREATE EXTENSION IF NOT EXISTS postgis;
    CREATE EXTENSION IF NOT EXISTS postgis_topology;

    -- Verify PostGIS installation
    SELECT PostGIS_version();

    -- Create enums and base tables if they don't exist
    DO \$\$
    BEGIN
        -- Check if vehicle_type table exists
        IF NOT EXISTS (SELECT FROM pg_tables WHERE schemaname = 'public' AND tablename = 'vehicle_type') THEN
            CREATE TABLE "vehicle_type"("value" text not null primary key);
            INSERT INTO "vehicle_type" ("value") VALUES
                ('ECONOMY'),
                ('COMFORT'),
                ('BUSINESS'),
                ('VAN'),
                ('PREMIUM');
        END IF;

        -- Check if users table exists (simplified version)
        IF NOT EXISTS (SELECT FROM pg_tables WHERE schemaname = 'public' AND tablename = 'users') THEN
            CREATE TABLE users (
                id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
                email VARCHAR(255) UNIQUE NOT NULL,
                created_at TIMESTAMPTZ NOT NULL DEFAULT NOW()
            );
        END IF;

        -- Check if rides table exists (simplified version)
        IF NOT EXISTS (SELECT FROM pg_tables WHERE schemaname = 'public' AND tablename = 'rides') THEN
            CREATE TABLE rides (
                id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
                ride_number VARCHAR(50) UNIQUE NOT NULL,
                created_at TIMESTAMPTZ NOT NULL DEFAULT NOW()
            );
        END IF;
    END
    \$\$;
EOSQL

echo "Database initialization completed successfully!"
