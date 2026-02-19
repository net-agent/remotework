// Mirrors Go agent/config.go

export interface AgentInfo {
  name: string;
  protocol: string;
  address: string;
  password: string;
  domain: string;
  url: string;
  wsPath: string;
}

export interface PortproxyInfo {
  listen: string;
  target: string;
  log: string;
}

export interface Socks5Info {
  listen: string;
  username: string;
  password: string;
  log: string;
}

export interface RDPInfo {
  listen: string;
  log: string;
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
  agents: AgentInfo[];
  portproxy: PortproxyInfo[];
  socks5: Socks5Info[];
  rdp: RDPInfo[];
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

export function emptyAgentInfo(): AgentInfo {
  return { name: "", protocol: "vtcp", address: "", password: "", domain: "", url: "", wsPath: "" };
}

export function emptyPortproxy(): PortproxyInfo {
  return { listen: "", target: "", log: "" };
}

export function emptySocks5(): Socks5Info {
  return { listen: "", username: "", password: "", log: "" };
}

export function emptyRDP(): RDPInfo {
  return { listen: "", log: "" };
}

export function emptyConfig(): AgentConfig {
  return {
    agents: [],
    portproxy: [],
    socks5: [],
    rdp: [],
    pprof: { enable: false, listen: "" },
    api: { enable: true, listen: "127.0.0.1:8080", pollInterval: 5 },
  };
}
