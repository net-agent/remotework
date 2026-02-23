import { useAgentStore } from "@/stores/agent-store";

export function StatusBar() {
  const { agentRunning, wsConnected, networks, services, streams } =
    useAgentStore();

  const activeStreams = streams.filter((s) => !s.isClosed).length;
  const userNetworks = networks.filter((n) => n.protocol !== "");
  const onlineNetworks = userNetworks.filter((n) =>
    ["online", "connected", "running"].includes(n.state.toLowerCase())
  ).length;
  const runningSvc = services.filter((s) =>
    ["running", "online", "connected"].includes(s.status.toLowerCase())
  ).length;
  const pendingSvc = services.filter((s) =>
    ["pending", "starting", "init", "connecting"].includes(s.status.toLowerCase())
  ).length;

  return (
    <div className="flex items-center gap-3 px-3 py-1 text-xs text-muted-foreground bg-card border-t shrink-0">
      {/* Agent status */}
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
          {agentRunning
            ? wsConnected
              ? "已连接"
              : "已启动"
            : "未运行"}
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
            服务 {runningSvc}{pendingSvc > 0 && `/${pendingSvc} pending`}/{services.length}
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
