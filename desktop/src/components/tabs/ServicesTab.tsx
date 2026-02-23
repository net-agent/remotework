import { Button } from "@/components/ui/button";
import { Plus, Plug } from "lucide-react";
import { ServiceRow } from "@/components/service/ServiceRow";
import { ServiceDetail } from "@/components/service/ServiceDetail";
import { useAgentStore } from "@/stores/agent-store";
import { useUIStore } from "@/stores/ui-store";

export function ServicesTab() {
  const { services } = useAgentStore();
  const { selectedService, openServiceForm } = useUIStore();

  if (services.length === 0) {
    return (
      <div className="flex flex-col items-center justify-center h-full gap-3 text-center px-8">
        <div className="rounded-full bg-accent/60 p-3">
          <Plug className="h-6 w-6 text-muted-foreground" />
        </div>
        <div>
          <p className="text-sm font-medium mb-0.5">还没有服务</p>
          <p className="text-xs text-muted-foreground">添加服务来转发端口流量</p>
        </div>
        <Button size="sm" onClick={() => openServiceForm()}>添加服务</Button>
      </div>
    );
  }

  const selected = services.find((s) => s.id === selectedService) ?? null;

  return (
    <div className="flex h-full">
      {/* Left: list */}
      <div className="w-48 shrink-0 border-r border-border flex flex-col">
        <div className="flex items-center justify-between px-3 pt-3 pb-1.5">
          <span className="text-xs font-medium text-muted-foreground uppercase tracking-wider">
            服务 ({services.length})
          </span>
          <Button
            variant="ghost"
            size="sm"
            className="h-6 text-xs px-1.5"
            onClick={() => openServiceForm()}
          >
            <Plus className="h-3.5 w-3.5" />
          </Button>
        </div>
        <div className="flex-1 overflow-auto px-1.5 py-1 space-y-0.5">
          {services.map((svc) => (
            <ServiceRow key={svc.id} service={svc} />
          ))}
        </div>
      </div>

      {/* Right: detail */}
      <div className="flex-1 min-w-0 bg-card">
        {selected ? (
          <ServiceDetail service={selected} />
        ) : (
          <div className="flex items-center justify-center h-full text-sm text-muted-foreground">
            选择一个服务查看详情
          </div>
        )}
      </div>
    </div>
  );
}
