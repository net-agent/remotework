import { describe, expect, it, vi } from "vitest";
import { persistThenMount } from "./hot-mount";

describe("persistThenMount", () => {
  it("成功路径：先持久化后热挂载，不回滚", async () => {
    const calls: string[] = [];
    const persist = vi.fn(async () => {
      calls.push("persist");
    });
    const mount = vi.fn(async () => {
      calls.push("mount");
      return "mounted";
    });
    const rollback = vi.fn(async () => {
      calls.push("rollback");
    });

    const outcome = await persistThenMount({ persist, mount, rollback });

    expect(outcome).toEqual({ ok: true, value: "mounted" });
    expect(calls).toEqual(["persist", "mount"]);
    expect(rollback).not.toHaveBeenCalled();
  });

  it("热挂载失败：回滚 profile 并报告 mount 阶段失败", async () => {
    const calls: string[] = [];
    const persist = vi.fn(async () => {
      calls.push("persist");
    });
    const mountError = new Error("mount boom");
    const mount = vi.fn(async () => {
      calls.push("mount");
      throw mountError;
    });
    const rollback = vi.fn(async () => {
      calls.push("rollback");
    });

    const outcome = await persistThenMount({ persist, mount, rollback });

    expect(outcome).toEqual({
      ok: false,
      stage: "mount",
      error: mountError,
      rolledBack: true,
    });
    expect(calls).toEqual(["persist", "mount", "rollback"]);
  });

  it("热挂载失败且回滚失败：rolledBack=false，仍报告 mount 错误", async () => {
    const mountError = new Error("mount boom");
    const outcome = await persistThenMount({
      persist: async () => {},
      mount: async () => {
        throw mountError;
      },
      rollback: async () => {
        throw new Error("rollback boom");
      },
    });

    expect(outcome).toEqual({
      ok: false,
      stage: "mount",
      error: mountError,
      rolledBack: false,
    });
  });

  it("保存失败：不发起热挂载，也不需要回滚", async () => {
    const persistError = new Error("persist boom");
    const persist = vi.fn(async () => {
      throw persistError;
    });
    const mount = vi.fn(async () => "mounted");
    const rollback = vi.fn(async () => {});

    const outcome = await persistThenMount({ persist, mount, rollback });

    expect(outcome).toEqual({ ok: false, stage: "persist", error: persistError });
    expect(mount).not.toHaveBeenCalled();
    expect(rollback).not.toHaveBeenCalled();
  });
});
