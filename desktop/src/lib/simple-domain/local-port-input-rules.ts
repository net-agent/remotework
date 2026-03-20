import type { AgentConfig } from "@/lib/config-types";
import type { ListeningPortDTO } from "@/lib/types";
import {
  isManagedLocalPortTunnel,
  parseManualPorts,
  sortListeningPorts,
} from "@/lib/simple-domain/local-port-rules";

export function matchesListeningPort(item: ListeningPortDTO, keyword: string) {
  const normalized = keyword.trim().toLowerCase();
  if (!normalized) {
    return true;
  }

  return [
    String(item.port),
    item.processName?.toLowerCase() ?? "",
    item.pid !== undefined ? String(item.pid) : "",
  ].some((part) => part.includes(normalized));
}

export function buildOpenPortSet(config: AgentConfig, alias: string | null) {
  if (!alias) {
    return new Set<number>();
  }

  return new Set(
    (config.tunnels ?? [])
      .filter((tunnel) => isManagedLocalPortTunnel(tunnel, alias))
      .map((tunnel) => {
        try {
          return Number(new URL(tunnel.target).port);
        } catch {
          return NaN;
        }
      })
      .filter((value) => Number.isInteger(value) && value > 0),
  );
}

export function buildFilteredListeningPorts(input: {
  listeningPorts: ListeningPortDTO[];
  filterText: string;
  openPortSet: Set<number>;
}) {
  const matched = input.listeningPorts.filter((item) =>
    matchesListeningPort(item, input.filterText),
  );

  return sortListeningPorts(matched).sort((left, right) => {
    const leftOpen = input.openPortSet.has(left.port);
    const rightOpen = input.openPortSet.has(right.port);
    if (leftOpen !== rightOpen) {
      return leftOpen ? -1 : 1;
    }
    return 0;
  });
}

export function buildSelectedPortList(selectedPorts: Set<number>) {
  return Array.from(selectedPorts).sort((left, right) => left - right);
}

export function buildCombinedPorts(input: {
  selectedPortList: number[];
  manualPortsText: string;
}) {
  const manualPorts = parseManualPorts(input.manualPortsText);
  const combinedPorts = Array.from(
    new Set([...input.selectedPortList, ...manualPorts]),
  ).sort((left, right) => left - right);

  return { manualPorts, combinedPorts };
}

export function buildLocalPortSaveLabel(input: {
  selectedPortCount: number;
  manualPortsText: string;
}) {
  return `开放选中端口（${input.selectedPortCount}${input.manualPortsText.trim() ? "+手动" : ""}）`;
}
