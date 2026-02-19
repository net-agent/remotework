import { ChevronDown, ChevronRight } from "lucide-react";
import { Card, CardContent } from "@/components/ui/card";
import { StatusDot } from "@/components/shared/StatusDot";
import { useUIStore } from "@/stores/ui-store";
import type { NetworkStateDTO } from "@/lib/types";

function formatLatency(ms: number): string {
  if (ms <= 0) return "-";
  if (ms < 1000) return `${ms}ms`;
  return `${(ms / 1000).toFixed(1)}s`;
}

export function NetworkCard({ network }: { network: NetworkStateDTO }) {
  const { expandedNetwork, setExpandedNetwork } = useUIStore();
  const isExpanded = expandedNetwork === network.name;

  return (
    <Card
      className="cursor-pointer transition-colors hover:bg-accent/50"
      onClick={() => setExpandedNetwork(network.name)}
    >
      <CardContent className="p-4">
        <div className="flex items-center gap-3">
          <StatusDot state={network.state} />
          <div className="flex-1 min-w-0">
            <div className="flex items-center gap-2">
              <span className="font-medium text-sm truncate">
                {network.name}
              </span>
              {network.domain && (
                <span className="text-xs text-muted-foreground truncate">
                  {network.domain}
                </span>
              )}
            </div>
            <div className="flex items-center gap-3 mt-0.5 text-xs text-muted-foreground">
              <span>{network.address}</span>
              {network.state === "online" && (
                <span>{formatLatency(network.aliveMs)}</span>
              )}
              {network.lastErr && (
                <span className="text-destructive truncate">{network.lastErr}</span>
              )}
            </div>
          </div>
          {isExpanded ? (
            <ChevronDown className="h-4 w-4 text-muted-foreground shrink-0" />
          ) : (
            <ChevronRight className="h-4 w-4 text-muted-foreground shrink-0" />
          )}
        </div>
      </CardContent>
    </Card>
  );
}
