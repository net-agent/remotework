import type { AgentConfig, TunnelInfo } from "@/lib/config-types";
import { validateLinkURL, validateTunnel } from "@/lib/config-validation";
import type { NetworkStateDTO } from "@/lib/types";

export interface SimpleConfigSummaryItem {
  title: string;
  value: string;
  tone?: "default" | "warning";
}

export interface SimpleLinkOption {
  alias: string;
  url: string;
  status: "usable" | "saved_needs_check";
  statusText: string;
}

export interface SimpleConfigVM {
  hasShareConfig: boolean;
  hasConnectConfig: boolean;
  shareSummary: SimpleConfigSummaryItem[];
  connectSummary: SimpleConfigSummaryItem[];
  requiresRestart: boolean;
  missingItems: string[];
  invalidItems: string[];
  allLinks: SimpleLinkOption[];
  validLinks: SimpleLinkOption[];
  usableLinks: SimpleLinkOption[];
}

function getLinkEntries(config: AgentConfig) {
  return Object.entries(config.links ?? {});
}

function getTunnels(config: AgentConfig): TunnelInfo[] {
  return config.tunnels ?? [];
}

function hasListenRole(tunnel: TunnelInfo) {
  return tunnel.listen.trim().length > 0;
}

function hasTargetRole(tunnel: TunnelInfo) {
  return tunnel.target.trim().length > 0;
}

function isUsableNetworkState(state: string) {
  return ["online", "connected", "running", "pending", "starting", "init", "connecting"].includes(
    state.toLowerCase(),
  );
}

function buildLinkProjections(
  config: AgentConfig,
  networks: NetworkStateDTO[],
  sidecarReady: boolean,
) {
  const runtimeAliases = new Set(
    networks
      .filter((network) => network.protocol !== "" && isUsableNetworkState(network.state))
      .map((network) => network.name),
  );

  const allLinks = getLinkEntries(config).map(([alias, url]) => {
    const error = validateLinkURL(url);
    const isRuntimeUsable = runtimeAliases.has(alias);
    const status = error
      ? "saved_needs_check"
      : !sidecarReady || isRuntimeUsable
        ? "usable"
        : "saved_needs_check";

    return {
      alias,
      url,
      status,
      statusText: status === "usable" ? "当前可用" : "需检查",
    } satisfies SimpleLinkOption;
  });

  const validLinks = allLinks.filter((item) => validateLinkURL(item.url) === null);
  const usableLinks = allLinks.filter((item) => item.status === "usable");

  return {
    allLinks,
    validLinks,
    usableLinks,
  };
}

export function buildSimpleConfigVM(
  config: AgentConfig,
  requiresRestart: boolean,
  runtime?: {
    networks: NetworkStateDTO[];
    wsConnected: boolean;
    agentRunning: boolean;
  },
): SimpleConfigVM {
  const linkEntries = getLinkEntries(config);
  const tunnels = getTunnels(config);
  const linkAliases = linkEntries.map(([alias]) => alias);

  const invalidLinks = linkEntries
    .map(([alias, url]) => ({ alias, error: validateLinkURL(url) }))
    .filter((item) => item.error);

  const invalidTunnels = tunnels
    .map((tunnel) => ({ tunnel, error: validateTunnel(tunnel, linkAliases) }))
    .filter((item) => item.error);

  const shareTunnels = tunnels.filter(hasListenRole);
  const connectTunnels = tunnels.filter(hasTargetRole);
  const sidecarReady = Boolean(runtime?.agentRunning && runtime?.wsConnected);
  const { allLinks, validLinks, usableLinks } = buildLinkProjections(
    config,
    runtime?.networks ?? [],
    sidecarReady,
  );

  const hasShareConfig = linkEntries.length > 0 || shareTunnels.length > 0;
  const hasConnectConfig = connectTunnels.length > 0;

  const missingItems = [
    ...(linkEntries.length === 0 ? ["还没有填写共享所需的连接信息"] : []),
    ...(shareTunnels.length === 0 ? ["还没有设置可被访问的共享入口"] : []),
    ...(connectTunnels.length === 0 ? ["还没有设置连接他人电脑所需的访问入口"] : []),
  ];

  const invalidItems = [
    ...invalidLinks.map((item) => `连接信息“${item.alias}”存在格式问题`),
    ...invalidTunnels.map((item) => `入口“${item.tunnel.name || "未命名"}”配置无效`),
  ];

  const savedNeedsCheckCount = allLinks.length - usableLinks.length;

  return {
    hasShareConfig,
    hasConnectConfig,
    requiresRestart,
    missingItems,
    invalidItems,
    allLinks,
    validLinks,
    usableLinks,
    shareSummary: [
      { title: "已保存连接信息", value: `${allLinks.length} 项` },
      { title: "当前可用连接信息", value: `${usableLinks.length} 项` },
      { title: "共享入口", value: `${shareTunnels.length} 项` },
      {
        title: "配置状态",
        value:
          invalidLinks.length > 0 || invalidTunnels.length > 0
            ? "有待修正"
            : hasShareConfig
              ? "基本就绪"
              : "尚未完成",
        tone:
          invalidLinks.length > 0 || invalidTunnels.length > 0 ? "warning" : "default",
      },
      ...(savedNeedsCheckCount > 0
        ? [
            {
              title: "待检查连接信息",
              value: `${savedNeedsCheckCount} 项`,
              tone: "warning" as const,
            },
          ]
        : []),
    ],
    connectSummary: [
      { title: "可用访问入口", value: `${connectTunnels.length} 项` },
      {
        title: "生效状态",
        value: requiresRestart ? "需要重启后生效" : "已与当前运行状态同步",
        tone: requiresRestart ? "warning" : "default",
      },
    ],
  };
}
