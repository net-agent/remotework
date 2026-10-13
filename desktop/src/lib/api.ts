import type {
  ApiResponse,
  ListeningPortDTO,
  NetworkStateDTO,
  ServiceStateDTO,
  StatusDTO,
  StreamStateDTO,
  PingResultDTO,
} from "./types";
import type { TunnelInfo } from "./config-types";

export interface ApiEndpointConfig {
  baseUrl: string;
  token: string | null;
}

// 当前 sidecar 连接凭证（协议 §7）：由 agent store 在启动/重启后经
// platform.getApiEndpoint() 刷新；token 为 null 对应 standalone 开放模式。
let endpoint: ApiEndpointConfig = { baseUrl: "", token: null };

export function setEndpoint(next: ApiEndpointConfig | null) {
  endpoint = next
    ? { baseUrl: next.baseUrl.replace(/\/$/, ""), token: next.token || null }
    : { baseUrl: "", token: null };
}

export function getEndpoint(): ApiEndpointConfig {
  return endpoint;
}

export function getBaseUrl() {
  return endpoint.baseUrl;
}

async function request<T>(path: string, init?: RequestInit): Promise<T> {
  const headers = new Headers(init?.headers);
  if (endpoint.token) {
    headers.set("Authorization", `Bearer ${endpoint.token}`);
  }
  const resp = await fetch(`${endpoint.baseUrl}${path}`, { ...init, headers });
  if (!resp.ok) {
    throw new Error(`HTTP ${resp.status}: ${resp.statusText}`);
  }
  const json: ApiResponse<T> = await resp.json();
  if (json.ErrCode !== 0) {
    throw new Error(json.ErrMsg || `API error code ${json.ErrCode}`);
  }
  return json.Data;
}

export async function getStatus(): Promise<StatusDTO> {
  return request("/api/v1/status");
}

export async function getNetworks(): Promise<NetworkStateDTO[]> {
  return request("/api/v1/networks");
}

export async function getServices(): Promise<ServiceStateDTO[]> {
  return request("/api/v1/services");
}

export async function getListeningPorts(): Promise<ListeningPortDTO[]> {
  return request("/api/v1/system/listening-ports");
}

export async function getStreams(
  limit = 50,
  network?: string,
): Promise<StreamStateDTO[]> {
  const params = new URLSearchParams({ limit: String(limit) });
  if (network) params.set("network", network);
  return request(`/api/v1/streams?${params}`);
}

export async function ping(): Promise<PingResultDTO[]> {
  return request("/api/v1/ping", { method: "POST" });
}

export async function pingSingle(
  network: string,
  domain: string,
): Promise<{ network: string; domain: string; result: string }> {
  return request(
    `/api/v1/ping/${encodeURIComponent(network)}/${encodeURIComponent(domain)}`,
    { method: "POST" },
  );
}

export async function setLogLevel(level: string): Promise<{ level: string }> {
  return request("/api/v1/loglevel", {
    method: "PUT",
    headers: { "Content-Type": "application/json" },
    body: JSON.stringify({ level }),
  });
}

export async function stopAgent(): Promise<{ status: string }> {
  return request("/api/v1/stop", { method: "POST" });
}

export async function addLink(
  alias: string,
  url: string,
): Promise<{ status: string; alias: string }> {
  return request("/api/v1/links", {
    method: "POST",
    headers: { "Content-Type": "application/json" },
    body: JSON.stringify({ alias, url }),
  });
}

export async function addTunnel(
  tunnel: TunnelInfo,
): Promise<{ status: string; name: string; id: string }> {
  return request("/api/v1/tunnels", {
    method: "POST",
    headers: { "Content-Type": "application/json" },
    body: JSON.stringify(tunnel),
  });
}
