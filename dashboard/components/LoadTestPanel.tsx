'use client';

import { useState } from 'react';
import { LoadTestConfig, LoadTestResult } from '@/types';
import { Play, Square, RefreshCw, TrendingUp, Clock, Users, Target } from 'lucide-react';
import { MetricCard } from './MetricCard';

interface LoadTestPanelProps {
  onStartTest: (config: LoadTestConfig) => Promise<LoadTestResult>;
  onStopTest: (testId: string) => void;
  activeTests: LoadTestResult[];
  completedTests: LoadTestResult[];
}

export function LoadTestPanel({ onStartTest, onStopTest, activeTests, completedTests }: LoadTestPanelProps) {
  const [config, setConfig] = useState<LoadTestConfig>({
    duration: 60,
    requests_per_second: 10,
    concurrent_users: 5,
    target_endpoint: 'proxy',
    scenario: 'create_orders',
  });
  const [isStarting, setIsStarting] = useState(false);

  const handleStartTest = async () => {
    setIsStarting(true);
    try {
      await onStartTest(config);
    } catch (error) {
      console.error('Failed to start load test:', error);
    } finally {
      setIsStarting(false);
    }
  };

  const getLatestResults = () => {
    if (completedTests.length === 0) return null;
    return completedTests[completedTests.length - 1];
  };

  const latestResults = getLatestResults();

  return (
    <div className="space-y-6">
      {/* Configuration Panel */}
      <div className="bg-card border rounded-lg p-6">
        <h3 className="text-lg font-semibold mb-4 flex items-center gap-2">
          <Target className="w-5 h-5" />
          Load Test Configuration
        </h3>
        
        <div className="grid grid-cols-1 md:grid-cols-2 gap-4">
          <div>
            <label className="block text-sm font-medium mb-2">Duration (seconds)</label>
            <input
              type="number"
              value={config.duration}
              onChange={(e) => setConfig({ ...config, duration: parseInt(e.target.value) || 60 })}
              className="w-full px-3 py-2 border rounded-md focus:outline-none focus:ring-2 focus:ring-primary"
              min="1"
              max="3600"
            />
          </div>
          
          <div>
            <label className="block text-sm font-medium mb-2">Requests per Second</label>
            <input
              type="number"
              value={config.requests_per_second}
              onChange={(e) => setConfig({ ...config, requests_per_second: parseInt(e.target.value) || 10 })}
              className="w-full px-3 py-2 border rounded-md focus:outline-none focus:ring-2 focus:ring-primary"
              min="1"
              max="1000"
            />
          </div>
          
          <div>
            <label className="block text-sm font-medium mb-2">Concurrent Users</label>
            <input
              type="number"
              value={config.concurrent_users}
              onChange={(e) => setConfig({ ...config, concurrent_users: parseInt(e.target.value) || 5 })}
              className="w-full px-3 py-2 border rounded-md focus:outline-none focus:ring-2 focus:ring-primary"
              min="1"
              max="100"
            />
          </div>
          
          <div>
            <label className="block text-sm font-medium mb-2">Target Endpoint</label>
            <select
              value={config.target_endpoint}
              onChange={(e) => setConfig({ ...config, target_endpoint: e.target.value as any })}
              className="w-full px-3 py-2 border rounded-md focus:outline-none focus:ring-2 focus:ring-primary"
            >
              <option value="proxy">Proxy Service</option>
              <option value="order_service">Order Service (Direct)</option>
              <option value="sap_mock">SAP Mock (Direct)</option>
            </select>
          </div>
          
          <div className="md:col-span-2">
            <label className="block text-sm font-medium mb-2">Test Scenario</label>
            <select
              value={config.scenario}
              onChange={(e) => setConfig({ ...config, scenario: e.target.value as any })}
              className="w-full px-3 py-2 border rounded-md focus:outline-none focus:ring-2 focus:ring-primary"
            >
              <option value="create_orders">Create Orders</option>
              <option value="get_orders">Get Orders</option>
              <option value="mixed">Mixed Operations</option>
            </select>
          </div>
        </div>
        
        <div className="flex items-center gap-3 mt-6">
          <button
            onClick={handleStartTest}
            disabled={isStarting || activeTests.length > 0}
            className="flex items-center gap-2 px-4 py-2 bg-primary text-primary-foreground rounded-md hover:bg-primary/90 disabled:opacity-50 disabled:cursor-not-allowed transition-colors"
          >
            {isStarting ? (
              <RefreshCw className="w-4 h-4 animate-spin" />
            ) : (
              <Play className="w-4 h-4" />
            )}
            {isStarting ? 'Starting...' : 'Start Load Test'}
          </button>
          
          {activeTests.length > 0 && (
            <button
              onClick={() => activeTests.forEach(test => onStopTest(test.id))}
              className="flex items-center gap-2 px-4 py-2 bg-destructive text-destructive-foreground rounded-md hover:bg-destructive/90 transition-colors"
            >
              <Square className="w-4 h-4" />
              Stop Test
            </button>
          )}
          
          <div className="ml-auto text-sm text-muted-foreground">
            Estimated total requests: {config.duration * config.requests_per_second}
          </div>
        </div>
      </div>

      {/* Active Tests */}
      {activeTests.length > 0 && (
        <div className="bg-card border rounded-lg p-6">
          <h3 className="text-lg font-semibold mb-4">Active Tests</h3>
          <div className="space-y-4">
            {activeTests.map(test => (
              <div key={test.id} className="border rounded-lg p-4 bg-blue-50">
                <div className="flex items-center justify-between mb-2">
                  <div className="flex items-center gap-2">
                    <div className="w-2 h-2 bg-blue-500 rounded-full animate-pulse" />
                    <span className="font-medium">Test {test.id}</span>
                    <span className="text-sm text-muted-foreground">
                      ({test.config.target_endpoint} - {test.config.scenario})
                    </span>
                  </div>
                  <button
                    onClick={() => onStopTest(test.id)}
                    className="text-destructive hover:text-destructive/80 transition-colors"
                  >
                    <Square className="w-4 h-4" />
                  </button>
                </div>
                
                <div className="grid grid-cols-4 gap-4 text-sm">
                  <div>
                    <span className="text-muted-foreground">Duration:</span>
                    <div className="font-medium">{test.config.duration}s</div>
                  </div>
                  <div>
                    <span className="text-muted-foreground">RPS:</span>
                    <div className="font-medium">{test.config.requests_per_second}</div>
                  </div>
                  <div>
                    <span className="text-muted-foreground">Users:</span>
                    <div className="font-medium">{test.config.concurrent_users}</div>
                  </div>
                  <div>
                    <span className="text-muted-foreground">Started:</span>
                    <div className="font-medium">
                      {new Date(test.start_time).toLocaleTimeString()}
                    </div>
                  </div>
                </div>
              </div>
            ))}
          </div>
        </div>
      )}

      {/* Latest Results */}
      {latestResults && (
        <div className="bg-card border rounded-lg p-6">
          <h3 className="text-lg font-semibold mb-4">Latest Test Results</h3>
          
          <div className="grid grid-cols-1 md:grid-cols-2 lg:grid-cols-4 gap-4 mb-6">
            <MetricCard
              title="Total Requests"
              value={latestResults.metrics.total_requests}
              icon={<TrendingUp className="w-4 h-4" />}
            />
            
            <MetricCard
              title="Success Rate"
              value={((latestResults.metrics.successful_requests / latestResults.metrics.total_requests) * 100).toFixed(1)}
              unit="%"
              icon={<Target className="w-4 h-4" />}
              status={latestResults.metrics.error_rate < 5 ? 'healthy' : 'warning'}
            />
            
            <MetricCard
              title="Avg Response Time"
              value={latestResults.metrics.avg_response_time}
              unit="ms"
              icon={<Clock className="w-4 h-4" />}
              status={latestResults.metrics.avg_response_time < 200 ? 'healthy' : 'warning'}
            />
            
            <MetricCard
              title="Throughput"
              value={latestResults.metrics.throughput.toFixed(1)}
              unit="req/s"
              icon={<Users className="w-4 h-4" />}
            />
          </div>
          
          <div className="grid grid-cols-1 lg:grid-cols-2 gap-6">
            <div>
              <h4 className="font-medium mb-3">Response Time Percentiles</h4>
              <div className="space-y-2">
                <div className="flex justify-between">
                  <span className="text-muted-foreground">Average:</span>
                  <span>{latestResults.metrics.avg_response_time}ms</span>
                </div>
                <div className="flex justify-between">
                  <span className="text-muted-foreground">Min:</span>
                  <span>{latestResults.metrics.min_response_time}ms</span>
                </div>
                <div className="flex justify-between">
                  <span className="text-muted-foreground">Max:</span>
                  <span>{latestResults.metrics.max_response_time}ms</span>
                </div>
                <div className="flex justify-between">
                  <span className="text-muted-foreground">P95:</span>
                  <span>{latestResults.metrics.p95_response_time}ms</span>
                </div>
                <div className="flex justify-between">
                  <span className="text-muted-foreground">P99:</span>
                  <span>{latestResults.metrics.p99_response_time}ms</span>
                </div>
              </div>
            </div>
            
            <div>
              <h4 className="font-medium mb-3">Test Summary</h4>
              <div className="space-y-2">
                <div className="flex justify-between">
                  <span className="text-muted-foreground">Test Duration:</span>
                  <span>
                    {latestResults.end_time && 
                      Math.round(
                        (new Date(latestResults.end_time).getTime() - 
                         new Date(latestResults.start_time).getTime()) / 1000
                      )
                    }s
                  </span>
                </div>
                <div className="flex justify-between">
                  <span className="text-muted-foreground">Target:</span>
                  <span>{latestResults.config.target_endpoint}</span>
                </div>
                <div className="flex justify-between">
                  <span className="text-muted-foreground">Scenario:</span>
                  <span>{latestResults.config.scenario}</span>
                </div>
                <div className="flex justify-between">
                  <span className="text-muted-foreground">Error Rate:</span>
                  <span className={latestResults.metrics.error_rate > 5 ? 'text-red-600' : 'text-green-600'}>
                    {latestResults.metrics.error_rate.toFixed(2)}%
                  </span>
                </div>
              </div>
            </div>
          </div>
          
          {latestResults.errors && latestResults.errors.length > 0 && (
            <div className="mt-6">
              <h4 className="font-medium mb-3 text-red-600">Errors</h4>
              <div className="space-y-2">
                {latestResults.errors.map((error, index) => (
                  <div key={index} className="text-sm p-2 bg-red-50 border border-red-200 rounded">
                    <div className="flex justify-between">
                      <span className="font-medium">{error.error}</span>
                      <span className="text-muted-foreground">Count: {error.count}</span>
                    </div>
                    <div className="text-xs text-muted-foreground mt-1">
                      {new Date(error.timestamp).toLocaleString()}
                    </div>
                  </div>
                ))}
              </div>
            </div>
          )}
        </div>
      )}
    </div>
  );
}