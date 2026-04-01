import type { ReactNode } from "react";
import { cn } from "@/lib/utils";

export type FeatureCardTheme = "violet" | "sky" | "emerald";
type BadgeState = "open" | "opening" | "error" | "closed";

const themeTokens: Record<
  FeatureCardTheme,
  {
    section: string;
    iconWrap: string;
    iconColor: string;
    addressBorder: string;
    addressBg: string;
    addressLabel: string;
    emptyBorder: string;
    emptyBg: string;
  }
> = {
  violet: {
    section: "border-violet-200/80 bg-gradient-to-b from-violet-50/60 to-violet-50/20 dark:border-violet-800/70 dark:from-violet-950/30 dark:to-violet-950/10",
    iconWrap: "bg-violet-100 dark:bg-violet-900/40",
    iconColor: "text-violet-600 dark:text-violet-400",
    addressBorder: "border-violet-200/60 dark:border-violet-800/50",
    addressBg: "bg-white/70 dark:bg-violet-950/20",
    addressLabel: "text-violet-500 dark:text-violet-400",
    emptyBorder: "border-violet-200/80 dark:border-violet-800/50",
    emptyBg: "bg-white/40 dark:bg-violet-950/10",
  },
  sky: {
    section: "border-sky-200/80 bg-gradient-to-b from-sky-50/60 to-sky-50/20 dark:border-sky-800/70 dark:from-sky-950/30 dark:to-sky-950/10",
    iconWrap: "bg-sky-100 dark:bg-sky-900/40",
    iconColor: "text-sky-600 dark:text-sky-400",
    addressBorder: "border-sky-200/60 dark:border-sky-800/50",
    addressBg: "bg-white/70 dark:bg-sky-950/20",
    addressLabel: "text-sky-500 dark:text-sky-400",
    emptyBorder: "border-sky-200/80 dark:border-sky-800/50",
    emptyBg: "bg-white/40 dark:bg-sky-950/10",
  },
  emerald: {
    section: "border-emerald-200/80 bg-gradient-to-b from-emerald-50/60 to-emerald-50/20 dark:border-emerald-800/70 dark:from-emerald-950/30 dark:to-emerald-950/10",
    iconWrap: "bg-emerald-100 dark:bg-emerald-900/40",
    iconColor: "text-emerald-600 dark:text-emerald-400",
    addressBorder: "border-emerald-200/60 dark:border-emerald-800/50",
    addressBg: "bg-white/70 dark:bg-emerald-950/20",
    addressLabel: "text-emerald-500 dark:text-emerald-400",
    emptyBorder: "border-emerald-200/80 dark:border-emerald-800/50",
    emptyBg: "bg-white/40 dark:bg-emerald-950/10",
  },
};

const badgeTokens: Record<BadgeState, string> = {
  open: "bg-emerald-100 text-emerald-700 dark:bg-emerald-950/40 dark:text-emerald-400",
  opening: "bg-amber-100 text-amber-700 dark:bg-amber-950/40 dark:text-amber-400",
  error: "bg-red-100 text-red-700 dark:bg-red-950/40 dark:text-red-400",
  closed: "bg-zinc-100 text-zinc-500 dark:bg-zinc-800 dark:text-zinc-400",
};

export interface FeatureCardAddressItem {
  label: string;
  url: string;
  onCopy: (url: string) => void;
}

export function FeatureCard({
  theme,
  icon,
  title,
  description,
  badge,
  errorText,
  headerAction,
  addresses,
  emptyText,
  footerAction,
  children,
}: {
  theme: FeatureCardTheme;
  icon: ReactNode;
  title: string;
  description: string;
  badge?: { text: string; state: BadgeState } | null;
  errorText?: string | null;
  headerAction?: ReactNode;
  addresses?: FeatureCardAddressItem[];
  emptyText?: string;
  footerAction?: ReactNode;
  children?: ReactNode;
}) {
  const t = themeTokens[theme];
  const hasAddresses = addresses && addresses.length > 0;
  const hasChildren = children != null;

  return (
    <section
      className={cn("flex flex-col gap-4 rounded-xl border p-4", t.section)}
    >
      {/* 标题行 */}
      <div className="flex items-start gap-3">
        <div
          className={cn(
            "flex h-9 w-9 shrink-0 items-center justify-center rounded-lg",
            t.iconWrap,
          )}
        >
          <span className={t.iconColor}>{icon}</span>
        </div>
        <div className="min-w-0 flex-1">
          <div className="flex flex-wrap items-center gap-2">
            <h4 className="text-sm font-semibold">{title}</h4>
            {badge ? (
              <span
                className={cn(
                  "shrink-0 rounded-full px-2 py-0.5 text-[11px] font-medium",
                  badgeTokens[badge.state],
                )}
              >
                {badge.text}
              </span>
            ) : null}
          </div>
          <p className="mt-0.5 text-xs text-muted-foreground">{description}</p>
        </div>
        {headerAction ? <div className="shrink-0">{headerAction}</div> : null}
      </div>

      {/* 错误 */}
      {errorText ? (
        <div className="rounded-lg border border-red-200 bg-red-50/60 px-3 py-2 text-xs text-destructive dark:border-red-900 dark:bg-red-950/20">
          {errorText}
        </div>
      ) : null}

      {/* 地址列表 */}
      {hasAddresses ? (
        addresses.map((addr) => (
          <div
            key={addr.url}
            className={cn(
              "rounded-lg border px-3 py-3",
              t.addressBorder,
              t.addressBg,
            )}
          >
            <div
              className={cn(
                "mb-1 text-[11px] font-medium uppercase tracking-wide",
                t.addressLabel,
              )}
            >
              {addr.label}
            </div>
            <button
              type="button"
              className="flex w-full items-center gap-2 text-left"
              onClick={() => addr.onCopy(addr.url)}
              title={addr.url}
            >
              <span className="min-w-0 flex-1 truncate font-mono text-xs text-foreground">
                {addr.url}
              </span>
              <svg
                xmlns="http://www.w3.org/2000/svg"
                width="14"
                height="14"
                viewBox="0 0 24 24"
                fill="none"
                stroke="currentColor"
                strokeWidth="2"
                strokeLinecap="round"
                strokeLinejoin="round"
                className="shrink-0 text-muted-foreground hover:text-foreground"
              >
                <rect width="14" height="14" x="8" y="8" rx="2" ry="2" />
                <path d="M4 16c-1.1 0-2-.9-2-2V4c0-1.1.9-2 2-2h10c1.1 0 2 .9 2 2" />
              </svg>
            </button>
          </div>
        ))
      ) : !hasChildren && emptyText ? (
        <div
          className={cn(
            "flex min-h-[80px] items-center justify-center rounded-lg border border-dashed px-3 py-6 text-center",
            t.emptyBorder,
            t.emptyBg,
          )}
        >
          <div className="text-xs text-muted-foreground">{emptyText}</div>
        </div>
      ) : null}

      {/* 中部自定义内容 */}
      {children}

      {/* 底部操作 */}
      {footerAction ? <div className="mt-auto">{footerAction}</div> : null}
    </section>
  );
}
