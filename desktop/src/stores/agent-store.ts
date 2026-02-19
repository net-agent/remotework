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
  networks: NetworkStateDTO[];
  services: ServiceStateDTO[];
  streams: StreamStateDTO[];

  setAgentRunning: (running: boolean) => void;
  setApiBaseUrl: (url: string) => void;
  setWsConnected: (connected: boolean) => void;
  fetchAll: () => Promise<void>;
  handleWsEvent: (event: WsEvent) => void;
  reset: () => void;
}

export const useAgentStore = create<AgentState>((set, get) => ({
  agentRunning: false,
  apiBaseUrl: "",
  wsConnected: false,
  networks: [],
  services: [],
  streams: [],

  setAgentRunning: (running) => set({ agentRunning: running }),
  setApiBaseUrl: (url) => {
    api.setBaseUrl(url);
    set({ apiBaseUrl: url });
  },
  setWsConnected: (connected) => set({ wsConnected: connected }),

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
        const networks = state.networks.map((n) =>
          n.name === data.name ? data.report : n
        );
        // If network not found, add it
        if (!networks.find((n) => n.name === data.name)) {
          networks.push(data.report);
        }
        set({ networks });
        break;
      }
      case "service.status": {
        const data = event.data as ServiceStatusEvent;
        const services = state.services.map((s) =>
          s.name === data.name ? data.service : s
        );
        if (!services.find((s) => s.name === data.name)) {
          services.push(data.service);
        }
        set({ services });
        break;
      }
      case "stream.update": {
        const data = event.data as StreamUpdateEvent;
        let streams = state.streams.filter(
          (s) =>
            !data.closed.some(
              (c) =>
                c.localAddr === s.localAddr && c.remoteAddr === s.remoteAddr
            )
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
      networks: [],
      services: [],
      streams: [],
    }),
}));
