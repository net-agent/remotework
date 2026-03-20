import type { AgentConfig, TunnelInfo } from "@/lib/config-types";
import type {
  SimpleConfigVM,
  SimpleLinkOption,
} from "@/lib/view-model/simple-config-vm";
import type { SimpleRuntimeVM } from "@/lib/view-model/simple-runtime-vm";
import { buildSimpleConfigVM } from "@/lib/view-model/simple-config-vm";
import { buildSimpleRuntimeVM } from "@/lib/view-model/simple-runtime-vm";
import type {
  NetworkStateDTO,
  ServiceStateDTO,
  StreamStateDTO,
} from "@/lib/types";

export type SimpleShareServiceType = "socks5" | "local-port";

export interface SimpleShareServiceItem {
  port: string | null;
  listenURL: string;
  targetURL: string;
  tunnelId: string | null;
  configIndex: number | null;
  tunnelName: string | null;
  processName: string | null;
  pid: number | null;
  state: "closed" | "opening" | "open" | "error";
  lastErr: string | null;
}

export interface SimpleShareServiceRow {
  key: SimpleShareServiceType;
  title: string;
  description: string;
  isOpen: boolean;
  state: "closed" | "opening" | "open" | "error";
  statusText: string;
  listenURL: string | null;
  targetURL: string | null;
  tunnelId: string | null;
  configIndex: number | null;
  lastErr: string | null;
  tunnelName: string | null;
  openCount: number;
  listenURLs: string[];
  targetURLs: string[];
  previewPorts: string[];
  extraPortCount: number;
  closeAction: "close" | "manage";
  items: SimpleShareServiceItem[];
}

export interface SimpleSessionVM {
  runtime: SimpleRuntimeVM;
  config: SimpleConfigVM;
  shareState: "ready" | "not_ready" | "degraded";
  connectState: "idle" | "validating" | "connected" | "error";
  activeConnectionCount: number;
  requiresRestart: boolean;
  hasUsableShareLinks: boolean;
  availableShareLinks: SimpleLinkOption[];
  mustCreateNewShareLink: boolean;
  shareLinkHint: string;
  userFacingHints: string[];
  selectedShareLink: SimpleLinkOption | null;
  shareServiceRows: SimpleShareServiceRow[];
}

function isRunningStatus(status: string) {
  return ["online", "connected", "running", "ready"].includes(
    status.toLowerCase(),
  );
}

function isPendingStatus(status: string) {
  return ["pending", "starting", "init", "connecting"].includes(
    status.toLowerCase(),
  );
}

function getSelectedShareLink(
  availableShareLinks: SimpleLinkOption[],
  selectedAlias?: string | null,
) {
  if (availableShareLinks.length === 0) {
    return null;
  }

  if (selectedAlias) {
    return (
      availableShareLinks.find((link) => link.alias === selectedAlias) ?? null
    );
  }

  return availableShareLinks[0] ?? null;
}

function extractPort(value: string) {
  try {
    const parsed = new URL(value);
    return parsed.port || null;
  } catch {
    return null;
  }
}

function resolveTunnelType(tunnel: TunnelInfo): SimpleShareServiceType | null {
  const target = tunnel.target.toLowerCase();

  if (target.startsWith("socks5://")) {
    return "socks5";
  }

  if (!target.startsWith("tcp://")) {
    return null;
  }

  try {
    const listen = new URL(tunnel.listen);
    const scheme = listen.protocol.replace(":", "").toLowerCase();
    if (
      ["tcp", "tcp4", "tcp6", "http", "https", "ws", "wss"].includes(scheme)
    ) {
      return null;
    }
    return "local-port";
  } catch {
    return null;
  }
}

function extractAliasFromListen(listen: string) {
  try {
    const parsed = new URL(listen);
    const scheme = parsed.protocol.replace(":", "").toLowerCase();

    if (scheme === "vtcp") {
      const hostname = parsed.hostname;
      const segments = hostname.split(".");
      if (segments.length < 2) {
        return null;
      }
      return segments[segments.length - 1] ?? null;
    }

    return scheme || null;
  } catch {
    return null;
  }
}

function findRuntimeService(
  tunnel: TunnelInfo,
  services: ServiceStateDTO[],
): ServiceStateDTO | null {
  if (tunnel.id) {
    const byId = services.find((service) => service.tunnelId === tunnel.id);
    if (byId) {
      return byId;
    }
  }

  return services.find((service) => service.name === tunnel.name) ?? null;
}

function buildLocalPortStatusText(input: {
  state: SimpleShareServiceRow["state"];
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

function buildItemState(runtimeService: ServiceStateDTO | null) {
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

function buildShareServiceRows(input: {
  selectedAlias: string | null;
  currentConfig: AgentConfig;
  services: ServiceStateDTO[];
}): SimpleShareServiceRow[] {
  const definitions: Array<{
    key: SimpleShareServiceType;
    title: string;
    description: string;
  }> = [
    {
      key: "socks5",
      title: "本地网络",
      description: "让对方通过你的电脑访问当前本地网络资源。",
    },
    {
      key: "local-port",
      title: "本地端口",
      description: "筛选并开放本机多个正在监听的端口。",
    },
  ];

  const tunnels = input.currentConfig.tunnels ?? [];

  return definitions.map((definition) => {
    const matches = tunnels
      .map((tunnel, configIndex) => {
        const runtimeService = findRuntimeService(tunnel, input.services);
        const alias = extractAliasFromListen(tunnel.listen);
        return {
          tunnel,
          configIndex,
          runtimeService,
          alias,
        };
      })
      .filter(
        (entry) =>
          entry.alias === input.selectedAlias &&
          resolveTunnelType(entry.tunnel) === definition.key,
      );

    if (matches.length === 0) {
      return {
        key: definition.key,
        title: definition.title,
        description: definition.description,
        isOpen: false,
        state: "closed",
        statusText: input.selectedAlias ? "尚未开放" : "请先选择连接信息",
        listenURL: null,
        targetURL: null,
        tunnelId: null,
        configIndex: null,
        lastErr: null,
        tunnelName: null,
        openCount: 0,
        listenURLs: [],
        targetURLs: [],
        previewPorts: [],
        extraPortCount: 0,
        closeAction: "close",
        items: [],
      } satisfies SimpleShareServiceRow;
    }

    const hasRuntime = matches.some((entry) => Boolean(entry.runtimeService));
    const hasError = matches.some((entry) =>
      Boolean(entry.runtimeService?.lastErr),
    );
    const hasPending = matches.some(
      (entry) =>
        entry.runtimeService && isPendingStatus(entry.runtimeService.status),
    );
    const allRunning = matches.every(
      (entry) =>
        entry.runtimeService &&
        isRunningStatus(entry.runtimeService.status) &&
        !entry.runtimeService.lastErr,
    );

    const rowState: SimpleShareServiceRow["state"] = hasError
      ? "error"
      : hasPending || !allRunning
        ? "opening"
        : "open";

    const listenURLs = matches.map(
      (entry) => entry.runtimeService?.listenURL || entry.tunnel.listen,
    );
    const targetURLs = matches.map(
      (entry) => entry.runtimeService?.targetURL || entry.tunnel.target,
    );
    const previewPorts = matches
      .map((entry) =>
        extractPort(entry.runtimeService?.listenURL || entry.tunnel.listen),
      )
      .filter((port): port is string => Boolean(port))
      .slice(0, 3);
    const extraPortCount = Math.max(matches.length - previewPorts.length, 0);
    const primary = matches[0] ?? null;

    const items: SimpleShareServiceItem[] = matches.map((entry) => ({
      port: extractPort(entry.runtimeService?.listenURL || entry.tunnel.listen),
      listenURL: entry.runtimeService?.listenURL || entry.tunnel.listen,
      targetURL: entry.runtimeService?.targetURL || entry.tunnel.target,
      tunnelId: entry.runtimeService?.tunnelId || entry.tunnel.id || null,
      configIndex: entry.configIndex,
      tunnelName: entry.tunnel.name || null,
      processName: null,
      pid: null,
      state: buildItemState(entry.runtimeService),
      lastErr: entry.runtimeService?.lastErr || null,
    }));

    if (definition.key === "local-port") {
      const runtimeErrors = matches
        .map((entry) => entry.runtimeService?.lastErr)
        .filter((value): value is string => Boolean(value));

      return {
        key: definition.key,
        title: definition.title,
        description: definition.description,
        isOpen: true,
        state: rowState,
        statusText: buildLocalPortStatusText({
          state: rowState,
          openCount: matches.length,
          hasRuntime,
        }),
        listenURL: listenURLs[0] ?? null,
        targetURL: targetURLs[0] ?? null,
        tunnelId:
          matches.length === 1
            ? primary?.runtimeService?.tunnelId || primary?.tunnel.id || null
            : null,
        configIndex:
          matches.length === 1 ? (primary?.configIndex ?? null) : null,
        lastErr: runtimeErrors[0] ?? null,
        tunnelName: primary?.tunnel.name ?? null,
        openCount: matches.length,
        listenURLs,
        targetURLs,
        previewPorts,
        extraPortCount,
        closeAction: "manage",
        items,
      } satisfies SimpleShareServiceRow;
    }

    const primaryRuntime = primary?.runtimeService ?? null;
    const primaryTunnel = primary?.tunnel ?? null;
    const statusText = primaryRuntime?.lastErr
      ? "运行异常"
      : rowState === "open"
        ? "已开放"
        : rowState === "opening"
          ? "开放中"
          : "运行异常";

    return {
      key: definition.key,
      title: definition.title,
      description: definition.description,
      isOpen: true,
      state: rowState,
      statusText,
      listenURL: primaryRuntime?.listenURL || primaryTunnel?.listen || null,
      targetURL: primaryRuntime?.targetURL || primaryTunnel?.target || null,
      tunnelId: primaryRuntime?.tunnelId || primaryTunnel?.id || null,
      configIndex: primary?.configIndex ?? null,
      lastErr: primaryRuntime?.lastErr || null,
      tunnelName: primaryTunnel?.name || null,
      openCount: matches.length,
      listenURLs,
      targetURLs,
      previewPorts,
      extraPortCount,
      closeAction: "close",
      items,
    } satisfies SimpleShareServiceRow;
  });
}

export function buildSimpleSessionVM(input: {
  agentRunning: boolean;
  wsConnected: boolean;
  networks: NetworkStateDTO[];
  services: ServiceStateDTO[];
  streams: StreamStateDTO[];
  currentConfig: AgentConfig;
  needsRestart: boolean;
  selectedShareLinkAlias?: string | null;
}): SimpleSessionVM {
  const runtime = buildSimpleRuntimeVM({
    agentRunning: input.agentRunning,
    wsConnected: input.wsConnected,
    networks: input.networks,
    services: input.services,
    streams: input.streams,
  });

  const config = buildSimpleConfigVM(input.currentConfig, input.needsRestart, {
    agentRunning: input.agentRunning,
    wsConnected: input.wsConnected,
    networks: input.networks,
  });

  const hasUsableShareLinks = config.usableLinks.length > 0;
  const savedNeedsCheckCount =
    config.allLinks.length - config.usableLinks.length;
  const mustCreateNewShareLink =
    config.allLinks.length === 0 || !hasUsableShareLinks;
  const shareLinkHint = mustCreateNewShareLink
    ? config.allLinks.length === 0
      ? "当前还没有可用连接信息，请先输入新的连接信息。"
      : `已保存 ${config.allLinks.length} 条连接信息，但当前都需检查，请先修正或重新输入。`
    : savedNeedsCheckCount > 0
      ? `已检测到 ${config.usableLinks.length} 条可用连接信息，另有 ${savedNeedsCheckCount} 条需检查。`
      : `已检测到 ${config.usableLinks.length} 条可用连接信息，可直接选择使用。`;

  const userFacingHints = [
    ...runtime.userFacingHints,
    ...(config.requiresRestart
      ? ["你有新配置尚未生效，重启后会按新配置运行"]
      : []),
    ...config.invalidItems,
    ...config.missingItems,
  ];

  const selectedShareLink = getSelectedShareLink(
    config.usableLinks,
    input.selectedShareLinkAlias,
  );
  const shareServiceRows = buildShareServiceRows({
    selectedAlias: selectedShareLink?.alias ?? null,
    currentConfig: input.currentConfig,
    services: input.services,
  });

  return {
    runtime,
    config,
    shareState:
      config.hasShareConfig && runtime.shareState === "ready"
        ? "ready"
        : config.invalidItems.length > 0
          ? "degraded"
          : "not_ready",
    connectState:
      config.hasConnectConfig || runtime.connectState !== "idle"
        ? runtime.connectState
        : "idle",
    activeConnectionCount: runtime.activeConnectionCount,
    requiresRestart: config.requiresRestart,
    hasUsableShareLinks,
    availableShareLinks: config.usableLinks,
    mustCreateNewShareLink,
    shareLinkHint,
    userFacingHints,
    selectedShareLink,
    shareServiceRows,
  };
}
