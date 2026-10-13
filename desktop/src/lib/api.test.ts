import { afterEach, describe, expect, it, vi } from "vitest";
import * as api from "./api";

function envelope(data: unknown) {
  return { ErrCode: 0, ErrMsg: "", Data: data };
}

function stubFetch(data: unknown) {
  const calls: { url: string; headers: Headers }[] = [];
  vi.stubGlobal(
    "fetch",
    vi.fn(async (url: string | URL | Request, init?: RequestInit) => {
      calls.push({ url: String(url), headers: new Headers(init?.headers) });
      return new Response(JSON.stringify(envelope(data)), { status: 200 });
    }),
  );
  return calls;
}

afterEach(() => {
  api.setEndpoint(null);
  vi.unstubAllGlobals();
});

describe("api client token 注入", () => {
  it("token 非空时所有请求携带 Authorization: Bearer", async () => {
    const calls = stubFetch([]);
    api.setEndpoint({ baseUrl: "http://127.0.0.1:1234", token: "secret-token" });

    await api.getNetworks();

    expect(calls).toHaveLength(1);
    expect(calls[0].url).toBe("http://127.0.0.1:1234/api/v1/networks");
    expect(calls[0].headers.get("Authorization")).toBe(
      "Bearer secret-token",
    );
  });

  it("token 为 null 时不携带 Authorization（standalone 开放模式）", async () => {
    const calls = stubFetch([]);
    api.setEndpoint({ baseUrl: "http://127.0.0.1:8080", token: null });

    await api.getNetworks();

    expect(calls[0].headers.get("Authorization")).toBeNull();
  });

  it("POST 请求同样注入 token 且保留原有头", async () => {
    const calls = stubFetch({ status: "ok", alias: "a" });
    api.setEndpoint({ baseUrl: "http://127.0.0.1:1234", token: "tok" });

    await api.addLink("a", "tcp://x@y:1");

    expect(calls[0].headers.get("Authorization")).toBe("Bearer tok");
    expect(calls[0].headers.get("Content-Type")).toBe("application/json");
  });

  it("stop 请求只携带 Authorization 头（确认头已废弃）", async () => {
    const calls = stubFetch({ status: "stopping" });
    api.setEndpoint({ baseUrl: "http://127.0.0.1:1234", token: "tok" });

    await api.stopAgent();

    expect([...calls[0].headers.keys()]).toEqual(["authorization"]);
    expect(calls[0].headers.get("Authorization")).toBe("Bearer tok");
  });

  it("setEndpoint(null) 清空凭证后不再携带 Authorization", async () => {
    const calls = stubFetch([]);
    api.setEndpoint({ baseUrl: "http://127.0.0.1:1234", token: "tok" });
    api.setEndpoint(null);

    await api.getNetworks();

    expect(calls[0].headers.get("Authorization")).toBeNull();
  });
});
