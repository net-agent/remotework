import { useAgentStore } from "@/stores/agent-store";
import { Badge } from "@/components/ui/badge";

export function StatusBar() {
  const { agentRunning, wsConnected, streams } = useAgentStore();
  const activeStreams = streams.filter((s) => !s.isClosed).length;

  return (
    <div className="flex items-center justify-between border-t px-4 py-1.5 text-xs text-muted-foreground bg-card">
      <div className="flex items-center gap-2">
        <span
          className={`inline-block h-1.5 w-1.5 rounded-full ${
            agentRunning ? "bg-emerald-500" : "bg-zinc-400"
          }`}
        />
        <span>
          {agentRunning
            ? wsConnected
              ? "已连接"
              : "已启动"
            : "未运行"}
        </span>
      </div>
      {agentRunning && activeStreams > 0 && (
        <Badge variant="secondary" className="text-xs font-normal">
          {activeStreams} 个活跃连接
        </Badge>
      )}
    </div>
  );
}
