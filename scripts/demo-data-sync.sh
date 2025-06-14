#!/bin/bash

# Data Synchronization Demo
# Shows data comparison and migration capabilities

set -e

SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
PROJECT_ROOT="$(dirname "$SCRIPT_DIR")"

# Configuration
ORDER_SERVICE_URL="http://localhost:8081"
SAP_URL="http://localhost:8082"

# Colors
BLUE='\033[0;34m'
GREEN='\033[0;32m'
YELLOW='\033[1;33m'
NC='\033[0m'

log() {
    echo -e "${BLUE}[$(date +'%H:%M:%S')]${NC} $1"
}

success() {
    echo -e "${GREEN}✅ $1${NC}"
}

demo_step() {
    echo -e "\n${YELLOW}=== $1 ===${NC}\n"
}

main() {
    echo "============================================"
    echo "🔄 Data Synchronization Demo"
    echo "============================================"
    
    # Build data tools
    demo_step "Building Data Tools"
    cd "$PROJECT_ROOT"
    go build -o ./data-tools ./cmd/data-tools
    success "Data tools compiled"
    
    # Show current data state
    demo_step "Current Data State"
    log "Comparing data between Order Service and SAP..."
    ./data-tools -command=compare -format=summary
    
    # Perform migration
    demo_step "Data Migration (Dry Run)"
    log "Testing bidirectional migration..."
    ./data-tools -command=migrate -direction=bidirectional -dry-run=true
    
    demo_step "Actual Migration"
    log "Migrating missing orders between systems..."
    ./data-tools -command=migrate -direction=bidirectional -batch-size=10
    
    # Validate results
    demo_step "Validation"
    log "Validating migration results..."
    ./data-tools -command=validate
    
    # Final comparison
    demo_step "Final Data State"
    log "Comparing data after migration..."
    ./data-tools -command=compare -format=summary
    
    success "Data synchronization demo completed!"
    
    # Cleanup
    rm -f ./data-tools
}

main "$@"