# Database Troubleshooting Guide - Driver Location Service

## Common Database Issues and Solutions

### Issue 1: "relation does not exist" errors

**Symptom:**
```
ERROR: relation "users" does not exist
ERROR: relation "coordinates" does not exist
ERROR: relation "drivers" does not exist
```

**Cause:** Migration scripts not running or running in wrong order.

**Solution:**

1. **Check if migrations ran:**
```bash
docker-compose exec postgres psql -U postgres -d ride_hail -c "\dt"
```

2. **If tables are missing, run migrations manually:**
```bash
# Run base init script
docker-compose exec postgres psql -U postgres -d ride_hail -f /docker-entrypoint-initdb.d/00-init.sql

# Run driver location migrations
docker-compose exec postgres psql -U postgres -d ride_hail -f /docker-entrypoint-initdb.d/02-driver-location.sql
```

3. **If that doesn't work, reset database:**
```bash
docker-compose down -v
docker-compose up -d postgres
# Wait 10 seconds for postgres to start
docker-compose up -d driver-location-service
```

---

### Issue 2: "failed to connect to database" or "connection refused"

**Symptom:**
```
failed to ping database: connection refused
failed to connect to `host=postgres`: dial tcp: lookup postgres: no such host
```

**Causes:**
- Database not ready yet
- Wrong connection string
- Network issues
- DNS resolution problems

**Solutions:**

1. **Check if PostgreSQL is running:**
```bash
docker-compose ps postgres
```

2. **Check PostgreSQL logs:**
```bash
docker-compose logs postgres
```

3. **Verify connection string:**
```bash
# Inside docker-compose.yml, check:
DATABASE_URL: postgres://postgres:postgres@postgres:5432/ride_hail?sslmode=disable
#                      ^user    ^pass    ^host   ^port ^dbname
```

4. **Test connection manually:**
```bash
docker-compose exec driver-location-service sh -c 'ping -c 2 postgres'
docker-compose exec postgres pg_isready -U postgres
```

5. **Check network connectivity:**
```bash
docker network inspect driver-location-network
```

---

### Issue 3: "authentication failed for user postgres"

**Symptom:**
```
failed to authenticate: password authentication failed for user "postgres"
```

**Causes:**
- Wrong password in connection string
- Password mismatch between service and database

**Solutions:**

1. **Verify environment variables:**
```bash
docker-compose exec driver-location-service env | grep DATABASE
docker-compose exec postgres env | grep POSTGRES
```

2. **Check docker-compose.yml:**
```yaml
postgres:
  environment:
    POSTGRES_USER: postgres
    POSTGRES_PASSWORD: postgres    # Must match

driver-location-service:
  environment:
    DATABASE_URL: postgres://postgres:postgres@...
    #                        ^user    ^password (must match)
```

3. **Reset database with correct password:**
```bash
docker-compose down -v
# Edit docker-compose.yml to ensure passwords match
docker-compose up -d
```

---

### Issue 4: "database does not exist"

**Symptom:**
```
failed to connect to `dbname=ride_hail`: database "ride_hail" does not exist
```

**Solution:**

1. **Check if database was created:**
```bash
docker-compose exec postgres psql -U postgres -l
```

2. **Create database manually:**
```bash
docker-compose exec postgres psql -U postgres -c "CREATE DATABASE ride_hail;"
```

3. **Restart service:**
```bash
docker-compose restart driver-location-service
```

---

### Issue 5: PostGIS extension not installed

**Symptom:**
```
ERROR: function st_makepoint(double precision, double precision) does not exist
ERROR: type geography does not exist
```

**Solution:**

1. **Install PostGIS extension:**
```bash
docker-compose exec postgres psql -U postgres -d ride_hail -c "CREATE EXTENSION IF NOT EXISTS postgis;"
docker-compose exec postgres psql -U postgres -d ride_hail -c "CREATE EXTENSION IF NOT EXISTS postgis_topology;"
```

2. **Verify PostGIS is installed:**
```bash
docker-compose exec postgres psql -U postgres -d ride_hail -c "SELECT PostGIS_version();"
```

---

### Issue 6: "too many clients already"

**Symptom:**
```
failed to connect: FATAL: sorry, too many clients already
```

**Causes:**
- Connection pool exhausted
- Connections not being closed
- Too many concurrent connections

**Solutions:**

1. **Check current connections:**
```bash
docker-compose exec postgres psql -U postgres -d ride_hail -c "SELECT count(*) FROM pg_stat_activity WHERE datname='ride_hail';"
```

2. **Reduce connection pool size in docker-compose.yml:**
```yaml
environment:
  DB_MAX_CONNS: 10  # Reduce from 25
  DB_MIN_CONNS: 2   # Reduce from 5
```

3. **Increase PostgreSQL max connections (postgres config):**
```bash
docker-compose exec postgres psql -U postgres -c "ALTER SYSTEM SET max_connections = 200;"
docker-compose restart postgres
```

---

### Issue 7: Migration scripts not running on startup

**Symptom:**
- Tables don't exist after first startup
- No errors in logs

**Causes:**
- Volume already exists with old data
- Migration files not mounted correctly

**Solutions:**

1. **Check if volumes are mounted:**
```bash
docker-compose exec postgres ls -la /docker-entrypoint-initdb.d/
```

Expected output:
```
00-init.sql
02-driver-location.sql
```

2. **Remove volumes and recreate:**
```bash
docker-compose down -v
docker volume rm driver-location-postgres-data
docker-compose up -d
```

3. **Check migration file paths in docker-compose.yml:**
```yaml
volumes:
  - ./migrations/00_init_db.sql:/docker-entrypoint-initdb.d/00-init.sql:ro
  - ./migrations/driver_location_service.sql:/docker-entrypoint-initdb.d/02-driver-location.sql:ro
```

---

### Issue 8: "UNIQUE constraint violation" on coordinates

**Symptom:**
```
ERROR: duplicate key value violates unique constraint "unique_current_location"
```

**Cause:** Multiple rows with `is_current=true` for same entity

**Solution:**

1. **Fix existing data:**
```bash
docker-compose exec postgres psql -U postgres -d ride_hail <<EOF
-- Find duplicates
SELECT entity_id, entity_type, COUNT(*) 
FROM coordinates 
WHERE is_current = true 
GROUP BY entity_id, entity_type 
HAVING COUNT(*) > 1;

-- Fix duplicates by keeping only the latest
UPDATE coordinates c1
SET is_current = false
WHERE is_current = true
  AND created_at < (
    SELECT MAX(created_at) 
    FROM coordinates c2 
    WHERE c2.entity_id = c1.entity_id 
      AND c2.entity_type = c1.entity_type
      AND c2.is_current = true
  );
EOF
```

---

## Diagnostic Commands

### Check Service Status
```bash
# All services
docker-compose ps

# Service logs
docker-compose logs -f driver-location-service

# Database logs
docker-compose logs -f postgres
```

### Database Health Checks
```bash
# Check PostgreSQL is accepting connections
docker-compose exec postgres pg_isready -U postgres

# Check database exists
docker-compose exec postgres psql -U postgres -l | grep ride_hail

# List all tables
docker-compose exec postgres psql -U postgres -d ride_hail -c "\dt"

# Check PostGIS
docker-compose exec postgres psql -U postgres -d ride_hail -c "SELECT PostGIS_version();"
```

### Connection Testing
```bash
# Test from service container
docker-compose exec driver-location-service sh

# Test network connectivity
ping postgres

# Test DNS resolution
nslookup postgres
```

### Database Queries
```bash
# Check users table
docker-compose exec postgres psql -U postgres -d ride_hail -c "SELECT * FROM users LIMIT 5;"

# Check drivers table
docker-compose exec postgres psql -U postgres -d ride_hail -c "SELECT * FROM drivers LIMIT 5;"

# Check coordinates table
docker-compose exec postgres psql -U postgres -d ride_hail -c "SELECT * FROM coordinates LIMIT 5;"

# Check active connections
docker-compose exec postgres psql -U postgres -d ride_hail -c "SELECT pid, usename, application_name, client_addr, state FROM pg_stat_activity WHERE datname='ride_hail';"
```

---

## Complete Reset Procedure

If all else fails, perform a complete reset:

```bash
# 1. Stop all services
docker-compose down

# 2. Remove all volumes (WARNING: DATA LOSS)
docker volume rm driver-location-postgres-data
docker volume rm driver-location-rabbitmq-data
docker volume rm driver-location-rabbitmq-logs

# 3. Remove the service image
docker rmi driver-location-service:latest

# 4. Rebuild everything
docker-compose build --no-cache

# 5. Start services
docker-compose up -d

# 6. Watch logs
docker-compose logs -f
```

---

## Configuration Checklist

Ensure your configuration is correct:

### docker-compose.yml
- [ ] PostgreSQL image: `postgis/postgis:15-3.4-alpine`
- [ ] Database name: `ride_hail`
- [ ] User/password: `postgres/postgres`
- [ ] Port: `5432`
- [ ] Migration volumes mounted correctly
- [ ] `DATABASE_URL` matches PostgreSQL settings

### Dockerfile
- [ ] Build context is project root (`.`)
- [ ] Go mod files copied from root
- [ ] Binary path: `./services/driver_location_service/cmd/main.go`

### main.go
- [ ] Reads `DATABASE_URL` environment variable
- [ ] Falls back to config file values
- [ ] Connection pool configured
- [ ] Timeout on connection: 10 seconds

---

## Environment Variables Reference

Required environment variables for database:

```bash
# Option 1: Full URL (recommended for Docker)
DATABASE_URL=postgres://postgres:postgres@postgres:5432/ride_hail?sslmode=disable

# Option 2: Individual fields
DB_HOST=postgres
DB_PORT=5432
DB_USER=postgres
DB_PASS=postgres
DB_NAME=ride_hail

# Connection pool settings
DB_MAX_CONNS=25
DB_MIN_CONNS=5
```

---

## Common Mistakes

1. ❌ **Wrong build context in docker-compose.yml**
   ```yaml
   build:
     context: ../../..  # WRONG - too many levels up
   ```
   ✅ **Correct:**
   ```yaml
   build:
     context: .  # Project root
     dockerfile: ./services/driver_location_service/Dockerfile
   ```

2. ❌ **Missing tables in database**
   - Forgot to mount migration files
   - Volume persists old state
   - Solution: `docker-compose down -v`

3. ❌ **Wrong host in connection string**
   ```
   DATABASE_URL=postgres://postgres:postgres@localhost:5432/...
   ```
   - Inside Docker, use service name, not `localhost`
   - ✅ Use: `@postgres:5432`

4. ❌ **Service starts before database ready**
   - Add `depends_on` with health check
   - ✅ Already configured in docker-compose.yml

---

## Getting Help

If issues persist:

1. **Collect logs:**
```bash
docker-compose logs postgres > postgres.log
docker-compose logs driver-location-service > service.log
```

2. **Check versions:**
```bash
docker --version
docker-compose --version
go version
```

3. **Verify files exist:**
```bash
ls -la migrations/
ls -la services/driver_location_service/
```

4. **Test database directly:**
```bash
docker run --rm -it --network driver-location-network postgres:15-alpine \
  psql -h postgres -U postgres -d ride_hail
```

---

**Last Updated:** 2024-12-16
**Status:** Production Ready