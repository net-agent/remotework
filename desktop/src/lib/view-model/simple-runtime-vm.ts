import type {
  NetworkStateDTO,
  ServiceStateDTO,
  StreamStateDTO,
} from "@/lib/types";

export type SimpleRuntimeState =
  | "stopped"
  | "ready"
  | "connecting"
  | "degraded"
  | "recovering";

export interface SimpleRuntimeVM {
  overallState: SimpleRuntimeState;
  shareState: "ready" | "not_ready" | "degraded";
  connectState: "idle" | "validating" | "connected" | "error";
  activeConnectionCount: number;
  onlineNetworkCount: number;
  configuredNetworkCount: number;
  runningServiceCount: number;
  totalServiceCount: number;
  userFacingHints: string[];
}

function isOnlineState(value: string) {
  return ["online", "connected", "running"].includes(value.toLowerCase());
}

function isPendingState(value: string) {
  return [
    "pending",
    "starting",
    "init",
    "connecting",
    "ready",
    "idle",
  ].includes(value.toLowerCase());
}

function isErrorState(value: string) {
  return ["error", "failed", "closed", "offline"].includes(value.toLowerCase());
}

export function buildSimpleRuntimeVM(input: {
  agentRunning: boolean;
  wsConnected: boolean;
  networks: NetworkStateDTO[];
  services: ServiceStateDTO[];
  streams: StreamStateDTO[];
}): SimpleRuntimeVM {
  const userNetworks = input.networks.filter(
    (network) => network.protocol !== "",
  );
  const onlineNetworks = userNetworks.filter((network) =>
    isOnlineState(network.state),
  );
  const pendingNetworks = userNetworks.filter((network) =>
    isPendingState(network.state),
  );
  const failedNetworks = userNetworks.filter(
    (network) => network.lastErr || isErrorState(network.state),
  );

  const runningServices = input.services.filter((service) =>
    isOnlineState(service.status),
  );
  const pendingServices = input.services.filter((service) =>
    isPendingState(service.status),
  );
  const failedServices = input.services.filter(
    (service) => service.lastErr || isErrorState(service.status),
  );

  const activeConnectionCount = input.streams.filter(
    (stream) => !stream.isClosed,
  ).length;

  const userFacingHints = [
    ...(!input.agentRunning ? ["当前服务未启动，请先启动后再共享或连接"] : []),
    ...(input.agentRunning && !input.wsConnected
      ? ["正在恢复实时状态，请稍候"]
      : []),
    ...(failedNetworks.length > 0
      ? ["部分连接信息异常，可能影响连接稳定性"]
      : []),
    ...(failedServices.length > 0
      ? ["部分访问入口异常，请检查配置或重试"]
      : []),
    ...(activeConnectionCount > 0
      ? [`当前有 ${activeConnectionCount} 个会话正在传输`]
      : []),
  ];

  const overallState: SimpleRuntimeState = !input.agentRunning
    ? "stopped"
    : !input.wsConnected
      ? "recovering"
      : pendingNetworks.length > 0 || pendingServices.length > 0
        ? "connecting"
        : failedNetworks.length > 0 || failedServices.length > 0
          ? "degraded"
          : "ready";

  const shareState = !input.agentRunning
    ? "not_ready"
    : failedNetworks.length > 0 || failedServices.length > 0
      ? "degraded"
      : onlineNetworks.length > 0 || runningServices.length > 0
        ? "ready"
        : "not_ready";

  const connectState = !input.agentRunning
    ? "idle"
    : activeConnectionCount > 0
      ? "connected"
      : pendingNetworks.length > 0 || pendingServices.length > 0
        ? "validating"
        : failedNetworks.length > 0 || failedServices.length > 0
          ? "error"
          : "idle";

  return {
    overallState,
    shareState,
    connectState,
    activeConnectionCount,
    onlineNetworkCount: onlineNetworks.length,
    configuredNetworkCount: userNetworks.length,
    runningServiceCount: runningServices.length,
    totalServiceCount: input.services.length,
    userFacingHints,
  };
}
