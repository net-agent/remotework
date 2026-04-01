import { describe, expect, it, vi } from "vitest";
import { emptyConfig } from "@/lib/config-types";
import {
  buildLocalPortTunnel,
  buildPresetTunnel,
  buildSocks5Target,
  ensureLinkAuthcode,
  hasPartialCredentials,
  validatePortInput,
} from "@/lib/simple-domain/share-preset-rules";

describe("share-preset-rules", () => {
  it("builds anonymous and authenticated socks5 targets", () => {
    expect(buildSocks5Target("", "")).toBe("socks5://local");
    expect(buildSocks5Target("user", "pass word")).toBe(
      "socks5://user:pass%20word@local",
    );
  });

  it("detects partial credentials", () => {
    expect(hasPartialCredentials("user", "")).toBe(true);
    expect(hasPartialCredentials("", "pass")).toBe(true);
    expect(hasPartialCredentials("", "")).toBe(false);
    expect(hasPartialCredentials("user", "pass")).toBe(false);
  });

  it("validates port input", () => {
    expect(validatePortInput("")).toBe("请填写本地端口");
    expect(validatePortInput("0")).toBe("端口范围应为 1-65535");
    expect(validatePortInput("8080")).toBeNull();
  });

  it("builds preset tunnels", () => {
    const socksTunnel = buildPresetTunnel({
      preset: "socks5",
      alias: "demo",
      asValue: "mynode",
      name: "本地网络",
      port: "8080",
      localAddress: "127.0.0.1",
      username: "",
      password: "",
    });

    expect(socksTunnel.listen).toBe("vtcp://mynode.demo:8080");
    expect(socksTunnel.target).toBe("socks5://local");

    const portTunnel = buildPresetTunnel({
      preset: "local-port",
      alias: "demo",
      asValue: "mynode",
      name: "本地端口 3000",
      port: "3000",
      localAddress: "127.0.0.1",
      username: "",
      password: "",
    });

    expect(portTunnel.listen).toBe("vtcp://mynode.demo:3000");
    expect(portTunnel.target).toBe("tcp://127.0.0.1:3000");
  });

  it("builds local-port tunnel with authcode", () => {
    const tunnel = buildLocalPortTunnel({
      alias: "demo",
      asValue: "relay",
      authcode: "ABC123",
      port: 9222,
      localAddress: "127.0.0.1",
    });

    expect(tunnel.listen).toBe("vtcp://relay.demo:9222?authcode=ABC123");
    expect(tunnel.target).toBe("tcp://127.0.0.1:9222");
  });

  it("keeps existing authcode when present", () => {
    const config = emptyConfig();
    config.links.demo = "tcp://example.com:7000?as=node-a&authcode=KEEPIT";

    const result = ensureLinkAuthcode({ alias: "demo", currentConfig: config });

    expect(result.authcode).toBe("KEEPIT");
    expect(result.updated).toBe(false);
    expect(result.nextConfig).toBe(config);
  });

  it("auto-completes authcode when missing", () => {
    vi.spyOn(Math, "random").mockReturnValue(0);

    const config = emptyConfig();
    config.links.demo = "tcp://example.com:7000?as=node-a";

    const result = ensureLinkAuthcode({ alias: "demo", currentConfig: config });

    expect(result.updated).toBe(true);
    expect(result.asValue).toBe("node-a");
    expect(result.authcode).toHaveLength(6);
    expect(result.nextConfig.links.demo).toContain("authcode=");

    vi.restoreAllMocks();
  });
});
