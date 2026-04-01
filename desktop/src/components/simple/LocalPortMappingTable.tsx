import { cn } from "@/lib/utils";
import { Copy, X } from "lucide-react";
import type { SimpleShareServiceItem } from "@/lib/view-model/simple-session-vm";

export function LocalPortMappingTable({
  items,
  isActive,
  onCopyAddress,
  onCloseItem,
}: {
  items: SimpleShareServiceItem[];
  isActive: boolean;
  onCopyAddress: (listenURL: string) => void;
  onCloseItem: (input: { tunnelId?: string; configIndex?: number }) => void;
}) {
  const frameClass = isActive
    ? "border-sky-300 bg-sky-50/70 dark:border-sky-700/80 dark:bg-sky-950/30"
    : "border-zinc-300 bg-zinc-50/70 dark:border-zinc-700/80 dark:bg-zinc-950/30";
  const headerClass = isActive
    ? "border-sky-200/90 dark:border-sky-800/70"
    : "border-zinc-200/90 dark:border-zinc-800/70";
  const rowClass = isActive
    ? "hover:bg-sky-100/40 dark:hover:bg-sky-900/20"
    : "hover:bg-zinc-100/50 dark:hover:bg-zinc-900/25";

  return (
    <div className={cn("overflow-hidden rounded-lg border", frameClass)}>
      <div
        className={cn(
          "grid grid-cols-[72px_minmax(0,1fr)_112px] gap-2 border-b px-2.5 py-2 text-[11px] text-muted-foreground",
          headerClass,
        )}
      >
        <div>端口</div>
        <div>映射地址</div>
        <div className="text-right">操作</div>
      </div>
      <div className="divide-y">
        {items.map((item) => (
          <div
            key={`${item.listenURL}-${item.tunnelId ?? item.configIndex ?? "unknown"}`}
            className={cn(
              "grid grid-cols-[72px_minmax(0,1fr)_112px] gap-2 px-2.5 py-2 text-xs",
              rowClass,
            )}
          >
            <div className="font-medium text-foreground">
              {item.port ? `:${item.port}` : "未知端口"}
            </div>
            <div className="min-w-0 space-y-1">
              <button
                type="button"
                className="w-full min-w-0 truncate font-mono text-left text-foreground underline-offset-2 hover:underline"
                title={item.listenURL}
                onClick={() => onCopyAddress(item.listenURL)}
              >
                {item.listenURL}
              </button>
              {item.lastErr ? (
                <div className="text-[11px] text-destructive">
                  {item.lastErr}
                </div>
              ) : null}
            </div>
            <div className="flex justify-end gap-1">
              <button
                type="button"
                className="inline-flex h-4 w-4 items-center justify-center text-muted-foreground transition-colors hover:text-foreground"
                onClick={() => onCopyAddress(item.listenURL)}
                aria-label="复制虚拟地址"
                title="复制虚拟地址"
              >
                <Copy className="h-3.5 w-3.5" />
              </button>
              <button
                type="button"
                className="inline-flex h-4 w-4 items-center justify-center text-muted-foreground transition-colors hover:text-destructive"
                onClick={() =>
                  onCloseItem({
                    tunnelId: item.tunnelId ?? undefined,
                    configIndex: item.configIndex ?? undefined,
                  })
                }
                aria-label="关闭映射"
                title="关闭映射"
              >
                <X className="h-3.5 w-3.5" />
              </button>
            </div>
          </div>
        ))}
      </div>
    </div>
  );
}
