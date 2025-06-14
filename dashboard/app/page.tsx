'use client';

import { useState, useEffect, useCallback } from 'react';
import { 
  Activity, 
  Database, 
  Server, 
  TrendingUp, 
  AlertTriangle, 
  RefreshCw,
  Settings,
  BarChart3,
  Clock,
  Users,
  CheckCircle,
  XCircle,
  GitCompare,
  Zap
} from 'lucide-react';
import toast from 'react-hot-toast';

import { DashboardState, SystemMetrics, OrderEvent, LoadTestResult, LoadTestConfig } from '@/types';
import { getWebSocketManager } from '@/lib/websocket';
import { ApiClient } from '@/lib/api';

import { ConnectionStatus } from '@/components/ConnectionStatus';
import { MetricCard } from '@/components/MetricCard';
import { OrdersTable } from '@/components/OrdersTable';
import { LoadTestPanel } from '@/components/LoadTestPanel';
import { PerformanceCharts } from '@/components/PerformanceCharts';

export default function Dashboard() {
  const [state, setState] = useState<DashboardState>({
    orders: [],
    recentOrders: [],
    systemMetrics: null,
    serviceHealth: {},
    loadTests: [],
    comparison: null,
    websocketStatus: 'disconnected',
    lastUpdate: new Date().toISOString(),
  });

  const [metricsHistory, setMetricsHistory] = useState<SystemMetrics[]>([]);
  const [activeTab, setActiveTab] = useState<'overview' | 'orders' | 'performance' | 'load-test' | 'comparison'>('overview');
  const [timeRange, setTimeRange] = useState<'5m' | '15m' | '1h' | '24h'>('15m');
  const [autoRefresh, setAutoRefresh] = useState(true);
  const [isLoading, setIsLoading] = useState(false);

  const wsManager = getWebSocketManager();

  // Initialize WebSocket and data fetching
  useEffect(() => {
    // Connect to WebSocket
    wsManager.connect().catch(console.error);

    // Set up WebSocket listeners
    const unsubscribeStatus = wsManager.onStatusChange((status) => {
      setState(prev => ({ ...prev, websocketStatus: status }));
      
      if (status === 'connected') {
        toast.success('Connected to real-time updates');
      } else if (status === 'disconnected') {
        toast.error('Lost connection to real-time updates');
      }
    });

    const unsubscribeOrders = wsManager.subscribe('order_created', (orderEvent: OrderEvent) => {
      setState(prev => ({
        ...prev,
        recentOrders: [orderEvent, ...prev.recentOrders.slice(0, 49)], // Keep last 50
        orders: [orderEvent.order, ...prev.orders],
        lastUpdate: new Date().toISOString(),
      }));
      
      toast.success(`New order created: ${orderEvent.order.id}`);
    });

    const unsubscribeMetrics = wsManager.subscribe('metrics_update', (metrics: SystemMetrics) => {
      setState(prev => ({ ...prev, systemMetrics: metrics, lastUpdate: new Date().toISOString() }));
      setMetricsHistory(prev => [...prev.slice(-59), metrics]); // Keep last 60 points
    });

    const unsubscribeHealth = wsManager.subscribe('health_update', (health) => {
      setState(prev => ({ ...prev, serviceHealth: health }));
    });

    // Initial data fetch
    fetchInitialData();
    
    // Also fetch orders separately to debug
    ApiClient.getOrders().then(orders => {
      console.log('Direct orders fetch succeeded:', orders.length);
      if (orders.length > 0) {
        setState(prev => ({ ...prev, orders }));
      }
    }).catch(error => {
      console.error('Direct orders fetch failed:', error);
    });

    // Set up auto-refresh
    const refreshInterval = setInterval(() => {
      if (autoRefresh) {
        refreshData();
      }
    }, 30000); // Refresh every 30 seconds

    return () => {
      unsubscribeStatus();
      unsubscribeOrders();
      unsubscribeMetrics();
      unsubscribeHealth();
      clearInterval(refreshInterval);
      wsManager.disconnect();
    };
  }, []);

  const fetchInitialData = async () => {
    setIsLoading(true);
    console.log('Starting to fetch initial data...');
    try {
      const [orders, health, metrics, comparison] = await Promise.allSettled([
        ApiClient.getOrders(),
        ApiClient.checkAllServicesHealth(),
        ApiClient.getSystemMetrics(),
        ApiClient.compareData().catch(() => null), // Don't fail if comparison fails
      ]);

      console.log('Fetch results:', {
        orders: orders.status === 'fulfilled' ? `${orders.value.length} orders` : 'failed',
        health: health.status,
        metrics: metrics.status,
        comparison: comparison.status
      });

      if (orders.status === 'rejected') {
        console.error('Failed to fetch orders:', orders.reason);
      }

      setState(prev => ({
        ...prev,
        orders: orders.status === 'fulfilled' ? orders.value : [],
        serviceHealth: health.status === 'fulfilled' ? health.value : {},
        systemMetrics: metrics.status === 'fulfilled' ? metrics.value : null,
        comparison: comparison.status === 'fulfilled' ? comparison.value : null,
        lastUpdate: new Date().toISOString(),
      }));

      if (metrics.status === 'fulfilled' && metrics.value) {
        setMetricsHistory([metrics.value]);
      }
    } catch (error) {
      console.error('Failed to fetch initial data:', error);
      toast.error('Failed to load dashboard data');
    } finally {
      setIsLoading(false);
    }
  };

  const refreshData = useCallback(async () => {
    try {
      const [orders, health, metrics] = await Promise.allSettled([
        ApiClient.getOrders(),
        ApiClient.checkAllServicesHealth(),
        ApiClient.getSystemMetrics(),
      ]);

      console.log('Refresh data - orders fetched:', orders.status === 'fulfilled' ? orders.value.length : 'failed');

      setState(prev => ({
        ...prev,
        orders: orders.status === 'fulfilled' ? orders.value : prev.orders,
        serviceHealth: health.status === 'fulfilled' ? health.value : prev.serviceHealth,
        systemMetrics: metrics.status === 'fulfilled' ? metrics.value : prev.systemMetrics,
        lastUpdate: new Date().toISOString(),
      }));

      if (metrics.status === 'fulfilled' && metrics.value) {
        setMetricsHistory(prev => [...prev.slice(-59), metrics.value!]);
      }
      
      if (orders.status === 'fulfilled') {
        toast.success(`Refreshed: ${orders.value.length} orders loaded`);
      }
    } catch (error) {
      console.error('Failed to refresh data:', error);
    }
  }, []);

  const handleReconnect = () => {
    wsManager.connect().catch(console.error);
  };

  const handleStartLoadTest = async (config: LoadTestConfig): Promise<LoadTestResult> => {
    try {
      const result = await ApiClient.startLoadTest(config);
      setState(prev => ({
        ...prev,
        loadTests: [...prev.loadTests, result],
      }));
      toast.success('Load test started');
      return result;
    } catch (error) {
      toast.error('Failed to start load test');
      throw error;
    }
  };

  const handleStopLoadTest = (testId: string) => {
    setState(prev => ({
      ...prev,
      loadTests: prev.loadTests.map(test => 
        test.id === testId ? { ...test, status: 'completed', end_time: new Date().toISOString() } : test
      ),
    }));
    toast.success('Load test stopped');
  };

  const handleCreateTestOrder = async () => {
    try {
      const order = await ApiClient.createOrder(ApiClient.generateSampleOrder());
      setState(prev => ({
        ...prev,
        orders: [order, ...prev.orders],
      }));
      toast.success(`Order created: ${order.id}`);
    } catch (error) {
      toast.error('Failed to create test order');
    }
  };

  const handleRunComparison = async () => {
    try {
      const comparison = await ApiClient.compareData();
      setState(prev => ({ ...prev, comparison }));
      toast.success('Data comparison completed');
    } catch (error) {
      toast.error('Failed to run data comparison');
    }
  };

  const getOverallHealthStatus = () => {
    const healthValues = Object.values(state.serviceHealth);
    if (healthValues.length === 0) return 'unknown';
    
    const unhealthyCount = healthValues.filter(h => h.status === 'unhealthy').length;
    if (unhealthyCount === 0) return 'healthy';
    if (unhealthyCount === healthValues.length) return 'critical';
    return 'degraded';
  };

  const activeLoadTests = state.loadTests.filter(test => test.status === 'running');
  const completedLoadTests = state.loadTests.filter(test => test.status === 'completed');

  return (
    <div className="min-h-screen bg-background">
      {/* Header */}
      <header className="border-b bg-card">
        <div className="px-6 py-4">
          <div className="flex items-center justify-between">
            <div>
              <h1 className="text-2xl font-bold">Strangler Pattern Dashboard</h1>
              <p className="text-muted-foreground">Real-time monitoring and load testing</p>
            </div>
            
            <div className="flex items-center gap-4">
              <ConnectionStatus 
                status={state.websocketStatus} 
                onReconnect={handleReconnect}
              />
              
              <div className="flex items-center gap-2">
                <button
                  onClick={refreshData}
                  disabled={isLoading}
                  className="flex items-center gap-2 px-3 py-2 text-sm border rounded-md hover:bg-secondary transition-colors"
                >
                  <RefreshCw className={`w-4 h-4 ${isLoading ? 'animate-spin' : ''}`} />
                  Refresh
                </button>
                
                <button
                  onClick={handleCreateTestOrder}
                  className="flex items-center gap-2 px-3 py-2 text-sm bg-primary text-primary-foreground rounded-md hover:bg-primary/90 transition-colors"
                >
                  <TrendingUp className="w-4 h-4" />
                  Create Test Order
                </button>
              </div>
            </div>
          </div>
          
          {/* Navigation */}
          <nav className="flex items-center gap-6 mt-4">
            {[
              { id: 'overview', label: 'Overview', icon: BarChart3 },
              { id: 'orders', label: 'Orders', icon: Database },
              { id: 'performance', label: 'Performance', icon: Activity },
              { id: 'load-test', label: 'Load Testing', icon: Zap },
              { id: 'comparison', label: 'Data Sync', icon: GitCompare },
            ].map(tab => (
              <button
                key={tab.id}
                onClick={() => setActiveTab(tab.id as any)}
                className={`flex items-center gap-2 px-3 py-2 text-sm rounded-md transition-colors ${
                  activeTab === tab.id 
                    ? 'bg-primary text-primary-foreground' 
                    : 'hover:bg-secondary'
                }`}
              >
                <tab.icon className="w-4 h-4" />
                {tab.label}
              </button>
            ))}
          </nav>
        </div>
      </header>

      {/* Main Content */}
      <main className="p-6">
        {activeTab === 'overview' && (
          <div className="space-y-6">
            {/* System Status Cards */}
            <div className="grid grid-cols-1 md:grid-cols-2 lg:grid-cols-4 gap-4">
              <MetricCard
                title="System Health"
                value={getOverallHealthStatus().toUpperCase()}
                icon={getOverallHealthStatus() === 'healthy' ? <CheckCircle className="w-4 h-4" /> : <XCircle className="w-4 h-4" />}
                status={getOverallHealthStatus() === 'healthy' ? 'healthy' : 'error'}
              />
              
              <MetricCard
                title="Total Orders"
                value={state.orders.length}
                icon={<Database className="w-4 h-4" />}
                change={{
                  value: state.recentOrders.length,
                  trend: 'up',
                  period: 'recent'
                }}
              />
              
              <MetricCard
                title="Active Tests"
                value={activeLoadTests.length}
                icon={<Zap className="w-4 h-4" />}
                status={activeLoadTests.length > 0 ? 'warning' : 'healthy'}
              />
              
              <MetricCard
                title="Data Sync"
                value={state.comparison?.analysis.sync_percentage ? `${state.comparison.analysis.sync_percentage.toFixed(1)}%` : 'N/A'}
                icon={<GitCompare className="w-4 h-4" />}
                status={
                  !state.comparison ? 'warning' :
                  state.comparison.analysis.sync_percentage >= 95 ? 'healthy' : 'warning'
                }
              />
            </div>

            {/* Service Health */}
            <div className="grid grid-cols-1 md:grid-cols-3 gap-4">
              {Object.entries(state.serviceHealth).map(([service, health]) => (
                <MetricCard
                  key={service}
                  title={service.replace('_', ' ').toUpperCase()}
                  value={health.status.toUpperCase()}
                  unit={health.response_time ? `${health.response_time}ms` : undefined}
                  icon={<Server className="w-4 h-4" />}
                  status={health.status === 'healthy' ? 'healthy' : 'error'}
                />
              ))}
            </div>

            {/* Performance Overview */}
            {state.systemMetrics && (
              <div className="grid grid-cols-1 md:grid-cols-2 lg:grid-cols-4 gap-4">
                <MetricCard
                  title="Proxy RPS"
                  value={state.systemMetrics.proxy.requests_per_second}
                  unit="req/s"
                  icon={<TrendingUp className="w-4 h-4" />}
                />
                
                <MetricCard
                  title="Avg Response Time"
                  value={state.systemMetrics.proxy.avg_response_time}
                  unit="ms"
                  icon={<Clock className="w-4 h-4" />}
                  status={state.systemMetrics.proxy.avg_response_time < 200 ? 'healthy' : 'warning'}
                />
                
                <MetricCard
                  title="Error Rate"
                  value={state.systemMetrics.proxy.error_rate.toFixed(2)}
                  unit="%"
                  icon={<AlertTriangle className="w-4 h-4" />}
                  status={state.systemMetrics.proxy.error_rate < 5 ? 'healthy' : 'error'}
                />
                
                <MetricCard
                  title="Active Connections"
                  value={state.systemMetrics.proxy.active_connections}
                  icon={<Users className="w-4 h-4" />}
                />
              </div>
            )}

            {/* Recent Orders */}
            <OrdersTable 
              orders={state.orders} 
              title="Recent Orders"
              maxRows={10}
              realTime={true}
            />
          </div>
        )}

        {activeTab === 'orders' && (
          <div className="space-y-6">
            <div className="flex items-center justify-between">
              <h2 className="text-xl font-semibold">Order Management</h2>
              <div className="flex items-center gap-2">
                <button
                  onClick={handleCreateTestOrder}
                  className="flex items-center gap-2 px-4 py-2 bg-primary text-primary-foreground rounded-md hover:bg-primary/90 transition-colors"
                >
                  <TrendingUp className="w-4 h-4" />
                  Create Test Order
                </button>
              </div>
            </div>
            
            <OrdersTable 
              orders={state.orders} 
              title="All Orders"
              maxRows={50}
              showActions={true}
              realTime={true}
            />
          </div>
        )}

        {activeTab === 'performance' && (
          <div className="space-y-6">
            <div className="flex items-center justify-between">
              <h2 className="text-xl font-semibold">Performance Metrics</h2>
              <div className="flex items-center gap-2">
                <select
                  value={timeRange}
                  onChange={(e) => setTimeRange(e.target.value as any)}
                  className="px-3 py-2 border rounded-md focus:outline-none focus:ring-2 focus:ring-primary"
                >
                  <option value="5m">Last 5 minutes</option>
                  <option value="15m">Last 15 minutes</option>
                  <option value="1h">Last hour</option>
                  <option value="24h">Last 24 hours</option>
                </select>
                
                <label className="flex items-center gap-2">
                  <input
                    type="checkbox"
                    checked={autoRefresh}
                    onChange={(e) => setAutoRefresh(e.target.checked)}
                    className="rounded"
                  />
                  <span className="text-sm">Auto refresh</span>
                </label>
              </div>
            </div>
            
            <PerformanceCharts 
              metrics={metricsHistory} 
              timeRange={timeRange}
            />
          </div>
        )}

        {activeTab === 'load-test' && (
          <LoadTestPanel
            onStartTest={handleStartLoadTest}
            onStopTest={handleStopLoadTest}
            activeTests={activeLoadTests}
            completedTests={completedLoadTests}
          />
        )}

        {activeTab === 'comparison' && (
          <div className="space-y-6">
            <div className="flex items-center justify-between">
              <h2 className="text-xl font-semibold">Data Synchronization</h2>
              <button
                onClick={handleRunComparison}
                className="flex items-center gap-2 px-4 py-2 bg-primary text-primary-foreground rounded-md hover:bg-primary/90 transition-colors"
              >
                <GitCompare className="w-4 h-4" />
                Run Comparison
              </button>
            </div>

            {state.comparison ? (
              <div className="space-y-6">
                {/* Comparison Summary */}
                <div className="grid grid-cols-1 md:grid-cols-2 lg:grid-cols-4 gap-4">
                  <MetricCard
                    title="Sync Percentage"
                    value={state.comparison.analysis.sync_percentage.toFixed(1)}
                    unit="%"
                    icon={<GitCompare className="w-4 h-4" />}
                    status={state.comparison.analysis.sync_percentage >= 95 ? 'healthy' : 'warning'}
                  />
                  
                  <MetricCard
                    title="Order Service Orders"
                    value={state.comparison.analysis.total_order_service}
                    icon={<Database className="w-4 h-4" />}
                  />
                  
                  <MetricCard
                    title="SAP Orders"
                    value={state.comparison.analysis.total_sap}
                    icon={<Server className="w-4 h-4" />}
                  />
                  
                  <MetricCard
                    title="Critical Issues"
                    value={state.comparison.statistics.critical_issues}
                    icon={<AlertTriangle className="w-4 h-4" />}
                    status={state.comparison.statistics.critical_issues === 0 ? 'healthy' : 'error'}
                  />
                </div>

                {/* Recommendations */}
                {state.comparison.recommendations.length > 0 && (
                  <div className="bg-card border rounded-lg p-6">
                    <h3 className="text-lg font-semibold mb-4">Recommendations</h3>
                    <ul className="space-y-2">
                      {state.comparison.recommendations.map((rec, index) => (
                        <li key={index} className="flex items-start gap-2">
                          <span className="text-muted-foreground">•</span>
                          <span>{rec}</span>
                        </li>
                      ))}
                    </ul>
                  </div>
                )}

                {/* Inconsistencies */}
                {state.comparison.inconsistencies.length > 0 && (
                  <div className="bg-card border rounded-lg p-6">
                    <h3 className="text-lg font-semibold mb-4">Data Inconsistencies</h3>
                    <div className="space-y-3">
                      {state.comparison.inconsistencies.map((issue, index) => (
                        <div 
                          key={index} 
                          className={`p-3 rounded border-l-4 ${
                            issue.severity === 'critical' ? 'border-red-500 bg-red-50' :
                            issue.severity === 'warning' ? 'border-yellow-500 bg-yellow-50' :
                            'border-blue-500 bg-blue-50'
                          }`}
                        >
                          <div className="flex items-center justify-between mb-2">
                            <span className="font-medium">{issue.order_id}</span>
                            <span className={`text-xs px-2 py-1 rounded ${
                              issue.severity === 'critical' ? 'bg-red-100 text-red-800' :
                              issue.severity === 'warning' ? 'bg-yellow-100 text-yellow-800' :
                              'bg-blue-100 text-blue-800'
                            }`}>
                              {issue.severity}
                            </span>
                          </div>
                          <p className="text-sm text-muted-foreground mb-1">{issue.description}</p>
                          <p className="text-xs text-muted-foreground">{issue.suggestion}</p>
                        </div>
                      ))}
                    </div>
                  </div>
                )}
              </div>
            ) : (
              <div className="text-center py-8 text-muted-foreground">
                <GitCompare className="w-12 h-12 mx-auto mb-4" />
                <p>No comparison data available. Click "Run Comparison" to analyze data synchronization.</p>
              </div>
            )}
          </div>
        )}
      </main>

      {/* Footer */}
      <footer className="border-t bg-card p-4 text-center text-sm text-muted-foreground">
        Last updated: {new Date(state.lastUpdate).toLocaleString()} | 
        WebSocket: {state.websocketStatus} | 
        Auto-refresh: {autoRefresh ? 'On' : 'Off'}
      </footer>
    </div>
  );
}