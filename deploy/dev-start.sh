#!/bin/bash

# OneTerm Development Environment Startup Script
# Usage: ./dev-start.sh [frontend|backend|full|stop]

set -e

SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
cd "$SCRIPT_DIR"

# Color definitions
RED='\033[0;31m'
GREEN='\033[0;32m'
YELLOW='\033[1;33m'
BLUE='\033[0;34m'
NC='\033[0m' # No Color

# Print colored messages
print_info() {
    echo -e "${BLUE}[INFO]${NC} $1"
}

print_success() {
    echo -e "${GREEN}[SUCCESS]${NC} $1"
}

print_warning() {
    echo -e "${YELLOW}[WARNING]${NC} $1"
}

print_error() {
    echo -e "${RED}[ERROR]${NC} $1"
}

# Check Docker and Docker Compose
check_requirements() {
    print_info "Checking environment requirements..."
    
    if ! command -v docker &> /dev/null; then
        print_error "Docker is not installed or not in PATH"
        exit 1
    fi
    
    if ! command -v docker-compose &> /dev/null && ! docker compose version &> /dev/null; then
        print_error "Docker Compose is not installed"
        exit 1
    fi
    
    print_success "Environment check passed"
}

# Wait for service startup
wait_for_service() {
    local service_name=$1
    local max_attempts=30
    local attempt=1
    
    print_info "Waiting for $service_name service to start..."
    
    while [ $attempt -le $max_attempts ]; do
        if docker compose -f "$compose_file" ps | grep -q "healthy"; then
            print_success "$service_name service is ready"
            return 0
        fi
        
        echo -n "."
        sleep 2
        ((attempt++))
    done
    
    print_error "$service_name service startup timeout"
    return 1
}

# Start frontend development environment
start_frontend_dev() {
    print_info "Starting frontend development environment..."
    local compose_file="docker-compose.frontend-dev.yaml"
    
    # Stop any existing services
    docker compose -f "$compose_file" down 2>/dev/null || true
    
    # Start services
    print_info "Starting backend dependencies (MySQL, Redis, ACL-API, Guacd, OneTerm-API)..."
    docker compose -f "$compose_file" up -d
    
    # Wait for services to be ready
    sleep 10
    
    # Show service status
    print_info "Service status:"
    docker compose -f "$compose_file" ps
    
    print_success "Frontend development environment started!"
    echo ""
    print_info "Next steps:"
    echo "1. cd ../oneterm-ui"
    echo "2. yarn install"
    echo "3. npm run serve (will start on http://localhost:8000)"
    echo "4. Access application via nginx proxy: http://localhost:8080"
    echo ""
    print_info "Service access URLs:"
    echo "- Application (via Nginx proxy): http://localhost:8080"
    echo "- Frontend Dev Server: http://localhost:8000"
    echo "- OneTerm API (direct): http://localhost:18888"
    echo "- ACL API (direct): http://localhost:15000"
    echo "- MySQL: localhost:13306 (root/123456)"
    echo "- Redis: localhost:16379"
    echo ""
    print_info "Development workflow:"
    echo "- Frontend changes: Edit in oneterm-ui/, hot reload via http://localhost:8000"
    echo "- Unified access: Use http://localhost:8080 (nginx proxy) for production-like experience"
    echo "- OneTerm APIs: Proxied to host.docker.internal:18888 (local backend)"
    echo "- ACL APIs: Proxied to acl-api:5000 (container backend)"
    echo "- No CORS issues when accessing via nginx proxy"
}

# Start backend development environment
start_backend_dev() {
    print_info "Starting backend development environment..."
    local compose_file="docker-compose.backend-dev.yaml"
    
    # Stop any existing services
    docker compose -f "$compose_file" down 2>/dev/null || true
    
    # Start services
    print_info "Starting frontend and dependencies (MySQL, Redis, ACL-API, Guacd, OneTerm-UI)..."
    docker compose -f "$compose_file" up -d
    
    # Wait for services to be ready
    sleep 10
    
    # Show service status
    print_info "Service status:"
    docker compose -f "$compose_file" ps
    
    print_success "Backend development environment started!"
    echo ""
    print_info "Next steps:"
    echo "1. cd ../backend/cmd/server"
    echo "2. cp ../../../deploy/dev-config.example.yaml config.yaml"
    echo "3. go mod tidy  (first time setup)"
    echo "4. go run main.go config.yaml"
    echo ""
    print_info "Service access URLs:"
    echo "- Frontend UI: http://localhost:8666 (admin/123456)"
    echo "- MySQL: localhost:13306 (root/123456)"
    echo "- Redis: localhost:16379"
    echo "- ACL API: http://localhost:15000"
}

# Start full environment
start_full_dev() {
    print_info "Starting full development environment..."
    local compose_file="docker-compose.yaml"
    
    # Stop any existing services
    docker compose -f "$compose_file" down 2>/dev/null || true
    
    # Start all services
    print_info "Starting all services..."
    docker compose -f "$compose_file" up -d
    
    # Wait for services to be ready
    sleep 15
    
    # Show service status
    print_info "Service status:"
    docker compose -f "$compose_file" ps
    
    print_success "Full development environment started!"
    echo ""
    print_info "Service access URLs:"
    echo "- OneTerm Application: http://localhost:8666 (admin/123456)"
    echo "- SSH Port: localhost:2222"
    echo "- MySQL: localhost:13306 (root/123456)"
    echo "- Redis: localhost:16379"
}

# Stop all development environments
stop_dev() {
    print_info "Stopping all development environments..."
    
    # Stop various possible environments
    docker compose -f "docker-compose.frontend-dev.yaml" down 2>/dev/null || true
    docker compose -f "docker-compose.backend-dev.yaml" down 2>/dev/null || true
    docker compose -f "docker-compose.yaml" down 2>/dev/null || true
    
    print_success "All development environments stopped"
}

# Show usage help
show_help() {
    echo "OneTerm Development Environment Management Script"
    echo ""
    echo "Usage: $0 [option]"
    echo ""
    echo "Options:"
    echo "  frontend    Start frontend dev environment (local frontend, containerized backend deps)"
    echo "  backend     Start backend dev environment (local backend, containerized frontend and deps)"
    echo "  full        Start full environment (all services in containers)"
    echo "  stop        Stop all development environments"
    echo "  help        Show this help message"
    echo ""
    echo "Examples:"
    echo "  $0 frontend     # Frontend development mode"
    echo "  $0 backend      # Backend development mode" 
    echo "  $0 full         # Full experience mode"
    echo "  $0 stop         # Stop all services"
}

# Main function
main() {
    case "${1:-help}" in
        "frontend")
            check_requirements
            start_frontend_dev
            ;;
        "backend")
            check_requirements
            start_backend_dev
            ;;
        "full")
            check_requirements
            start_full_dev
            ;;
        "stop")
            stop_dev
            ;;
        "help"|*)
            show_help
            ;;
    esac
}

# Execute main function
main "$@"