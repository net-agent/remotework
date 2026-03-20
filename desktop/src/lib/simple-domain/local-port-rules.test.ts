import { describe, expect, it } from "vitest";
import {
  isLocalPortTunnel,
  isManagedLocalPortTunnel,
  parseManualPorts,
  parsePortFromUrl,
  sortListeningPorts,
} from "@/lib/simple-domain/local-port-rules";
import type { ListeningPortDTO } from "@/lib/types";

describe("local-port-rules", () => {
  it("parses manual ports with dedupe and ascending sort", () => {
    expect(parseManualPorts("8080, 3000 8080，9222")).toEqual([3000, 8080, 9222]);
  });

  it("returns empty list for blank manual port input", () => {
    expect(parseManualPorts("  ,   ")).toEqual([]);
  });

  it("throws for invalid manual port input", () => {
    expect(() => parseManualPorts("0, 70000, abc")).toThrow(
      /端口 .* 不合法/,
    );
  });

  it("recognizes managed local-port tunnels for selected alias", () => {
    const tunnel = {
      id: "1",
      name: "本地端口 8080",
      listen: "vtcp://relay.demo:8080?authcode=ABC123",
      target: "tcp://127.0.0.1:8080",
    };

    expect(isLocalPortTunnel(tunnel)).toBe(true);
    expect(isManagedLocalPortTunnel(tunnel, "demo")).toBe(true);
    expect(isManagedLocalPortTunnel(tunnel, "other")).toBe(false);
  });

  it("rejects plain tcp listens as managed local-port tunnels", () => {
    const tunnel = {
      id: "1",
      name: "plain",
      listen: "tcp://0.0.0.0:8080",
      target: "tcp://127.0.0.1:8080",
    };

    expect(isLocalPortTunnel(tunnel)).toBe(false);
  });

  it("parses port from url", () => {
    expect(parsePortFromUrl("vtcp://relay.demo:8080")).toBe("8080");
    expect(parsePortFromUrl("invalid")).toBeNull();
  });

  it("sorts listening ports with 3389 pinned first", () => {
    const ports: ListeningPortDTO[] = [
      { port: 8080, protocol: "tcp" },
      { port: 22, protocol: "tcp" },
      { port: 3389, protocol: "tcp" },
    ];

    expect(sortListeningPorts(ports).map((item) => item.port)).toEqual([
      3389,
      22,
      8080,
    ]);
  });
});
