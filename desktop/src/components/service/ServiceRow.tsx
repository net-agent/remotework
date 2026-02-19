import { Badge } from "@/components/ui/badge";
import { StatusDot } from "@/components/shared/StatusDot";
import type { ServiceStateDTO } from "@/lib/types";

export function ServiceRow({ service }: { service: ServiceStateDTO }) {
  return (
    <div className="flex items-center gap-2 py-1.5 px-1 rounded hover:bg-accent/50">
      <StatusDot state={service.status} className="h-2 w-2" />
      <div className="flex-1 min-w-0">
        <div className="flex items-center gap-2">
          <span className="text-sm truncate">{service.name}</span>
          <Badge variant="outline" className="text-[10px] px-1 py-0 font-normal">
            {service.type}
          </Badge>
        </div>
        <div className="text-xs text-muted-foreground truncate">
          {service.listenURL}
          {service.targetURL ? ` → ${service.targetURL}` : ""}
        </div>
      </div>
      {service.actives > 0 && (
        <span className="text-xs text-muted-foreground shrink-0">
          {service.actives}
        </span>
      )}
    </div>
  );
}
