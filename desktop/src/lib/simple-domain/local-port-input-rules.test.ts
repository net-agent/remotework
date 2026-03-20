import { describe, expect, it } from "vitest";
import { emptyConfig } from "@/lib/config-types";
import {
  buildCombinedPorts,
  buildFilteredListeningPorts,
  buildLocalPortSaveLabel,
  buildOpenPortSet,
  buildSelectedPortList,
  matchesListeningPort,
} from "@/lib/simple-domain/local-port-input-rules";
import type { ListeningPortDTO } from "@/lib/types";

describe("local-port-input-rules", () => {
  const listeningPorts: ListeningPortDTO[] = [
    { port: 8080, protocol: "tcp", processName: "node", pid: 10 },
    { port: 3000, protocol: "tcp", processName: "vite", pid: 11 },
    { port: 3389, protocol: "tcp", processName: "rdp", pid: 12 },
  ];

  it("matches listening ports by port process name and pid", () => {
    expect(matchesListeningPort(listeningPorts[0]!, "8080")).toBe(true);
    expect(matchesListeningPort(listeningPorts[0]!, "node")).toBe(true);
    expect(matchesListeningPort(listeningPorts[0]!, "10")).toBe(true);
    expect(matchesListeningPort(listeningPorts[0]!, "nginx")).toBe(false);
  });

  it("builds open port set from managed local-port tunnels", () => {
    const config = emptyConfig();
    config.tunnels = [
      {
        id: "1",
        name: "本地端口 8080",
        listen: "vtcp://relay.demo:8080?authcode=ABC123",
        target: "tcp://127.0.0.1:8080",
      },
      {
        id: "2",
        name: "other",
        listen: "vtcp://relay.other:3000?authcode=ABC123",
        target: "tcp://127.0.0.1:3000",
      },
    ];

    expect(Array.from(buildOpenPortSet(config, "demo"))).toEqual([8080]);
    expect(Array.from(buildOpenPortSet(config, null))).toEqual([]);
  });

  it("builds filtered listening ports with opened ports pinned first", () => {
    const result = buildFilteredListeningPorts({
      listeningPorts,
      filterText: "",
      openPortSet: new Set([8080]),
    });

    expect(result.map((item) => item.port)).toEqual([8080, 3389, 3000]);
  });

  it("builds sorted selected port list", () => {
    expect(buildSelectedPortList(new Set([9222, 3000, 8080]))).toEqual([
      3000,
      8080,
      9222,
    ]);
  });

  it("combines selected and manual ports with dedupe", () => {
    const result = buildCombinedPorts({
      selectedPortList: [3000, 8080],
      manualPortsText: "8080, 9222",
    });

    expect(result.combinedPorts).toEqual([3000, 8080, 9222]);
  });

  it("builds local-port save label", () => {
    expect(
      buildLocalPortSaveLabel({ selectedPortCount: 2, manualPortsText: "" }),
    ).toBe("开放选中端口（2）");
    expect(
      buildLocalPortSaveLabel({ selectedPortCount: 2, manualPortsText: "9000" }),
    ).toBe("开放选中端口（2+手动）");
  });
});
