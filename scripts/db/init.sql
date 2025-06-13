-- Database initialization script for Order Service
-- This script runs automatically when PostgreSQL container starts
-- Database: orderservice
-- User: orderservice

-- Ensure database exists (this is mostly redundant since POSTGRES_DB creates it,
-- but included for completeness)
SELECT 'CREATE DATABASE orderservice' WHERE NOT EXISTS (SELECT FROM pg_database WHERE datname = 'orderservice')\gexec

-- Connect to the orderservice database
\c orderservice;

-- Create orders table
CREATE TABLE IF NOT EXISTS orders (
    id VARCHAR(255) PRIMARY KEY,
    customer_id VARCHAR(255) NOT NULL,
    total_amount DECIMAL(10,2) NOT NULL,
    delivery_date TIMESTAMP NOT NULL,
    status VARCHAR(50) NOT NULL,
    created_at TIMESTAMP NOT NULL
);

-- Create order_items table
CREATE TABLE IF NOT EXISTS order_items (
    id SERIAL PRIMARY KEY,
    order_id VARCHAR(255) NOT NULL REFERENCES orders(id) ON DELETE CASCADE,
    product_id VARCHAR(255) NOT NULL,
    quantity INTEGER NOT NULL,
    unit_price DECIMAL(10,2) NOT NULL,
    specifications JSONB,
    created_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP
);

-- Create indexes for performance
CREATE INDEX IF NOT EXISTS idx_orders_customer_id ON orders(customer_id);
CREATE INDEX IF NOT EXISTS idx_orders_created_at ON orders(created_at);
CREATE INDEX IF NOT EXISTS idx_order_items_order_id ON order_items(order_id);
CREATE INDEX IF NOT EXISTS idx_order_items_product_id ON order_items(product_id);

-- Create view for order summaries
CREATE OR REPLACE VIEW order_summaries AS
SELECT 
    o.id,
    o.customer_id,
    o.total_amount,
    o.delivery_date,
    o.status,
    o.created_at,
    COUNT(oi.id) as item_count,
    SUM(oi.quantity) as total_quantity
FROM orders o
LEFT JOIN order_items oi ON o.id = oi.order_id
GROUP BY o.id, o.customer_id, o.total_amount, o.delivery_date, o.status, o.created_at;