import { useCallback, useEffect, useRef } from "react";
import { toast } from "sonner";
import { platform } from "@/lib/platform";
import { useAgentStore } from "@/stores/agent-store";
import { useProfileStore } from "@/stores/profile-store";

// agent 启动/重启（或 UI 重连）后统一经 get_api_endpoint 取端口与 token（协议 §7.2），
// 不依赖 start_agent 的返回值。
async function connectEndpoint(
  setApiEndpoint: (ep: { baseUrl: string; token: string | null }) => void,
) {
  const { port, token } = await platform.getApiEndpoint();
  setApiEndpoint({ baseUrl: `http://127.0.0.1:${port}`, token });
  return port;
}

export function useSidecar() {
  const { setAgentRunning, setApiEndpoint, setStartError, fetchAll, reset } =
    useAgentStore();

  const startAgent = useCallback(
    async (profileName: string) => {
      setStartError("");
      try {
        await platform.startAgent(profileName);
        const port = await connectEndpoint(setApiEndpoint);
        setAgentRunning(true);
        await fetchAll();
        return port;
      } catch (e) {
        const msg = e instanceof Error ? e.message : String(e);
        setStartError(msg);
        throw e;
      }
    },
    [setAgentRunning, setApiEndpoint, setStartError, fetchAll],
  );

  const stopAgent = useCallback(async () => {
    await platform.stopAgent();
    reset();
  }, [reset]);

  const restartAgent = useCallback(
    async (profileName: string) => {
      setStartError("");
      try {
        await platform.restartAgent(profileName);
        const port = await connectEndpoint(setApiEndpoint);
        setAgentRunning(true);
        await fetchAll();
        return port;
      } catch (e) {
        const msg = e instanceof Error ? e.message : String(e);
        setStartError(msg);
        throw e;
      }
    },
    [setAgentRunning, setApiEndpoint, setStartError, fetchAll],
  );

  const isRunning = useCallback(async () => {
    return platform.agentRunning();
  }, []);

  return { startAgent, stopAgent, restartAgent, isRunning };
}

export function useStartup() {
  const { startAgent, isRunning } = useSidecar();
  const { loadProfiles, loadConfig } = useProfileStore();
  const { setAgentRunning, setApiEndpoint, fetchAll } = useAgentStore();
  const booted = useRef(false);

  const boot = useCallback(async () => {
    if (booted.current) return;
    booted.current = true;
    try {
      // If agent is already running (e.g. UI reload), just reconnect
      const running = await isRunning();
      if (running) {
        try {
          await connectEndpoint(setApiEndpoint);
          setAgentRunning(true);
          await fetchAll();
        } catch {
          // endpoint 不可用（如 agent 刚好退出）则按未运行处理，走正常启动流程
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
  }, [
    loadProfiles,
    loadConfig,
    startAgent,
    isRunning,
    setAgentRunning,
    setApiEndpoint,
    fetchAll,
  ]);

  return { boot };
}

// sidecar 进程退出感知：同步 running 状态；非预期且非零退出码视为崩溃（协议 §6.3）
export function useSidecarExit() {
  const reset = useAgentStore((s) => s.reset);

  useEffect(() => {
    return platform.onSidecarExited(({ code, intended }) => {
      reset();
      if (!intended && code !== 0) {
        toast.error(
          `Agent 异常退出（退出码 ${code ?? "未知"}），请到日志页查看详情`,
        );
      }
    });
  }, [reset]);
}
