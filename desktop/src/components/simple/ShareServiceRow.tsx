import { useState } from "react";
import { Button } from "@/components/ui/button";
import { cn } from "@/lib/utils";
import { ChevronDown } from "lucide-react";
import type { SimpleShareServiceRow } from "@/lib/view-model/simple-session-vm";
import { LocalPortMappingTable } from "@/components/simple/LocalPortMappingTable";

function getRowFrameClass(
  key: SimpleShareServiceRow["key"],
  state: SimpleShareServiceRow["state"],
) {
  if (state === "closed") {
    return "border-zinc-300 bg-zinc-50/70 dark:border-zinc-700/80 dark:bg-zinc-950/30";
  }

  if (key === "local-port") {
    return "border-sky-300 bg-sky-50/70 dark:border-sky-700/80 dark:bg-sky-950/30";
  }

  return "border-violet-300 bg-violet-50/70 dark:border-violet-700/80 dark:bg-violet-950/30";
}

function getOpenButtonClass(key: SimpleShareServiceRow["key"]) {
  if (key === "local-port") {
    return "bg-sky-600 text-white hover:bg-sky-500 dark:bg-sky-600 dark:hover:bg-sky-500";
  }

  return "bg-violet-600 text-white hover:bg-violet-500 dark:bg-violet-600 dark:hover:bg-violet-500";
}

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

export function ShareServiceRowCard({
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
  const [showLocalPortDetails, setShowLocalPortDetails] = useState(true);
  const hasDetails = Boolean(row.listenURL) && row.key !== "local-port";
  const localPortPreview = row.previewPorts
    .map((port) => `:${port}`)
    .join("、");

  return (
    <div className="space-y-2">
      <div
        className={cn(
          "flex flex-col gap-2 rounded-lg border px-3 py-3 md:flex-row md:items-center md:justify-between",
          getRowFrameClass(row.key, row.state),
        )}
      >
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
            <Button
              size="sm"
              onClick={onOpen}
              disabled={disabled}
              className={getOpenButtonClass(row.key)}
            >
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
        <LocalPortMappingTable
          items={row.items}
          isActive={row.state !== "closed"}
          onCopyAddress={onCopyAddress}
          onCloseItem={onCloseItem}
        />
      ) : null}

      {row.key !== "local-port" && row.listenURL ? (
        <ShareServiceAddressBlock
          listenURL={row.listenURL}
          targetURL={row.targetURL}
          onCopyAddress={onCopyAddress}
        />
      ) : null}
    </div>
  );
}
