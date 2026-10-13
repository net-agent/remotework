import { useEffect } from "react";
import { platform } from "@/lib/platform";
import { toLogEntryInput, useLogStore } from "@/stores/log-store";

export function useSidecarLogs() {
  const addEntry = useLogStore((s) => s.addEntry);

  useEffect(() => {
    return platform.onSidecarLog((payload) => {
      addEntry(toLogEntryInput(payload));
    });
  }, [addEntry]);
}
