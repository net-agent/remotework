import { useEffect, useRef, useState } from "react";
import { Button } from "@/components/ui/button";
import { Trash2, ScrollText } from "lucide-react";
import {
  filterEntries,
  useLogStore,
  type LogLevel,
} from "@/stores/log-store";

const levelColors: Record<LogLevel, string> = {
  error: "text-red-500",
  warn: "text-amber-500",
  debug: "text-muted-foreground/50",
  info: "",
};

const LEVEL_FILTERS: LogLevel[] = ["debug", "info", "warn", "error"];

function formatTime(iso: string): string {
  const date = new Date(iso);
  return Number.isNaN(date.getTime())
    ? iso
    : date.toLocaleTimeString("en-GB", { hour12: false });
}

export function LogsTab() {
  const { entries, clear } = useLogStore();
  const [minLevel, setMinLevel] = useState<LogLevel>("info");
  const bottomRef = useRef<HTMLDivElement>(null);
  const containerRef = useRef<HTMLDivElement>(null);
  const autoScroll = useRef(true);

  const visibleEntries = filterEntries(entries, minLevel);

  // Track whether user is at bottom
  const handleScroll = () => {
    const el = containerRef.current;
    if (!el) return;
    const atBottom = el.scrollHeight - el.scrollTop - el.clientHeight < 40;
    autoScroll.current = atBottom;
  };

  // Auto-scroll when new entries arrive
  useEffect(() => {
    if (autoScroll.current) {
      bottomRef.current?.scrollIntoView({ behavior: "smooth" });
    }
  }, [visibleEntries.length]);

  return (
    <div className="flex flex-col h-full">
      <div className="flex items-center justify-between px-3 pt-3 pb-1.5 shrink-0">
        <span className="text-xs font-medium text-muted-foreground uppercase tracking-wider">
          日志 ({visibleEntries.length}/{entries.length})
        </span>
        <div className="flex items-center gap-2">
          <div className="flex items-center rounded-md border p-0.5">
            {LEVEL_FILTERS.map((level) => (
              <button
                key={level}
                type="button"
                onClick={() => setMinLevel(level)}
                className={`rounded px-1.5 py-0.5 text-[10px] uppercase ${
                  minLevel === level
                    ? "bg-accent text-foreground"
                    : "text-muted-foreground hover:text-foreground"
                }`}
              >
                {level}
              </button>
            ))}
          </div>
          <Button
            variant="ghost"
            size="sm"
            className="h-6 text-xs px-1.5"
            onClick={clear}
          >
            <Trash2 className="h-3.5 w-3.5 mr-0.5" />
            清除
          </Button>
        </div>
      </div>
      {visibleEntries.length === 0 ? (
        <div className="flex flex-col items-center justify-center flex-1 gap-3 text-center px-8">
          <div className="rounded-full bg-accent/60 p-3">
            <ScrollText className="h-6 w-6 text-muted-foreground" />
          </div>
          <div>
            <p className="text-sm font-medium mb-0.5">暂无日志</p>
            <p className="text-xs text-muted-foreground">
              {entries.length > 0
                ? "当前级别过滤下没有日志"
                : "启动 Agent 后日志将在此显示"}
            </p>
          </div>
        </div>
      ) : (
        <div
          ref={containerRef}
          onScroll={handleScroll}
          className="flex-1 overflow-y-auto px-3 pb-2 font-mono text-xs leading-5"
        >
          {visibleEntries.map((entry) => (
            <div
              key={entry.id}
              className={`flex gap-2 ${levelColors[entry.level] ?? ""}`}
            >
              <span className="text-muted-foreground/60 shrink-0 select-none">
                {formatTime(entry.timestamp)}
              </span>
              <span
                className={`shrink-0 w-12 uppercase ${levelColors[entry.level] ?? ""}`}
              >
                {entry.level}
              </span>
              <span className="break-all">
                {entry.module ? `[${entry.module}] ` : ""}
                {entry.message}
              </span>
            </div>
          ))}
          <div ref={bottomRef} />
        </div>
      )}
    </div>
  );
}
