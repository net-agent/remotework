import { useEffect, useRef } from "react";
import { Button } from "@/components/ui/button";
import { Trash2, ScrollText } from "lucide-react";
import { useLogStore } from "@/stores/log-store";

const levelColors: Record<string, string> = {
  ERROR: "text-red-500",
  WARN: "text-amber-500",
  DEBUG: "text-muted-foreground/50",
  INFO: "",
};

export function LogsTab() {
  const { entries, clear } = useLogStore();
  const bottomRef = useRef<HTMLDivElement>(null);
  const containerRef = useRef<HTMLDivElement>(null);
  const autoScroll = useRef(true);

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
  }, [entries.length]);

  if (entries.length === 0) {
    return (
      <div className="flex flex-col items-center justify-center h-full gap-3 text-center px-8">
        <div className="rounded-full bg-accent/60 p-3">
          <ScrollText className="h-6 w-6 text-muted-foreground" />
        </div>
        <div>
          <p className="text-sm font-medium mb-0.5">暂无日志</p>
          <p className="text-xs text-muted-foreground">启动 Agent 后日志将在此显示</p>
        </div>
      </div>
    );
  }

  return (
    <div className="flex flex-col h-full">
      <div className="flex items-center justify-between px-3 pt-3 pb-1.5 shrink-0">
        <span className="text-xs font-medium text-muted-foreground uppercase tracking-wider">
          日志 ({entries.length})
        </span>
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
      <div
        ref={containerRef}
        onScroll={handleScroll}
        className="flex-1 overflow-y-auto px-3 pb-2 font-mono text-xs leading-5"
      >
        {entries.map((entry) => (
          <div key={entry.id} className={`flex gap-2 ${levelColors[entry.level] ?? ""}`}>
            <span className="text-muted-foreground/60 shrink-0 select-none">
              {entry.timestamp.slice(11, 19)}
            </span>
            <span className={`shrink-0 w-12 ${levelColors[entry.level] ?? ""}`}>
              {entry.level}
            </span>
            <span className="break-all">{entry.message}</span>
          </div>
        ))}
        <div ref={bottomRef} />
      </div>
    </div>
  );
}
