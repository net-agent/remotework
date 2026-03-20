import type { TunnelInfo } from "@/lib/config-types";
import type { ListeningPortDTO } from "@/lib/types";
import { extractAliasFromListen } from "@/lib/simple-domain/link-rules";

const FORBIDDEN_LOCAL_PORT_SCHEMES = [
  "tcp",
  "tcp4",
  "tcp6",
  "http",
  "https",
  "ws",
  "wss",
];

export function parsePortFromUrl(value: string) {
  try {
    const parsed = new URL(value);
    return parsed.port || null;
  } catch {
    return null;
  }
}

export function parseManualPorts(text: string) {
  const tokens = text
    .split(/[，,\s]+/)
    .map((item) => item.trim())
    .filter(Boolean);

  if (tokens.length === 0) {
    return [];
  }

  const values = tokens.map((token) => {
    const value = Number(token);
    if (!Number.isInteger(value) || value < 1 || value > 65535) {
      throw new Error(`端口 ${token} 不合法，范围应为 1-65535`);
    }
    return value;
  });

  return Array.from(new Set(values)).sort((left, right) => left - right);
}

export function isLocalPortTunnel(tunnel: TunnelInfo) {
  const target = tunnel.target.toLowerCase();

  if (!target.startsWith("tcp://")) {
    return false;
  }

  try {
    const listen = new URL(tunnel.listen);
    const scheme = listen.protocol.replace(":", "").toLowerCase();
    return !FORBIDDEN_LOCAL_PORT_SCHEMES.includes(scheme);
  } catch {
    return false;
  }
}

export function isManagedLocalPortTunnel(tunnel: TunnelInfo, alias: string) {
  return (
    extractAliasFromListen(tunnel.listen) === alias && isLocalPortTunnel(tunnel)
  );
}

export function sortListeningPorts(items: ListeningPortDTO[]) {
  return [...items].sort((left, right) => {
    if (left.port === 3389 && right.port !== 3389) {
      return -1;
    }
    if (left.port !== 3389 && right.port === 3389) {
      return 1;
    }
    return left.port - right.port;
  });
}
