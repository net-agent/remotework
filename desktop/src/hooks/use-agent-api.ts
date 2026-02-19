import { useCallback } from "react";
import * as api from "@/lib/api";

export function useAgentApi() {
  const getNetworks = useCallback(() => api.getNetworks(), []);
  const getServices = useCallback(() => api.getServices(), []);
  const getStreams = useCallback(
    (limit?: number, network?: string) => api.getStreams(limit, network),
    []
  );
  const pingAll = useCallback(() => api.ping(), []);
  const setLogLevel = useCallback((level: string) => api.setLogLevel(level), []);
  const stopAgent = useCallback(() => api.stopAgent(), []);

  return { getNetworks, getServices, getStreams, pingAll, setLogLevel, stopAgent };
}
