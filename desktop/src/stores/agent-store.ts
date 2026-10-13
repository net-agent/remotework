import { create } from "zustand";
import type {
  NetworkStateDTO,
  ServiceStateDTO,
  StatusDTO,
  StreamStateDTO,
  WsEvent,
  NetworkStateEvent,
  ServiceStatusEvent,
  StreamUpdateEvent,
} from "@/lib/types";
import * as api from "@/lib/api";
import type { ApiEndpointConfig } from "@/lib/api";

interface AgentState {
  agentRunning: boolean;
  apiEndpoint: ApiEndpointConfig | null;
  wsConnected: boolean;
  startError: string;
  status: StatusDTO | null;
  networks: NetworkStateDTO[];
  services: ServiceStateDTO[];
  streams: StreamStateDTO[];

  setAgentRunning: (running: boolean) => void;
  setApiEndpoint: (endpoint: ApiEndpointConfig | null) => void;
  setWsConnected: (connected: boolean) => void;
  setStartError: (error: string) => void;
  setStatus: (status: StatusDTO | null) => void;
  fetchAll: () => Promise<void>;
  handleWsEvent: (event: WsEvent) => void;
  reset: () => void;
}

export const useAgentStore = create<AgentState>((set, get) => ({
  agentRunning: false,
  apiEndpoint: null,
  wsConnected: false,
  startError: "",
  status: null,
  networks: [],
  services: [],
  streams: [],

  setAgentRunning: (running) => set({ agentRunning: running }),
  setApiEndpoint: (endpoint) => {
    api.setEndpoint(endpoint);
    set({ apiEndpoint: endpoint });
  },
  setWsConnected: (connected) => set({ wsConnected: connected }),
  setStartError: (error) => set({ startError: error }),
  setStatus: (status) => set({ status }),

  fetchAll: async () => {
    const [status, networks, services, streams] = await Promise.allSettled([
      api.getStatus(),
      api.getNetworks(),
      api.getServices(),
      api.getStreams(),
    ]);
    set({
      status: status.status === "fulfilled" ? status.value : get().status,
      networks: networks.status === "fulfilled" ? networks.value : [],
      services: services.status === "fulfilled" ? services.value : [],
      streams: streams.status === "fulfilled" ? streams.value : [],
      agentRunning:
        status.status === "fulfilled" ? status.value.running : true,
    });
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
        set({
          agentRunning: false,
          status: null,
          networks: [],
          services: [],
          streams: [],
        });
        break;
      }
    }
  },

  reset: () => {
    api.setEndpoint(null);
    set({
      agentRunning: false,
      apiEndpoint: null,
      wsConnected: false,
      startError: "",
      status: null,
      networks: [],
      services: [],
      streams: [],
    });
  },
}));
