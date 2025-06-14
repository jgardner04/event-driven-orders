'use client';

import { ReactNode } from 'react';
import { TrendingUp, TrendingDown, Minus } from 'lucide-react';

interface MetricCardProps {
  title: string;
  value: string | number;
  unit?: string;
  change?: {
    value: number;
    trend: 'up' | 'down' | 'neutral';
    period?: string;
  };
  icon?: ReactNode;
  status?: 'healthy' | 'warning' | 'error';
  className?: string;
}

export function MetricCard({ 
  title, 
  value, 
  unit, 
  change, 
  icon, 
  status = 'healthy',
  className = '' 
}: MetricCardProps) {
  const getStatusColor = () => {
    switch (status) {
      case 'healthy':
        return 'border-green-200 bg-green-50';
      case 'warning':
        return 'border-yellow-200 bg-yellow-50';
      case 'error':
        return 'border-red-200 bg-red-50';
      default:
        return 'border-gray-200 bg-white';
    }
  };

  const getTrendIcon = () => {
    if (!change) return null;
    
    switch (change.trend) {
      case 'up':
        return <TrendingUp className="w-4 h-4 text-green-600" />;
      case 'down':
        return <TrendingDown className="w-4 h-4 text-red-600" />;
      case 'neutral':
        return <Minus className="w-4 h-4 text-gray-600" />;
    }
  };

  const getTrendColor = () => {
    if (!change) return '';
    
    switch (change.trend) {
      case 'up':
        return 'text-green-600';
      case 'down':
        return 'text-red-600';
      case 'neutral':
        return 'text-gray-600';
    }
  };

  return (
    <div className={`metric-card ${getStatusColor()} ${className}`}>
      <div className="flex items-start justify-between">
        <div className="flex-1">
          <div className="flex items-center gap-2 mb-1">
            {icon && <div className="text-gray-600">{icon}</div>}
            <h3 className="metric-label">{title}</h3>
          </div>
          
          <div className="flex items-baseline gap-1">
            <span className="metric-value">
              {typeof value === 'number' ? value.toLocaleString() : value}
            </span>
            {unit && <span className="text-sm text-muted-foreground">{unit}</span>}
          </div>
          
          {change && (
            <div className={`flex items-center gap-1 mt-2 text-sm ${getTrendColor()}`}>
              {getTrendIcon()}
              <span>
                {change.value > 0 ? '+' : ''}{change.value}%
                {change.period && <span className="text-muted-foreground ml-1">vs {change.period}</span>}
              </span>
            </div>
          )}
        </div>
        
        {status !== 'healthy' && (
          <div className={`status-dot ${status}`} />
        )}
      </div>
    </div>
  );
}