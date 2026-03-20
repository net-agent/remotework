import type { AgentConfig, TunnelInfo } from "@/lib/config-types";
import { validateLinkURL, validateTunnel } from "@/lib/config-validation";
import { buildValidatedLinkOptions } from "@/lib/simple-domain/link-rules";
import type { SimpleLinkOption } from "@/lib/simple-domain/link-rules";

export interface SimpleConfigSummaryItem {
  title: string;
  value: string;
  tone?: "default" | "warning";
}

export type { SimpleLinkOption };

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

export function buildSimpleConfigVM(
  config: AgentConfig,
  requiresRestart: boolean,
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
  const { allLinks, validLinks } = buildValidatedLinkOptions(config);
  const savedNeedsCheckCount = allLinks.length - validLinks.length;
  const hasShareConfig = linkEntries.length > 0 || shareTunnels.length > 0;
  const hasConnectConfig = connectTunnels.length > 0;

  const missingItems = [
    ...(linkEntries.length === 0 ? ["还没有填写共享所需的连接信息"] : []),
    ...(shareTunnels.length === 0 ? ["还没有设置可被访问的共享入口"] : []),
    ...(connectTunnels.length === 0
      ? ["还没有设置连接他人电脑所需的访问入口"]
      : []),
  ];

  const invalidItems = [
    ...invalidLinks.map((item) => `连接信息“${item.alias}”存在格式问题`),
    ...invalidTunnels.map(
      (item) => `入口“${item.tunnel.name || "未命名"}”配置无效`,
    ),
  ];

  return {
    hasShareConfig,
    hasConnectConfig,
    requiresRestart,
    missingItems,
    invalidItems,
    allLinks,
    validLinks,
    usableLinks: validLinks,
    shareSummary: [
      { title: "已保存连接信息", value: `${allLinks.length} 项` },
      { title: "当前可用连接信息", value: `${validLinks.length} 项` },
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
          invalidLinks.length > 0 || invalidTunnels.length > 0
            ? "warning"
            : "default",
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
