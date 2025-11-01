# Docker Deployment Guide - Driver Location Service

## Overview

This guide covers Docker deployment for the Driver Location Service, including both production and development environments.

## Table of Contents

- [Quick Start](#quick-start)
- [Production Deployment](#production-deployment)
- [Development Environment](#development-environment)
- [Configuration](#configuration)
- [Troubleshooting](#troubleshooting)
- [Monitoring](#monitoring)

---

## Quick Start

### Prerequisites

- Docker 20.10+
- Docker Compose 2.0+
- 2GB RAM minimum
- 10GB disk space

### Start Everything

```bash
# Production environment
docker-compose up -d

# Development environment (with additional tools)
docker-compose -f docker-compose.dev.yml up -d
```

### Check Status

```bash
docker-compose ps
```

### View Logs

```bash
docker-compose logs -f driver-location-service
```

---

## Production Deployment

### Architecture

```
┌─────────────────────────────────────────────────────────┐
│                  Driver Location Service                 │
│                    (Port 8082)                           │
└────────────┬────────────────────────────┬────────────────┘
             │                            │
             │                            │
    ┌────────▼────────┐          ┌───────▼────────┐
    │   PostgreSQL    │          │    RabbitMQ    │
    │   (PostGIS)     │          │                │
    │   Port 5432     │          │  Ports 5672    │
    │                 │          │       15672    │
    └─────────────────┘          └────────────────┘
```

### 1. Build the Image

```bash
# From the service directory
cd ride-hail/services/driver_location_service

# Build the image
docker-compose build

# Or using Makefile
make docker-build
```

### 2. Start Services

```bash
# Start all services
docker-compose up -d

# Or using Makefile
make docker-up
```

**Services Started:**
- PostgreSQL with PostGIS (Port 5432)
- RabbitMQ with Management UI (Ports 5672, 15672)
- Driver Location Service (Port 8082)

### 3. Verify Deployment

```bash
# Check health
curl http://localhost:8082/health

# Expected response:
# {
#   "status": "healthy",
#   "service": "driver-location-service",
#   "time": "2024-12-16T10:00:00Z"
# }

# Check RabbitMQ Management UI
# http://localhost:15672 (guest/guest)
```

### 4. View Logs

```bash
# All services
docker-compose logs -f

# Specific service
docker-compose logs -f driver-location-service

# Last 100 lines
docker-compose logs --tail=100 driver-location-service
```

### 5. Stop Services

```bash
# Stop all services
docker-compose down

# Stop and remove volumes
docker-compose down -v
```

---

## Development Environment

The development environment includes additional tools for easier development.

### Additional Services

- **pgAdmin** - PostgreSQL admin interface (Port 5050)
- **Redis** - Caching layer (Port 6379)
- **Mailhog** - Email testing tool (Ports 1025, 8025)

### Start Development Environment

```bash
# Start all development services
docker-compose -f docker-compose.dev.yml up -d

# Or using Makefile
make docker-dev-up
```

### Access Development Tools

| Service | URL | Credentials |
|---------|-----|-------------|
| Driver Location Service | http://localhost:8082 | - |
| RabbitMQ Management | http://localhost:15672 | guest/guest |
| pgAdmin | http://localhost:5050 | admin@ridehail.com/admin |
| Mailhog Web UI | http://localhost:8025 | - |
| Redis | localhost:6379 | - |

### Hot Reload

The development environment supports hot reload using Air:

```bash
# Watch logs to see reload in action
docker-compose -f docker-compose.dev.yml logs -f driver-location-service

# Make changes to code - service will automatically reload
```

### Connect to PostgreSQL via pgAdmin

1. Open http://localhost:5050
2. Login with `admin@ridehail.com` / `admin`
3. Add New Server:
   - **Name:** Driver Location DB
   - **Host:** postgres
   - **Port:** 5432
   - **Database:** ride_hail
   - **Username:** postgres
   - **Password:** postgres

### Stop Development Environment

```bash
docker-compose -f docker-compose.dev.yml down

# Or using Makefile
make docker-dev-down
```

---

## Configuration

### Environment Variables

All configuration is done via environment variables in `docker-compose.yml`:

```yaml
environment:
  # Server
  HOST: 0.0.0.0
  PORT: 8082
  
  # Database
  DATABASE_URL: postgres://postgres:postgres@postgres:5432/ride_hail?sslmode=disable
  
  # RabbitMQ
  RABBITMQ_URL: amqp://guest:guest@rabbitmq:5672/
  
  # Logging
  LOG_LEVEL: INFO
```

### Custom Configuration

Create a `.env` file in the service directory:

```bash
# .env
HOST=0.0.0.0
PORT=8082
DATABASE_URL=postgres://postgres:postgres@postgres:5432/ride_hail
RABBITMQ_URL=amqp://guest:guest@rabbitmq:5672/
LOG_LEVEL=DEBUG
```

Then reference it in docker-compose.yml:

```yaml
services:
  driver-location-service:
    env_file:
      - .env
```

### Resource Limits

Adjust resource limits in `docker-compose.yml`:

```yaml
deploy:
  resources:
    limits:
      cpus: '2.0'      # Increase CPU limit
      memory: 1024M    # Increase memory limit
    reservations:
      cpus: '1.0'
      memory: 512M
```

---

## Database Initialization

### Automatic Initialization

The database is automatically initialized on first startup:

1. PostGIS extension installed
2. Base tables created
3. Migration scripts executed

### Manual Migration

```bash
# Run migrations manually
docker-compose exec postgres psql -U postgres -d ride_hail -f /docker-entrypoint-initdb.d/02-driver-location.sql
```

### Database Backup

```bash
# Backup database
docker-compose exec postgres pg_dump -U postgres ride_hail > backup.sql

# Restore database
docker-compose exec -T postgres psql -U postgres ride_hail < backup.sql
```

---

## Networking

### Internal Network

Services communicate via internal network `driver-location-network`:

```yaml
networks:
  driver-location-network:
    driver: bridge
    ipam:
      config:
        - subnet: 172.28.0.0/16
```

### Service Discovery

Services use container names for DNS resolution:
- `postgres` - PostgreSQL database
- `rabbitmq` - RabbitMQ broker
- `driver-location-service` - Main service

### External Access

| Port | Service | Description |
|------|---------|-------------|
| 5432 | PostgreSQL | Database |
| 5672 | RabbitMQ | AMQP |
| 8082 | Service | HTTP API |
| 15672 | RabbitMQ | Management UI |

---

## Volumes

### Data Persistence

Data is persisted in named volumes:

```yaml
volumes:
  postgres_data:       # PostgreSQL data
  rabbitmq_data:       # RabbitMQ data
  rabbitmq_logs:       # RabbitMQ logs
```

### Volume Management

```bash
# List volumes
docker volume ls

# Inspect volume
docker volume inspect driver-location-postgres-data

# Remove all volumes (⚠️ DATA LOSS)
docker-compose down -v
```

### Backup Volumes

```bash
# Backup PostgreSQL volume
docker run --rm \
  -v driver-location-postgres-data:/data \
  -v $(pwd):/backup \
  alpine tar czf /backup/postgres-backup.tar.gz /data

# Restore PostgreSQL volume
docker run --rm \
  -v driver-location-postgres-data:/data \
  -v $(pwd):/backup \
  alpine tar xzf /backup/postgres-backup.tar.gz -C /
```

---

## Health Checks

### Service Health

All services have health checks configured:

```yaml
healthcheck:
  test: ["CMD", "curl", "-f", "http://localhost:8082/health"]
  interval: 30s
  timeout: 3s
  retries: 3
  start_period: 40s
```

### Check Health Status

```bash
# View health status
docker-compose ps

# Manual health check
docker-compose exec driver-location-service curl http://localhost:8082/health
```

---

## Logging

### Log Configuration

Logs are configured with rotation:

```yaml
logging:
  driver: "json-file"
  options:
    max-size: "10m"
    max-file: "3"
```

### View Logs

```bash
# Follow all logs
docker-compose logs -f

# Specific service
docker-compose logs -f driver-location-service

# Last 100 lines
docker-compose logs --tail=100 driver-location-service

# Since timestamp
docker-compose logs --since 2024-12-16T10:00:00

# Save logs to file
docker-compose logs > logs.txt
```

### Log Rotation

Logs are automatically rotated:
- Maximum file size: 10MB
- Maximum files: 3
- Total log storage: ~30MB per service

---

## Troubleshooting

### Service Won't Start

**Check logs:**
```bash
docker-compose logs driver-location-service
```

**Common issues:**
1. **Port already in use**
   ```bash
   # Check what's using the port
   lsof -i :8082
   
   # Kill the process or change the port in docker-compose.yml
   ```

2. **Database not ready**
   ```bash
   # Check PostgreSQL health
   docker-compose exec postgres pg_isready -U postgres
   
   # View PostgreSQL logs
   docker-compose logs postgres
   ```

3. **RabbitMQ connection failed**
   ```bash
   # Check RabbitMQ health
   docker-compose exec rabbitmq rabbitmq-diagnostics ping
   
   # View RabbitMQ logs
   docker-compose logs rabbitmq
   ```

### Database Connection Issues

```bash
# Test database connection
docker-compose exec driver-location-service sh -c '
  apk add --no-cache postgresql-client
  psql "$DATABASE_URL" -c "SELECT 1"
'

# Check database exists
docker-compose exec postgres psql -U postgres -l

# Recreate database
docker-compose exec postgres psql -U postgres -c "DROP DATABASE IF EXISTS ride_hail"
docker-compose exec postgres psql -U postgres -c "CREATE DATABASE ride_hail"
```

### RabbitMQ Issues

```bash
# Check RabbitMQ status
docker-compose exec rabbitmq rabbitmqctl status

# List exchanges
docker-compose exec rabbitmq rabbitmqctl list_exchanges

# List queues
docker-compose exec rabbitmq rabbitmqctl list_queues
```

### Container Keeps Restarting

```bash
# Check restart count
docker-compose ps

# View last 100 logs
docker-compose logs --tail=100 driver-location-service

# Check resource usage
docker stats driver-location-service

# Increase memory limit if needed (in docker-compose.yml)
```

### Reset Everything

```bash
# Stop and remove everything
docker-compose down -v

# Remove images
docker rmi driver-location-service:latest

# Rebuild and start
docker-compose up -d --build
```

---

## Monitoring

### Container Stats

```bash
# Real-time stats
docker stats

# Single snapshot
docker stats --no-stream

# Specific container
docker stats driver-location-service
```

### Health Monitoring

```bash
# Check all health statuses
docker-compose ps

# Continuous health check
watch -n 5 'docker-compose ps'
```

### Resource Usage

```bash
# Disk usage
docker system df

# Volume usage
docker volume ls -q | xargs docker volume inspect | grep Mountpoint

# Network usage
docker network inspect driver-location-network
```

---

## Production Best Practices

### 1. Use Secrets Management

Don't hardcode passwords in docker-compose.yml:

```yaml
services:
  postgres:
    environment:
      POSTGRES_PASSWORD_FILE: /run/secrets/postgres_password
    secrets:
      - postgres_password

secrets:
  postgres_password:
    file: ./secrets/postgres_password.txt
```

### 2. Enable TLS

Use TLS for external connections:

```yaml
services:
  driver-location-service:
    environment:
      DATABASE_URL: postgres://postgres@postgres:5432/ride_hail?sslmode=require
```

### 3. Limit Resources

Always set resource limits:

```yaml
deploy:
  resources:
    limits:
      cpus: '1.0'
      memory: 512M
```

### 4. Use Health Checks

Enable health checks for all services:

```yaml
healthcheck:
  test: ["CMD", "curl", "-f", "http://localhost:8082/health"]
  interval: 30s
  timeout: 3s
  retries: 3
```

### 5. Log Management

Configure log rotation:

```yaml
logging:
  driver: "json-file"
  options:
    max-size: "10m"
    max-file: "3"
```

### 6. Network Isolation

Use internal networks:

```yaml
networks:
  internal:
    internal: true
  external:
    internal: false
```

### 7. Regular Backups

```bash
# Backup script
#!/bin/bash
DATE=$(date +%Y%m%d_%H%M%S)
docker-compose exec postgres pg_dump -U postgres ride_hail > backup_$DATE.sql
```

---

## Makefile Commands

Quick reference for Makefile commands:

```bash
make docker-build        # Build Docker image
make docker-up           # Start production environment
make docker-down         # Stop production environment
make docker-logs         # View logs
make docker-dev-up       # Start development environment
make docker-dev-down     # Stop development environment
make docker-restart      # Restart services
make docker-rebuild      # Rebuild and restart
make docker-clean        # Remove all Docker resources
make docker-ps           # Show running containers
make docker-stats        # Show resource usage
```

---

## CI/CD Integration

### GitHub Actions Example

```yaml
name: Build and Deploy

on:
  push:
    branches: [main]

jobs:
  build:
    runs-on: ubuntu-latest
    steps:
      - uses: actions/checkout@v3
      
      - name: Build Docker image
        run: |
          cd services/driver_location_service
          docker-compose build
      
      - name: Run tests
        run: |
          docker-compose up -d postgres rabbitmq
          sleep 10
          docker-compose run driver-location-service go test ./...
```

---

## Security Considerations

### 1. Update Base Images Regularly

```bash
# Pull latest images
docker-compose pull

# Rebuild
docker-compose up -d --build
```

### 2. Scan for Vulnerabilities

```bash
# Scan image
docker scan driver-location-service:latest
```

### 3. Use Non-Root User

Already configured in Dockerfile:

```dockerfile
USER appuser
```

### 4. Limit Network Exposure

Only expose necessary ports:

```yaml
ports:
  - "127.0.0.1:8082:8082"  # Bind to localhost only
```

---

## Useful Commands

```bash
# Execute command in container
docker-compose exec driver-location-service sh

# Copy file from container
docker-compose cp driver-location-service:/app/config.json ./

# Copy file to container
docker-compose cp ./config.json driver-location-service:/app/

# Inspect container
docker inspect driver-location-service

# View container processes
docker-compose top

# Restart single service
docker-compose restart driver-location-service

# Scale service (if stateless)
docker-compose up -d --scale driver-location-service=3
```

---

## Support

For issues or questions:
1. Check logs: `docker-compose logs -f`
2. Check health: `curl http://localhost:8082/health`
3. Review this guide
4. Open an issue in the project repository

---

**Last Updated:** 2024-12-16  
**Docker Version:** 20.10+  
**Docker Compose Version:** 2.0+