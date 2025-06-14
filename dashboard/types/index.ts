export interface Order {
  id: string;
  customer_id: string;
  items: OrderItem[];
  total_amount: number;
  delivery_date: string;
  status: string;
  created_at: string;
}

export interface OrderItem {
  product_id: string;
  quantity: number;
  unit_price: number;
  specifications: Record<string, string>;
}

export interface OrderResponse {
  success: boolean;
  message: string;
  order?: Order;
}

export interface ServiceHealth {
  status: 'healthy' | 'unhealthy';
  service: string;
  error?: string;
  response_time?: number;
  last_check?: string;
}

export interface SystemMetrics {
  timestamp: string;
  proxy: {
    requests_per_second: number;
    avg_response_time: number;
    error_rate: number;
    active_connections: number;
  };
  order_service: {
    orders_created: number;
    avg_processing_time: number;
    database_connections: number;
    kafka_events_published: number;
  };
  sap_mock: {
    orders_processed: number;
    avg_response_time: number;
    events_consumed: number;
    failure_rate: number;
  };
  kafka: {
    messages_per_second: number;
    consumer_lag: number;
    partition_count: number;
    broker_status: 'up' | 'down';
  };
}

export interface LoadTestConfig {
  duration: number; // seconds
  requests_per_second: number;
  concurrent_users: number;
  target_endpoint: 'proxy' | 'order_service' | 'sap_mock';
  scenario: 'create_orders' | 'get_orders' | 'mixed';
}

export interface LoadTestResult {
  id: string;
  config: LoadTestConfig;
  status: 'running' | 'completed' | 'failed';
  start_time: string;
  end_time?: string;
  metrics: {
    total_requests: number;
    successful_requests: number;
    failed_requests: number;
    avg_response_time: number;
    min_response_time: number;
    max_response_time: number;
    p95_response_time: number;
    p99_response_time: number;
    requests_per_second: number;
    error_rate: number;
    throughput: number;
  };
  errors?: Array<{
    timestamp: string;
    error: string;
    count: number;
  }>;
}

export interface WebSocketMessage {
  type: 'order_created' | 'order_updated' | 'metrics_update' | 'health_update' | 'load_test_update';
  data: any;
  timestamp: string;
  source: 'proxy' | 'order_service' | 'sap_mock' | 'kafka';
}

export interface OrderEvent {
  type: 'order_created' | 'order_updated';
  order: Order;
  source: 'proxy' | 'order_service' | 'sap_mock';
  timestamp: string;
  processing_time?: number;
}

export interface ComparisonResult {
  order_service_data: Order[];
  sap_data: Order[];
  analysis: {
    total_order_service: number;
    total_sap: number;
    perfect_matches: number;
    partial_matches: number;
    missing_in_sap: string[];
    missing_in_order_service: string[];
    sync_percentage: number;
    overall_status: string;
  };
  inconsistencies: Array<{
    order_id: string;
    type: string;
    severity: 'critical' | 'warning' | 'info';
    field?: string;
    description: string;
    impact: string;
    suggestion: string;
  }>;
  statistics: {
    data_consistency_score: number;
    critical_issues: number;
    warning_issues: number;
    info_issues: number;
  };
  recommendations: string[];
  timestamp: string;
}

export interface DashboardState {
  orders: Order[];
  recentOrders: OrderEvent[];
  systemMetrics: SystemMetrics | null;
  serviceHealth: Record<string, ServiceHealth>;
  loadTests: LoadTestResult[];
  comparison: ComparisonResult | null;
  websocketStatus: 'connected' | 'disconnected' | 'connecting';
  lastUpdate: string;
}