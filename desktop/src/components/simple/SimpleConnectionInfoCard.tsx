import { cn } from "@/lib/utils";
import type { SimpleConfigSummaryItem } from "@/lib/view-model/simple-config-vm";

export function SimpleConnectionInfoCard({
  title,
  description,
  items,
}: {
  title: string;
  description: string;
  items: SimpleConfigSummaryItem[];
}) {
  return (
    <div className="rounded-xl border bg-card p-4 shadow-sm">
      <div className="space-y-1">
        <h3 className="text-sm font-medium">{title}</h3>
        <p className="text-xs text-muted-foreground">{description}</p>
      </div>
      <div className="mt-4 grid gap-3 sm:grid-cols-2 xl:grid-cols-3">
        {items.map((item) => (
          <div
            key={item.title}
            className="rounded-lg border bg-background px-3 py-2"
          >
            <div className="text-[11px] text-muted-foreground">
              {item.title}
            </div>
            <div
              className={cn(
                "mt-1 text-sm font-medium",
                item.tone === "warning" && "text-amber-600 dark:text-amber-400",
              )}
            >
              {item.value}
            </div>
          </div>
        ))}
      </div>
    </div>
  );
}
