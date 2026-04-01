import { cn } from "@/lib/utils";
import { Copy, X } from "lucide-react";
import type { ServiceStateDTO } from "@/lib/types";
import type { TunnelInfo } from "@/lib/config-types";

function matchService(
  tunnel: TunnelInfo,
  services: ServiceStateDTO[],
): ServiceStateDTO | null {
  if (tunnel.id) {
    const byId = services.find((s) => s.tunnelId === tunnel.id);
    if (byId) return byId;
  }
  return services.find((s) => s.name === tunnel.name) ?? null;
}

type StatusTone = "open" | "opening" | "error" | "closed";

function getTunnelStatusTone(
  service: ServiceStateDTO | null,
  agentRunning: boolean,
): StatusTone {
  if (!agentRunning || !service) return "closed";
  if (service.lastErr) return "error";
  const s = service.status.toLowerCase();
  if (["online", "connected", "running", "ready"].includes(s)) return "open";
  if (["pending", "starting", "init", "connecting"].includes(s))
    return "opening";
  return "error";
}

function getTunnelStatusText(tone: StatusTone, agentRunning: boolean) {
  if (!agentRunning) return "未启动";
  switch (tone) {
    case "open":
      return "已连接";
    case "opening":
      return "连接中";
    case "error":
      return "异常";
    default:
      return "等待生效";
  }
}

const toneTextClass: Record<StatusTone, string> = {
  open: "text-emerald-600 dark:text-emerald-400",
  opening: "text-amber-600 dark:text-amber-400",
  error: "text-destructive",
  closed: "text-muted-foreground",
};

export function ConnectMappingTable({
  tunnels,
  services,
  agentRunning,
  onCopy,
  onRemove,
}: {
  tunnels: TunnelInfo[];
  services: ServiceStateDTO[];
  agentRunning: boolean;
  onCopy: (url: string) => void;
  onRemove: (tunnel: TunnelInfo) => void;
}) {
  const headerBorder =
    "border-emerald-200/90 dark:border-emerald-800/70";
  const rowHover =
    "hover:bg-emerald-100/40 dark:hover:bg-emerald-900/20";
  const frameBorder =
    "border-emerald-300 bg-emerald-50/70 dark:border-emerald-700/80 dark:bg-emerald-950/30";

  return (
    <div className={cn("overflow-hidden rounded-lg border", frameBorder)}>
      <div
        className={cn(
          "grid grid-cols-[64px_minmax(0,1fr)_minmax(0,1fr)_80px] gap-2 border-b px-2.5 py-2 text-[11px] text-muted-foreground",
          headerBorder,
        )}
      >
        <div>本地端口</div>
        <div>目标地址</div>
        <div>名称</div>
        <div className="text-right">操作</div>
      </div>
      <div className="divide-y">
        {tunnels.map((tunnel) => {
          const service = matchService(tunnel, services);
          const tone = getTunnelStatusTone(service, agentRunning);
          const statusText = getTunnelStatusText(tone, agentRunning);
          const port = tunnel.listen.split(":").pop() ?? "";

          return (
            <div
              key={tunnel.id || tunnel.listen}
              className={cn(
                "grid grid-cols-[64px_minmax(0,1fr)_minmax(0,1fr)_80px] gap-2 px-2.5 py-2 text-xs",
                rowHover,
              )}
            >
              <div className="flex items-center gap-1.5">
                <span className="font-medium text-foreground">:{port}</span>
                <span className={cn("text-[11px]", toneTextClass[tone])}>
                  {statusText}
                </span>
              </div>
              <div className="min-w-0">
                <button
                  type="button"
                  className="w-full truncate text-left font-mono text-foreground underline-offset-2 hover:underline"
                  title={tunnel.target}
                  onClick={() => onCopy(tunnel.target)}
                >
                  {tunnel.target}
                </button>
                {service?.lastErr ? (
                  <div className="mt-0.5 text-[11px] text-destructive">
                    {service.lastErr}
                  </div>
                ) : null}
              </div>
              <div className="min-w-0 truncate text-muted-foreground">
                {tunnel.name || "—"}
              </div>
              <div className="flex items-center justify-end gap-1">
                <button
                  type="button"
                  className="inline-flex h-4 w-4 items-center justify-center text-muted-foreground transition-colors hover:text-foreground"
                  title="复制本地地址"
                  onClick={() => onCopy(tunnel.listen)}
                >
                  <Copy className="h-3.5 w-3.5" />
                </button>
                <button
                  type="button"
                  className="inline-flex h-4 w-4 items-center justify-center text-muted-foreground transition-colors hover:text-destructive"
                  title="删除"
                  onClick={() => onRemove(tunnel)}
                >
                  <X className="h-3.5 w-3.5" />
                </button>
              </div>
            </div>
          );
        })}
      </div>
    </div>
  );
}
