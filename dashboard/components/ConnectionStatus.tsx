'use client';

import { useState, useEffect } from 'react';
import { Wifi, WifiOff, Loader2 } from 'lucide-react';

interface ConnectionStatusProps {
  status: 'connected' | 'disconnected' | 'connecting';
  onReconnect?: () => void;
}

export function ConnectionStatus({ status, onReconnect }: ConnectionStatusProps) {
  const [lastConnected, setLastConnected] = useState<string | null>(null);

  useEffect(() => {
    if (status === 'connected') {
      setLastConnected(new Date().toLocaleTimeString());
    }
  }, [status]);

  const getStatusColor = () => {
    switch (status) {
      case 'connected':
        return 'text-green-600';
      case 'connecting':
        return 'text-yellow-600';
      case 'disconnected':
        return 'text-red-600';
    }
  };

  const getStatusIcon = () => {
    switch (status) {
      case 'connected':
        return <Wifi className="w-4 h-4" />;
      case 'connecting':
        return <Loader2 className="w-4 h-4 animate-spin" />;
      case 'disconnected':
        return <WifiOff className="w-4 h-4" />;
    }
  };

  const getStatusText = () => {
    switch (status) {
      case 'connected':
        return `Connected${lastConnected ? ` at ${lastConnected}` : ''}`;
      case 'connecting':
        return 'Connecting...';
      case 'disconnected':
        return 'Disconnected';
    }
  };

  return (
    <div className={`connection-indicator ${status} ${getStatusColor()}`}>
      {getStatusIcon()}
      <span className="text-xs">{getStatusText()}</span>
      {status === 'disconnected' && onReconnect && (
        <button
          onClick={onReconnect}
          className="ml-2 text-xs underline hover:no-underline"
        >
          Reconnect
        </button>
      )}
    </div>
  );
}