// Mirrors Go api/dto.go

export interface ServiceStateDTO {
  id: number;
  tunnelId?: string;
  type: string;
  name: string;
  status: string;
  lastErr?: string;
  listenURL: string;
  targetURL: string;
  actives: number;
  dones: number;
}

export interface NetworkStateDTO {
  name: string;
  protocol: string;
  address: string;
  domain: string;
  state: string;
  lastErr?: string;
  aliveMs: number;
  listens: number;
  dials: number;
}

export interface StreamStateDTO {
  network: string;
  localDomain: string;
  localAddr: string;
  remoteDomain: string;
  remoteAddr: string;
  readBytes: number;
  writeBytes: number;
  aliveMs: number;
  isClosed: boolean;
}

export interface PingResultDTO {
  network: string;
  domain: string;
  result: string;
  usedServices: string[];
}

// API response wrapper
export interface ApiResponse<T> {
  ErrCode: number;
  ErrMsg: string;
  Data: T;
}

// WebSocket event
export interface WsEvent<T = unknown> {
  type: string;
  timestamp: number;
  data: T;
}

export interface NetworkStateEvent {
  name: string;
  oldState: string;
  newState: string;
  report: NetworkStateDTO;
}

export interface ServiceStatusEvent {
  name: string;
  oldStatus: string;
  newStatus: string;
  service: ServiceStateDTO;
}

export interface StreamUpdateEvent {
  network: string;
  opened: StreamStateDTO[];
  closed: StreamStateDTO[];
}
