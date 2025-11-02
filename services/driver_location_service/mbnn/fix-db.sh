#!/bin/bash

# Quick Database Fix Script for Driver Location Service
# This script fixes common database issues automatically

set -e

echo "========================================"
echo "Database Quick Fix Script"
echo "========================================"
echo ""

# Colors
RED='\033[0;31m'
GREEN='\033[0;32m'
YELLOW='\033[1;33m'
BLUE='\033[0;34m'
NC='\033[0m' # No Color

# Step 1: Stop all services
echo -e "${BLUE}Step 1:${NC} Stopping all services..."
docker-compose down
echo -e "${GREEN}✓${NC} Services stopped"
echo ""

# Step 2: Remove volumes (fresh start)
echo -e "${BLUE}Step 2:${NC} Removing old database volumes..."
docker volume rm driver-location-postgres-data 2>/dev/null || echo "  (Volume already removed)"
docker volume rm driver-location-rabbitmq-data 2>/dev/null || echo "  (Volume already removed)"
docker volume rm driver-location-rabbitmq-logs 2>/dev/null || echo "  (Volume already removed)"
echo -e "${GREEN}✓${NC} Volumes removed"
echo ""

# Step 3: Remove old images
echo -e "${BLUE}Step 3:${NC} Removing old service image..."
docker rmi driver-location-service:latest 2>/dev/null || echo "  (Image already removed)"
echo -e "${GREEN}✓${NC} Old image removed"
echo ""

# Step 4: Rebuild service
echo -e "${BLUE}Step 4:${NC} Rebuilding service (this may take a few minutes)..."
docker-compose build --no-cache driver-location-service
echo -e "${GREEN}✓${NC} Service rebuilt"
echo ""

# Step 5: Start PostgreSQL first
echo -e "${BLUE}Step 5:${NC} Starting PostgreSQL..."
docker-compose up -d postgres
echo "  Waiting for PostgreSQL to be ready..."
sleep 10

# Wait for PostgreSQL to be healthy
echo "  Checking PostgreSQL health..."
for i in {1..30}; do
    if docker-compose exec -T postgres pg_isready -U postgres > /dev/null 2>&1; then
        echo -e "${GREEN}✓${NC} PostgreSQL is ready"
        break
    fi
    echo "  Still waiting... ($i/30)"
    sleep 2
done
echo ""

# Step 6: Verify migrations ran
echo -e "${BLUE}Step 6:${NC} Verifying database setup..."
echo "  Checking if database exists..."
if docker-compose exec -T postgres psql -U postgres -lqt | cut -d \| -f 1 | grep -qw ride_hail; then
    echo -e "${GREEN}✓${NC} Database 'ride_hail' exists"
else
    echo -e "${RED}✗${NC} Database not found, creating..."
    docker-compose exec -T postgres psql -U postgres -c "CREATE DATABASE ride_hail;"
fi

echo "  Checking PostGIS extension..."
docker-compose exec -T postgres psql -U postgres -d ride_hail -c "CREATE EXTENSION IF NOT EXISTS postgis;" > /dev/null 2>&1
docker-compose exec -T postgres psql -U postgres -d ride_hail -c "CREATE EXTENSION IF NOT EXISTS postgis_topology;" > /dev/null 2>&1
echo -e "${GREEN}✓${NC} PostGIS installed"

echo "  Checking tables..."
TABLE_COUNT=$(docker-compose exec -T postgres psql -U postgres -d ride_hail -tAc "SELECT COUNT(*) FROM information_schema.tables WHERE table_schema='public';" 2>/dev/null || echo "0")
echo "  Found $TABLE_COUNT tables"

if [ "$TABLE_COUNT" -lt "5" ]; then
    echo -e "${YELLOW}⚠${NC}  Too few tables, running migrations manually..."

    # Check if migration files are mounted
    if docker-compose exec postgres test -f /docker-entrypoint-initdb.d/00-init.sql; then
        echo "  Running 00-init.sql..."
        docker-compose exec -T postgres psql -U postgres -d ride_hail -f /docker-entrypoint-initdb.d/00-init.sql > /dev/null 2>&1 || echo "  (Already applied)"
    fi

    if docker-compose exec postgres test -f /docker-entrypoint-initdb.d/02-driver-location.sql; then
        echo "  Running 02-driver-location.sql..."
        docker-compose exec -T postgres psql -U postgres -d ride_hail -f /docker-entrypoint-initdb.d/02-driver-location.sql > /dev/null 2>&1 || echo "  (Already applied)"
    fi
fi

echo -e "${GREEN}✓${NC} Database setup verified"
echo ""

# Step 7: Start RabbitMQ
echo -e "${BLUE}Step 7:${NC} Starting RabbitMQ..."
docker-compose up -d rabbitmq
echo "  Waiting for RabbitMQ to be ready..."
sleep 15
echo -e "${GREEN}✓${NC} RabbitMQ started"
echo ""

# Step 8: Start Driver Location Service
echo -e "${BLUE}Step 8:${NC} Starting Driver Location Service..."
docker-compose up -d driver-location-service
echo "  Waiting for service to be ready..."
sleep 10
echo ""

# Step 9: Verify everything is running
echo -e "${BLUE}Step 9:${NC} Verifying services..."
echo ""
docker-compose ps
echo ""

# Step 10: Test health endpoint
echo -e "${BLUE}Step 10:${NC} Testing health endpoint..."
sleep 5

for i in {1..10}; do
    if curl -f http://localhost:8082/health > /dev/null 2>&1; then
        echo -e "${GREEN}✓${NC} Health check passed!"
        echo ""
        echo "Response:"
        curl -s http://localhost:8082/health | python3 -m json.tool 2>/dev/null || curl -s http://localhost:8082/health
        echo ""
        break
    else
        if [ $i -eq 10 ]; then
            echo -e "${YELLOW}⚠${NC}  Health check failed after 10 attempts"
            echo "  Service may still be starting up. Check logs with:"
            echo "  docker-compose logs -f driver-location-service"
        else
            echo "  Attempt $i/10 failed, retrying..."
            sleep 3
        fi
    fi
done
echo ""

# Step 11: Show logs preview
echo -e "${BLUE}Step 11:${NC} Recent service logs:"
echo "----------------------------------------"
docker-compose logs --tail=15 driver-location-service
echo "----------------------------------------"
echo ""

# Summary
echo "========================================"
echo "Fix Complete!"
echo "========================================"
echo ""
echo -e "${GREEN}Services Status:${NC}"
docker-compose ps
echo ""
echo -e "${GREEN}Access URLs:${NC}"
echo "  Service:     http://localhost:8082"
echo "  Health:      http://localhost:8082/health"
echo "  RabbitMQ UI: http://localhost:15672 (guest/guest)"
echo ""
echo -e "${GREEN}Useful Commands:${NC}"
echo "  View logs:         docker-compose logs -f driver-location-service"
echo "  Restart service:   docker-compose restart driver-location-service"
echo "  Stop all:          docker-compose down"
echo ""
echo -e "${GREEN}Troubleshooting:${NC}"
echo "  If issues persist, check: DATABASE_TROUBLESHOOTING.md"
echo "  Run diagnostics: ./diagnose-db.sh"
echo ""
