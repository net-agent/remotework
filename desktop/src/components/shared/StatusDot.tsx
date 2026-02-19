import { cn } from "@/lib/utils";

const stateColors: Record<string, string> = {
  online: "bg-emerald-500",
  connected: "bg-emerald-500",
  running: "bg-emerald-500",
  connecting: "bg-amber-500",
  starting: "bg-amber-500",
  offline: "bg-zinc-400",
  stopped: "bg-zinc-400",
  error: "bg-red-500",
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
      className={cn("inline-block h-2.5 w-2.5 rounded-full shrink-0", color, className)}
    />
  );
}
