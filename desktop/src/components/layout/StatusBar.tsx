import { useAgentStore } from "@/stores/agent-store";
import { useProfileStore } from "@/stores/profile-store";
import { useUIStore } from "@/stores/ui-store";
import { buildSimpleSessionVM } from "@/lib/view-model/simple-session-vm";

function AdvancedStatusBar() {
  const { agentRunning, wsConnected, networks, services, streams } = useAgentStore();

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
    <div className="flex items-center gap-3 px-3 py-1 text-xs text-muted-foreground bg-card border-t shrink-0">
      <div className="flex items-center gap-1.5">
        <span
          className={`inline-block h-1.5 w-1.5 rounded-full ${
            agentRunning ? (wsConnected ? "bg-primary" : "bg-emerald-500") : "bg-zinc-400"
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
            {pendingServices > 0 && `/${pendingServices} pending`}/{services.length}
          </span>
          {activeStreams > 0 && (
            <>
              <span className="text-border">|</span>
              <span className="tabular-nums text-primary/80">{activeStreams} 连接</span>
            </>
          )}
        </>
      )}
    </div>
  );
}

function SimpleStatusBar() {
  const { agentRunning, wsConnected, networks, services, streams } = useAgentStore();
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
          : session.shareState === "degraded" || session.connectState === "error"
            ? "有问题待处理"
            : "待配置";

  const secondaryText = session.requiresRestart
    ? "新配置需重启后生效"
    : session.activeConnectionCount > 0
      ? `当前 ${session.activeConnectionCount} 个连接`
      : session.userFacingHints[0] ?? "已就绪";

  return (
    <div className="flex items-center gap-3 px-3 py-1 text-xs text-muted-foreground bg-card border-t shrink-0">
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
        <span className={agentRunning ? "text-foreground" : ""}>{statusText}</span>
      </div>
      <span className="text-border">|</span>
      <span className="truncate">{secondaryText}</span>
    </div>
  );
}

export function StatusBar() {
  const { uiMode } = useUIStore();
  return uiMode === "advanced" ? <AdvancedStatusBar /> : <SimpleStatusBar />;
}
