# Use official Go image as builder
FROM golang:1.23-alpine AS builder

# Install build dependencies
RUN apk add --no-cache git

# Set working directory
WORKDIR /app

# Copy go mod files
COPY go.mod go.sum ./

# Download dependencies
RUN go mod download

# Copy source code
COPY . .

# Build the application
RUN CGO_ENABLED=0 GOOS=linux go build -a -installsuffix cgo -o ride-service cmd/ride-service/main.go

# Use minimal alpine image for final stage
FROM alpine:latest

# Install ca-certificates for HTTPS and curl for healthcheck
RUN apk --no-cache add ca-certificates curl

WORKDIR /root/

# Copy binary from builder
COPY --from=builder /app/ride-service .
# NOTE: Don't copy .env - use docker-compose environment variables instead

# Expose port
EXPOSE 3000

# Run the application
CMD ["./ride-service"]
