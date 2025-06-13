# CLAUDE.md

## Current Status: Phase 2 Complete ✅

The dual-write pattern is now implemented with order service, PostgreSQL database, and Kafka event streaming.

## Project Overview

This demo shows the strangler pattern in action. We're building a Go service that sits between an ecommerce system and an SAP backend. The goal is to slowly move from SAP-driven architecture to event-driven microservices.

**The Problem**: Direct calls to SAP are slow. Orders take too long to process. Adding new features is hard.

**The Solution**: Build a proxy service that handles requests faster while gradually moving SAP from driver to consumer.

## Architecture Goals

1. **Phase 1**: Proxy passes requests to SAP (current behavior, but logged) ✅ COMPLETED
2. **Phase 2**: Write to both new service and SAP (safety net) ✅ COMPLETED
3. **Phase 3**: New service only, SAP consumes events

## Project Structure

```
strangler-demo/
├── cmd/
│   ├── proxy/          # Main proxy service (✅ implemented)
│   ├── order-service/  # New order microservice (✅ implemented)
│   └── sap-mock/       # Mock SAP for demo (✅ implemented)
├── internal/
│   ├── orders/         # Order domain logic (✅ implemented)
│   ├── events/         # Event publishing (✅ implemented)
│   └── sap/           # SAP integration (✅ implemented)
├── pkg/
│   └── models/        # Shared data models (✅ implemented)
├── docker-compose.yml  # (✅ implemented)
└── scripts/           # Demo and test scripts (✅ implemented)
```

## Tech Stack

- **Go**: Main language (developer is novice level)
- **Kafka**: Event streaming
- **PostgreSQL**: New order service database
- **Docker**: Everything containerized
- **Gorilla Mux**: HTTP routing

## Key Models

### Order
```go
type Order struct {
    ID           string    `json:"id"`
    CustomerID   string    `json:"customer_id"`
    Items        []OrderItem `json:"items"`
    TotalAmount  float64   `json:"total_amount"`
    DeliveryDate time.Time `json:"delivery_date"`
    Status       string    `json:"status"`
    CreatedAt    time.Time `json:"created_at"`
}
```

### OrderItem
```go
type OrderItem struct {
    ProductID    string  `json:"product_id"`
    Quantity     int     `json:"quantity"`
    UnitPrice    float64 `json:"unit_price"`
    Specifications map[string]string `json:"specifications"`
}
```

## Build Steps

### Step 1: Basic Proxy ✅ COMPLETED
Create HTTP server that:
- Accepts order POST requests ✅
- Logs all requests ✅
- Passes requests to mock SAP ✅
- Returns SAP response ✅

Current implementation includes:
- Gorilla Mux router with middleware
- Structured JSON logging with Logrus
- SAP client with proper error handling
- Health check endpoints
- Graceful shutdown support
- Docker containerization

### Step 2: Add Events ✅ COMPLETED
- Set up Kafka producer ✅
- Publish `order.created` events ✅
- Keep passing to SAP ✅

Current implementation includes:
- Kafka producer with Sarama client
- Order created events published to Kafka
- PostgreSQL database with orders and order_items tables
- Dual write pattern: writes to both Order Service and SAP
- Docker Compose with Kafka, Zookeeper, PostgreSQL
- Health checks and proper error handling

### Step 3: New Order Service ✅ COMPLETED  
- Create order microservice with PostgreSQL ✅
- Write orders to both places ✅
- Publish events from new service ✅

Current implementation includes:
- Standalone order microservice on port 8081
- Full CRUD operations for orders
- PostgreSQL integration with proper schema
- Event publishing integrated into order creation
- RESTful API with proper error handling

### Step 4: SAP Consumer (NEXT PHASE)
- Mock SAP consumes events from Kafka
- Remove direct SAP calls from proxy
- Show complete strangler pattern

## Demo Data

Create sample products:
- Generic widgets, components, assemblies
- Different price points
- Custom specifications (color, finish, delivery)

## Configuration

Use environment variables for all config:
- Database connections
- Kafka brokers
- SAP endpoints
- Port numbers

## Testing

Include these for each component:
- Unit tests for handlers
- Integration tests for Kafka
- End-to-end demo script
- Load test examples

## Development Notes

- **Error Handling**: Return proper HTTP status codes
- **Logging**: Use structured logging (logrus)
- **Graceful Shutdown**: Handle SIGTERM properly
- **Health Checks**: Add `/health` endpoints
- **Metrics**: Basic counters for demo

## Git Workflow Guidelines

### Commit Strategy
When working on this project, create logical commits that represent complete features or logical units of work:

1. **Feature Implementation**: Each major feature should be a separate commit
2. **Documentation Updates**: Group related documentation changes together
3. **Infrastructure Changes**: Docker, dependencies, configuration changes
4. **Testing & Scripts**: Test files, scripts, and verification tools

### Commit Message Format
Use conventional commit format with Claude Code attribution:

```
<type>: <description>

<optional body explaining what and why>

🤖 Generated with [Claude Code](https://claude.ai/code)

Co-Authored-By: Claude <noreply@anthropic.com>
```

**Types**: feat, fix, docs, deps, refactor, test, chore

### Example Commit Workflow
```bash
# After implementing a feature
git add <relevant-files>
git commit -m "feat: implement order comparison endpoints

- Add GET /compare/orders for system-wide comparison
- Add GET /compare/orders/{id} for individual comparison
- Include detailed analysis and sync status indicators

🤖 Generated with [Claude Code](https://claude.ai/code)

Co-Authored-By: Claude <noreply@anthropic.com>"
```

### When to Commit
- ✅ After completing a logical unit of work
- ✅ Before major refactoring or architectural changes
- ✅ After adding new services or major components
- ✅ When documentation significantly changes
- ✅ Before requesting code review or testing

### What NOT to Commit Together
- ❌ Multiple unrelated features in one commit
- ❌ Code changes mixed with large documentation updates
- ❌ Infrastructure changes mixed with business logic
- ❌ Debug code, commented-out code, or temporary fixes

### Branch Strategy (if using branches)
- `main` - Working production-ready code
- `feature/phase-3` - New feature development
- `fix/sap-timeout` - Bug fixes
- `docs/api-updates` - Documentation improvements

This ensures clean history and makes it easy to understand, review, and potentially rollback changes.

## Development Process with Claude Code

### Task Management
Use TodoWrite tool to plan and track implementation:
```
1. Break down large features into smaller tasks
2. Mark tasks as in_progress while working
3. Mark tasks as completed immediately after finishing
4. Create new tasks as requirements emerge
```

### Implementation Workflow
1. **Plan**: Use TodoWrite to create task list
2. **Implement**: Write code incrementally
3. **Test**: Verify functionality as you build
4. **Commit**: Create logical git commits
5. **Document**: Update relevant documentation
6. **Verify**: Run tests and validation scripts

### Code Quality Guidelines
- **Build verification**: Always test `go build` before committing
- **Dependency management**: Run `go mod tidy` after adding dependencies
- **Error handling**: Check for and handle errors appropriately
- **Logging**: Add structured logging for debugging
- **Testing**: Include test scripts and verification

### Documentation Maintenance
Keep these files updated as the project evolves:
- **CLAUDE.md**: Development guidelines and project status
- **README.md**: User-facing documentation and getting started
- **API.md**: Endpoint documentation and examples
- **DOCKER.md**: Infrastructure and deployment guide

## Demo Script Goals

The final demo should show:
1. Ecommerce calls proxy → SAP (slow)
2. Ecommerce calls proxy → new service + events (fast)
3. SAP receives events, updates its data
4. Complete decoupling achieved

## Development Tips

- Build one component at a time
- Test each step before moving forward
- Keep it simple - this is a demo, not production
- Add docker-compose early for easy testing
- Include curl examples for manual testing

## Success Criteria

- Orders process faster than direct SAP calls
- Events flow correctly through Kafka
- SAP mock shows it can consume events
- Clean separation between old and new systems
- Easy to run demo script
