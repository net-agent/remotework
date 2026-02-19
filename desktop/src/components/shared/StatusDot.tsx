import { cn } from "@/lib/utils";

const stateColors: Record<string, string> = {
  online: "bg-emerald-500 shadow-[0_0_4px_rgba(16,185,129,0.6)]",
  connected: "bg-emerald-500 shadow-[0_0_4px_rgba(16,185,129,0.6)]",
  running: "bg-emerald-500 shadow-[0_0_4px_rgba(16,185,129,0.6)]",
  connecting: "bg-amber-500 animate-pulse",
  starting: "bg-amber-500 animate-pulse",
  init: "bg-amber-500 animate-pulse",
  pending: "bg-sky-400 animate-pulse",
  offline: "bg-zinc-400",
  stopped: "bg-zinc-400",
  uninit: "bg-zinc-300",
  error: "bg-red-500",
  "init failed": "bg-red-500",
};

export function StatusDot({
  state,
  className,
}: {
  state: string;
  className?: string;
}) {
  const color = stateColors[state.toLowerCase()] ?? "bg-zinc-400";
  return (
    <span
      className={cn("inline-block h-2 w-2 rounded-full shrink-0", color, className)}
    />
  );
}
