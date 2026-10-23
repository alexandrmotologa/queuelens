import { useState, useEffect, useRef } from 'react';
import { JobEvent, MetricsSummary } from '../types';

export function useQueueStream() {
  const [events, setEvents] = useState<JobEvent[]>([]);
  const [metrics, setMetrics] = useState<MetricsSummary | null>(null);
  const [connected, setConnected] = useState<boolean>(false);
  const wsRef = useRef<WebSocket | null>(null);

  useEffect(() => {
    let reconnectTimeout: any;

    function connect() {
      const protocol = window.location.protocol === 'https:' ? 'wss:' : 'ws:';
      const wsUrl = `${protocol}//${window.location.host}/api/ws`;

      const ws = new WebSocket(wsUrl);
      wsRef.current = ws;

      ws.onopen = () => {
        setConnected(true);
      };

      ws.onmessage = (event) => {
        try {
          const data = JSON.parse(event.data);
          if (data.type === 'history') {
            setEvents(data.payload || []);
          } else if (data.type === 'event') {
            setEvents((prev) => [data.payload, ...prev].slice(0, 150));
          } else if (data.type === 'metrics') {
            setMetrics(data.payload);
          }
        } catch (e) {
          console.error('Failed to parse websocket message', e);
        }
      };

      ws.onclose = () => {
        setConnected(false);
        reconnectTimeout = setTimeout(connect, 2000);
      };

      ws.onerror = () => {
        ws.close();
      };
    }

    connect();

    return () => {
      clearTimeout(reconnectTimeout);
      if (wsRef.current) {
        wsRef.current.close();
      }
    };
  }, []);

  return { events, metrics, connected };
}
