#!/bin/bash

# Start Dashboard Development Server
# This script starts the Next.js dashboard in development mode

set -e

SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
PROJECT_ROOT="$(dirname "$SCRIPT_DIR")"
DASHBOARD_DIR="$PROJECT_ROOT/dashboard"

# Colors for output
GREEN='\033[0;32m'
BLUE='\033[0;34m'
YELLOW='\033[1;33m'
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

# Check if Node.js is installed
if ! command -v node &> /dev/null; then
    echo "❌ Node.js is not installed. Please install Node.js 18+ to continue."
    exit 1
fi

# Check Node.js version
NODE_VERSION=$(node --version | cut -d'v' -f2 | cut -d'.' -f1)
if [ "$NODE_VERSION" -lt 18 ]; then
    echo "❌ Node.js version 18+ is required. Current version: $(node --version)"
    exit 1
fi

success "Node.js $(node --version) detected"

# Navigate to dashboard directory
if [ ! -d "$DASHBOARD_DIR" ]; then
    echo "❌ Dashboard directory not found at $DASHBOARD_DIR"
    exit 1
fi

cd "$DASHBOARD_DIR"

# Check if package.json exists
if [ ! -f "package.json" ]; then
    echo "❌ package.json not found. Please run this script from the project root."
    exit 1
fi

# Install dependencies if node_modules doesn't exist or package-lock.json is newer
if [ ! -d "node_modules" ] || [ "package-lock.json" -nt "node_modules" ]; then
    log "Installing dependencies..."
    npm install
    success "Dependencies installed"
else
    log "Dependencies already installed"
fi

# Create .env.local if it doesn't exist
if [ ! -f ".env.local" ]; then
    log "Creating .env.local from example..."
    cp .env.local.example .env.local
    warning "Please review and update .env.local with your service URLs"
else
    log ".env.local already exists"
fi

# Check if Go services are running (optional check)
log "Checking if Go services are running..."

check_service() {
    local url=$1
    local name=$2
    
    if curl -s "$url/health" > /dev/null 2>&1; then
        success "$name is running at $url"
        return 0
    else
        warning "$name is not responding at $url"
        return 1
    fi
}

check_service "http://localhost:8080" "Proxy Service"
check_service "http://localhost:8081" "Order Service"  
check_service "http://localhost:8082" "SAP Mock"

echo ""
log "Starting Next.js development server..."
echo ""
echo "🎯 Dashboard will be available at: http://localhost:3000"
echo "📊 Features available:"
echo "   • Real-time order tracking"
echo "   • Performance metrics"
echo "   • Load testing controls"
echo "   • Data synchronization monitoring"
echo ""
echo "Press Ctrl+C to stop the server"
echo ""

# Start the development server
npm run dev