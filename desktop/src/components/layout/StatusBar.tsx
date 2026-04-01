import { useState } from "react";
import { useAgentStore } from "@/stores/agent-store";
import { useProfileStore } from "@/stores/profile-store";
import { useUIStore } from "@/stores/ui-store";
import { buildSimpleRuntimeVM } from "@/lib/view-model/simple-runtime-vm";
import { buildSimpleSessionVM } from "@/lib/view-model/simple-session-vm";
import { useSimpleActions } from "@/lib/view-model/simple-actions";
import { Button } from "@/components/ui/button";
import { toast } from "sonner";

function ShareLinkChip() {
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
  if (!link) return null;

  const handleCopy = async () => {
    try {
      await navigator.clipboard.writeText(link.url);
      toast.success("已复制连接信息");
    } catch (error) {
      toast.error(`复制失败: ${String(error)}`);
    }
  };

  const statusColorClass =
    session.runtime.overallState === "degraded"
      ? "text-amber-600 dark:text-amber-400"
      : session.runtime.overallState === "recovering" ||
          session.runtime.overallState === "connecting"
        ? "text-sky-600 dark:text-sky-400"
        : "text-emerald-600 dark:text-emerald-400";

  return (
    <>
      <button
        type="button"
        className="rounded-md border border-transparent px-2 py-0.5 text-xs font-medium text-foreground transition-colors hover:border-border hover:bg-background"
        onClick={() => setOpen(true)}
      >
        {link.alias}
      </button>

      {open ? (
        <div className="absolute bottom-8 right-3 z-20 w-[360px] rounded-xl border bg-card p-4 shadow-lg">
          <div className="flex items-start justify-between gap-3">
            <div className="space-y-1">
              <h3 className="text-sm font-medium">连接信息</h3>
              <div className="text-sm font-medium text-foreground">
                {link.alias}
              </div>
              <div className={`text-xs ${statusColorClass}`}>
                {link.statusText}
              </div>
            </div>
            <Button size="sm" variant="ghost" onClick={() => setOpen(false)}>
              关闭
            </Button>
          </div>

          <div className="mt-3 rounded-lg border bg-background px-3 py-3">
            <div className="text-[11px] text-muted-foreground">完整连接信息</div>
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

export function StatusBar() {
  const { agentRunning, wsConnected, networks, services, streams } =
    useAgentStore();
  const { needsRestart } = useProfileStore();
  const { uiMode } = useUIStore();

  const runtime = buildSimpleRuntimeVM({
    agentRunning,
    wsConnected,
    networks,
    services,
    streams,
  });

  const {
    overallState,
    onlineNetworkCount,
    configuredNetworkCount,
    runningServiceCount,
    totalServiceCount,
    activeConnectionCount,
  } = runtime;

  const dotColor =
    overallState === "stopped"
      ? "bg-zinc-400"
      : overallState === "degraded"
        ? "bg-amber-500"
        : overallState === "ready"
          ? "bg-primary"
          : "bg-sky-500";

  const dotAnimation =
    overallState === "stopped"
      ? ""
      : overallState === "connecting" || overallState === "recovering"
        ? "animate-dot-blink"
        : overallState === "degraded"
          ? "animate-dot-breathe-fast"
          : "animate-dot-breathe-slow";

  const primaryText =
    overallState === "stopped"
      ? "未运行"
      : overallState === "recovering"
        ? "重连中"
        : overallState === "connecting"
          ? "初始化"
          : overallState === "degraded"
            ? "有异常"
            : "就绪";

  let secondaryText: string | undefined;
  if (overallState === "connecting") {
    if (onlineNetworkCount < configuredNetworkCount) {
      secondaryText = `网络 ${onlineNetworkCount}/${configuredNetworkCount}`;
    } else if (runningServiceCount < totalServiceCount) {
      secondaryText = `服务 ${runningServiceCount}/${totalServiceCount}`;
    }
  } else if (overallState === "ready") {
    if (needsRestart) {
      secondaryText = "配置待重启";
    } else if (activeConnectionCount > 0) {
      secondaryText = `${activeConnectionCount} 个连接`;
    }
  }

  return (
    <div className="relative flex items-center gap-3 border-t bg-card px-3 py-1 text-xs text-muted-foreground shrink-0">
      <div className="flex items-center gap-1.5">
        <span
          className={`inline-block h-1.5 w-1.5 rounded-full ${dotColor} ${dotAnimation}`}
        />
        <span className={overallState !== "stopped" ? "text-foreground" : ""}>
          {primaryText}
        </span>
      </div>
      {secondaryText ? (
        <>
          <span className="text-border">·</span>
          <span className="truncate">{secondaryText}</span>
        </>
      ) : null}
      {uiMode === "simple" && (
        <div className="ml-auto">
          <ShareLinkChip />
        </div>
      )}
    </div>
  );
}
