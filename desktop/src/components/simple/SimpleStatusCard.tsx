import type { ReactNode } from "react";
import { cn } from "@/lib/utils";

const toneClasses = {
  default: "border-border bg-card text-card-foreground",
  success:
    "border-emerald-200 bg-emerald-50 text-emerald-700 dark:border-emerald-900 dark:bg-emerald-950/30 dark:text-emerald-300",
  warning:
    "border-amber-200 bg-amber-50 text-amber-700 dark:border-amber-900 dark:bg-amber-950/30 dark:text-amber-300",
  danger:
    "border-red-200 bg-red-50 text-red-700 dark:border-red-900 dark:bg-red-950/30 dark:text-red-300",
  muted: "border-border bg-muted/40 text-muted-foreground",
} as const;

export function SimpleStatusCard({
  title,
  description,
  tone = "default",
  extra,
}: {
  title: string;
  description: string;
  tone?: keyof typeof toneClasses;
  extra?: ReactNode;
}) {
  return (
    <div className={cn("rounded-xl border p-4", toneClasses[tone])}>
      <div className="flex items-start justify-between gap-3">
        <div>
          <div className="text-sm font-medium">{title}</div>
          <div className="mt-1 text-xs opacity-90">{description}</div>
        </div>
        {extra}
      </div>
    </div>
  );
}
