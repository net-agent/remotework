import { useCallback, useRef } from "react";
import { invoke } from "@tauri-apps/api/core";
import { useAgentStore } from "@/stores/agent-store";
import { useProfileStore } from "@/stores/profile-store";

export function useSidecar() {
  const { setAgentRunning, setApiBaseUrl, fetchAll, reset } = useAgentStore();

  const startAgent = useCallback(
    async (profileName: string) => {
      const port = await invoke<number>("start_agent", {
        profileName,
      });
      const url = `http://127.0.0.1:${port}`;
      setApiBaseUrl(url);
      setAgentRunning(true);
      await fetchAll();
      return port;
    },
    [setAgentRunning, setApiBaseUrl, fetchAll]
  );

  const stopAgent = useCallback(async () => {
    await invoke("stop_agent");
    reset();
  }, [reset]);

  const restartAgent = useCallback(
    async (profileName: string) => {
      const port = await invoke<number>("restart_agent", {
        profileName,
      });
      const url = `http://127.0.0.1:${port}`;
      setApiBaseUrl(url);
      setAgentRunning(true);
      await fetchAll();
      return port;
    },
    [setAgentRunning, setApiBaseUrl, fetchAll]
  );

  const isRunning = useCallback(async () => {
    return invoke<boolean>("agent_running");
  }, []);

  return { startAgent, stopAgent, restartAgent, isRunning };
}

export function useStartup() {
  const { startAgent, isRunning } = useSidecar();
  const { loadProfiles, loadConfig } = useProfileStore();
  const { setAgentRunning, setApiBaseUrl, fetchAll } = useAgentStore();
  const booted = useRef(false);

  const boot = useCallback(async () => {
    if (booted.current) return;
    booted.current = true;
    try {
      // If agent is already running (e.g. UI reload), just reconnect
      const running = await isRunning();
      if (running) {
        const port = await invoke<number>("get_agent_port");
        if (port > 0) {
          const url = `http://127.0.0.1:${port}`;
          setApiBaseUrl(url);
          setAgentRunning(true);
          await fetchAll();
        }
        await loadProfiles();
        const { activeProfile: active } = useProfileStore.getState();
        if (active) await loadConfig(active);
        return;
      }

      // Normal first boot
      await loadProfiles();
      const { activeProfile: active, profiles } = useProfileStore.getState();
      if (active && profiles.some((p) => p.name === active)) {
        await loadConfig(active);
        await startAgent(active);
      }
    } catch (e) {
      console.error("Boot failed:", e);
    }
  }, [loadProfiles, loadConfig, startAgent, isRunning, setAgentRunning, setApiBaseUrl, fetchAll]);

  return { boot };
}
