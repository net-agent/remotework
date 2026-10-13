import { create } from "zustand";
import type { SidecarLogLevel, SidecarLogPayload } from "@/lib/platform";

export type LogLevel = SidecarLogLevel;

export interface LogEntry {
  id: number;
  timestamp: string;
  level: LogLevel;
  source: "stdout" | "stderr";
  module?: string;
  message: string;
}

const MAX_ENTRIES = 500;

const LEVEL_ORDER: Record<LogLevel, number> = {
  debug: 0,
  info: 1,
  warn: 2,
  error: 3,
};

export function matchesMinLevel(level: LogLevel, minLevel: LogLevel): boolean {
  return LEVEL_ORDER[level] >= LEVEL_ORDER[minLevel];
}

export function filterEntries(
  entries: LogEntry[],
  minLevel: LogLevel,
): LogEntry[] {
  return entries.filter((entry) => matchesMinLevel(entry.level, minLevel));
}

// 结构化 sidecar-log 负载 → 日志条目；msg 缺失时回退原始行（协议 §5.1）
export function toLogEntryInput(
  payload: SidecarLogPayload,
): Omit<LogEntry, "id"> {
  return {
    timestamp: new Date().toISOString(),
    level: payload.level,
    source: payload.source,
    module: payload.module,
    message: payload.msg ?? payload.line,
  };
}

interface LogState {
  entries: LogEntry[];
  nextId: number;
  addEntry: (entry: Omit<LogEntry, "id">) => void;
  clear: () => void;
}

export const useLogStore = create<LogState>((set, get) => ({
  entries: [],
  nextId: 1,
  addEntry: (entry) => {
    const { entries, nextId } = get();
    const newEntries = [...entries, { ...entry, id: nextId }];
    if (newEntries.length > MAX_ENTRIES) {
      newEntries.splice(0, newEntries.length - MAX_ENTRIES);
    }
    set({ entries: newEntries, nextId: nextId + 1 });
  },
  clear: () => set({ entries: [], nextId: 1 }),
}));
