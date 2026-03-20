import type { ListeningPortDTO } from "@/lib/types";
import { cn } from "@/lib/utils";

export function SimpleListeningPortList({
  ports,
  selectedPorts,
  disabledPorts,
  onToggle,
}: {
  ports: ListeningPortDTO[];
  selectedPorts: ReadonlySet<number>;
  disabledPorts: ReadonlySet<number>;
  onToggle: (port: number) => void;
}) {
  if (ports.length === 0) {
    return null;
  }

  return (
    <div className="overflow-hidden rounded-lg border bg-background">
      <div className="max-h-72 overflow-y-auto">
        <div className="divide-y">
          {ports.map((item) => {
            const checked = selectedPorts.has(item.port);
            const disabled = disabledPorts.has(item.port);
            return (
              <label
                key={`${item.protocol}-${item.port}-${item.pid ?? "none"}`}
                className={cn(
                  "flex items-center gap-3 px-3 py-2 text-sm transition-colors",
                  disabled
                    ? "cursor-default bg-muted/20"
                    : "cursor-pointer hover:bg-muted/40",
                  checked ? "bg-muted/30" : "",
                )}
              >
                <input
                  type="checkbox"
                  checked={checked || disabled}
                  disabled={disabled}
                  onChange={() => onToggle(item.port)}
                  className="h-4 w-4 rounded border-input"
                />
                <span className="font-medium text-foreground">:{item.port}</span>
                <span className="min-w-0 truncate text-muted-foreground">
                  {item.processName?.trim() || "未知进程"}
                </span>
                {typeof item.pid === "number" ? (
                  <span className="shrink-0 text-xs text-muted-foreground">
                    PID {item.pid}
                  </span>
                ) : null}
                {disabled ? (
                  <span className="shrink-0 rounded-full bg-emerald-50 px-2 py-0.5 text-[11px] font-medium text-emerald-600 dark:bg-emerald-950/40 dark:text-emerald-400">
                    已开放
                  </span>
                ) : null}
              </label>
            );
          })}
        </div>
      </div>
    </div>
  );
}
