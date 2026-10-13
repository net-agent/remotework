import { describe, expect, it } from "vitest";
import { buildWsUrl } from "./ws";

describe("buildWsUrl", () => {
  it("token 非空时追加 ?token= 查询参数", () => {
    expect(buildWsUrl("http://127.0.0.1:1234", "secret")).toBe(
      "ws://127.0.0.1:1234/api/v1/ws?token=secret",
    );
  });

  it("token 为 null/空串时不带查询参数", () => {
    expect(buildWsUrl("http://127.0.0.1:8080", null)).toBe(
      "ws://127.0.0.1:8080/api/v1/ws",
    );
    expect(buildWsUrl("http://127.0.0.1:8080", "")).toBe(
      "ws://127.0.0.1:8080/api/v1/ws",
    );
    expect(buildWsUrl("http://127.0.0.1:8080")).toBe(
      "ws://127.0.0.1:8080/api/v1/ws",
    );
  });

  it("token 含特殊字符时进行编码", () => {
    expect(buildWsUrl("http://127.0.0.1:1234", "a+b/c=")).toBe(
      "ws://127.0.0.1:1234/api/v1/ws?token=a%2Bb%2Fc%3D",
    );
  });
});
