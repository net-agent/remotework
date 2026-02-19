import { Button } from "@/components/ui/button";
import { Plus } from "lucide-react";
import { NetworkCard } from "@/components/network/NetworkCard";
import { EmptyState } from "@/components/network/EmptyState";
import { useAgentStore } from "@/stores/agent-store";
import { useUIStore } from "@/stores/ui-store";

export function MainPage() {
  const { networks, agentRunning } = useAgentStore();
  const { openNetworkForm, openServiceForm } = useUIStore();

  // Filter out built-in networks (tcp, tcp4, tcp6 — those with empty protocol)
  const userNetworks = networks.filter((n) => n.protocol !== "");

  if (!agentRunning) {
    return (
      <div className="flex flex-col items-center justify-center h-full gap-2 text-center px-8">
        <p className="text-sm text-muted-foreground">
          Agent 未运行，请选择一个 Profile 启动
        </p>
      </div>
    );
  }

  if (userNetworks.length === 0) {
    return <EmptyState />;
  }

  return (
    <div className="p-3 space-y-0.5">
      <div className="flex items-center justify-between px-0.5">
        <span className="text-xs font-medium text-muted-foreground uppercase tracking-wider">
          网络 ({userNetworks.length})
        </span>
        <div className="flex gap-0.5">
          <Button
            variant="ghost"
            size="sm"
            className="h-6 text-xs px-1.5"
            onClick={() => openServiceForm()}
          >
            <Plus className="h-3.5 w-3.5 mr-0.5" />
            服务
          </Button>
          <Button
            variant="ghost"
            size="sm"
            className="h-6 text-xs px-1.5"
            onClick={() => openNetworkForm()}
          >
            <Plus className="h-3.5 w-3.5 mr-0.5" />
            网络
          </Button>
        </div>
      </div>
      {userNetworks.map((net) => (
        <NetworkCard key={net.name} network={net} />
      ))}
    </div>
  );
}
