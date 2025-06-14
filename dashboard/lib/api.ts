'use client';

import axios from 'axios';
import { Order, ServiceHealth, SystemMetrics, LoadTestConfig, LoadTestResult, ComparisonResult } from '@/types';

const api = axios.create({
  timeout: 10000,
});

// Service URLs - can be overridden by environment variables
const PROXY_URL = process.env.NEXT_PUBLIC_PROXY_URL || 'http://localhost:8080';
const ORDER_SERVICE_URL = process.env.NEXT_PUBLIC_ORDER_SERVICE_URL || 'http://localhost:8081';
const SAP_URL = process.env.NEXT_PUBLIC_SAP_URL || 'http://localhost:8082';

export class ApiClient {
  // Health checks
  static async checkServiceHealth(service: 'proxy' | 'order_service' | 'sap_mock'): Promise<ServiceHealth> {
    const urls = {
      proxy: `${PROXY_URL}/health`,
      order_service: `${ORDER_SERVICE_URL}/health`,
      sap_mock: `${SAP_URL}/health`,
    };

    const startTime = Date.now();
    
    try {
      const response = await api.get(urls[service]);
      const responseTime = Date.now() - startTime;
      
      return {
        status: 'healthy',
        service,
        response_time: responseTime,
        last_check: new Date().toISOString(),
      };
    } catch (error) {
      const responseTime = Date.now() - startTime;
      
      return {
        status: 'unhealthy',
        service,
        error: error instanceof Error ? error.message : 'Unknown error',
        response_time: responseTime,
        last_check: new Date().toISOString(),
      };
    }
  }

  static async checkAllServicesHealth(): Promise<Record<string, ServiceHealth>> {
    try {
      // Use the consolidated health endpoint from the proxy
      const response = await api.get(`${PROXY_URL}/api/health/all`);
      const healthData = response.data;
      
      // Transform the response to match our ServiceHealth interface
      const results: Record<string, ServiceHealth> = {};
      
      for (const [service, data] of Object.entries(healthData)) {
        results[service] = data as ServiceHealth;
      }
      
      return results;
    } catch (error) {
      // If the consolidated endpoint fails, fall back to individual checks
      console.error('Failed to fetch consolidated health status:', error);
      
      const services = ['proxy', 'order_service', 'sap_mock'] as const;
      
      const healthChecks = await Promise.allSettled(
        services.map(service => this.checkServiceHealth(service))
      );

      const results: Record<string, ServiceHealth> = {};
      
      healthChecks.forEach((result, index) => {
        const service = services[index];
        if (result.status === 'fulfilled') {
          results[service] = result.value;
        } else {
          results[service] = {
            status: 'unhealthy',
            service,
            error: 'Health check failed',
            last_check: new Date().toISOString(),
          };
        }
      });

      return results;
    }
  }

  // Orders
  static async getOrders(source: 'proxy' | 'order_service' | 'sap_mock' = 'proxy'): Promise<Order[]> {
    const urls = {
      proxy: `${PROXY_URL}/orders`,
      order_service: `${ORDER_SERVICE_URL}/orders`,
      sap_mock: `${SAP_URL}/orders`,
    };

    try {
      console.log(`Fetching orders from ${source} at ${urls[source]}`);
      const response = await api.get(urls[source]);
      console.log(`Orders response from ${source}:`, response.data);
      
      // All services now return orders in the same format
      const orders = response.data.orders || [];
      console.log(`Extracted ${orders.length} orders from ${source}`);
      
      return orders;
    } catch (error) {
      console.error(`Failed to fetch orders from ${source}:`, error);
      return [];
    }
  }

  static async createOrder(order: Partial<Order>, target: 'proxy' | 'order_service' | 'sap_mock' = 'proxy'): Promise<Order> {
    const urls = {
      proxy: `${PROXY_URL}/orders`,
      order_service: `${ORDER_SERVICE_URL}/orders`,
      sap_mock: `${SAP_URL}/orders`,
    };

    // Generate order if not complete
    const completeOrder = {
      id: order.id || `order-${Date.now()}-${Math.random().toString(36).substr(2, 9)}`,
      customer_id: order.customer_id || `customer-${Math.random().toString(36).substr(2, 9)}`,
      items: order.items || [
        {
          product_id: `product-${Math.random().toString(36).substr(2, 9)}`,
          quantity: Math.floor(Math.random() * 5) + 1,
          unit_price: Math.round((Math.random() * 100 + 10) * 100) / 100,
          specifications: {
            color: ['red', 'blue', 'green', 'yellow'][Math.floor(Math.random() * 4)],
            finish: ['matte', 'glossy', 'textured'][Math.floor(Math.random() * 3)],
          },
        },
      ],
      total_amount: order.total_amount || Math.round((Math.random() * 500 + 50) * 100) / 100,
      delivery_date: order.delivery_date || new Date(Date.now() + Math.random() * 30 * 24 * 60 * 60 * 1000).toISOString(),
      status: order.status || 'pending',
      created_at: new Date().toISOString(),
    };

    const response = await api.post(urls[target], completeOrder);
    return response.data.order || response.data;
  }

  // Metrics
  static async getSystemMetrics(): Promise<SystemMetrics | null> {
    try {
      // This would typically come from a metrics endpoint
      // For now, we'll simulate metrics data
      const [proxyHealth, orderServiceHealth, sapHealth] = await Promise.allSettled([
        this.checkServiceHealth('proxy'),
        this.checkServiceHealth('order_service'),
        this.checkServiceHealth('sap_mock'),
      ]);

      return {
        timestamp: new Date().toISOString(),
        proxy: {
          requests_per_second: Math.round(Math.random() * 100 + 50),
          avg_response_time: Math.round(Math.random() * 200 + 50),
          error_rate: Math.round(Math.random() * 5 * 100) / 100,
          active_connections: Math.round(Math.random() * 50 + 10),
        },
        order_service: {
          orders_created: Math.round(Math.random() * 1000 + 500),
          avg_processing_time: Math.round(Math.random() * 100 + 20),
          database_connections: Math.round(Math.random() * 20 + 5),
          kafka_events_published: Math.round(Math.random() * 500 + 100),
        },
        sap_mock: {
          orders_processed: Math.round(Math.random() * 800 + 400),
          avg_response_time: Math.round(Math.random() * 500 + 100),
          events_consumed: Math.round(Math.random() * 400 + 80),
          failure_rate: Math.round(Math.random() * 3 * 100) / 100,
        },
        kafka: {
          messages_per_second: Math.round(Math.random() * 200 + 100),
          consumer_lag: Math.round(Math.random() * 10),
          partition_count: 3,
          broker_status: Math.random() > 0.1 ? 'up' : 'down',
        },
      };
    } catch (error) {
      console.error('Failed to fetch system metrics:', error);
      return null;
    }
  }

  // Load testing
  static async startLoadTest(config: LoadTestConfig): Promise<LoadTestResult> {
    try {
      // This would typically call a load testing service
      // For now, we'll simulate a load test
      const loadTest: LoadTestResult = {
        id: `test-${Date.now()}`,
        config,
        status: 'running',
        start_time: new Date().toISOString(),
        metrics: {
          total_requests: 0,
          successful_requests: 0,
          failed_requests: 0,
          avg_response_time: 0,
          min_response_time: 0,
          max_response_time: 0,
          p95_response_time: 0,
          p99_response_time: 0,
          requests_per_second: 0,
          error_rate: 0,
          throughput: 0,
        },
      };

      // Simulate load test progression
      setTimeout(() => {
        loadTest.status = 'completed';
        loadTest.end_time = new Date().toISOString();
        loadTest.metrics = {
          total_requests: config.requests_per_second * config.duration,
          successful_requests: Math.round(config.requests_per_second * config.duration * 0.95),
          failed_requests: Math.round(config.requests_per_second * config.duration * 0.05),
          avg_response_time: Math.round(Math.random() * 200 + 50),
          min_response_time: Math.round(Math.random() * 50 + 10),
          max_response_time: Math.round(Math.random() * 500 + 200),
          p95_response_time: Math.round(Math.random() * 300 + 100),
          p99_response_time: Math.round(Math.random() * 400 + 150),
          requests_per_second: config.requests_per_second,
          error_rate: Math.round(Math.random() * 5 * 100) / 100,
          throughput: Math.round(config.requests_per_second * 0.95),
        };
      }, config.duration * 1000);

      return loadTest;
    } catch (error) {
      throw new Error(`Failed to start load test: ${error}`);
    }
  }

  // Data comparison
  static async compareData(): Promise<ComparisonResult> {
    try {
      // This would typically call the data-tools CLI
      // For now, we'll simulate comparison data
      const [osOrders, sapOrders] = await Promise.all([
        this.getOrders('order_service'),
        this.getOrders('sap_mock'),
      ]);

      const perfectMatches = Math.min(osOrders.length, sapOrders.length) - Math.floor(Math.random() * 3);
      const partialMatches = Math.floor(Math.random() * 3);
      const missingInSap = osOrders.slice(sapOrders.length).map(o => o.id);
      const missingInOrderService = sapOrders.slice(osOrders.length).map(o => o.id);

      const totalComparable = Math.max(osOrders.length, sapOrders.length);
      const syncPercentage = totalComparable > 0 ? (perfectMatches / totalComparable) * 100 : 100;

      return {
        order_service_data: osOrders,
        sap_data: sapOrders,
        analysis: {
          total_order_service: osOrders.length,
          total_sap: sapOrders.length,
          perfect_matches: perfectMatches,
          partial_matches: partialMatches,
          missing_in_sap: missingInSap,
          missing_in_order_service: missingInOrderService,
          sync_percentage: Math.round(syncPercentage * 100) / 100,
          overall_status: syncPercentage >= 95 ? 'excellent' : syncPercentage >= 85 ? 'good' : 'poor',
        },
        inconsistencies: [],
        statistics: {
          data_consistency_score: Math.round(syncPercentage * 100) / 100,
          critical_issues: Math.floor(Math.random() * 2),
          warning_issues: Math.floor(Math.random() * 5),
          info_issues: Math.floor(Math.random() * 3),
        },
        recommendations: [
          syncPercentage < 95 ? '🔄 Run data synchronization to improve consistency' : '✅ Data consistency is excellent',
          missingInSap.length > 0 ? `📤 Migrate ${missingInSap.length} missing orders to SAP` : '',
          missingInOrderService.length > 0 ? `📥 Import ${missingInOrderService.length} historical SAP orders` : '',
        ].filter(Boolean),
        timestamp: new Date().toISOString(),
      };
    } catch (error) {
      throw new Error(`Failed to compare data: ${error}`);
    }
  }

  // Utility function to generate sample orders
  static generateSampleOrder(customerId?: string): Partial<Order> {
    return {
      customer_id: customerId || `customer-${Math.random().toString(36).substr(2, 9)}`,
      items: [
        {
          product_id: `product-${Math.random().toString(36).substr(2, 9)}`,
          quantity: Math.floor(Math.random() * 5) + 1,
          unit_price: Math.round((Math.random() * 100 + 10) * 100) / 100,
          specifications: {
            color: ['red', 'blue', 'green', 'yellow', 'purple'][Math.floor(Math.random() * 5)],
            finish: ['matte', 'glossy', 'textured', 'brushed'][Math.floor(Math.random() * 4)],
            size: ['small', 'medium', 'large', 'xl'][Math.floor(Math.random() * 4)],
          },
        },
      ],
      status: ['pending', 'confirmed', 'processing', 'shipped'][Math.floor(Math.random() * 4)],
    };
  }
}