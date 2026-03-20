import { describe, expect, it } from "vitest";
import {
  buildValidatedLinkOptions,
  extractAliasFromListen,
  projectUsableLinks,
} from "@/lib/simple-domain/link-rules";
import { emptyConfig } from "@/lib/config-types";
import type { NetworkStateDTO } from "@/lib/types";

describe("link-rules", () => {
  it("extracts alias from vtcp listen url", () => {
    expect(extractAliasFromListen("vtcp://share.demo:8080")).toBe("demo");
  });

  it("returns scheme for non-vtcp listen url", () => {
    expect(extractAliasFromListen("tcp://127.0.0.1:8080")).toBe("tcp");
  });

  it("returns null for invalid listen url", () => {
    expect(extractAliasFromListen("not-a-url")).toBeNull();
  });

  it("builds validated link options from config facts", () => {
    const config = emptyConfig();
    config.links = {
      good: "tcp://example.com:7000?as=node-a",
      bad: "tcp://example.com:7000",
    };

    const result = buildValidatedLinkOptions(config);

    expect(result.allLinks).toEqual([
      {
        alias: "good",
        url: "tcp://example.com:7000?as=node-a",
        status: "usable",
        statusText: "当前可用",
      },
      {
        alias: "bad",
        url: "tcp://example.com:7000",
        status: "saved_needs_check",
        statusText: "需检查",
      },
    ]);
    expect(result.validLinks.map((item) => item.alias)).toEqual(["good"]);
  });

  it("projects usable links with runtime availability when sidecar is ready", () => {
    const networks: NetworkStateDTO[] = [
      {
        name: "online-link",
        kind: "virtual",
        protocol: "tcp",
        address: "",
        domain: "",
        state: "running",
        aliveMs: 1,
        listens: 0,
        dials: 0,
      },
      {
        name: "offline-link",
        kind: "virtual",
        protocol: "tcp",
        address: "",
        domain: "",
        state: "closed",
        aliveMs: 1,
        listens: 0,
        dials: 0,
      },
    ];

    const usable = projectUsableLinks({
      links: [
        {
          alias: "online-link",
          url: "tcp://example.com:7000?as=node-a",
          status: "usable",
          statusText: "当前可用",
        },
        {
          alias: "offline-link",
          url: "tcp://example.com:7001?as=node-b",
          status: "usable",
          statusText: "当前可用",
        },
      ],
      networks,
      sidecarReady: true,
    });

    expect(usable.map((item) => item.alias)).toEqual(["online-link"]);
    expect(usable[0]?.statusText).toBe("当前可用");
  });

  it("treats valid links as usable before sidecar is ready", () => {
    const usable = projectUsableLinks({
      links: [
        {
          alias: "demo",
          url: "tcp://example.com:7000?as=node-a",
          status: "usable",
          statusText: "当前可用",
        },
      ],
      networks: [],
      sidecarReady: false,
    });

    expect(usable.map((item) => item.alias)).toEqual(["demo"]);
  });
});
