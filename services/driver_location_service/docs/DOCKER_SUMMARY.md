# Docker Deployment Summary - Driver Location Service

## 🚀 Quick Start

### Fastest Way to Start

```bash
# Make script executable
chmod +x start.sh

# Start production environment
./start.sh prod

# Or start development environment
./start.sh dev
```

### Using Docker Compose Directly

```bash
# Production
docker-compose up -d

# Development
docker-compose -f docker-compose.dev.yml up -d
```

### Using Makefile

```bash
# Production
make docker-up

# Development
make docker-dev-up
```

---

## 📦 What Gets Deployed

### Production Environment

```
┌─────────────────────────────────────┐
│  Driver Location Service (Port 8082) │
└───────────┬─────────────┬───────────┘
            │             │
    ┌───────▼──────┐  ┌──▼────────┐
    │  PostgreSQL  │  │ RabbitMQ  │
    │  (PostGIS)   │  │           │
    │  Port 5432   │  │ Ports     │
    │              │  │ 5672      │
    │              │  │ 15672     │
    └──────────────┘  └───────────┘
```

**Services:**
- ✅ PostgreSQL 15 with PostGIS extension
- ✅ RabbitMQ 3.12 with Management UI
- ✅ Driver Location Service

### Development Environment

Everything from production PLUS:
- ✅ pgAdmin (PostgreSQL GUI)
- ✅ Redis (Caching)
- ✅ Mailhog (Email testing)
- ✅ Hot reload with Air

---

## 🌐 Access URLs

### Production

| Service | URL | Credentials |
|---------|-----|-------------|
| Driver Location API | http://localhost:8082 | - |
| Health Check | http://localhost:8082/health | - |
| RabbitMQ Management | http://localhost:15672 | guest/guest |
| PostgreSQL | localhost:5432 | postgres/postgres |

### Development (Additional)

| Service | URL | Credentials |
|---------|-----|-------------|
| pgAdmin | http://localhost:5050 | admin@ridehail.com/admin |
| Mailhog UI | http://localhost:8025 | - |
| Redis | localhost:6379 | - |

---

## 📝 Common Commands

### Start/Stop

```bash
# Start production
docker-compose up -d
./start.sh prod
make docker-up

# Start development
docker-compose -f docker-compose.dev.yml up -d
./start.sh dev
make docker-dev-up

# Stop production
docker-compose down
./start.sh stop
make docker-down

# Stop development
docker-compose -f docker-compose.dev.yml down
make docker-dev-down
```

### Logs

```bash
# View all logs
docker-compose logs -f

# View service logs only
docker-compose logs -f driver-location-service

# Last 100 lines
docker-compose logs --tail=100 driver-location-service

# Using script
./start.sh logs       # production
./start.sh logs dev   # development
```

### Status & Health

```bash
# Check status
docker-compose ps
./start.sh status

# Health check
curl http://localhost:8082/health

# Expected response:
# {
#   "status": "healthy",
#   "service": "driver-location-service",
#   "time": "2024-12-16T10:00:00Z"
# }
```

### Rebuild & Restart

```bash
# Rebuild service
docker-compose build
make docker-build

# Restart services
docker-compose restart
make docker-restart

# Rebuild and restart
docker-compose down
docker-compose up -d --build
make docker-rebuild
```

### Database Operations

```bash
# Access PostgreSQL CLI
docker-compose exec postgres psql -U postgres -d ride_hail

# Run query
docker-compose exec postgres psql -U postgres -d ride_hail -c "SELECT version();"

# Backup database
docker-compose exec postgres pg_dump -U postgres ride_hail > backup.sql

# Restore database
cat backup.sql | docker-compose exec -T postgres psql -U postgres ride_hail

# View tables
docker-compose exec postgres psql -U postgres -d ride_hail -c "\dt"
```

### RabbitMQ Operations

```bash
# Check RabbitMQ status
docker-compose exec rabbitmq rabbitmqctl status

# List queues
docker-compose exec rabbitmq rabbitmqctl list_queues

# List exchanges
docker-compose exec rabbitmq rabbitmqctl list_exchanges

# View connections
docker-compose exec rabbitmq rabbitmqctl list_connections
```

### Cleanup

```bash
# Stop and remove containers
docker-compose down

# Stop and remove volumes (⚠️ DATA LOSS)
docker-compose down -v

# Complete cleanup
./start.sh clean
make docker-clean
```

---

## 🔧 Configuration

### Environment Variables

Edit `docker-compose.yml` to configure:

```yaml
environment:
  # Server
  HOST: 0.0.0.0
  PORT: 8082
  
  # Database
  DATABASE_URL: postgres://postgres:postgres@postgres:5432/ride_hail
  DB_MAX_CONNS: 25
  
  # RabbitMQ
  RABBITMQ_URL: amqp://guest:guest@rabbitmq:5672/
  
  # Logging
  LOG_LEVEL: INFO
  
  # Rate Limiting
  RATE_LIMIT_LOCATION_INTERVAL: 3s
  RATE_LIMIT_LOCATION_CAPACITY: 1
```

### Custom .env File

Create `.env` file:

```bash
HOST=0.0.0.0
PORT=8082
LOG_LEVEL=DEBUG
DATABASE_URL=postgres://postgres:postgres@postgres:5432/ride_hail
RABBITMQ_URL=amqp://guest:guest@rabbitmq:5672/
```

Reference in docker-compose.yml:

```yaml
services:
  driver-location-service:
    env_file:
      - .env
```

---

## 🐛 Troubleshooting

### Service Won't Start

```bash
# Check logs
docker-compose logs driver-location-service

# Check if ports are in use
lsof -i :8082
lsof -i :5432
lsof -i :5672

# Restart everything
docker-compose down
docker-compose up -d
```

### Database Connection Failed

```bash
# Check PostgreSQL is running
docker-compose ps postgres

# Test connection
docker-compose exec postgres pg_isready -U postgres

# Check logs
docker-compose logs postgres

# Restart PostgreSQL
docker-compose restart postgres
```

### RabbitMQ Issues

```bash
# Check RabbitMQ status
docker-compose exec rabbitmq rabbitmq-diagnostics ping

# Check logs
docker-compose logs rabbitmq

# Reset RabbitMQ
docker-compose stop rabbitmq
docker-compose rm -f rabbitmq
docker-compose up -d rabbitmq
```

### Container Keeps Restarting

```bash
# Check why it's restarting
docker-compose logs --tail=50 driver-location-service

# Check resource usage
docker stats driver-location-service

# Increase memory limit in docker-compose.yml
```

### Port Already in Use

```bash
# Find what's using the port
lsof -i :8082

# Kill the process
kill -9 <PID>

# Or change port in docker-compose.yml
ports:
  - "8083:8082"  # Use different external port
```

### Complete Reset

```bash
# Nuclear option - removes everything
docker-compose down -v
docker-compose -f docker-compose.dev.yml down -v
docker rmi driver-location-service:latest
docker volume prune -f
docker-compose up -d --build
```

---

## 📊 Monitoring

### Real-time Stats

```bash
# All containers
docker stats

# Specific service
docker stats driver-location-service

# One-time snapshot
docker stats --no-stream
```

### Resource Usage

```bash
# Disk usage
docker system df

# Container sizes
docker ps -s

# Volume sizes
docker volume ls -q | xargs docker volume inspect | grep Mountpoint
```

### Health Monitoring

```bash
# Continuous monitoring
watch -n 5 'curl -s http://localhost:8082/health | jq'

# Check all services
docker-compose ps
```

---

## 🔒 Security Best Practices

### 1. Change Default Passwords

```yaml
environment:
  POSTGRES_PASSWORD: <strong-password>
  RABBITMQ_DEFAULT_PASS: <strong-password>
```

### 2. Use Secrets (Production)

```yaml
secrets:
  postgres_password:
    file: ./secrets/postgres_password.txt

services:
  postgres:
    environment:
      POSTGRES_PASSWORD_FILE: /run/secrets/postgres_password
    secrets:
      - postgres_password
```

### 3. Limit Port Exposure

```yaml
ports:
  - "127.0.0.1:8082:8082"  # Localhost only
```

### 4. Enable TLS

```yaml
environment:
  DATABASE_URL: postgres://...?sslmode=require
```

---

## 📦 Data Persistence

### Volumes Created

- `driver-location-postgres-data` - PostgreSQL data
- `driver-location-rabbitmq-data` - RabbitMQ data
- `driver-location-rabbitmq-logs` - RabbitMQ logs

### Backup Volumes

```bash
# Backup PostgreSQL data
docker run --rm \
  -v driver-location-postgres-data:/data \
  -v $(pwd):/backup \
  alpine tar czf /backup/postgres-backup.tar.gz /data

# Restore
docker run --rm \
  -v driver-location-postgres-data:/data \
  -v $(pwd):/backup \
  alpine tar xzf /backup/postgres-backup.tar.gz -C /
```

---

## 🚀 Deployment Checklist

### Before First Deploy

- [ ] Review `docker-compose.yml` configuration
- [ ] Change default passwords
- [ ] Set appropriate resource limits
- [ ] Configure logging
- [ ] Set up backup strategy

### Deploy Steps

```bash
# 1. Build image
docker-compose build

# 2. Start services
docker-compose up -d

# 3. Wait for health checks
sleep 30

# 4. Verify deployment
curl http://localhost:8082/health

# 5. Check logs
docker-compose logs -f driver-location-service
```

### After Deploy

- [ ] Verify all services are healthy
- [ ] Check RabbitMQ queues are created
- [ ] Test database connectivity
- [ ] Monitor resource usage
- [ ] Set up log aggregation

---

## 📚 File Reference

### Docker Files

- `Dockerfile` - Multi-stage build for service
- `docker-compose.yml` - Production environment
- `docker-compose.dev.yml` - Development environment with tools
- `.dockerignore` - Files to exclude from build
- `init-db.sh` - Database initialization script
- `.air.toml` - Hot reload configuration

### Helper Scripts

- `start.sh` - Quick start script
- All commands in `Makefile`

---

## 🎯 Testing the Deployment

### 1. Health Check

```bash
curl http://localhost:8082/health
```

### 2. Test API Endpoints

```bash
# Driver goes online
curl -X POST http://localhost:8082/drivers/test-driver-id/online \
  -H "Authorization: Bearer test-token" \
  -H "Content-Type: application/json" \
  -d '{"latitude": 43.238949, "longitude": 76.889709}'
```

### 3. Check Database

```bash
docker-compose exec postgres psql -U postgres -d ride_hail -c "\dt"
```

### 4. Check RabbitMQ

```bash
# List exchanges
docker-compose exec rabbitmq rabbitmqctl list_exchanges

# Expected: ride_topic, driver_topic, location_fanout
```

---

## 💡 Tips & Tricks

### Speed Up Builds

```bash
# Use BuildKit
DOCKER_BUILDKIT=1 docker-compose build

# Parallel builds
docker-compose build --parallel
```

### View Real-time Logs

```bash
# Color-coded logs
docker-compose logs -f --tail=100

# Filter by service
docker-compose logs -f driver-location-service | grep ERROR
```

### Quick Database Access

```bash
# Add alias to ~/.bashrc
alias psql-driver="docker-compose exec postgres psql -U postgres -d ride_hail"

# Usage
psql-driver -c "SELECT COUNT(*) FROM drivers;"
```

### Resource Monitoring

```bash
# Add to crontab for periodic checks
*/5 * * * * docker stats --no-stream >> /var/log/docker-stats.log
```

---

## 📞 Getting Help

1. **Check logs first:** `docker-compose logs -f`
2. **Check service health:** `curl http://localhost:8082/health`
3. **Review documentation:** See `DOCKER.md` for detailed guide
4. **Common issues:** Check troubleshooting section above

---

## 🎉 Success Indicators

Your deployment is successful when:

✅ All containers show as "Up (healthy)" in `docker-compose ps`
✅ Health endpoint returns `{"status":"healthy"}`
✅ RabbitMQ Management UI is accessible
✅ Database accepts connections
✅ Service logs show "Service ready on 0.0.0.0:8082"

---

**Quick Reference:**

```bash
# Start everything
./start.sh prod

# View logs
docker-compose logs -f

# Stop everything
./start.sh stop

# Health check
curl http://localhost:8082/health
```

**Last Updated:** 2024-12-16  
**Docker Compose Version:** 3.8  
**Status:** ✅ Production Ready