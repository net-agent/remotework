import { create } from "zustand";

export interface LogEntry {
  id: number;
  timestamp: string;
  level: string;
  source: "stdout" | "stderr";
  message: string;
}

const MAX_ENTRIES = 500;

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
