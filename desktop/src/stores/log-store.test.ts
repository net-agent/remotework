import { beforeEach, describe, expect, it } from "vitest";
import {
  filterEntries,
  matchesMinLevel,
  toLogEntryInput,
  useLogStore,
  type LogEntry,
} from "./log-store";

function makeEntry(level: LogEntry["level"], id: number): LogEntry {
  return {
    id,
    timestamp: "2026-10-13T10:00:00.000Z",
    level,
    source: "stdout",
    message: `msg-${id}`,
  };
}

describe("toLogEntryInput", () => {
  it("msg 存在时优先使用 msg，并透传 level/module/source", () => {
    const input = toLogEntryInput({
      source: "stdout",
      level: "warn",
      module: "sys",
      msg: "read config",
      line: '{"level":"WARN","msg":"read config","module":"sys"}',
    });
    expect(input.level).toBe("warn");
    expect(input.module).toBe("sys");
    expect(input.source).toBe("stdout");
    expect(input.message).toBe("read config");
    expect(input.timestamp).toBeTruthy();
  });

  it("msg 缺失时回退显示原始行", () => {
    const input = toLogEntryInput({
      source: "stderr",
      level: "error",
      line: "panic: runtime error",
    });
    expect(input.message).toBe("panic: runtime error");
    expect(input.module).toBeUndefined();
    expect(input.level).toBe("error");
  });
});

describe("matchesMinLevel / filterEntries", () => {
  it("按级别阈值过滤：info 档隐藏 debug", () => {
    expect(matchesMinLevel("debug", "info")).toBe(false);
    expect(matchesMinLevel("info", "info")).toBe(true);
    expect(matchesMinLevel("warn", "info")).toBe(true);
    expect(matchesMinLevel("error", "info")).toBe(true);
  });

  it("error 档只保留 error", () => {
    const entries = [
      makeEntry("debug", 1),
      makeEntry("info", 2),
      makeEntry("warn", 3),
      makeEntry("error", 4),
    ];
    expect(filterEntries(entries, "error").map((e) => e.level)).toEqual([
      "error",
    ]);
    expect(filterEntries(entries, "warn").map((e) => e.level)).toEqual([
      "warn",
      "error",
    ]);
    expect(filterEntries(entries, "debug")).toHaveLength(4);
  });
});

describe("log store", () => {
  beforeEach(() => {
    useLogStore.getState().clear();
  });

  it("addEntry 追加并递增 id", () => {
    const { addEntry } = useLogStore.getState();
    addEntry({
      timestamp: "t",
      level: "info",
      source: "stdout",
      message: "hello",
    });
    addEntry({
      timestamp: "t",
      level: "error",
      source: "stderr",
      message: "boom",
    });
    const { entries } = useLogStore.getState();
    expect(entries).toHaveLength(2);
    expect(entries[0].id).toBe(1);
    expect(entries[1].id).toBe(2);
  });

  it("超过 500 条时丢弃最旧条目（环形缓冲）", () => {
    const { addEntry } = useLogStore.getState();
    for (let i = 0; i < 510; i++) {
      addEntry({
        timestamp: "t",
        level: "info",
        source: "stdout",
        message: `m${i}`,
      });
    }
    const { entries } = useLogStore.getState();
    expect(entries).toHaveLength(500);
    expect(entries[0].message).toBe("m10");
    expect(entries[entries.length - 1].message).toBe("m509");
  });
});
