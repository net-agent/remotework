import { useState } from "react";
import { Button } from "@/components/ui/button";
import { cn } from "@/lib/utils";
import { Copy, ChevronDown, X } from "lucide-react";
import type {
  SimpleShareServiceItem,
  SimpleShareServiceRow,
} from "@/lib/view-model/simple-session-vm";

function getToneClass(state: SimpleShareServiceRow["state"]) {
  switch (state) {
    case "open":
      return "text-emerald-600 dark:text-emerald-400";
    case "opening":
      return "text-amber-600 dark:text-amber-400";
    case "error":
      return "text-destructive";
    default:
      return "text-muted-foreground";
  }
}

function LocalPortMappingTable({
  items,
  onCopyAddress,
  onCloseItem,
}: {
  items: SimpleShareServiceItem[];
  onCopyAddress: (listenURL: string) => void;
  onCloseItem: (input: { tunnelId?: string; configIndex?: number }) => void;
}) {
  return (
    <div className="overflow-hidden rounded-lg border bg-background">
      <div className="grid grid-cols-[72px_minmax(0,1fr)_112px] gap-2 border-b px-2.5 py-2 text-[11px] text-muted-foreground">
        <div>端口</div>
        <div>映射地址</div>
        <div className="text-right">操作</div>
      </div>
      <div className="divide-y">
        {items.map((item) => (
          <div
            key={`${item.listenURL}-${item.tunnelId ?? item.configIndex ?? "unknown"}`}
            className="grid grid-cols-[72px_minmax(0,1fr)_112px] gap-2 px-2.5 py-2 text-xs"
          >
            <div className="font-medium text-foreground">
              {item.port ? `:${item.port}` : "未知端口"}
            </div>
            <div className="min-w-0 space-y-1">
              <button
                type="button"
                className="break-all font-mono text-left text-foreground underline-offset-2 hover:underline"
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

function ShareServiceAddressBlock({
  listenURL,
  targetURL,
  onCopyAddress,
}: {
  listenURL: string;
  targetURL: string | null;
  onCopyAddress: (listenURL: string) => void;
}) {
  return (
    <div className="rounded-lg border bg-background px-3 py-3 text-xs">
      <div className="space-y-1">
        <div className="text-[11px] text-muted-foreground">当前开放地址</div>
        <button
          type="button"
          className="break-all font-mono text-left text-foreground underline-offset-2 hover:underline"
          onClick={() => onCopyAddress(listenURL)}
        >
          {listenURL}
        </button>
      </div>
      {targetURL ? (
        <div className="mt-3 space-y-1">
          <div className="text-[11px] text-muted-foreground">映射到</div>
          <div className="break-all font-mono text-muted-foreground">
            {targetURL}
          </div>
        </div>
      ) : null}
    </div>
  );
}

export function SimpleShareServiceRowCard({
  row,
  disabled,
  onOpen,
  onClose,
  onCopyAddress,
  onCloseItem,
}: {
  row: SimpleShareServiceRow;
  disabled?: boolean;
  onOpen: () => void;
  onClose: () => void;
  onCopyAddress: (listenURL: string) => void;
  onCloseItem: (input: { tunnelId?: string; configIndex?: number }) => void;
}) {
  const [showLocalPortDetails, setShowLocalPortDetails] = useState(false);
  const hasDetails = Boolean(row.listenURL) && row.key !== "local-port";
  const localPortPreview = row.previewPorts
    .map((port) => `:${port}`)
    .join("、");

  return (
    <div className="py-2 first:pt-0 last:pb-0">
      <div className="flex flex-col gap-3 rounded-lg border bg-background px-3 py-3 md:flex-row md:items-center md:justify-between">
        <div className="min-w-0 flex-1 space-y-1">
          <div className="flex flex-wrap items-center gap-x-2 gap-y-1">
            <h4 className="text-sm font-medium">{row.title}</h4>
            <span
              className={cn("text-xs font-medium", getToneClass(row.state))}
            >
              {row.statusText}
            </span>
          </div>
          <p className="text-xs text-muted-foreground">
            {row.key === "local-port" && row.openCount > 0
              ? `已开放 ${row.openCount} 个端口${
                  localPortPreview ? `：${localPortPreview}` : ""
                }${row.extraPortCount > 0 ? `，另外 ${row.extraPortCount} 个` : ""}`
              : row.description}
          </p>
          {row.lastErr ? (
            <div className="text-xs text-destructive">{row.lastErr}</div>
          ) : null}
        </div>

        <div className="flex shrink-0 items-center gap-2 md:pl-4">
          {row.key === "local-port" && row.items.length > 0 ? (
            <Button
              size="sm"
              type="button"
              variant="ghost"
              onClick={() => setShowLocalPortDetails((value) => !value)}
            >
              <ChevronDown
                className={cn(
                  "mr-1 h-3.5 w-3.5 transition-transform",
                  showLocalPortDetails && "rotate-180",
                )}
              />
              {showLocalPortDetails ? "收起已开放端口" : "查看已开放端口"}
            </Button>
          ) : hasDetails ? (
            <Button
              size="sm"
              type="button"
              variant="ghost"
              onClick={() => {
                if (row.listenURL) {
                  onCopyAddress(row.listenURL);
                }
              }}
            >
              <ChevronDown className="mr-1 h-3.5 w-3.5" />
              复制地址
            </Button>
          ) : null}
          {row.primaryActionKind === "open" ? (
            <Button size="sm" onClick={onOpen} disabled={disabled}>
              {row.primaryActionLabel}
            </Button>
          ) : (
            <Button size="sm" variant="outline" onClick={onClose}>
              {row.primaryActionLabel}
            </Button>
          )}
        </div>
      </div>

      {row.key === "local-port" &&
      row.items.length > 0 &&
      showLocalPortDetails ? (
        <div className="mt-2">
          <LocalPortMappingTable
            items={row.items}
            onCopyAddress={onCopyAddress}
            onCloseItem={onCloseItem}
          />
        </div>
      ) : null}

      {row.key !== "local-port" && row.listenURL ? (
        <div className="mt-2">
          <ShareServiceAddressBlock
            listenURL={row.listenURL}
            targetURL={row.targetURL}
            onCopyAddress={onCopyAddress}
          />
        </div>
      ) : null}
    </div>
  );
}
