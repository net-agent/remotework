import { Button } from "@/components/ui/button";
import { StatusDot } from "@/components/shared/StatusDot";
import { ServiceRow } from "@/components/service/ServiceRow";
import { useAgentStore } from "@/stores/agent-store";
import { useUIStore } from "@/stores/ui-store";
import { ChevronRight, Pencil, Plus } from "lucide-react";
import type { NetworkStateDTO } from "@/lib/types";

function formatLatency(ms: number): string {
  if (ms <= 0) return "";
  if (ms < 1000) return `${ms}ms`;
  return `${(ms / 1000).toFixed(1)}s`;
}

const onlineStates = new Set(["online", "connected", "running"]);

export function NetworkCard({ network }: { network: NetworkStateDTO }) {
  const { services, streams } = useAgentStore();
  const { expandedNetwork, setExpandedNetwork, openNetworkForm, openServiceForm } =
    useUIStore();

  const expanded = expandedNetwork === network.name;
  const latency = formatLatency(network.aliveMs);
  const isOnline = onlineStates.has(network.state.toLowerCase());

  const configIndex = useAgentStore
    .getState()
    .networks.findIndex((n) => n.name === network.name);

  const networkStreams = streams.filter(
    (s) => s.network === network.name && !s.isClosed
  );

  return (
    <div
      className={`rounded-sm transition-colors ${
        expanded ? "bg-accent" : "hover:bg-accent/50"
      }`}
    >
      {/* Header — identical layout in both states */}
      <div
        className="flex items-center gap-2.5 px-3 py-2 cursor-pointer select-none"
        onClick={() => setExpandedNetwork(network.name)}
      >
        <StatusDot state={network.state} />
        <span
          className={`text-sm font-medium truncate ${
            isOnline ? "text-primary" : ""
          }`}
        >
          {network.name}
        </span>
        {network.domain && (
          <span className="text-xs text-muted-foreground truncate">
            {network.domain}
          </span>
        )}
        <span className="ml-auto flex items-center gap-2 shrink-0">
          {network.lastErr ? (
            <span className="text-xs text-destructive truncate max-w-[140px]">
              {network.lastErr}
            </span>
          ) : (
            <>
              <span className="text-xs text-muted-foreground font-mono">
                {network.address}
              </span>
              {latency && (
                <span className="text-xs text-muted-foreground tabular-nums">
                  {latency}
                </span>
              )}
            </>
          )}
          {expanded && (
            <Button
              variant="ghost"
              size="icon"
              className="h-6 w-6"
              onClick={(e) => {
                e.stopPropagation();
                openNetworkForm(configIndex);
              }}
            >
              <Pencil className="h-3.5 w-3.5" />
            </Button>
          )}
          <ChevronRight
            className={`h-4 w-4 text-muted-foreground/40 transition-transform duration-150 ${
              expanded ? "rotate-90" : ""
            }`}
          />
        </span>
      </div>

      {/* Expanded content */}
      {expanded && (
        <div className="mx-3 mb-2 pt-1.5 border-t border-border/60">
          <div>
            <div className="flex items-center justify-between mb-0.5">
              <span className="text-xs font-medium text-muted-foreground/70 uppercase tracking-wider">
                服务
              </span>
              <Button
                variant="ghost"
                size="sm"
                className="h-6 text-xs px-1.5"
                onClick={() => openServiceForm()}
              >
                <Plus className="h-3.5 w-3.5 mr-0.5" />
                添加
              </Button>
            </div>
            {services.length === 0 ? (
              <p className="text-xs text-muted-foreground py-2 text-center">
                暂无服务
              </p>
            ) : (
              <div>
                {services.map((svc) => (
                  <ServiceRow key={svc.id} service={svc} />
                ))}
              </div>
            )}
          </div>

          {/* Active connections */}
          {networkStreams.length > 0 && (
            <div className="pt-1">
              <span className="text-xs text-primary/80 font-medium tabular-nums">
                {networkStreams.length} 个活跃连接
              </span>
            </div>
          )}
        </div>
      )}
    </div>
  );
}
