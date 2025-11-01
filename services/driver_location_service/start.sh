#!/bin/bash

# Driver Location Service - Quick Start Script
# This script helps you quickly start the service in different modes

set -e

# Colors for output
RED='\033[0;31m'
GREEN='\033[0;32m'
YELLOW='\033[1;33m'
BLUE='\033[0;34m'
NC='\033[0m' # No Color

# Functions
print_header() {
    echo -e "${BLUE}=================================${NC}"
    echo -e "${BLUE}Driver Location Service${NC}"
    echo -e "${BLUE}=================================${NC}"
    echo ""
}

print_success() {
    echo -e "${GREEN}✓${NC} $1"
}

print_error() {
    echo -e "${RED}✗${NC} $1"
}

print_warning() {
    echo -e "${YELLOW}⚠${NC} $1"
}

print_info() {
    echo -e "${BLUE}ℹ${NC} $1"
}

check_requirements() {
    print_info "Checking requirements..."

    # Check Docker
    if ! command -v docker &> /dev/null; then
        print_error "Docker is not installed"
        exit 1
    fi
    print_success "Docker found: $(docker --version | cut -d' ' -f3)"

    # Check Docker Compose
    if ! command -v docker-compose &> /dev/null; then
        print_error "Docker Compose is not installed"
        exit 1
    fi
    print_success "Docker Compose found: $(docker-compose --version | cut -d' ' -f4)"

    echo ""
}

start_production() {
    print_header
    print_info "Starting production environment..."
    echo ""

    # Build image
    print_info "Building Docker image..."
    docker-compose build
    print_success "Image built successfully"
    echo ""

    # Start services
    print_info "Starting services..."
    docker-compose up -d
    print_success "Services started"
    echo ""

    # Wait for services to be healthy
    print_info "Waiting for services to be ready..."
    sleep 10

    # Check health
    if curl -f http://localhost:8082/health > /dev/null 2>&1; then
        print_success "Service is healthy!"
    else
        print_warning "Service may not be ready yet. Check logs with: docker-compose logs -f"
    fi

    echo ""
    print_success "Production environment started!"
    echo ""
    echo -e "${GREEN}Access URLs:${NC}"
    echo "  Service:     http://localhost:8082"
    echo "  Health:      http://localhost:8082/health"
    echo "  RabbitMQ UI: http://localhost:15672 (guest/guest)"
    echo ""
    echo -e "${YELLOW}View logs:${NC} docker-compose logs -f"
    echo -e "${YELLOW}Stop:${NC}      docker-compose down"
    echo ""
}

start_development() {
    print_header
    print_info "Starting development environment..."
    echo ""

    # Start services
    print_info "Starting development services..."
    docker-compose -f docker-compose.dev.yml up -d
    print_success "Development services started"
    echo ""

    # Wait for services to be healthy
    print_info "Waiting for services to be ready..."
    sleep 15

    # Check health
    if curl -f http://localhost:8082/health > /dev/null 2>&1; then
        print_success "Service is healthy!"
    else
        print_warning "Service may not be ready yet. Check logs with: docker-compose -f docker-compose.dev.yml logs -f"
    fi

    echo ""
    print_success "Development environment started!"
    echo ""
    echo -e "${GREEN}Access URLs:${NC}"
    echo "  Service:     http://localhost:8082"
    echo "  Health:      http://localhost:8082/health"
    echo "  RabbitMQ UI: http://localhost:15672 (guest/guest)"
    echo "  pgAdmin:     http://localhost:5050 (admin@ridehail.com/admin)"
    echo "  Mailhog:     http://localhost:8025"
    echo "  Redis:       localhost:6379"
    echo ""
    echo -e "${YELLOW}Features:${NC}"
    echo "  - Hot reload enabled (Air)"
    echo "  - Debug logging"
    echo "  - Additional dev tools"
    echo ""
    echo -e "${YELLOW}View logs:${NC} docker-compose -f docker-compose.dev.yml logs -f"
    echo -e "${YELLOW}Stop:${NC}      docker-compose -f docker-compose.dev.yml down"
    echo ""
}

stop_all() {
    print_header
    print_info "Stopping all environments..."
    echo ""

    docker-compose down 2>/dev/null || true
    docker-compose -f docker-compose.dev.yml down 2>/dev/null || true

    print_success "All environments stopped"
    echo ""
}

show_status() {
    print_header
    print_info "Service Status:"
    echo ""

    docker-compose ps 2>/dev/null || print_warning "No production environment running"
    echo ""

    if docker-compose -f docker-compose.dev.yml ps 2>/dev/null | grep -q "Up"; then
        print_info "Development Environment:"
        docker-compose -f docker-compose.dev.yml ps
    fi
    echo ""
}

show_logs() {
    MODE=${1:-"prod"}

    if [ "$MODE" = "dev" ]; then
        docker-compose -f docker-compose.dev.yml logs -f
    else
        docker-compose logs -f
    fi
}

clean_all() {
    print_header
    print_warning "This will remove all containers, volumes, and data!"
    read -p "Are you sure? (yes/no): " -r
    echo ""

    if [[ $REPLY =~ ^[Yy][Ee][Ss]$ ]]; then
        print_info "Cleaning up..."

        docker-compose down -v 2>/dev/null || true
        docker-compose -f docker-compose.dev.yml down -v 2>/dev/null || true
        docker rmi driver-location-service:latest 2>/dev/null || true

        print_success "Cleanup complete"
    else
        print_info "Cleanup cancelled"
    fi
    echo ""
}

show_help() {
    print_header
    echo "Usage: ./start.sh [command]"
    echo ""
    echo "Commands:"
    echo "  prod, production    Start production environment"
    echo "  dev, development    Start development environment"
    echo "  stop                Stop all environments"
    echo "  status              Show service status"
    echo "  logs [dev]          Show logs (default: production)"
    echo "  clean               Remove all containers and volumes"
    echo "  help                Show this help message"
    echo ""
    echo "Examples:"
    echo "  ./start.sh prod              Start production"
    echo "  ./start.sh dev               Start development"
    echo "  ./start.sh logs              View production logs"
    echo "  ./start.sh logs dev          View development logs"
    echo "  ./start.sh stop              Stop everything"
    echo ""
}

# Main script
check_requirements

case "${1:-prod}" in
    prod|production)
        start_production
        ;;
    dev|development)
        start_development
        ;;
    stop)
        stop_all
        ;;
    status)
        show_status
        ;;
    logs)
        show_logs "${2:-prod}"
        ;;
    clean)
        clean_all
        ;;
    help|-h|--help)
        show_help
        ;;
    *)
        print_error "Unknown command: $1"
        echo ""
        show_help
        exit 1
        ;;
esac
