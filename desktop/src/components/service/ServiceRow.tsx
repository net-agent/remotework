import { StatusDot } from "@/components/shared/StatusDot";
import type { ServiceStateDTO } from "@/lib/types";

export function ServiceRow({ service }: { service: ServiceStateDTO }) {
  return (
    <div className="flex items-center gap-2 py-1 px-1.5 rounded-sm hover:bg-accent/40 text-sm">
      <StatusDot state={service.status} className="h-1.5 w-1.5" />
      <span className="truncate font-medium">{service.name}</span>
      <span className="text-xs text-muted-foreground/60 shrink-0">
        {service.type}
      </span>
      <span className="text-xs text-muted-foreground font-mono truncate ml-auto">
        {service.listenURL}
        {service.targetURL ? ` → ${service.targetURL}` : ""}
      </span>
      {service.actives > 0 && (
        <span className="text-xs text-emerald-600 font-medium shrink-0 tabular-nums">
          {service.actives}
        </span>
      )}
    </div>
  );
}
