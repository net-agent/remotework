import { Button } from "@/components/ui/button";
import { StatusDot } from "@/components/shared/StatusDot";
import { InfoRow } from "@/components/shared/InfoRow";
import { useUIStore } from "@/stores/ui-store";
import { useProfileStore } from "@/stores/profile-store";
import { Pencil } from "lucide-react";
import type { ServiceStateDTO } from "@/lib/types";

function findServiceConfigIndex(
  config: ReturnType<typeof useProfileStore.getState>["currentConfig"],
  service: ServiceStateDTO,
): number {
  const arr =
    service.type === "portproxy"
      ? config.portproxy
      : service.type === "socks5"
        ? config.socks5
        : service.type === "rdp"
          ? config.rdp
          : [];
  return arr.findIndex((item) => item.log === service.name);
}

export function ServiceDetail({ service }: { service: ServiceStateDTO }) {
  const { openServiceForm } = useUIStore();
  const { currentConfig } = useProfileStore();

  const configIndex = findServiceConfigIndex(currentConfig, service);

  return (
    <div className="p-4 space-y-3 overflow-auto h-full text-xs">
      <div className="flex items-center gap-2">
        <StatusDot state={service.status} />
        <span className="text-sm font-semibold">{service.name}</span>
        <span className="px-1.5 py-0.5 rounded bg-muted text-muted-foreground capitalize">
          {service.status}
        </span>
        {configIndex >= 0 && (
          <Button
            variant="ghost"
            size="icon"
            className="h-6 w-6 ml-auto"
            onClick={() =>
              openServiceForm(
                service.type as "portproxy" | "socks5" | "rdp",
                configIndex,
              )
            }
          >
            <Pencil className="h-3.5 w-3.5" />
          </Button>
        )}
      </div>

      {service.lastErr && (
        <div className="px-2.5 py-1.5 rounded bg-destructive/10 text-destructive">
          {service.lastErr}
        </div>
      )}

      <div className="rounded-md border border-border/60 px-3">
        <InfoRow label="类型" value={service.type} />
        <InfoRow label="监听地址" value={service.listenURL} mono />
        {service.targetURL && (
          <InfoRow label="目标地址" value={service.targetURL} mono />
        )}
        <InfoRow label="活跃连接" value={String(service.actives)} />
        <InfoRow label="已完成" value={String(service.dones)} />
      </div>
    </div>
  );
}
