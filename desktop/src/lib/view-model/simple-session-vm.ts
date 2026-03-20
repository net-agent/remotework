import type { AgentConfig } from "@/lib/config-types";
import type {
  SimpleConfigVM,
  SimpleLinkOption,
} from "@/lib/view-model/simple-config-vm";
import type { SimpleRuntimeVM } from "@/lib/view-model/simple-runtime-vm";
import { buildSimpleConfigVM } from "@/lib/view-model/simple-config-vm";
import { buildSimpleRuntimeVM } from "@/lib/view-model/simple-runtime-vm";
import { projectUsableLinks } from "@/lib/simple-domain/link-rules";
import {
  buildSimpleSessionFacts,
  buildSimpleShareServiceRowFacts,
} from "@/lib/simple-domain/simple-session-domain";
import {
  buildShareServiceRows,
  buildSimpleSessionPresenter,
} from "@/lib/view-model/simple-session-presenter";
import type {
  NetworkStateDTO,
  ServiceStateDTO,
  StreamStateDTO,
} from "@/lib/types";
import type { SimpleShareServiceType } from "@/lib/simple-domain/share-service-rules";

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
  primaryActionLabel: string;
  primaryActionKind: "open" | "close" | "manage";
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

  const config = buildSimpleConfigVM(input.currentConfig, input.needsRestart);
  const runtimeUsableShareLinks = projectUsableLinks({
    links: config.validLinks,
    networks: input.networks,
    sidecarReady: Boolean(input.agentRunning && input.wsConnected),
  });

  const facts = buildSimpleSessionFacts({
    allLinks: config.allLinks,
    availableShareLinks: config.validLinks,
    runtimeUsableShareLinks,
    selectedShareLinkAlias: input.selectedShareLinkAlias,
  });
  const rowFacts = buildSimpleShareServiceRowFacts({
    selectedAlias: facts.selectedAlias,
    currentConfig: input.currentConfig,
    services: input.services,
  });
  const shareServiceRows = buildShareServiceRows(rowFacts, facts.selectedAlias);

  return buildSimpleSessionPresenter({
    runtime,
    config,
    facts,
    shareServiceRows,
  });
}
