import { useEffect } from "react";
import { listen } from "@tauri-apps/api/event";
import { useLogStore } from "@/stores/log-store";

interface SidecarLogPayload {
  source: "stdout" | "stderr";
  line: string;
}

// Format: "2026/02/18 21:40:25 INFO  [module] message key=value ..."
const linePattern = /^(\d{4}\/\d{2}\/\d{2} \d{2}:\d{2}:\d{2})\s+(DEBUG|INFO|WARN|ERROR)\s+(.*)$/;

function parseLine(line: string) {
  const m = line.match(linePattern);
  if (m) {
    return { timestamp: m[1], level: m[2], message: m[3] };
  }
  return { timestamp: new Date().toISOString().slice(0, 19).replace("T", " "), level: "INFO", message: line };
}

export function useSidecarLogs() {
  const addEntry = useLogStore((s) => s.addEntry);

  useEffect(() => {
    const unlisten = listen<SidecarLogPayload>("sidecar-log", (event) => {
      const { source, line: raw } = event.payload;
      const parsed = parseLine(raw.trim());
      addEntry({ ...parsed, source });
    });

    return () => {
      unlisten.then((fn) => fn());
    };
  }, [addEntry]);
}
