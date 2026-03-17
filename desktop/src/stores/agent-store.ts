import { create } from "zustand";
import type {
  NetworkStateDTO,
  ServiceStateDTO,
  StreamStateDTO,
  WsEvent,
  NetworkStateEvent,
  ServiceStatusEvent,
  StreamUpdateEvent,
} from "@/lib/types";
import * as api from "@/lib/api";

interface AgentState {
  agentRunning: boolean;
  apiBaseUrl: string;
  wsConnected: boolean;
  startError: string;
  networks: NetworkStateDTO[];
  services: ServiceStateDTO[];
  streams: StreamStateDTO[];

  setAgentRunning: (running: boolean) => void;
  setApiBaseUrl: (url: string) => void;
  setWsConnected: (connected: boolean) => void;
  setStartError: (error: string) => void;
  fetchAll: () => Promise<void>;
  handleWsEvent: (event: WsEvent) => void;
  reset: () => void;
}

export const useAgentStore = create<AgentState>((set, get) => ({
  agentRunning: false,
  apiBaseUrl: "",
  wsConnected: false,
  startError: "",
  networks: [],
  services: [],
  streams: [],

  setAgentRunning: (running) => set({ agentRunning: running }),
  setApiBaseUrl: (url) => {
    api.setBaseUrl(url);
    set({ apiBaseUrl: url });
  },
  setWsConnected: (connected) => set({ wsConnected: connected }),
  setStartError: (error) => set({ startError: error }),

  fetchAll: async () => {
    try {
      const [networks, services, streams] = await Promise.allSettled([
        api.getNetworks(),
        api.getServices(),
        api.getStreams(),
      ]);
      set({
        networks: networks.status === "fulfilled" ? networks.value : [],
        services: services.status === "fulfilled" ? services.value : [],
        streams: streams.status === "fulfilled" ? streams.value : [],
        agentRunning: true,
      });
    } catch {
      set({ agentRunning: false });
    }
  },

  handleWsEvent: (event: WsEvent) => {
    const state = get();
    switch (event.type) {
      case "network.state": {
        const data = event.data as NetworkStateEvent;
        const mapped = state.networks.map((n) =>
          n.name === data.name ? data.report : n,
        );
        const networks = mapped.find((n) => n.name === data.name)
          ? mapped
          : [...mapped, data.report];
        set({ networks });
        break;
      }
      case "service.status": {
        const data = event.data as ServiceStatusEvent;
        const mapped = state.services.map((s) =>
          s.name === data.name ? data.service : s,
        );
        const services = mapped.find((s) => s.name === data.name)
          ? mapped
          : [...mapped, data.service];
        set({ services });
        break;
      }
      case "stream.update": {
        const data = event.data as StreamUpdateEvent;
        let streams = state.streams.filter(
          (s) =>
            !data.closed.some(
              (c) =>
                c.localAddr === s.localAddr && c.remoteAddr === s.remoteAddr,
            ),
        );
        streams = [...streams, ...data.opened];
        set({ streams });
        break;
      }
      case "hub.stopped": {
        set({ agentRunning: false, networks: [], services: [], streams: [] });
        break;
      }
    }
  },

  reset: () =>
    set({
      agentRunning: false,
      apiBaseUrl: "",
      wsConnected: false,
      startError: "",
      networks: [],
      services: [],
      streams: [],
    }),
}));
