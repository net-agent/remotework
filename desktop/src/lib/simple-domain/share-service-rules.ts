import type { TunnelInfo } from "@/lib/config-types";
import type { ServiceStateDTO } from "@/lib/types";
import { isLocalPortTunnel, parsePortFromUrl } from "@/lib/simple-domain/local-port-rules";

export type SimpleShareServiceType = "socks5" | "local-port";
export type SimpleShareRowState = "closed" | "opening" | "open" | "error";

export function isRunningStatus(status: string) {
  return ["online", "connected", "running", "ready"].includes(
    status.toLowerCase(),
  );
}

export function isPendingStatus(status: string) {
  return ["pending", "starting", "init", "connecting"].includes(
    status.toLowerCase(),
  );
}

export function resolveShareServiceType(
  tunnel: TunnelInfo,
): SimpleShareServiceType | null {
  const target = tunnel.target.toLowerCase();

  if (target.startsWith("socks5://")) {
    return "socks5";
  }

  return isLocalPortTunnel(tunnel) ? "local-port" : null;
}

export function findRuntimeService(
  tunnel: TunnelInfo,
  services: ServiceStateDTO[],
) {
  if (tunnel.id) {
    const byId = services.find((service) => service.tunnelId === tunnel.id);
    if (byId) {
      return byId;
    }
  }

  return services.find((service) => service.name === tunnel.name) ?? null;
}

export function buildLocalPortStatusText(input: {
  state: SimpleShareRowState;
  openCount: number;
  hasRuntime: boolean;
}) {
  if (input.openCount === 0) {
    return "尚未开放";
  }

  if (!input.hasRuntime) {
    return input.openCount === 1
      ? "已保存，等待生效"
      : `已保存 ${input.openCount} 个端口，等待生效`;
  }

  if (input.state === "error") {
    return input.openCount === 1
      ? "运行异常"
      : `${input.openCount} 个端口中存在异常`;
  }

  if (input.state === "opening") {
    return input.openCount === 1
      ? "开放中"
      : `正在开放 ${input.openCount} 个端口`;
  }

  return input.openCount === 1 ? "已开放" : `已开放 ${input.openCount} 个端口`;
}

export function buildShareServiceItemState(runtimeService: ServiceStateDTO | null) {
  if (!runtimeService) {
    return "opening" as const;
  }

  if (runtimeService.lastErr) {
    return "error" as const;
  }

  if (isRunningStatus(runtimeService.status)) {
    return "open" as const;
  }

  if (isPendingStatus(runtimeService.status)) {
    return "opening" as const;
  }

  return "error" as const;
}

export { parsePortFromUrl };
