import type { AgentConfig } from "@/lib/config-types";
import type { ServiceStateDTO } from "@/lib/types";
import type { SimpleLinkOption } from "@/lib/view-model/simple-config-vm";
import { extractAliasFromListen } from "@/lib/simple-domain/link-rules";
import {
  buildShareServiceItemState,
  findRuntimeService,
  isPendingStatus,
  isRunningStatus,
  parsePortFromUrl,
  resolveShareServiceType,
  type SimpleShareServiceType,
} from "@/lib/simple-domain/share-service-rules";

export interface SimpleSessionDomainMatch {
  tunnel: AgentConfig["tunnels"][number];
  configIndex: number;
  runtimeService: ServiceStateDTO | null;
  alias: string | null;
}

export interface SimpleSessionDomainFacts {
  availableShareLinks: SimpleLinkOption[];
  runtimeUsableShareLinks: SimpleLinkOption[];
  selectedShareLink: SimpleLinkOption | null;
  selectedAlias: string | null;
  savedNeedsCheckCount: number;
  mustCreateNewShareLink: boolean;
  hasUsableShareLinks: boolean;
}

export interface SimpleShareServiceItemFact {
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

export interface SimpleShareServiceRowFact {
  key: SimpleShareServiceType;
  title: string;
  description: string;
  isOpen: boolean;
  state: "closed" | "opening" | "open" | "error";
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
  items: SimpleShareServiceItemFact[];
  hasRuntime: boolean;
}

export function getSelectedShareLink(
  availableShareLinks: SimpleLinkOption[],
  selectedAlias?: string | null,
) {
  if (availableShareLinks.length === 0) {
    return null;
  }

  if (selectedAlias) {
    return (
      availableShareLinks.find((link) => link.alias === selectedAlias) ??
      availableShareLinks[0] ??
      null
    );
  }

  return availableShareLinks[0] ?? null;
}

export function buildSimpleSessionFacts(input: {
  allLinks: SimpleLinkOption[];
  availableShareLinks: SimpleLinkOption[];
  runtimeUsableShareLinks: SimpleLinkOption[];
  selectedShareLinkAlias?: string | null;
}) {
  const selectedShareLink = getSelectedShareLink(
    input.availableShareLinks,
    input.selectedShareLinkAlias,
  );
  const savedNeedsCheckCount =
    input.allLinks.length - input.availableShareLinks.length;

  return {
    availableShareLinks: input.availableShareLinks,
    runtimeUsableShareLinks: input.runtimeUsableShareLinks,
    selectedShareLink,
    selectedAlias: selectedShareLink?.alias ?? null,
    savedNeedsCheckCount,
    mustCreateNewShareLink:
      input.allLinks.length === 0 || input.availableShareLinks.length === 0,
    hasUsableShareLinks: input.availableShareLinks.length > 0,
  } satisfies SimpleSessionDomainFacts;
}

function buildSessionDomainMatches(input: {
  selectedAlias: string | null;
  currentConfig: AgentConfig;
  services: ServiceStateDTO[];
  key: SimpleShareServiceType;
}) {
  return (input.currentConfig.tunnels ?? [])
    .map((tunnel, configIndex) => {
      const runtimeService = findRuntimeService(tunnel, input.services);
      const alias = extractAliasFromListen(tunnel.listen);
      return {
        tunnel,
        configIndex,
        runtimeService,
        alias,
      } satisfies SimpleSessionDomainMatch;
    })
    .filter(
      (entry) =>
        entry.alias === input.selectedAlias &&
        resolveShareServiceType(entry.tunnel) === input.key,
    );
}

export function buildSimpleShareServiceRowFacts(input: {
  selectedAlias: string | null;
  currentConfig: AgentConfig;
  services: ServiceStateDTO[];
}) {
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

  return definitions.map((definition) => {
    const matches = buildSessionDomainMatches({
      selectedAlias: input.selectedAlias,
      currentConfig: input.currentConfig,
      services: input.services,
      key: definition.key,
    });

    if (matches.length === 0) {
      return {
        key: definition.key,
        title: definition.title,
        description: definition.description,
        isOpen: false,
        state: "closed",
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
        hasRuntime: false,
      } satisfies SimpleShareServiceRowFact;
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

    const rowState: SimpleShareServiceRowFact["state"] = hasError
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
        parsePortFromUrl(
          entry.runtimeService?.listenURL || entry.tunnel.listen,
        ),
      )
      .filter((port): port is string => Boolean(port))
      .slice(0, 3);
    const extraPortCount = Math.max(matches.length - previewPorts.length, 0);
    const primary = matches[0] ?? null;

    const items: SimpleShareServiceItemFact[] = matches.map((entry) => ({
      port: parsePortFromUrl(
        entry.runtimeService?.listenURL || entry.tunnel.listen,
      ),
      listenURL: entry.runtimeService?.listenURL || entry.tunnel.listen,
      targetURL: entry.runtimeService?.targetURL || entry.tunnel.target,
      tunnelId: entry.runtimeService?.tunnelId || entry.tunnel.id || null,
      configIndex: entry.configIndex,
      tunnelName: entry.tunnel.name || null,
      processName: null,
      pid: null,
      state: buildShareServiceItemState(entry.runtimeService),
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
        hasRuntime,
      } satisfies SimpleShareServiceRowFact;
    }

    const primaryRuntime = primary?.runtimeService ?? null;
    const primaryTunnel = primary?.tunnel ?? null;

    return {
      key: definition.key,
      title: definition.title,
      description: definition.description,
      isOpen: true,
      state: rowState,
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
      hasRuntime,
    } satisfies SimpleShareServiceRowFact;
  });
}
