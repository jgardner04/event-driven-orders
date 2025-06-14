'use client';

import { SystemMetrics } from '@/types';
import { 
  LineChart, 
  Line, 
  AreaChart, 
  Area, 
  BarChart, 
  Bar, 
  XAxis, 
  YAxis, 
  CartesianGrid, 
  Tooltip, 
  ResponsiveContainer,
  PieChart,
  Pie,
  Cell
} from 'recharts';

interface PerformanceChartsProps {
  metrics: SystemMetrics[];
  timeRange: '5m' | '15m' | '1h' | '24h';
}

export function PerformanceCharts({ metrics, timeRange }: PerformanceChartsProps) {
  // Prepare data for charts
  const chartData = metrics.map((metric, index) => ({
    timestamp: new Date(metric.timestamp).toLocaleTimeString(),
    time: index,
    
    // Proxy metrics
    proxyRps: metric.proxy.requests_per_second,
    proxyResponseTime: metric.proxy.avg_response_time,
    proxyErrorRate: metric.proxy.error_rate,
    proxyConnections: metric.proxy.active_connections,
    
    // Order Service metrics
    osOrders: metric.order_service.orders_created,
    osProcessingTime: metric.order_service.avg_processing_time,
    osDbConnections: metric.order_service.database_connections,
    osKafkaEvents: metric.order_service.kafka_events_published,
    
    // SAP metrics
    sapOrders: metric.sap_mock.orders_processed,
    sapResponseTime: metric.sap_mock.avg_response_time,
    sapEventsConsumed: metric.sap_mock.events_consumed,
    sapFailureRate: metric.sap_mock.failure_rate,
    
    // Kafka metrics
    kafkaMessages: metric.kafka.messages_per_second,
    kafkaLag: metric.kafka.consumer_lag,
  }));

  // Service comparison data
  const serviceComparisonData = metrics.length > 0 ? [
    {
      name: 'Proxy',
      responseTime: metrics[metrics.length - 1]?.proxy.avg_response_time || 0,
      errorRate: metrics[metrics.length - 1]?.proxy.error_rate || 0,
      throughput: metrics[metrics.length - 1]?.proxy.requests_per_second || 0,
    },
    {
      name: 'Order Service',
      responseTime: metrics[metrics.length - 1]?.order_service.avg_processing_time || 0,
      errorRate: 0, // Order service doesn't report error rate in our metrics
      throughput: metrics[metrics.length - 1]?.order_service.orders_created / 60 || 0, // Convert to per second
    },
    {
      name: 'SAP Mock',
      responseTime: metrics[metrics.length - 1]?.sap_mock.avg_response_time || 0,
      errorRate: metrics[metrics.length - 1]?.sap_mock.failure_rate || 0,
      throughput: metrics[metrics.length - 1]?.sap_mock.orders_processed / 60 || 0, // Convert to per second
    },
  ] : [];

  // Order flow data for pie chart
  const orderFlowData = metrics.length > 0 ? [
    {
      name: 'Order Service',
      value: metrics[metrics.length - 1]?.order_service.orders_created || 0,
      color: '#3B82F6',
    },
    {
      name: 'SAP Mock',
      value: metrics[metrics.length - 1]?.sap_mock.orders_processed || 0,
      color: '#10B981',
    },
  ] : [];

  const colors = {
    primary: '#3B82F6',
    secondary: '#10B981',
    tertiary: '#F59E0B',
    danger: '#EF4444',
    purple: '#8B5CF6',
  };

  if (metrics.length === 0) {
    return (
      <div className="text-center py-8 text-muted-foreground">
        <p>No performance data available</p>
      </div>
    );
  }

  return (
    <div className="space-y-6">
      {/* Response Time Trends */}
      <div className="bg-card border rounded-lg p-6">
        <h3 className="text-lg font-semibold mb-4">Response Time Trends</h3>
        <div className="h-64">
          <ResponsiveContainer width="100%" height="100%">
            <LineChart data={chartData}>
              <CartesianGrid strokeDasharray="3 3" />
              <XAxis 
                dataKey="timestamp" 
                tick={{ fontSize: 12 }}
                interval="preserveStartEnd"
              />
              <YAxis tick={{ fontSize: 12 }} />
              <Tooltip 
                labelFormatter={(label) => `Time: ${label}`}
                formatter={(value: number, name: string) => [
                  `${value}ms`,
                  name.replace(/([A-Z])/g, ' $1').replace(/^./, str => str.toUpperCase())
                ]}
              />
              <Line 
                type="monotone" 
                dataKey="proxyResponseTime" 
                stroke={colors.primary} 
                strokeWidth={2}
                name="Proxy"
                dot={false}
              />
              <Line 
                type="monotone" 
                dataKey="osProcessingTime" 
                stroke={colors.secondary} 
                strokeWidth={2}
                name="Order Service"
                dot={false}
              />
              <Line 
                type="monotone" 
                dataKey="sapResponseTime" 
                stroke={colors.tertiary} 
                strokeWidth={2}
                name="SAP Mock"
                dot={false}
              />
            </LineChart>
          </ResponsiveContainer>
        </div>
      </div>

      {/* Throughput and Error Rates */}
      <div className="grid grid-cols-1 lg:grid-cols-2 gap-6">
        <div className="bg-card border rounded-lg p-6">
          <h3 className="text-lg font-semibold mb-4">Throughput (Requests/sec)</h3>
          <div className="h-64">
            <ResponsiveContainer width="100%" height="100%">
              <AreaChart data={chartData}>
                <CartesianGrid strokeDasharray="3 3" />
                <XAxis 
                  dataKey="timestamp" 
                  tick={{ fontSize: 12 }}
                  interval="preserveStartEnd"
                />
                <YAxis tick={{ fontSize: 12 }} />
                <Tooltip 
                  formatter={(value: number, name: string) => [
                    `${value} req/s`,
                    name.replace(/([A-Z])/g, ' $1').replace(/^./, str => str.toUpperCase())
                  ]}
                />
                <Area 
                  type="monotone" 
                  dataKey="proxyRps" 
                  stackId="1"
                  stroke={colors.primary} 
                  fill={colors.primary}
                  fillOpacity={0.6}
                  name="Proxy RPS"
                />
                <Area 
                  type="monotone" 
                  dataKey="kafkaMessages" 
                  stackId="2"
                  stroke={colors.purple} 
                  fill={colors.purple}
                  fillOpacity={0.6}
                  name="Kafka Messages/sec"
                />
              </AreaChart>
            </ResponsiveContainer>
          </div>
        </div>

        <div className="bg-card border rounded-lg p-6">
          <h3 className="text-lg font-semibold mb-4">Error Rates (%)</h3>
          <div className="h-64">
            <ResponsiveContainer width="100%" height="100%">
              <LineChart data={chartData}>
                <CartesianGrid strokeDasharray="3 3" />
                <XAxis 
                  dataKey="timestamp" 
                  tick={{ fontSize: 12 }}
                  interval="preserveStartEnd"
                />
                <YAxis tick={{ fontSize: 12 }} />
                <Tooltip 
                  formatter={(value: number, name: string) => [
                    `${value}%`,
                    name.replace(/([A-Z])/g, ' $1').replace(/^./, str => str.toUpperCase())
                  ]}
                />
                <Line 
                  type="monotone" 
                  dataKey="proxyErrorRate" 
                  stroke={colors.danger} 
                  strokeWidth={2}
                  name="Proxy Error Rate"
                  dot={false}
                />
                <Line 
                  type="monotone" 
                  dataKey="sapFailureRate" 
                  stroke={colors.tertiary} 
                  strokeWidth={2}
                  name="SAP Failure Rate"
                  dot={false}
                />
              </LineChart>
            </ResponsiveContainer>
          </div>
        </div>
      </div>

      {/* Service Comparison */}
      <div className="bg-card border rounded-lg p-6">
        <h3 className="text-lg font-semibold mb-4">Service Performance Comparison</h3>
        <div className="h-64">
          <ResponsiveContainer width="100%" height="100%">
            <BarChart data={serviceComparisonData}>
              <CartesianGrid strokeDasharray="3 3" />
              <XAxis dataKey="name" tick={{ fontSize: 12 }} />
              <YAxis tick={{ fontSize: 12 }} />
              <Tooltip 
                formatter={(value: number, name: string) => {
                  const unit = name.includes('responseTime') ? 'ms' : 
                               name.includes('errorRate') ? '%' : 'req/s';
                  return [`${value.toFixed(2)}${unit}`, name];
                }}
              />
              <Bar dataKey="responseTime" fill={colors.primary} name="Response Time (ms)" />
              <Bar dataKey="throughput" fill={colors.secondary} name="Throughput (req/s)" />
            </BarChart>
          </ResponsiveContainer>
        </div>
      </div>

      {/* Order Processing Distribution */}
      <div className="grid grid-cols-1 lg:grid-cols-2 gap-6">
        <div className="bg-card border rounded-lg p-6">
          <h3 className="text-lg font-semibold mb-4">Order Processing Distribution</h3>
          <div className="h-64">
            <ResponsiveContainer width="100%" height="100%">
              <PieChart>
                <Pie
                  data={orderFlowData}
                  cx="50%"
                  cy="50%"
                  labelLine={false}
                  label={({ name, percent }) => `${name} (${(percent * 100).toFixed(0)}%)`}
                  outerRadius={80}
                  fill="#8884d8"
                  dataKey="value"
                >
                  {orderFlowData.map((entry, index) => (
                    <Cell key={`cell-${index}`} fill={entry.color} />
                  ))}
                </Pie>
                <Tooltip formatter={(value: number) => [`${value} orders`, 'Orders Processed']} />
              </PieChart>
            </ResponsiveContainer>
          </div>
        </div>

        <div className="bg-card border rounded-lg p-6">
          <h3 className="text-lg font-semibold mb-4">System Resources</h3>
          <div className="h-64">
            <ResponsiveContainer width="100%" height="100%">
              <AreaChart data={chartData}>
                <CartesianGrid strokeDasharray="3 3" />
                <XAxis 
                  dataKey="timestamp" 
                  tick={{ fontSize: 12 }}
                  interval="preserveStartEnd"
                />
                <YAxis tick={{ fontSize: 12 }} />
                <Tooltip 
                  formatter={(value: number, name: string) => [
                    value,
                    name.replace(/([A-Z])/g, ' $1').replace(/^./, str => str.toUpperCase())
                  ]}
                />
                <Area 
                  type="monotone" 
                  dataKey="proxyConnections" 
                  stroke={colors.primary} 
                  fill={colors.primary}
                  fillOpacity={0.6}
                  name="Active Connections"
                />
                <Area 
                  type="monotone" 
                  dataKey="osDbConnections" 
                  stroke={colors.secondary} 
                  fill={colors.secondary}
                  fillOpacity={0.6}
                  name="DB Connections"
                />
              </AreaChart>
            </ResponsiveContainer>
          </div>
        </div>
      </div>

      {/* Kafka Metrics */}
      <div className="bg-card border rounded-lg p-6">
        <h3 className="text-lg font-semibold mb-4">Kafka Event Stream Metrics</h3>
        <div className="h-64">
          <ResponsiveContainer width="100%" height="100%">
            <LineChart data={chartData}>
              <CartesianGrid strokeDasharray="3 3" />
              <XAxis 
                dataKey="timestamp" 
                tick={{ fontSize: 12 }}
                interval="preserveStartEnd"
              />
              <YAxis tick={{ fontSize: 12 }} />
              <Tooltip 
                formatter={(value: number, name: string) => [
                  value,
                  name.replace(/([A-Z])/g, ' $1').replace(/^./, str => str.toUpperCase())
                ]}
              />
              <Line 
                type="monotone" 
                dataKey="osKafkaEvents" 
                stroke={colors.primary} 
                strokeWidth={2}
                name="Events Published"
                dot={false}
              />
              <Line 
                type="monotone" 
                dataKey="sapEventsConsumed" 
                stroke={colors.secondary} 
                strokeWidth={2}
                name="Events Consumed"
                dot={false}
              />
              <Line 
                type="monotone" 
                dataKey="kafkaLag" 
                stroke={colors.danger} 
                strokeWidth={2}
                name="Consumer Lag"
                dot={false}
              />
            </LineChart>
          </ResponsiveContainer>
        </div>
      </div>
    </div>
  );
}