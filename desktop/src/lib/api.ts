import type {
  ApiResponse,
  NetworkStateDTO,
  ServiceStateDTO,
  StreamStateDTO,
  PingResultDTO,
} from "./types";

let baseUrl = "";

export function setBaseUrl(url: string) {
  baseUrl = url.replace(/\/$/, "");
}

export function getBaseUrl() {
  return baseUrl;
}

async function request<T>(path: string, init?: RequestInit): Promise<T> {
  const resp = await fetch(`${baseUrl}${path}`, init);
  if (!resp.ok) {
    throw new Error(`HTTP ${resp.status}: ${resp.statusText}`);
  }
  const json: ApiResponse<T> = await resp.json();
  if (json.ErrCode !== 0) {
    throw new Error(json.ErrMsg || `API error code ${json.ErrCode}`);
  }
  return json.Data;
}

export async function getStatus(): Promise<{ running: boolean }> {
  return request("/api/v1/status");
}

export async function getNetworks(): Promise<NetworkStateDTO[]> {
  return request("/api/v1/networks");
}

export async function getServices(): Promise<ServiceStateDTO[]> {
  return request("/api/v1/services");
}

export async function getStreams(limit = 50, network?: string): Promise<StreamStateDTO[]> {
  const params = new URLSearchParams({ limit: String(limit) });
  if (network) params.set("network", network);
  return request(`/api/v1/streams?${params}`);
}

export async function ping(): Promise<PingResultDTO[]> {
  return request("/api/v1/ping", { method: "POST" });
}

export async function setLogLevel(level: string): Promise<{ level: string }> {
  return request("/api/v1/loglevel", {
    method: "PUT",
    headers: { "Content-Type": "application/json" },
    body: JSON.stringify({ level }),
  });
}

export async function stopAgent(): Promise<{ status: string }> {
  return request("/api/v1/stop", {
    method: "POST",
    headers: { "X-Confirm": "yes" },
  });
}
