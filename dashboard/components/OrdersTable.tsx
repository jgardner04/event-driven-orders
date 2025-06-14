'use client';

import { useState, useMemo } from 'react';
import { Order } from '@/types';
import { formatDistanceToNow } from 'date-fns';
import { Search, Filter, Download, ExternalLink } from 'lucide-react';

interface OrdersTableProps {
  orders: Order[];
  title?: string;
  maxRows?: number;
  showActions?: boolean;
  realTime?: boolean;
}

export function OrdersTable({ 
  orders, 
  title = "Recent Orders",
  maxRows = 10,
  showActions = true,
  realTime = false
}: OrdersTableProps) {
  console.log(`OrdersTable rendering with ${orders.length} orders for "${title}"`);
  
  const [searchTerm, setSearchTerm] = useState('');
  const [statusFilter, setStatusFilter] = useState<string>('all');
  const [sortField, setSortField] = useState<keyof Order>('created_at');
  const [sortDirection, setSortDirection] = useState<'asc' | 'desc'>('desc');

  const filteredOrders = useMemo(() => {
    let filtered = orders.filter(order => {
      const matchesSearch = 
        order.id.toLowerCase().includes(searchTerm.toLowerCase()) ||
        order.customer_id.toLowerCase().includes(searchTerm.toLowerCase()) ||
        order.status.toLowerCase().includes(searchTerm.toLowerCase());
      
      const matchesStatus = statusFilter === 'all' || order.status === statusFilter;
      
      return matchesSearch && matchesStatus;
    });

    // Sort orders
    filtered.sort((a, b) => {
      const aValue = a[sortField];
      const bValue = b[sortField];
      
      if (sortDirection === 'asc') {
        return aValue < bValue ? -1 : aValue > bValue ? 1 : 0;
      } else {
        return aValue > bValue ? -1 : aValue < bValue ? 1 : 0;
      }
    });

    return filtered.slice(0, maxRows);
  }, [orders, searchTerm, statusFilter, sortField, sortDirection, maxRows]);

  const uniqueStatuses = useMemo(() => {
    const statuses = new Set(orders.map(order => order.status));
    return Array.from(statuses);
  }, [orders]);

  const handleSort = (field: keyof Order) => {
    if (sortField === field) {
      setSortDirection(sortDirection === 'asc' ? 'desc' : 'asc');
    } else {
      setSortField(field);
      setSortDirection('desc');
    }
  };

  const getStatusColor = (status: string) => {
    switch (status.toLowerCase()) {
      case 'pending':
        return 'bg-yellow-100 text-yellow-800 border-yellow-200';
      case 'confirmed':
        return 'bg-blue-100 text-blue-800 border-blue-200';
      case 'processing':
        return 'bg-purple-100 text-purple-800 border-purple-200';
      case 'shipped':
        return 'bg-green-100 text-green-800 border-green-200';
      case 'delivered':
        return 'bg-green-100 text-green-800 border-green-200';
      case 'cancelled':
        return 'bg-red-100 text-red-800 border-red-200';
      default:
        return 'bg-gray-100 text-gray-800 border-gray-200';
    }
  };

  const exportOrders = () => {
    const csv = [
      ['Order ID', 'Customer ID', 'Status', 'Total Amount', 'Items', 'Created At'].join(','),
      ...filteredOrders.map(order => [
        order.id,
        order.customer_id,
        order.status,
        order.total_amount,
        order.items.length,
        new Date(order.created_at).toISOString(),
      ].join(','))
    ].join('\n');

    const blob = new Blob([csv], { type: 'text/csv' });
    const url = URL.createObjectURL(blob);
    const a = document.createElement('a');
    a.href = url;
    a.download = `orders-${new Date().toISOString().split('T')[0]}.csv`;
    a.click();
    URL.revokeObjectURL(url);
  };

  return (
    <div className="bg-card border rounded-lg shadow-sm">
      <div className="p-4 border-b">
        <div className="flex items-center justify-between mb-4">
          <div className="flex items-center gap-2">
            <h3 className="text-lg font-semibold">{title}</h3>
            {realTime && (
              <div className="flex items-center gap-1 text-green-600">
                <div className="w-2 h-2 bg-green-500 rounded-full animate-pulse" />
                <span className="text-xs">Live</span>
              </div>
            )}
          </div>
          
          {showActions && (
            <div className="flex items-center gap-2">
              <button
                onClick={exportOrders}
                className="flex items-center gap-1 px-3 py-1 text-sm border rounded hover:bg-secondary"
              >
                <Download className="w-4 h-4" />
                Export
              </button>
            </div>
          )}
        </div>

        <div className="flex items-center gap-4">
          <div className="relative flex-1 max-w-sm">
            <Search className="absolute left-3 top-1/2 transform -translate-y-1/2 w-4 h-4 text-muted-foreground" />
            <input
              type="text"
              placeholder="Search orders..."
              value={searchTerm}
              onChange={(e) => setSearchTerm(e.target.value)}
              className="w-full pl-10 pr-4 py-2 border rounded-md text-sm focus:outline-none focus:ring-2 focus:ring-primary"
            />
          </div>
          
          <div className="flex items-center gap-2">
            <Filter className="w-4 h-4 text-muted-foreground" />
            <select
              value={statusFilter}
              onChange={(e) => setStatusFilter(e.target.value)}
              className="px-3 py-2 border rounded-md text-sm focus:outline-none focus:ring-2 focus:ring-primary"
            >
              <option value="all">All Status</option>
              {uniqueStatuses.map(status => (
                <option key={status} value={status}>
                  {status.charAt(0).toUpperCase() + status.slice(1)}
                </option>
              ))}
            </select>
          </div>
        </div>
      </div>

      <div className="overflow-x-auto">
        <table className="w-full">
          <thead className="bg-muted/50 border-b">
            <tr>
              <th 
                className="text-left p-3 cursor-pointer hover:bg-muted/70 transition-colors"
                onClick={() => handleSort('id')}
              >
                <div className="flex items-center gap-1">
                  Order ID
                  {sortField === 'id' && (
                    <span className="text-xs">
                      {sortDirection === 'asc' ? '↑' : '↓'}
                    </span>
                  )}
                </div>
              </th>
              <th 
                className="text-left p-3 cursor-pointer hover:bg-muted/70 transition-colors"
                onClick={() => handleSort('customer_id')}
              >
                <div className="flex items-center gap-1">
                  Customer ID
                  {sortField === 'customer_id' && (
                    <span className="text-xs">
                      {sortDirection === 'asc' ? '↑' : '↓'}
                    </span>
                  )}
                </div>
              </th>
              <th 
                className="text-left p-3 cursor-pointer hover:bg-muted/70 transition-colors"
                onClick={() => handleSort('status')}
              >
                <div className="flex items-center gap-1">
                  Status
                  {sortField === 'status' && (
                    <span className="text-xs">
                      {sortDirection === 'asc' ? '↑' : '↓'}
                    </span>
                  )}
                </div>
              </th>
              <th 
                className="text-left p-3 cursor-pointer hover:bg-muted/70 transition-colors"
                onClick={() => handleSort('total_amount')}
              >
                <div className="flex items-center gap-1">
                  Total Amount
                  {sortField === 'total_amount' && (
                    <span className="text-xs">
                      {sortDirection === 'asc' ? '↑' : '↓'}
                    </span>
                  )}
                </div>
              </th>
              <th className="text-left p-3">Items</th>
              <th 
                className="text-left p-3 cursor-pointer hover:bg-muted/70 transition-colors"
                onClick={() => handleSort('created_at')}
              >
                <div className="flex items-center gap-1">
                  Created
                  {sortField === 'created_at' && (
                    <span className="text-xs">
                      {sortDirection === 'asc' ? '↑' : '↓'}
                    </span>
                  )}
                </div>
              </th>
              {showActions && <th className="text-left p-3">Actions</th>}
            </tr>
          </thead>
          <tbody>
            {filteredOrders.map((order, index) => (
              <tr 
                key={order.id} 
                className={`border-b hover:bg-muted/30 transition-colors ${
                  realTime && index === 0 ? 'animate-slide-up bg-green-50' : ''
                }`}
              >
                <td className="p-3">
                  <code className="text-sm bg-muted px-2 py-1 rounded">
                    {order.id}
                  </code>
                </td>
                <td className="p-3 text-sm text-muted-foreground">
                  {order.customer_id}
                </td>
                <td className="p-3">
                  <span className={`inline-flex items-center px-2 py-1 rounded-full text-xs font-medium border ${getStatusColor(order.status)}`}>
                    {order.status}
                  </span>
                </td>
                <td className="p-3 font-medium">
                  ${order.total_amount.toFixed(2)}
                </td>
                <td className="p-3 text-sm text-muted-foreground">
                  {order.items.length} item{order.items.length !== 1 ? 's' : ''}
                </td>
                <td className="p-3 text-sm text-muted-foreground">
                  {formatDistanceToNow(new Date(order.created_at), { addSuffix: true })}
                </td>
                {showActions && (
                  <td className="p-3">
                    <button
                      className="text-primary hover:text-primary/80 transition-colors"
                      title="View details"
                    >
                      <ExternalLink className="w-4 h-4" />
                    </button>
                  </td>
                )}
              </tr>
            ))}
          </tbody>
        </table>

        {filteredOrders.length === 0 && (
          <div className="text-center py-8 text-muted-foreground">
            <p>No orders found matching your criteria.</p>
          </div>
        )}
      </div>

      {orders.length > maxRows && (
        <div className="p-3 text-center border-t bg-muted/20">
          <p className="text-sm text-muted-foreground">
            Showing {Math.min(filteredOrders.length, maxRows)} of {orders.length} orders
          </p>
        </div>
      )}
    </div>
  );
}