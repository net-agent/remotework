import { useEffect, useRef } from "react";
import { AgentWebSocket } from "@/lib/ws";
import { useAgentStore } from "@/stores/agent-store";

export function useWebSocket() {
  const { apiBaseUrl, agentRunning, handleWsEvent, setWsConnected } =
    useAgentStore();
  const wsRef = useRef<AgentWebSocket | null>(null);

  useEffect(() => {
    if (!apiBaseUrl || !agentRunning) {
      if (wsRef.current) {
        wsRef.current.disconnect();
        wsRef.current = null;
        setWsConnected(false);
      }
      return;
    }

    const ws = new AgentWebSocket((event) => {
      handleWsEvent(event);
    });

    // Track connection state via polling (simple approach)
    const interval = setInterval(() => {
      setWsConnected(ws.connected);
    }, 1000);

    ws.connect(apiBaseUrl);
    wsRef.current = ws;
    setWsConnected(true);

    return () => {
      clearInterval(interval);
      ws.disconnect();
      wsRef.current = null;
      setWsConnected(false);
    };
  }, [apiBaseUrl, agentRunning, handleWsEvent, setWsConnected]);
}
