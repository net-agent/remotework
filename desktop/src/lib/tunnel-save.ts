import { toast } from "sonner";
import type { TunnelInfo } from "@/lib/config-types";
import { validateTunnel } from "@/lib/config-validation";
import { useAgentStore } from "@/stores/agent-store";
import type { useProfileStore } from "@/stores/profile-store";
import * as api from "@/lib/api";

export interface TunnelSaveContext {
  currentConfig: ReturnType<typeof useProfileStore.getState>["currentConfig"];
  setCurrentConfig: ReturnType<
    typeof useProfileStore.getState
  >["setCurrentConfig"];
  saveConfig: ReturnType<typeof useProfileStore.getState>["saveConfig"];
  activeProfile: ReturnType<typeof useProfileStore.getState>["activeProfile"];
  setNeedsRestart: ReturnType<
    typeof useProfileStore.getState
  >["setNeedsRestart"];
}

export interface TunnelBatchSaveResult {
  ok: boolean;
  addedCount: number;
  unchangedCount: number;
  skippedCount: number;
  removedCount: number;
  updatedCount: number;
  runtimeAppliedCount: number;
  runtimeFailedCount: number;
  needsRestart: boolean;
}

function buildLinkDomains(configLinks: Record<string, string>) {
  const runtimeNetworks = useAgentStore.getState().networks;
  const linkDomains: Record<string, string> = {};

  for (const [alias, url] of Object.entries(configLinks)) {
    if (!alias) continue;
    try {
      const asParam = new URL(url).searchParams.get("as");
      if (asParam) linkDomains[alias] = asParam;
    } catch {
      // ignore invalid URLs
    }
  }

  for (const network of runtimeNetworks) {
    if (network.name && network.domain && network.kind === "virtual") {
      linkDomains[network.name] = network.domain;
    }
  }

  return linkDomains;
}

function getTunnelDedupKey(tunnel: Pick<TunnelInfo, "listen" | "target">) {
  return `${tunnel.listen}__${tunnel.target}`;
}

function isSameTunnel(
  left: Pick<TunnelInfo, "name" | "listen" | "target">,
  right: Pick<TunnelInfo, "name" | "listen" | "target">,
) {
  return (
    left.name === right.name &&
    left.listen === right.listen &&
    left.target === right.target
  );
}

export function dedupeTunnelConfigs(tunnels: TunnelInfo[]) {
  const seen = new Set<string>();
  const unique: TunnelInfo[] = [];
  let skipped = 0;

  for (const tunnel of tunnels) {
    const key = getTunnelDedupKey(tunnel);
    if (seen.has(key)) {
      skipped += 1;
      continue;
    }
    seen.add(key);
    unique.push(tunnel);
  }

  return { unique, skipped };
}

export async function saveTunnelConfig(input: {
  tunnel: TunnelInfo;
  context: TunnelSaveContext;
  editingServiceIndex?: number | null;
}): Promise<boolean> {
  const { tunnel, context, editingServiceIndex = null } = input;
  const isEditing = editingServiceIndex !== null;
  const configLinks = context.currentConfig.links ?? {};
  const linkAliases = Object.keys(buildLinkDomains(configLinks));
  const tunnelErr = validateTunnel(tunnel, linkAliases);
  if (tunnelErr) {
    toast.error(tunnelErr);
    return false;
  }

  const nextConfig = { ...context.currentConfig };
  const tunnels = [...(nextConfig.tunnels ?? [])];
  let isNew = false;

  if (isEditing && editingServiceIndex !== null) {
    tunnels[editingServiceIndex] = tunnel;
  } else {
    tunnels.push(tunnel);
    isNew = true;
  }

  nextConfig.tunnels = tunnels;
  context.setCurrentConfig(nextConfig);
  if (context.activeProfile) {
    await context.saveConfig(context.activeProfile, nextConfig);
  }

  const agentRunning = useAgentStore.getState().agentRunning;
  if (isNew && agentRunning) {
    try {
      await api.addTunnel(tunnel);
      toast.success("隧道已添加并启动");
    } catch (error) {
      toast.warning(`已保存配置，但动态添加失败: ${error}`);
      context.setNeedsRestart();
    }
  } else if (isEditing && agentRunning) {
    toast.success("隧道已更新，重启后生效");
    context.setNeedsRestart();
  } else {
    toast.success(isEditing ? "隧道已更新" : "隧道已添加");
  }

  return true;
}

export async function saveTunnelConfigsBatch(input: {
  tunnels: TunnelInfo[];
  context: TunnelSaveContext;
  replaceFilter?: (tunnel: TunnelInfo) => boolean;
  successLabel?: string;
}): Promise<TunnelBatchSaveResult> {
  const {
    tunnels,
    context,
    replaceFilter,
    successLabel = "端口",
  } = input;

  const configLinks = context.currentConfig.links ?? {};
  const linkAliases = Object.keys(buildLinkDomains(configLinks));

  for (const tunnel of tunnels) {
    const tunnelErr = validateTunnel(tunnel, linkAliases);
    if (tunnelErr) {
      toast.error(tunnelErr);
      return {
        ok: false,
        addedCount: 0,
        unchangedCount: 0,
        skippedCount: 0,
        removedCount: 0,
        updatedCount: 0,
        runtimeAppliedCount: 0,
        runtimeFailedCount: 0,
        needsRestart: false,
      };
    }
  }

  const existingTunnels = [...(context.currentConfig.tunnels ?? [])];
  const keptTunnels = replaceFilter
    ? existingTunnels.filter((tunnel) => !replaceFilter(tunnel))
    : existingTunnels;
  const replaceGroup = replaceFilter
    ? existingTunnels.filter((tunnel) => replaceFilter(tunnel))
    : [];
  const { unique: uniqueCandidates, skipped: duplicateCandidateCount } =
    dedupeTunnelConfigs(tunnels);
  const keptByListen = new Map(keptTunnels.map((tunnel) => [tunnel.listen, tunnel]));
  const replaceByListen = new Map(
    replaceGroup.map((tunnel) => [tunnel.listen, tunnel]),
  );

  const nextGroup: TunnelInfo[] = [];
  let unchangedCount = 0;
  let skippedCount = duplicateCandidateCount;
  let addedCount = 0;
  let updatedCount = 0;

  for (const candidate of uniqueCandidates) {
    const existingKept = keptByListen.get(candidate.listen);
    if (existingKept) {
      if (isSameTunnel(existingKept, candidate)) {
        unchangedCount += 1;
      } else {
        skippedCount += 1;
      }
      continue;
    }

    const existingReplace = replaceByListen.get(candidate.listen);
    if (existingReplace && isSameTunnel(existingReplace, candidate)) {
      nextGroup.push(existingReplace);
      unchangedCount += 1;
      continue;
    }

    nextGroup.push(candidate);
    if (existingReplace) {
      updatedCount += 1;
    } else {
      addedCount += 1;
    }
  }

  const nextGroupKeys = new Set(nextGroup.map(getTunnelDedupKey));
  const removedCount = replaceGroup.filter(
    (tunnel) => !nextGroupKeys.has(getTunnelDedupKey(tunnel)),
  ).length;
  const nextConfig = {
    ...context.currentConfig,
    tunnels: [...keptTunnels, ...nextGroup],
  };

  context.setCurrentConfig(nextConfig);
  if (context.activeProfile) {
    await context.saveConfig(context.activeProfile, nextConfig);
  }

  const agentRunning = useAgentStore.getState().agentRunning;
  const needsRestart = agentRunning && (removedCount > 0 || updatedCount > 0);
  let runtimeAppliedCount = 0;
  let runtimeFailedCount = 0;

  if (agentRunning && !needsRestart) {
    const newTunnels = nextGroup.filter(
      (tunnel) => !replaceByListen.has(tunnel.listen),
    );

    for (const tunnel of newTunnels) {
      try {
        await api.addTunnel(tunnel);
        runtimeAppliedCount += 1;
      } catch {
        runtimeFailedCount += 1;
      }
    }
  }

  if (needsRestart || runtimeFailedCount > 0) {
    context.setNeedsRestart();
  }

  const summaryParts: string[] = [];
  if (addedCount > 0) {
    summaryParts.push(`成功开放 ${addedCount} 个${successLabel}`);
  }
  if (updatedCount > 0) {
    summaryParts.push(`更新 ${updatedCount} 个${successLabel}`);
  }
  if (removedCount > 0) {
    summaryParts.push(`关闭 ${removedCount} 个${successLabel}`);
  }
  if (unchangedCount > 0) {
    summaryParts.push(`跳过 ${unchangedCount} 个已存在${successLabel}`);
  }
  if (skippedCount > 0) {
    summaryParts.push(`忽略 ${skippedCount} 个重复项`);
  }
  if (runtimeFailedCount > 0) {
    summaryParts.push(`${runtimeFailedCount} 个需要重启后生效`);
  } else if (needsRestart) {
    summaryParts.push("更改需重启后生效");
  }

  if (summaryParts.length === 0) {
    toast.message(`未新增${successLabel}`);
  } else if (addedCount > 0 || updatedCount > 0 || removedCount > 0) {
    toast.success(summaryParts.join("，"));
  } else {
    toast.message(summaryParts.join("，"));
  }

  return {
    ok: true,
    addedCount,
    unchangedCount,
    skippedCount,
    removedCount,
    updatedCount,
    runtimeAppliedCount,
    runtimeFailedCount,
    needsRestart: needsRestart || runtimeFailedCount > 0,
  };
}

export async function removeTunnelConfig(input: {
  context: TunnelSaveContext;
  tunnelId?: string;
  configIndex?: number;
}): Promise<boolean> {
  const { context, tunnelId, configIndex } = input;
  const tunnels = [...(context.currentConfig.tunnels ?? [])];
  const resolvedIndex =
    tunnelId !== undefined
      ? tunnels.findIndex((tunnel) => tunnel.id === tunnelId)
      : (configIndex ?? -1);

  if (resolvedIndex < 0 || resolvedIndex >= tunnels.length) {
    toast.error("未找到要关闭的共享配置");
    return false;
  }

  const nextConfig = {
    ...context.currentConfig,
    tunnels: tunnels.filter((_, index) => index !== resolvedIndex),
  };

  context.setCurrentConfig(nextConfig);
  if (context.activeProfile) {
    await context.saveConfig(context.activeProfile, nextConfig);
  }

  const agentRunning = useAgentStore.getState().agentRunning;
  if (agentRunning) {
    context.setNeedsRestart();
    toast.success("共享已关闭，重启后生效");
  } else {
    toast.success("共享已关闭");
  }

  return true;
}
