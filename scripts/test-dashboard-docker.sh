#!/bin/bash

# Test Dashboard Docker Build
# This script tests building and running the dashboard in Docker

set -e

SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
PROJECT_ROOT="$(dirname "$SCRIPT_DIR")"

# Colors for output
GREEN='\033[0;32m'
BLUE='\033[0;34m'
YELLOW='\033[1;33m'
RED='\033[0;31m'
NC='\033[0m' # No Color

log() {
    echo -e "${BLUE}[$(date +'%H:%M:%S')]${NC} $1"
}

success() {
    echo -e "${GREEN}✅ $1${NC}"
}

warning() {
    echo -e "${YELLOW}⚠️  $1${NC}"
}

error() {
    echo -e "${RED}❌ $1${NC}"
}

main() {
    echo "============================================"
    echo "🐳 Dashboard Docker Build Test"
    echo "============================================"
    echo ""
    
    cd "$PROJECT_ROOT"
    
    # Test Docker build
    log "Building dashboard Docker image..."
    if docker build -t strangler-dashboard ./dashboard; then
        success "Dashboard Docker image built successfully"
    else
        error "Failed to build dashboard Docker image"
        exit 1
    fi
    
    # Test Docker run (without starting)
    log "Testing Docker image..."
    if docker run --rm strangler-dashboard node --version; then
        success "Dashboard Docker image runs correctly"
    else
        error "Dashboard Docker image failed to run"
        exit 1
    fi
    
    echo ""
    log "Docker image details:"
    docker images strangler-dashboard --format "table {{.Repository}}\t{{.Tag}}\t{{.Size}}\t{{.CreatedAt}}"
    
    echo ""
    success "Dashboard Docker build test completed successfully! 🎉"
    
    echo ""
    log "To run the dashboard with Docker:"
    echo "docker run -p 3000:3000 \\"
    echo "  -e NEXT_PUBLIC_WS_URL=ws://localhost:8080/ws \\"
    echo "  -e NEXT_PUBLIC_PROXY_URL=http://localhost:8080 \\"
    echo "  -e NEXT_PUBLIC_ORDER_SERVICE_URL=http://localhost:8081 \\"
    echo "  -e NEXT_PUBLIC_SAP_URL=http://localhost:8082 \\"
    echo "  strangler-dashboard"
    
    echo ""
    log "Or use Docker Compose:"
    echo "docker-compose up dashboard"
}

main "$@"