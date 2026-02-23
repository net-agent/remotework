import { Button } from "@/components/ui/button";
import { Plus, Network } from "lucide-react";
import { NetworkCard } from "@/components/network/NetworkCard";
import { NetworkDetail } from "@/components/network/NetworkDetail";
import { useAgentStore } from "@/stores/agent-store";
import { useUIStore } from "@/stores/ui-store";

export function NetworksTab() {
  const { networks } = useAgentStore();
  const { selectedNetwork, openNetworkForm } = useUIStore();

  const userNetworks = networks.filter((n) => n.protocol !== "");

  if (userNetworks.length === 0) {
    return (
      <div className="flex flex-col items-center justify-center h-full gap-3 text-center px-8">
        <div className="rounded-full bg-accent/60 p-3">
          <Network className="h-6 w-6 text-muted-foreground" />
        </div>
        <div>
          <p className="text-sm font-medium mb-0.5">还没有网络</p>
          <p className="text-xs text-muted-foreground">添加网络连接来开始使用</p>
        </div>
        <Button size="sm" onClick={() => openNetworkForm()}>添加网络</Button>
      </div>
    );
  }

  const selected = userNetworks.find((n) => n.name === selectedNetwork) ?? null;

  return (
    <div className="flex h-full">
      {/* Left: list */}
      <div className="w-48 shrink-0 border-r border-border flex flex-col">
        <div className="flex items-center justify-between px-3 pt-3 pb-1.5">
          <span className="text-xs font-medium text-muted-foreground uppercase tracking-wider">
            网络 ({userNetworks.length})
          </span>
          <Button
            variant="ghost"
            size="sm"
            className="h-6 text-xs px-1.5"
            onClick={() => openNetworkForm()}
          >
            <Plus className="h-3.5 w-3.5" />
          </Button>
        </div>
        <div className="flex-1 overflow-auto px-1.5 py-1 space-y-0.5">
          {userNetworks.map((net) => (
            <NetworkCard key={net.name} network={net} />
          ))}
        </div>
      </div>

      {/* Right: detail */}
      <div className="flex-1 min-w-0 bg-card">
        {selected ? (
          <NetworkDetail network={selected} />
        ) : (
          <div className="flex items-center justify-center h-full text-sm text-muted-foreground">
            选择一个网络查看详情
          </div>
        )}
      </div>
    </div>
  );
}
