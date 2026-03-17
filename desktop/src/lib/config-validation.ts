// Frontend config validation — mirrors Go configv2/validate.go rules.
// Catches obvious errors before save to prevent sidecar startup failures.

const ALLOWED_LINK_SCHEMES = ["tcp", "ws", "wss"];
const FORBIDDEN_TUNNEL_SCHEMES = ["http", "https", "ws", "wss"];

/**
 * Validate a link registration URL.
 * Returns null if valid, or an error message string.
 */
export function validateLinkURL(rawUrl: string): string | null {
  let u: URL;
  try {
    u = new URL(rawUrl);
  } catch {
    return "URL 格式无效";
  }

  const scheme = u.protocol.replace(":", "");
  if (!ALLOWED_LINK_SCHEMES.includes(scheme)) {
    return `不支持的协议 "${scheme}"，可用: ${ALLOWED_LINK_SCHEMES.join(", ")}`;
  }

  const as = u.searchParams.get("as");
  if (!as) {
    return "缺少必需参数 'as'（节点身份标识）";
  }

  const keepalive = u.searchParams.get("keepalive");
  if (keepalive !== null) {
    const val = parseInt(keepalive, 10);
    if (isNaN(val) || val < 1) {
      return `keepalive 必须是正整数，当前值: "${keepalive}"`;
    }
  }

  const via = u.searchParams.get("via");
  if (via) {
    const lastColon = via.lastIndexOf(":");
    if (lastColon < 0) {
      return `via 格式错误，应为 host.alias:port，当前: "${via}"`;
    }
    const host = via.substring(0, lastColon);
    const port = via.substring(lastColon + 1);
    if (!host || !port || !host.includes(".")) {
      return `via 格式错误，应为 host.alias:port，当前: "${via}"`;
    }
    if (isNaN(parseInt(port, 10))) {
      return `via 端口不是有效数字: "${port}"`;
    }
  }

  return null;
}

/**
 * Validate a tunnel endpoint URL (listen or target).
 * Returns null if valid, or an error message string.
 */
export function validateTunnelEndpoint(
  rawUrl: string,
  role: "listen" | "target",
): string | null {
  let u: URL;
  try {
    u = new URL(rawUrl);
  } catch {
    return `${role === "listen" ? "监听" : "目标"}地址格式无效`;
  }

  const scheme = u.protocol.replace(":", "");

  if (FORBIDDEN_TUNNEL_SCHEMES.includes(scheme)) {
    return `隧道端点不允许使用 "${scheme}" 协议`;
  }

  if (scheme === "vtcp") {
    const hostname = u.hostname;
    if (!hostname.includes(".")) {
      return `vtcp 地址须为 vhostname.alias 格式，当前: "${hostname}"`;
    }
    const lastDot = hostname.lastIndexOf(".");
    const vhostname = hostname.substring(0, lastDot);
    const alias = hostname.substring(lastDot + 1);
    if (!vhostname || !alias) {
      return `vtcp 地址 vhostname 或 alias 为空: "${hostname}"`;
    }
  }

  if (scheme !== "socks5" && !u.port) {
    return "缺少端口号";
  }

  if (u.searchParams.get("via")) {
    return "隧道端点不允许使用 via 参数（via 仅用于注册 URL）";
  }

  return null;
}

/**
 * Validate a complete tunnel config.
 * Returns null if valid, or an error message string.
 */
export function validateTunnel(
  tunnel: { name: string; listen: string; target: string; id: string },
  linkAliases: string[],
): string | null {
  if (!tunnel.name.trim()) {
    return "隧道名称不能为空";
  }
  if (!tunnel.listen.trim()) {
    return "监听地址不能为空";
  }
  if (!tunnel.target.trim()) {
    return "目标地址不能为空";
  }

  const listenErr = validateTunnelEndpoint(tunnel.listen, "listen");
  if (listenErr) return listenErr;

  const targetErr = validateTunnelEndpoint(tunnel.target, "target");
  if (targetErr) return targetErr;

  // Check vtcp alias references exist in links
  for (const [role, rawUrl] of [
    ["listen", tunnel.listen],
    ["target", tunnel.target],
  ] as const) {
    try {
      const u = new URL(rawUrl);
      if (u.protocol === "vtcp:") {
        const hostname = u.hostname;
        const lastDot = hostname.lastIndexOf(".");
        if (lastDot >= 0) {
          const alias = hostname.substring(lastDot + 1);
          if (alias && !linkAliases.includes(alias)) {
            return `${role === "listen" ? "监听" : "目标"}地址引用的链路别名 "${alias}" 不存在`;
          }
        }
      }
    } catch {
      // Already validated above
    }
  }

  return null;
}
