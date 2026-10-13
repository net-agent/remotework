// 热挂载双写编排（sidecar 协议 §8）：
// 1. 先经 Host 持久化到活动 profile（save_profile）；
// 2. 再调 sidecar POST /links|/tunnels 热挂载；
// 3. 第 2 步失败回滚第 1 步（重新保存旧值）；第 1 步失败则不发起热挂载。

export type HotMountOutcome<T> =
  | { ok: true; value: T }
  | { ok: false; stage: "persist"; error: unknown }
  | { ok: false; stage: "mount"; error: unknown; rolledBack: boolean };

export async function persistThenMount<T>(options: {
  persist: () => Promise<void>;
  mount: () => Promise<T>;
  rollback: () => Promise<void>;
}): Promise<HotMountOutcome<T>> {
  try {
    await options.persist();
  } catch (error) {
    return { ok: false, stage: "persist", error };
  }

  try {
    const value = await options.mount();
    return { ok: true, value };
  } catch (error) {
    try {
      await options.rollback();
      return { ok: false, stage: "mount", error, rolledBack: true };
    } catch {
      return { ok: false, stage: "mount", error, rolledBack: false };
    }
  }
}

export function errorMessage(error: unknown): string {
  return error instanceof Error ? error.message : String(error);
}
