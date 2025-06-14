# Strangler Pattern Dashboard

A real-time monitoring dashboard for the strangler pattern demo, built with Next.js, TypeScript, Tailwind CSS, and WebSocket integration.

## Features

### 🔴 Real-time Monitoring
- **Live Order Tracking**: Real-time order creation and updates via WebSocket
- **System Health**: Continuous monitoring of all services (Proxy, Order Service, SAP Mock)
- **Performance Metrics**: Real-time visualization of throughput, response times, and error rates
- **Connection Status**: Live WebSocket connection indicator with auto-reconnect

### 📊 Performance Analytics
- **Interactive Charts**: Recharts-powered visualizations with multiple chart types
- **Service Comparison**: Side-by-side performance comparison between services
- **Time Range Selection**: 5m, 15m, 1h, and 24h time windows
- **Historical Data**: Metrics history with configurable retention
- **Kafka Metrics**: Event stream monitoring and consumer lag tracking

### ⚡ Load Testing
- **Configurable Tests**: Adjustable duration, RPS, concurrency, and target endpoints
- **Multiple Scenarios**: Create orders, get orders, and mixed operation patterns
- **Real-time Results**: Live test progress and metrics
- **Performance Analysis**: Detailed response time percentiles and error analysis
- **Test Management**: Start, stop, and monitor multiple concurrent tests

### 🔄 Data Synchronization
- **System Comparison**: Compare data consistency between Order Service and SAP Mock
- **Inconsistency Detection**: Identify missing orders and data mismatches
- **Sync Percentage**: Calculate and display data synchronization health
- **Actionable Recommendations**: Smart suggestions for improving data consistency

### 📱 Responsive Design
- **Mobile-First**: Optimized for all screen sizes with Tailwind CSS
- **Dark Mode Ready**: CSS custom properties for easy theme switching
- **Accessibility**: Screen reader friendly with proper ARIA labels
- **Progressive Enhancement**: Works with JavaScript disabled

## Tech Stack

- **Framework**: Next.js 14 with App Router
- **Language**: TypeScript
- **Styling**: Tailwind CSS
- **Charts**: Recharts
- **Icons**: Lucide React
- **WebSocket**: Native WebSocket API with reconnection logic
- **HTTP Client**: Axios
- **Notifications**: React Hot Toast
- **Date Handling**: date-fns

## Getting Started

### Prerequisites

- Node.js 18+ 
- npm or yarn
- Go services running (Proxy, Order Service, SAP Mock)

### Installation

1. **Install dependencies**:
   ```bash
   cd dashboard
   npm install
   ```

2. **Configure environment**:
   ```bash
   cp .env.local.example .env.local
   # Edit .env.local with your service URLs
   ```

3. **Start development server**:
   ```bash
   npm run dev
   ```

4. **Open dashboard**:
   Navigate to [http://localhost:3000](http://localhost:3000)

### Environment Variables

```bash
# WebSocket connection
NEXT_PUBLIC_WS_URL=ws://localhost:8080/ws

# Service endpoints
NEXT_PUBLIC_PROXY_URL=http://localhost:8080
NEXT_PUBLIC_ORDER_SERVICE_URL=http://localhost:8081
NEXT_PUBLIC_SAP_URL=http://localhost:8082

# Performance tuning
NEXT_PUBLIC_REFRESH_INTERVAL=30000
NEXT_PUBLIC_MAX_METRICS_HISTORY=60
```

### Production Build

```bash
npm run build
npm start
```

## Dashboard Sections

### 1. Overview
- **System Health Cards**: Overall status, total orders, active tests, data sync
- **Service Status**: Health check results for all services
- **Performance Summary**: Key metrics at a glance
- **Recent Orders**: Live table of newest orders

### 2. Orders
- **Order Management**: Complete order listing with search and filters
- **Real-time Updates**: Live order creation notifications
- **Export Functionality**: CSV export of order data
- **Test Order Creation**: Generate sample orders for testing

### 3. Performance
- **Response Time Trends**: Multi-service response time comparison
- **Throughput Analysis**: Requests per second and Kafka message rates
- **Error Rate Monitoring**: Service-specific error tracking
- **Resource Usage**: Connection pools and system resources

### 4. Load Testing
- **Test Configuration**: Duration, RPS, concurrency, target selection
- **Scenario Selection**: Create orders, fetch orders, mixed operations
- **Live Monitoring**: Real-time test progress and metrics
- **Results Analysis**: Detailed performance breakdown and percentiles

### 5. Data Synchronization
- **Sync Health**: Overall data consistency between services
- **Inconsistency Detection**: Missing orders and data mismatches
- **Recommendations**: Actionable suggestions for data sync issues
- **Manual Comparison**: On-demand data analysis

## WebSocket Integration

The dashboard connects to Go services via WebSocket for real-time updates:

### Message Types
- `order_created`: New order notifications
- `order_updated`: Order status changes
- `metrics_update`: Performance metrics
- `health_update`: Service health changes
- `load_test_update`: Load test progress

### Connection Management
- **Auto-reconnect**: Exponential backoff on connection loss
- **Status Indicator**: Visual connection state
- **Graceful Degradation**: Continues working without WebSocket
- **Manual Reconnect**: User-initiated reconnection

## API Integration

### Service Health Checks
```typescript
GET /health
Response: { status: 'healthy' | 'unhealthy', service: string, response_time?: number }
```

### Order Management
```typescript
GET /orders
Response: { success: boolean, orders: Order[], count: number }

POST /orders
Body: Order
Response: { success: boolean, order: Order, message: string }
```

### Metrics Collection
```typescript
// Simulated metrics endpoint
SystemMetrics: {
  timestamp: string,
  proxy: { requests_per_second, avg_response_time, error_rate, active_connections },
  order_service: { orders_created, avg_processing_time, database_connections, kafka_events_published },
  sap_mock: { orders_processed, avg_response_time, events_consumed, failure_rate },
  kafka: { messages_per_second, consumer_lag, partition_count, broker_status }
}
```

## Customization

### Adding New Metrics
1. Update `types/index.ts` with new metric types
2. Modify `lib/api.ts` to fetch new data
3. Update visualization components
4. Add new charts to `PerformanceCharts.tsx`

### Custom Charts
```typescript
import { LineChart, Line, XAxis, YAxis, Tooltip, ResponsiveContainer } from 'recharts';

<ResponsiveContainer width="100%" height={300}>
  <LineChart data={data}>
    <XAxis dataKey="timestamp" />
    <YAxis />
    <Tooltip />
    <Line type="monotone" dataKey="value" stroke="#3B82F6" />
  </LineChart>
</ResponsiveContainer>
```

### Styling Customization
- **Colors**: Edit CSS custom properties in `globals.css`
- **Components**: Extend Tailwind classes in component files
- **Layout**: Modify grid layouts and responsive breakpoints
- **Animations**: Add custom animations in `tailwind.config.js`

## Performance Optimization

### Data Management
- **Metrics History**: Limited to last 60 data points
- **Order History**: Configurable maximum order retention
- **WebSocket Buffering**: Efficient message handling and debouncing
- **Chart Optimization**: Recharts with performance optimizations

### Memory Management
- **Component Cleanup**: Proper WebSocket cleanup on unmount
- **State Optimization**: Efficient state updates and re-renders
- **Image Optimization**: Next.js automatic image optimization
- **Bundle Splitting**: Code splitting for better loading performance

## Troubleshooting

### Common Issues

**WebSocket Connection Failed**
```bash
# Check if Go services are running
curl http://localhost:8080/health
curl http://localhost:8081/health
curl http://localhost:8082/health

# Verify WebSocket endpoint
curl -i -N -H "Connection: Upgrade" -H "Upgrade: websocket" http://localhost:8080/ws
```

**Charts Not Rendering**
- Verify data format matches chart expectations
- Check browser console for JavaScript errors
- Ensure Recharts peer dependencies are installed

**Performance Issues**
- Reduce metrics history size in environment variables
- Disable auto-refresh for large datasets
- Check network tab for API response times

**Styling Issues**
- Run `npm run build` to regenerate Tailwind classes
- Check for CSS custom property browser support
- Verify Tailwind configuration and PostCSS setup

### Development Tips

**Hot Reload**
```bash
# Enable fast refresh
npm run dev
# Modify files and see instant updates
```

**TypeScript Checking**
```bash
# Run type checking
npx tsc --noEmit
```

**Linting**
```bash
# ESLint checking
npm run lint
```

## Deployment

### Production Deployment

**Docker**
```dockerfile
FROM node:18-alpine
WORKDIR /app
COPY package*.json ./
RUN npm ci --only=production
COPY . .
RUN npm run build
EXPOSE 3000
CMD ["npm", "start"]
```

**Environment Variables**
```bash
# Production environment
NODE_ENV=production
NEXT_PUBLIC_WS_URL=wss://your-domain.com/ws
NEXT_PUBLIC_PROXY_URL=https://your-domain.com
```

**Static Export** (optional)
```bash
npm run build
npm run export
# Deploy static files to CDN
```

### Docker Compose Integration

```yaml
services:
  dashboard:
    build: ./dashboard
    ports:
      - "3000:3000"
    environment:
      - NEXT_PUBLIC_WS_URL=ws://proxy:8080/ws
      - NEXT_PUBLIC_PROXY_URL=http://proxy:8080
      - NEXT_PUBLIC_ORDER_SERVICE_URL=http://order-service:8081
      - NEXT_PUBLIC_SAP_URL=http://sap-mock:8082
    depends_on:
      - proxy
      - order-service
      - sap-mock
    networks:
      - strangler-net
```

## Contributing

1. Fork the repository
2. Create a feature branch
3. Make changes with proper TypeScript types
4. Add tests for new functionality
5. Update documentation
6. Submit a pull request

## License

This project is part of the strangler pattern demo and is intended for educational purposes.