# Build stage
FROM golang:1.23-alpine AS builder

# Install build dependencies
RUN apk add --no-cache git

# Set working directory
WORKDIR /build

# Copy go mod files from project root
COPY go.mod go.sum ./

# Download dependencies
RUN go mod download

# Copy entire project source
COPY . .

# Build the service with correct path
RUN CGO_ENABLED=0 GOOS=linux GOARCH=amd64 go build \
    -ldflags="-w -s" \
    -o /build/bin/auth_service \
    ./cmd/auth-service

# Runtime stage
FROM alpine:3.19

# Install runtime dependencies
RUN apk add --no-cache ca-certificates tzdata curl

# Create non-root user
RUN addgroup -g 1000 appuser && \
    adduser -D -u 1000 -G appuser appuser

# Set working directory
WORKDIR /app

# Copy binary from builder
COPY --from=builder /build/bin/auth_service /app/auth_service

# Ensure an .env file exists so the service's config loader succeeds even if
# environment variables are injected at runtime.
RUN touch /app/.env

# Change ownership
RUN chown -R appuser:appuser /app

# Switch to non-root user
USER appuser

# Expose port
EXPOSE 3005

# Run the service
ENTRYPOINT ["/app/auth_service"]
