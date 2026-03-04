// Mirrors Go agent/configv2/model.go

export interface TunnelInfo {
  id: string;
  name: string;
  listen: string;
  target: string;
}

export interface PprofInfo {
  enable: boolean;
  listen: string;
}

export interface APIInfo {
  enable: boolean;
  listen: string;
  pollInterval: number;
}

export interface AgentConfig {
  links: Record<string, string>; // alias -> Registration URL
  tunnels: TunnelInfo[];
  pprof: PprofInfo;
  api: APIInfo;
}

// Profile metadata (from Rust config.rs)
export interface ProfileMeta {
  name: string;
  filename: string;
}

export interface ProfilesIndex {
  active: string;
  profiles: ProfileMeta[];
}

// Helpers

export function emptyTunnel(): TunnelInfo {
  return {
    id: crypto.randomUUID(),
    name: "",
    listen: "",
    target: "",
  };
}

export function emptyConfig(): AgentConfig {
  return {
    links: {},
    tunnels: [],
    pprof: { enable: false, listen: "" },
    api: { enable: true, listen: "127.0.0.1:8080", pollInterval: 5 },
  };
}
