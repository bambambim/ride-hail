#!/bin/bash

# Database Diagnostic Script for Driver Location Service
# This script checks common database issues

set -e

echo "========================================"
echo "Database Diagnostic Script"
echo "========================================"
echo ""

# Colors
RED='\033[0;31m'
GREEN='\033[0;32m'
YELLOW='\033[1;33m'
NC='\033[0m' # No Color

# Check if docker-compose is running
echo "1. Checking if services are running..."
if docker-compose ps | grep -q "driver-location-postgres"; then
    echo -e "${GREEN}✓${NC} PostgreSQL container is running"
else
    echo -e "${RED}✗${NC} PostgreSQL container is not running"
    echo "   Run: docker-compose up -d postgres"
    exit 1
fi

if docker-compose ps | grep -q "driver-location-service"; then
    echo -e "${GREEN}✓${NC} Driver Location Service container is running"
else
    echo -e "${YELLOW}⚠${NC} Driver Location Service container is not running"
fi
echo ""

# Check PostgreSQL health
echo "2. Checking PostgreSQL health..."
if docker-compose exec -T postgres pg_isready -U postgres > /dev/null 2>&1; then
    echo -e "${GREEN}✓${NC} PostgreSQL is accepting connections"
else
    echo -e "${RED}✗${NC} PostgreSQL is not ready"
    exit 1
fi
echo ""

# Check if database exists
echo "3. Checking if database exists..."
if docker-compose exec -T postgres psql -U postgres -lqt | cut -d \| -f 1 | grep -qw ride_hail; then
    echo -e "${GREEN}✓${NC} Database 'ride_hail' exists"
else
    echo -e "${RED}✗${NC} Database 'ride_hail' does not exist"
    echo "   Creating database..."
    docker-compose exec -T postgres psql -U postgres -c "CREATE DATABASE ride_hail;"
fi
echo ""

# Check PostGIS extension
echo "4. Checking PostGIS extension..."
POSTGIS_VERSION=$(docker-compose exec -T postgres psql -U postgres -d ride_hail -tAc "SELECT PostGIS_version();" 2>/dev/null || echo "")
if [ -n "$POSTGIS_VERSION" ]; then
    echo -e "${GREEN}✓${NC} PostGIS is installed: $POSTGIS_VERSION"
else
    echo -e "${YELLOW}⚠${NC} PostGIS not installed, installing..."
    docker-compose exec -T postgres psql -U postgres -d ride_hail -c "CREATE EXTENSION IF NOT EXISTS postgis;"
    docker-compose exec -T postgres psql -U postgres -d ride_hail -c "CREATE EXTENSION IF NOT EXISTS postgis_topology;"
    echo -e "${GREEN}✓${NC} PostGIS installed"
fi
echo ""

# Check required tables
echo "5. Checking required tables..."
TABLES=("users" "drivers" "driver_sessions" "coordinates" "location_history" "rides" "vehicle_type" "driver_status")

for table in "${TABLES[@]}"; do
    if docker-compose exec -T postgres psql -U postgres -d ride_hail -tAc "SELECT to_regclass('public.$table');" | grep -q "$table"; then
        echo -e "${GREEN}✓${NC} Table '$table' exists"
    else
        echo -e "${RED}✗${NC} Table '$table' does not exist"
    fi
done
echo ""

# Check migration files
echo "6. Checking migration files in container..."
docker-compose exec postgres ls -la /docker-entrypoint-initdb.d/ 2>/dev/null || echo -e "${YELLOW}⚠${NC} Cannot list migration files"
echo ""

# Check connections
echo "7. Checking active database connections..."
CONN_COUNT=$(docker-compose exec -T postgres psql -U postgres -d ride_hail -tAc "SELECT count(*) FROM pg_stat_activity WHERE datname='ride_hail';" 2>/dev/null || echo "0")
echo "   Active connections: $CONN_COUNT"
echo ""

# Test connection from service
echo "8. Testing network connectivity..."
if docker-compose exec -T driver-location-service ping -c 2 postgres > /dev/null 2>&1; then
    echo -e "${GREEN}✓${NC} Service can reach postgres container"
else
    echo -e "${YELLOW}⚠${NC} Cannot test connectivity (service may not be running)"
fi
echo ""

# Check environment variables
echo "9. Checking environment variables..."
DB_URL=$(docker-compose exec -T driver-location-service env | grep DATABASE_URL || echo "NOT SET")
echo "   DATABASE_URL: ${DB_URL#DATABASE_URL=}"
echo ""

# Check service logs for errors
echo "10. Checking recent service logs for database errors..."
if docker-compose logs --tail=20 driver-location-service 2>/dev/null | grep -i -E "(error|failed|panic)" | head -5; then
    echo ""
else
    echo -e "${GREEN}✓${NC} No recent errors found in service logs"
fi
echo ""

# Summary
echo "========================================"
echo "Diagnostic Summary"
echo "========================================"
echo ""
echo "If all checks passed, your database setup is correct."
echo ""
echo "Common fixes:"
echo "  - Reset database: docker-compose down -v && docker-compose up -d"
echo "  - View logs: docker-compose logs -f postgres"
echo "  - Run migrations: docker-compose exec postgres psql -U postgres -d ride_hail -f /docker-entrypoint-initdb.d/00-init.sql"
echo ""
echo "For more help, see DATABASE_TROUBLESHOOTING.md"
echo ""
