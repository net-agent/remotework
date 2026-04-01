import { emptyTunnel, type TunnelInfo } from "@/lib/config-types";
import type { SimpleLinkOption } from "@/lib/view-model/simple-config-vm";

export interface ConnectFormAnalysis {
  extractedAlias: string | null;
  needsAliasPicker: boolean;
  resolvedAlias: string | null;
  portFromUrl: string | null;
  error: string | null;
}

export function isConnectTunnel(tunnel: TunnelInfo): boolean {
  return (
    tunnel.listen.toLowerCase().startsWith("tcp://") &&
    tunnel.target.toLowerCase().startsWith("vtcp://")
  );
}

export function extractAliasFromVtcpUrl(url: string): string | null {
  try {
    const parsed = new URL(url);
    if (parsed.protocol.replace(":", "").toLowerCase() !== "vtcp") return null;
    const segments = parsed.hostname.split(".");
    return segments[segments.length - 1] ?? null;
  } catch {
    return null;
  }
}

export function extractPortFromVtcpUrl(url: string): string | null {
  try {
    const parsed = new URL(url);
    return parsed.port || null;
  } catch {
    return null;
  }
}

export function resolveTargetWithLocalAlias(
  vtcpUrl: string,
  localAlias: string,
): string | null {
  try {
    const parsed = new URL(vtcpUrl);
    const segments = parsed.hostname.split(".");
    if (segments.length < 2) return null;
    const serviceParts = segments.slice(0, -1);
    const newHostname = [...serviceParts, localAlias].join(".");
    const search = parsed.search;
    return `vtcp://${newHostname}:${parsed.port}${search}`;
  } catch {
    return null;
  }
}

export function analyzeConnectForm(
  targetVtcpUrl: string,
  links: SimpleLinkOption[],
): ConnectFormAnalysis {
  const trimmed = targetVtcpUrl.trim();

  if (!trimmed) {
    return {
      extractedAlias: null,
      needsAliasPicker: false,
      resolvedAlias: links[0]?.alias ?? null,
      portFromUrl: null,
      error: null,
    };
  }

  if (!trimmed.toLowerCase().startsWith("vtcp://")) {
    return {
      extractedAlias: null,
      needsAliasPicker: false,
      resolvedAlias: null,
      portFromUrl: null,
      error: "目标地址必须是 vtcp:// 格式",
    };
  }

  const extractedAlias = extractAliasFromVtcpUrl(trimmed);
  const portFromUrl = extractPortFromVtcpUrl(trimmed);
  const exactMatch = extractedAlias
    ? links.find((l) => l.alias === extractedAlias)
    : null;
  const needsAliasPicker = !exactMatch && links.length > 0;
  const resolvedAlias = exactMatch?.alias ?? links[0]?.alias ?? null;

  return {
    extractedAlias,
    needsAliasPicker,
    resolvedAlias,
    portFromUrl,
    error: null,
  };
}

export function validateConnectForm(input: {
  targetVtcpUrl: string;
  localPort: string;
  localAlias: string | null;
}): string | null {
  if (!input.targetVtcpUrl.trim()) return "请输入目标虚拟地址";
  if (!input.targetVtcpUrl.trim().toLowerCase().startsWith("vtcp://"))
    return "目标地址必须是 vtcp:// 格式";
  const port = Number(input.localPort);
  if (!input.localPort || !Number.isInteger(port) || port < 1 || port > 65535)
    return "本地端口需为 1–65535 的整数";
  if (!input.localAlias) return "请选择本地连接别名";
  return null;
}

export function buildConnectTunnelName(
  targetVtcpUrl: string,
  localPort: string,
): string {
  try {
    const parsed = new URL(targetVtcpUrl);
    const segments = parsed.hostname.split(".");
    const service =
      segments.length >= 2 ? segments.slice(0, -1).join(".") : parsed.hostname;
    return `连接 ${service}:${parsed.port || localPort}`;
  } catch {
    return `连接端口 ${localPort}`;
  }
}

export function buildConnectTunnel(input: {
  targetVtcpUrl: string;
  localAlias: string;
  localPort: string;
  name: string;
}): TunnelInfo | null {
  const resolvedTarget = resolveTargetWithLocalAlias(
    input.targetVtcpUrl.trim(),
    input.localAlias,
  );
  if (!resolvedTarget) return null;
  return {
    ...emptyTunnel(),
    name: input.name,
    listen: `tcp://127.0.0.1:${input.localPort}`,
    target: resolvedTarget,
  };
}
