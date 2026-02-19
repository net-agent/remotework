import { Card, CardContent } from "@/components/ui/card";
import { Button } from "@/components/ui/button";
import { Separator } from "@/components/ui/separator";
import { StatusDot } from "@/components/shared/StatusDot";
import { ServiceRow } from "@/components/service/ServiceRow";
import { useAgentStore } from "@/stores/agent-store";
import { useUIStore } from "@/stores/ui-store";
import { Plus, Pencil } from "lucide-react";
import type { NetworkStateDTO } from "@/lib/types";

function formatLatency(ms: number): string {
  if (ms <= 0) return "-";
  if (ms < 1000) return `${ms}ms`;
  return `${(ms / 1000).toFixed(1)}s`;
}

export function NetworkCardExpanded({ network }: { network: NetworkStateDTO }) {
  const { services, streams } = useAgentStore();
  const { openNetworkForm, openServiceForm, setExpandedNetwork } = useUIStore();

  // Find services that belong to this network (by matching listenURL/targetURL containing the network name)
  // For now, show all services since the API doesn't directly link services to networks
  const networkStreams = streams.filter(
    (s) => s.network === network.name && !s.isClosed
  );

  // Find the index of this network in the config for editing
  const configIndex = useAgentStore
    .getState()
    .networks.findIndex((n) => n.name === network.name);

  return (
    <Card className="border-primary/30">
      <CardContent className="p-4">
        {/* Header */}
        <div
          className="flex items-center gap-3 cursor-pointer"
          onClick={() => setExpandedNetwork(network.name)}
        >
          <StatusDot state={network.state} />
          <div className="flex-1 min-w-0">
            <div className="flex items-center gap-2">
              <span className="font-medium text-sm">{network.name}</span>
              {network.domain && (
                <span className="text-xs text-muted-foreground">
                  {network.domain}
                </span>
              )}
            </div>
            <div className="flex items-center gap-3 mt-0.5 text-xs text-muted-foreground">
              <span>{network.address}</span>
              {network.state === "online" && (
                <span>{formatLatency(network.aliveMs)}</span>
              )}
            </div>
          </div>
          <Button
            variant="ghost"
            size="icon"
            className="h-7 w-7"
            onClick={(e) => {
              e.stopPropagation();
              openNetworkForm(configIndex);
            }}
          >
            <Pencil className="h-3.5 w-3.5" />
          </Button>
        </div>

        <Separator className="my-3" />

        {/* Services */}
        <div className="space-y-1">
          <div className="flex items-center justify-between mb-2">
            <span className="text-xs font-medium text-muted-foreground">
              服务
            </span>
            <Button
              variant="ghost"
              size="sm"
              className="h-6 text-xs"
              onClick={() => openServiceForm()}
            >
              <Plus className="h-3 w-3 mr-1" />
              添加
            </Button>
          </div>
          {services.length === 0 ? (
            <p className="text-xs text-muted-foreground py-2 text-center">
              暂无服务
            </p>
          ) : (
            services.map((svc) => <ServiceRow key={svc.id} service={svc} />)
          )}
        </div>

        {/* Active connections */}
        {networkStreams.length > 0 && (
          <>
            <Separator className="my-3" />
            <div className="text-xs text-muted-foreground">
              {networkStreams.length} 个活跃连接
            </div>
          </>
        )}
      </CardContent>
    </Card>
  );
}
