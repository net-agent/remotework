import type { AgentConfig } from "@/lib/config-types";
import { validateLinkURL } from "@/lib/config-validation";
import type { NetworkStateDTO } from "@/lib/types";

export type SimpleLinkStatus = "usable" | "saved_needs_check";

export interface SimpleLinkOption {
  alias: string;
  url: string;
  status: SimpleLinkStatus;
  statusText: string;
}

export function extractAliasFromListen(listen: string) {
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

export function isUsableNetworkState(state: string) {
  return [
    "online",
    "connected",
    "running",
    "pending",
    "starting",
    "init",
    "connecting",
  ].includes(state.toLowerCase());
}

export function buildValidatedLinkOptions(config: AgentConfig) {
  const allLinks = Object.entries(config.links ?? {}).map(([alias, url]) => {
    const error = validateLinkURL(url);

    return {
      alias,
      url,
      status: error ? "saved_needs_check" : "usable",
      statusText: error ? "需检查" : "当前可用",
    } satisfies SimpleLinkOption;
  });

  return {
    allLinks,
    validLinks: allLinks.filter((item) => validateLinkURL(item.url) === null),
  };
}

export function projectUsableLinks(input: {
  links: SimpleLinkOption[];
  networks: NetworkStateDTO[];
  sidecarReady: boolean;
}) {
  const runtimeAliases = new Set(
    input.networks
      .filter(
        (network) =>
          network.protocol !== "" && isUsableNetworkState(network.state),
      )
      .map((network) => network.name),
  );

  const projectedLinks = input.links.map((link) => {
    const error = validateLinkURL(link.url);
    const isRuntimeUsable = runtimeAliases.has(link.alias);
    const status = error
      ? "saved_needs_check"
      : !input.sidecarReady || isRuntimeUsable
        ? "usable"
        : "saved_needs_check";

    return {
      ...link,
      status,
      statusText: status === "usable" ? "当前可用" : "需检查",
    } satisfies SimpleLinkOption;
  });

  return projectedLinks.filter((link) => link.status === "usable");
}
