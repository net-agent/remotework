import { emptyTunnel, type AgentConfig, type TunnelInfo } from "@/lib/config-types";

export const DEFAULT_LOCAL_PORT = "8080";
const AUTHCODE_LENGTH = 6;
const AUTHCODE_ALPHABET = "ABCDEFGHJKLMNPQRSTUVWXYZ23456789";

export type SimpleSharePresetType = "socks5" | "local-port";

export function buildShareListen(alias: string, port: string) {
  return `vtcp://share.${alias}:${port}`;
}

export function buildLocalPortListen(input: {
  alias: string;
  asValue: string;
  port: number;
  authcode: string;
}) {
  return `vtcp://${input.asValue}.${input.alias}:${input.port}?authcode=${input.authcode}`;
}

export function buildSocks5Target(username: string, password: string) {
  if (!username && !password) {
    return "socks5://local";
  }

  const encodedUser = encodeURIComponent(username);
  const encodedPassword = encodeURIComponent(password);
  return `socks5://${encodedUser}:${encodedPassword}@local`;
}

export function buildPresetTunnel(input: {
  preset: SimpleSharePresetType;
  alias: string;
  name: string;
  port: string;
  localAddress: string;
  username: string;
  password: string;
}): TunnelInfo {
  const { preset, alias, name, port, localAddress, username, password } = input;
  const tunnel = emptyTunnel();

  if (preset === "socks5") {
    return {
      ...tunnel,
      name,
      listen: buildShareListen(alias, DEFAULT_LOCAL_PORT),
      target: buildSocks5Target(username, password),
    };
  }

  return {
    ...tunnel,
    name,
    listen: buildShareListen(alias, port),
    target: `tcp://${localAddress}:${port}`,
  };
}

export function buildLocalPortTunnel(input: {
  alias: string;
  asValue: string;
  authcode: string;
  port: number;
  localAddress: string;
}) {
  const tunnel = emptyTunnel();
  return {
    ...tunnel,
    name: `本地端口 ${input.port}`,
    listen: buildLocalPortListen({
      alias: input.alias,
      asValue: input.asValue,
      port: input.port,
      authcode: input.authcode,
    }),
    target: `tcp://${input.localAddress}:${input.port}`,
  } satisfies TunnelInfo;
}

export function getPresetMeta(preset: SimpleSharePresetType) {
  switch (preset) {
    case "socks5":
      return {
        title: "开放本地网络",
        description: "让对方通过这台电脑访问你的本地网络资源。",
        defaultName: "本地网络",
        saveLabel: "开放本地网络",
      };
    default:
      return {
        title: "开放本地端口",
        description: "筛选本机监听端口，支持多选并批量开放。",
        defaultName: "本地端口",
        saveLabel: "开放选中端口",
      };
  }
}

export function getDefaultName(preset: SimpleSharePresetType, port: string) {
  if (preset === "local-port" && port) {
    return `本地端口 ${port}`;
  }

  return getPresetMeta(preset).defaultName;
}

export function hasPartialCredentials(username: string, password: string) {
  return (
    (username.length > 0 && password.length === 0) ||
    (username.length === 0 && password.length > 0)
  );
}

export function validatePortInput(port: string) {
  const trimmedPort = port.trim();
  if (!trimmedPort) {
    return "请填写本地端口";
  }

  const portNumber = Number(trimmedPort);
  if (!Number.isInteger(portNumber) || portNumber < 1 || portNumber > 65535) {
    return "端口范围应为 1-65535";
  }

  return null;
}

function generateRandomAuthcode() {
  return Array.from({ length: AUTHCODE_LENGTH }, () => {
    const index = Math.floor(Math.random() * AUTHCODE_ALPHABET.length);
    return AUTHCODE_ALPHABET[index] ?? "A";
  }).join("");
}

export function ensureLinkAuthcode(input: {
  alias: string;
  currentConfig: AgentConfig;
}): {
  asValue: string;
  authcode: string;
  nextConfig: AgentConfig;
  updated: boolean;
} {
  const rawLink = input.currentConfig.links?.[input.alias];
  if (!rawLink) {
    throw new Error("未找到当前连接信息");
  }

  let parsed: URL;
  try {
    parsed = new URL(rawLink);
  } catch {
    throw new Error("当前连接信息格式无效");
  }

  const asValue = parsed.searchParams.get("as")?.trim() ?? "";
  if (!asValue) {
    throw new Error("当前连接信息缺少 as 参数");
  }

  let authcode = parsed.searchParams.get("authcode")?.trim() ?? "";
  let updated = false;

  if (!authcode) {
    authcode = generateRandomAuthcode();
    parsed.searchParams.set("authcode", authcode);
    updated = true;
  }

  if (!updated) {
    return {
      asValue,
      authcode,
      nextConfig: input.currentConfig,
      updated: false,
    };
  }

  return {
    asValue,
    authcode,
    updated: true,
    nextConfig: {
      ...input.currentConfig,
      links: {
        ...(input.currentConfig.links ?? {}),
        [input.alias]: parsed.toString(),
      },
    },
  };
}
