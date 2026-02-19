import { Button } from "@/components/ui/button";
import { Plus } from "lucide-react";
import { NetworkCard } from "@/components/network/NetworkCard";
import { NetworkCardExpanded } from "@/components/network/NetworkCardExpanded";
import { EmptyState } from "@/components/network/EmptyState";
import { useAgentStore } from "@/stores/agent-store";
import { useUIStore } from "@/stores/ui-store";

export function MainPage() {
  const { networks, agentRunning } = useAgentStore();
  const { expandedNetwork, openNetworkForm, openServiceForm } = useUIStore();

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
    <div className="p-4 space-y-3">
      <div className="flex items-center justify-between">
        <span className="text-xs font-medium text-muted-foreground">
          网络 ({userNetworks.length})
        </span>
        <div className="flex gap-1">
          <Button
            variant="ghost"
            size="sm"
            className="h-7 text-xs"
            onClick={() => openServiceForm()}
          >
            <Plus className="h-3 w-3 mr-1" />
            服务
          </Button>
          <Button
            variant="ghost"
            size="sm"
            className="h-7 text-xs"
            onClick={() => openNetworkForm()}
          >
            <Plus className="h-3 w-3 mr-1" />
            网络
          </Button>
        </div>
      </div>
      {userNetworks.map((net) =>
        expandedNetwork === net.name ? (
          <NetworkCardExpanded key={net.name} network={net} />
        ) : (
          <NetworkCard key={net.name} network={net} />
        )
      )}
    </div>
  );
}
