import { useEffect, useRef } from "react";
import { AgentWebSocket } from "@/lib/ws";
import { useAgentStore } from "@/stores/agent-store";

export function useWebSocket() {
  const { apiEndpoint, agentRunning, handleWsEvent, setWsConnected, fetchAll } =
    useAgentStore();
  const wsRef = useRef<AgentWebSocket | null>(null);

  useEffect(() => {
    if (!apiEndpoint || !agentRunning) {
      if (wsRef.current) {
        wsRef.current.disconnect();
        wsRef.current = null;
        setWsConnected(false);
      }
      return;
    }

    const ws = new AgentWebSocket(
      (event) => {
        handleWsEvent(event);
      },
      () => {
        // 每次 WebSocket（重新）连接后刷新全量状态
        fetchAll();
        setWsConnected(true);
      },
    );

    // Track connection state via polling (simple approach)
    const interval = setInterval(() => {
      setWsConnected(ws.connected);
    }, 1000);

    ws.connect(apiEndpoint.baseUrl, apiEndpoint.token);
    wsRef.current = ws;

    return () => {
      clearInterval(interval);
      ws.disconnect();
      wsRef.current = null;
      setWsConnected(false);
    };
  }, [apiEndpoint, agentRunning, handleWsEvent, setWsConnected, fetchAll]);
}
