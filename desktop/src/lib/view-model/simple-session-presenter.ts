import type { SimpleConfigVM } from "@/lib/view-model/simple-config-vm";
import type { SimpleRuntimeVM } from "@/lib/view-model/simple-runtime-vm";
import { buildLocalPortStatusText } from "@/lib/simple-domain/share-service-rules";
import type {
  SimpleSessionDomainFacts,
  SimpleShareServiceRowFact,
} from "@/lib/simple-domain/simple-session-domain";
import type {
  SimpleSessionVM,
  SimpleShareServiceRow,
} from "@/lib/view-model/simple-session-vm";

export function buildShareLinkHint(input: {
  allLinkCount: number;
  availableLinkCount: number;
  mustCreateNewShareLink: boolean;
  savedNeedsCheckCount: number;
}) {
  if (input.mustCreateNewShareLink) {
    return input.allLinkCount === 0
      ? "先输入连接信息，才能继续。"
      : "当前连接需检查，请先修正或重新输入。";
  }

  return input.savedNeedsCheckCount > 0
    ? `当前使用 1 条连接，另有 ${input.savedNeedsCheckCount} 条待检查。`
    : "当前连接已就绪。";
}

export function buildUserFacingHints(input: {
  runtime: SimpleRuntimeVM;
  config: SimpleConfigVM;
}) {
  return [
    ...input.runtime.userFacingHints,
    ...(input.config.requiresRestart
      ? ["你有新配置尚未生效，重启后会按新配置运行"]
      : []),
    ...input.config.invalidItems,
    ...input.config.missingItems,
  ];
}

export function buildShareServiceRows(
  facts: SimpleShareServiceRowFact[],
  selectedAlias: string | null,
): SimpleShareServiceRow[] {
  return facts.map((fact) => {
    if (!fact.isOpen) {
      return {
        ...fact,
        statusText: selectedAlias ? "尚未开放" : "请先选择连接信息",
        primaryActionLabel: "开放",
        primaryActionKind: "open",
      } satisfies SimpleShareServiceRow;
    }

    if (fact.key === "local-port") {
      return {
        ...fact,
        statusText: buildLocalPortStatusText({
          state: fact.state,
          openCount: fact.openCount,
          hasRuntime: fact.hasRuntime,
        }),
        primaryActionLabel: "开放",
        primaryActionKind: "manage",
      } satisfies SimpleShareServiceRow;
    }

    const statusText = fact.lastErr
      ? "运行异常"
      : fact.state === "open"
        ? "已开放"
        : fact.state === "opening"
          ? "开放中"
          : "运行异常";

    return {
      ...fact,
      statusText,
      primaryActionLabel:
        fact.state === "open" ||
        fact.state === "opening" ||
        fact.state === "error"
          ? "关闭"
          : "开放",
      primaryActionKind:
        fact.state === "open" ||
        fact.state === "opening" ||
        fact.state === "error"
          ? "close"
          : "open",
    } satisfies SimpleShareServiceRow;
  });
}

export function buildSimpleSessionPresenter(input: {
  runtime: SimpleRuntimeVM;
  config: SimpleConfigVM;
  facts: SimpleSessionDomainFacts;
  shareServiceRows: SimpleShareServiceRow[];
}) {
  return {
    runtime: input.runtime,
    config: input.config,
    shareState:
      input.config.hasShareConfig && input.runtime.shareState === "ready"
        ? "ready"
        : input.config.invalidItems.length > 0
          ? "degraded"
          : "not_ready",
    connectState:
      input.config.hasConnectConfig || input.runtime.connectState !== "idle"
        ? input.runtime.connectState
        : "idle",
    activeConnectionCount: input.runtime.activeConnectionCount,
    requiresRestart: input.config.requiresRestart,
    hasUsableShareLinks: input.facts.hasUsableShareLinks,
    availableShareLinks: input.facts.availableShareLinks,
    mustCreateNewShareLink: input.facts.mustCreateNewShareLink,
    shareLinkHint: buildShareLinkHint({
      allLinkCount: input.config.allLinks.length,
      availableLinkCount: input.facts.availableShareLinks.length,
      mustCreateNewShareLink: input.facts.mustCreateNewShareLink,
      savedNeedsCheckCount: input.facts.savedNeedsCheckCount,
    }),
    userFacingHints: buildUserFacingHints({
      runtime: input.runtime,
      config: input.config,
    }),
    selectedShareLink: input.facts.selectedShareLink,
    shareServiceRows: input.shareServiceRows,
  } satisfies SimpleSessionVM;
}
