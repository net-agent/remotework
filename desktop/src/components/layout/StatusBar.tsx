import { useState } from "react";
import { useAgentStore } from "@/stores/agent-store";
import { useProfileStore } from "@/stores/profile-store";
import { useUIStore } from "@/stores/ui-store";
import { buildSimpleSessionVM } from "@/lib/view-model/simple-session-vm";
import { useSimpleActions } from "@/lib/view-model/simple-actions";
import { Button } from "@/components/ui/button";
import { toast } from "sonner";

function AdvancedStatusBar() {
  const { agentRunning, wsConnected, networks, services, streams } =
    useAgentStore();

  const activeStreams = streams.filter((stream) => !stream.isClosed).length;
  const userNetworks = networks.filter((network) => network.protocol !== "");
  const onlineNetworks = userNetworks.filter((network) =>
    ["online", "connected", "running"].includes(network.state.toLowerCase()),
  ).length;
  const runningServices = services.filter((service) =>
    ["running", "online", "connected"].includes(service.status.toLowerCase()),
  ).length;
  const pendingServices = services.filter((service) =>
    ["pending", "starting", "init", "connecting"].includes(
      service.status.toLowerCase(),
    ),
  ).length;

  return (
    <div className="flex items-center gap-3 border-t bg-card px-3 py-1 text-xs text-muted-foreground shrink-0">
      <div className="flex items-center gap-1.5">
        <span
          className={`inline-block h-1.5 w-1.5 rounded-full ${
            agentRunning
              ? wsConnected
                ? "bg-primary"
                : "bg-emerald-500"
              : "bg-zinc-400"
          }`}
        />
        <span className={agentRunning && wsConnected ? "text-primary" : ""}>
          {agentRunning ? (wsConnected ? "已连接" : "已启动") : "未运行"}
        </span>
      </div>

      {agentRunning && (
        <>
          <span className="text-border">|</span>
          <span className="tabular-nums">
            网络 {onlineNetworks}/{userNetworks.length}
          </span>
          <span className="text-border">|</span>
          <span className="tabular-nums">
            服务 {runningServices}
            {pendingServices > 0 && `/${pendingServices} pending`}/
            {services.length}
          </span>
          {activeStreams > 0 && (
            <>
              <span className="text-border">|</span>
              <span className="tabular-nums text-primary/80">
                {activeStreams} 连接
              </span>
            </>
          )}
        </>
      )}
    </div>
  );
}

function SimpleConnectionStatusChip() {
  const [open, setOpen] = useState(false);
  const { agentRunning, wsConnected, networks, services, streams } =
    useAgentStore();
  const { currentConfig, needsRestart } = useProfileStore();
  const actions = useSimpleActions();

  const session = buildSimpleSessionVM({
    agentRunning,
    wsConnected,
    networks,
    services,
    streams,
    currentConfig,
    needsRestart,
  });

  const link = session.selectedShareLink;

  if (!link) {
    return null;
  }

  const handleCopy = async () => {
    try {
      await navigator.clipboard.writeText(link.url);
      toast.success("已复制连接信息");
    } catch (error) {
      toast.error(`复制失败: ${String(error)}`);
    }
  };

  return (
    <>
      <button
        type="button"
        className="flex items-center gap-1.5 rounded-md border border-transparent px-2 py-0.5 text-xs text-muted-foreground transition-colors hover:border-border hover:bg-background hover:text-foreground"
        onClick={() => setOpen(true)}
      >
        <span className="font-medium text-foreground">{link.alias}</span>
      </button>

      {open ? (
        <div className="absolute bottom-8 right-3 z-20 w-[360px] rounded-xl border bg-card p-4 shadow-lg">
          <div className="flex items-start justify-between gap-3">
            <div className="space-y-1">
              <h3 className="text-sm font-medium">连接信息</h3>
              <div className="text-sm font-medium text-foreground">
                {link.alias}
              </div>
              <div className="text-xs text-emerald-600 dark:text-emerald-400">
                {link.statusText}
              </div>
            </div>
            <Button size="sm" variant="ghost" onClick={() => setOpen(false)}>
              关闭
            </Button>
          </div>

          <div className="mt-3 rounded-lg border bg-background px-3 py-3">
            <div className="text-[11px] text-muted-foreground">
              完整连接信息
            </div>
            <div className="mt-1 break-all font-mono text-xs text-foreground">
              {link.url}
            </div>
          </div>

          <div className="mt-3 flex flex-wrap gap-2">
            <Button size="sm" onClick={() => void handleCopy()}>
              复制
            </Button>
            <Button
              size="sm"
              variant="outline"
              onClick={() => actions.editShareLink(link.alias)}
            >
              修改
            </Button>
          </div>
        </div>
      ) : null}
    </>
  );
}

function SimpleStatusBar() {
  const { agentRunning, wsConnected, networks, services, streams } =
    useAgentStore();
  const { currentConfig, needsRestart } = useProfileStore();

  const session = buildSimpleSessionVM({
    agentRunning,
    wsConnected,
    networks,
    services,
    streams,
    currentConfig,
    needsRestart,
  });

  const statusText = !agentRunning
    ? "未运行"
    : session.runtime.overallState === "recovering"
      ? "正在恢复"
      : session.runtime.overallState === "connecting"
        ? "连接中"
        : session.shareState === "ready" || session.connectState === "connected"
          ? "可连接"
          : session.shareState === "degraded" ||
              session.connectState === "error"
            ? "有问题待处理"
            : "待配置";

  const secondaryText = session.requiresRestart
    ? "新配置需重启后生效"
    : session.activeConnectionCount > 0
      ? `当前 ${session.activeConnectionCount} 个连接`
      : undefined;

  return (
    <div className="relative flex items-center gap-3 border-t bg-card px-3 py-1 text-xs text-muted-foreground shrink-0">
      <div className="flex items-center gap-1.5">
        <span
          className={`inline-block h-1.5 w-1.5 rounded-full ${
            !agentRunning
              ? "bg-zinc-400"
              : session.runtime.overallState === "degraded"
                ? "bg-amber-500"
                : session.runtime.overallState === "recovering"
                  ? "bg-sky-500"
                  : "bg-primary"
          }`}
        />
        <span className={agentRunning ? "text-foreground" : ""}>
          {statusText}
        </span>
      </div>
      {secondaryText ? (
        <>
          <span className="text-border">|</span>
          <span className="truncate">{secondaryText}</span>
        </>
      ) : null}
      <div className="ml-auto">
        <SimpleConnectionStatusChip />
      </div>
    </div>
  );
}

export function StatusBar() {
  const { uiMode } = useUIStore();
  return uiMode === "advanced" ? <AdvancedStatusBar /> : <SimpleStatusBar />;
}
