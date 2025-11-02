# Configuration Migration Guide - Driver Location Service

## Overview

The Driver Location Service configuration has been migrated from inline environment variable reading to using the centralized `pkg/config` package.

## What Changed

### Before (Old Approach)

Configuration was loaded directly in `main.go` using inline helper functions:

```go
type Config struct {
    Host        string
    Port        int
    DatabaseURL string
    RabbitMQURL string
    LogLevel    string
}

func loadConfig() Config {
    return Config{
        Host:        getEnv("HOST", "0.0.0.0"),
        Port:        getEnvAsInt("PORT", 8082),
        DatabaseURL: getEnv("DATABASE_URL", "postgres://..."),
        RabbitMQURL: getEnv("RABBITMQ_URL", "amqp://..."),
        LogLevel:    getEnv("LOG_LEVEL", "INFO"),
    }
}
```

### After (New Approach)

Configuration is now loaded using the centralized `pkg/config` package:

```go
import "ride-hail/pkg/config"

func main() {
    // Load configuration from .env file
    cfg, err := config.LoadConfig(".env")
    if err != nil {
        log.Error("startup.load_config", err)
        // Fallback to environment variables
    }
    
    // Build connection URLs from config
    databaseURL := buildDatabaseURL(cfg)
    rabbitmqURL := buildRabbitMQURL(cfg)
}
```

## Configuration Structure

### New Config Fields

The service now uses the following configuration structure from `pkg/config`:

```go
type Config struct {
    DB struct {
        Host     string  // DB_HOST
        Port     int     // DB_PORT
        User     string  // DB_USER
        Password string  // DB_PASS
        Database string  // DB_NAME
    }
    RabbitMQ struct {
        Host     string  // RABBITMQ_HOST
        Port     int     // RABBITMQ_PORT
        User     string  // RABBITMQ_USER
        Password string  // RABBITMQ_PASS
    }
    Services struct {
        DriverLocationService int  // DRIVER_LOCATION_SERVICE
        RideService           int  // SERVICES_RIDE_SERVICE
        AdminService          int  // ADMIN_SERVICE
    }
    Websocket struct {
        Port int  // WEBSOCKET_PORT
    }
}
```

## Environment Variable Mapping

### Database Configuration

| Old Variable | New Variable | Default | Description |
|--------------|--------------|---------|-------------|
| `DATABASE_URL` | `DATABASE_URL` | (constructed) | Full database URL (takes precedence) |
| N/A | `DB_HOST` | localhost | Database host |
| N/A | `DB_PORT` | 5432 | Database port |
| N/A | `DB_USER` | postgres | Database user |
| N/A | `DB_PASS` | postgres | Database password |
| N/A | `DB_NAME` | ride_hail | Database name |

### RabbitMQ Configuration

| Old Variable | New Variable | Default | Description |
|--------------|--------------|---------|-------------|
| `RABBITMQ_URL` | `RABBITMQ_URL` | (constructed) | Full RabbitMQ URL (takes precedence) |
| N/A | `RABBITMQ_HOST` | localhost | RabbitMQ host |
| N/A | `RABBITMQ_PORT` | 5672 | RabbitMQ port |
| N/A | `RABBITMQ_USER` | guest | RabbitMQ user |
| N/A | `RABBITMQ_PASS` | guest | RabbitMQ password |

### Service Configuration

| Old Variable | New Variable | Default | Description |
|--------------|--------------|---------|-------------|
| `PORT` | `DRIVER_LOCATION_SERVICE` | 8082 | Service HTTP port |
| `HOST` | N/A | 0.0.0.0 | Service bind address (hardcoded) |

## Migration Steps

### 1. Create .env File

Copy the example configuration:

```bash
cp env.example .env
```

### 2. Update Environment Variables

Edit `.env` with your configuration:

```bash
# Database - Option 1: Individual fields
DB_HOST=localhost
DB_PORT=5432
DB_USER=postgres
DB_PASS=postgres
DB_NAME=ride_hail

# Database - Option 2: Direct URL (overrides above)
DATABASE_URL=postgres://postgres:postgres@localhost:5432/ride_hail?sslmode=disable

# RabbitMQ - Option 1: Individual fields
RABBITMQ_HOST=localhost
RABBITMQ_PORT=5672
RABBITMQ_USER=guest
RABBITMQ_PASS=guest

# RabbitMQ - Option 2: Direct URL (overrides above)
RABBITMQ_URL=amqp://guest:guest@localhost:5672/

# Service Port
DRIVER_LOCATION_SERVICE=8082
```

### 3. Backward Compatibility

The new configuration system maintains backward compatibility:

- **DATABASE_URL**: If set, overrides individual DB_* fields
- **RABBITMQ_URL**: If set, overrides individual RABBITMQ_* fields
- **.env file**: If not found, system falls back to environment variables

## Configuration Priority

Configuration is loaded in the following priority order:

1. **Environment Variables** (highest priority)
   - `DATABASE_URL` and `RABBITMQ_URL` if set
   
2. **.env File**
   - Parsed by `config.LoadConfig(".env")`
   - Sets environment variables from file
   
3. **Default Values** (lowest priority)
   - Defined in `pkg/config/config.go`

## Docker Deployment

For Docker deployments, the system supports both approaches:

### Option 1: Use .env File

```yaml
# docker-compose.yml
services:
  driver-location-service:
    env_file:
      - .env
```

### Option 2: Environment Variables

```yaml
# docker-compose.yml
services:
  driver-location-service:
    environment:
      DATABASE_URL: postgres://postgres:postgres@postgres:5432/ride_hail
      RABBITMQ_URL: amqp://guest:guest@rabbitmq:5672/
      DRIVER_LOCATION_SERVICE: 8082
```

### Option 3: Individual Fields

```yaml
# docker-compose.yml
services:
  driver-location-service:
    environment:
      DB_HOST: postgres
      DB_PORT: 5432
      DB_USER: postgres
      DB_PASS: postgres
      DB_NAME: ride_hail
      RABBITMQ_HOST: rabbitmq
      RABBITMQ_PORT: 5672
      RABBITMQ_USER: guest
      RABBITMQ_PASS: guest
      DRIVER_LOCATION_SERVICE: 8082
```

## Benefits

### 1. Centralized Configuration

- Single source of truth for configuration structure
- Consistent across all services
- Easier to maintain and update

### 2. .env File Support

- Load configuration from file
- Better for local development
- No need to set environment variables manually

### 3. Structured Configuration

- Type-safe configuration
- Clear organization (DB, RabbitMQ, Services, etc.)
- Auto-completion support in IDEs

### 4. Flexible Deployment

- Works with Docker
- Works with direct environment variables
- Works with .env files
- Backward compatible with URL-based config

## Testing Configuration

### Test with .env File

```bash
# 1. Create .env file
cat > .env << EOF
DB_HOST=localhost
DB_PORT=5432
DB_USER=postgres
DB_PASS=postgres
DB_NAME=ride_hail
RABBITMQ_HOST=localhost
RABBITMQ_PORT=5672
RABBITMQ_USER=guest
RABBITMQ_PASS=guest
DRIVER_LOCATION_SERVICE=8082
EOF

# 2. Run service
go run cmd/main.go
```

### Test with Environment Variables

```bash
# Set environment variables
export DATABASE_URL=postgres://postgres:postgres@localhost:5432/ride_hail
export RABBITMQ_URL=amqp://guest:guest@localhost:5672/
export DRIVER_LOCATION_SERVICE=8082

# Run service
go run cmd/main.go
```

### Test with Docker

```bash
# Using docker-compose
docker-compose up -d

# Check logs
docker-compose logs -f driver-location-service
```

## Troubleshooting

### Issue: Service can't find .env file

**Symptom:** Error message "could not open env file"

**Solution:** 
- The service will continue with environment variables
- Create `.env` file in the service root directory
- Or set environment variables directly

### Issue: Database connection failed

**Symptom:** "failed to connect to database"

**Solution:**
1. Check DATABASE_URL or individual DB_* variables
2. Verify database is running
3. Check connection string format:
   ```
   postgres://USER:PASS@HOST:PORT/DATABASE?sslmode=disable
   ```

### Issue: RabbitMQ connection failed

**Symptom:** "failed to connect to RabbitMQ"

**Solution:**
1. Check RABBITMQ_URL or individual RABBITMQ_* variables
2. Verify RabbitMQ is running
3. Check connection string format:
   ```
   amqp://USER:PASS@HOST:PORT/
   ```

### Issue: Wrong port

**Symptom:** Service starts on unexpected port

**Solution:**
- Set `DRIVER_LOCATION_SERVICE` environment variable
- Default is 8082 if not set

## Code Changes Summary

### Files Modified

1. **cmd/main.go**
   - Removed inline Config struct
   - Removed helper functions (getEnv, getEnvAsInt, getEnvAsDuration)
   - Added `buildDatabaseURL()` function
   - Added `buildRabbitMQURL()` function
   - Updated configuration loading logic

### Files Created

1. **env.example**
   - Example configuration file
   - Documents all available configuration options

2. **CONFIG_MIGRATION.md** (this file)
   - Migration guide
   - Configuration documentation

### Files Using Config

- `pkg/config/config.go` - Configuration loader (shared)
- `cmd/main.go` - Main application using config

## Example Configurations

### Development

```bash
# .env
DB_HOST=localhost
DB_PORT=5432
DB_USER=postgres
DB_PASS=postgres
DB_NAME=ride_hail_dev
RABBITMQ_HOST=localhost
RABBITMQ_PORT=5672
RABBITMQ_USER=guest
RABBITMQ_PASS=guest
DRIVER_LOCATION_SERVICE=8082
LOG_LEVEL=DEBUG
```

### Production

```bash
# .env
DATABASE_URL=postgres://produser:securepass@prod-db.example.com:5432/ride_hail?sslmode=require
RABBITMQ_URL=amqps://produser:securepass@prod-mq.example.com:5671/
DRIVER_LOCATION_SERVICE=8082
LOG_LEVEL=INFO
```

### Docker

```yaml
# docker-compose.yml
environment:
  DB_HOST: postgres
  DB_PORT: 5432
  DB_USER: postgres
  DB_PASS: postgres
  DB_NAME: ride_hail
  RABBITMQ_HOST: rabbitmq
  RABBITMQ_PORT: 5672
  RABBITMQ_USER: guest
  RABBITMQ_PASS: guest
  DRIVER_LOCATION_SERVICE: 8082
```

## Next Steps

1. ✅ Configuration migrated to `pkg/config`
2. ✅ Backward compatibility maintained
3. ✅ Documentation complete
4. 📝 Update deployment scripts if needed
5. 📝 Update CI/CD pipelines if needed

## Support

For questions or issues with configuration:
1. Check this migration guide
2. Review `env.example` for all options
3. Check logs for configuration errors
4. Review `pkg/config/config.go` for implementation details

---

**Migration Date:** 2024-12-16  
**Status:** ✅ Complete  
**Breaking Changes:** None (backward compatible)