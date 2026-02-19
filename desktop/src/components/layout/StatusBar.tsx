import { useAgentStore } from "@/stores/agent-store";

export function StatusBar() {
  const { agentRunning, wsConnected, streams } = useAgentStore();
  const activeStreams = streams.filter((s) => !s.isClosed).length;

  return (
    <div className="flex items-center justify-between px-3 py-1 text-xs text-muted-foreground bg-card border-t shrink-0">
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
      {agentRunning && activeStreams > 0 && (
        <span className="tabular-nums text-primary/80">{activeStreams} 连接</span>
      )}
    </div>
  );
}
