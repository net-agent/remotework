import { describe, expect, it } from "vitest";
import { emptyConfig } from "@/lib/config-types";
import {
  buildSimpleSessionFacts,
  buildSimpleShareServiceRowFacts,
} from "@/lib/simple-domain/simple-session-domain";
import {
  buildShareServiceRows,
  buildShareLinkHint,
} from "@/lib/view-model/simple-session-presenter";

describe("simple-session split", () => {
  it("falls back to the first available share link", () => {
    const facts = buildSimpleSessionFacts({
      allLinks: [
        {
          alias: "alpha",
          url: "tcp://example.com:7000?as=a",
          status: "usable",
          statusText: "当前可用",
        },
        {
          alias: "beta",
          url: "tcp://example.com:7001?as=b",
          status: "usable",
          statusText: "当前可用",
        },
      ],
      availableShareLinks: [
        {
          alias: "alpha",
          url: "tcp://example.com:7000?as=a",
          status: "usable",
          statusText: "当前可用",
        },
        {
          alias: "beta",
          url: "tcp://example.com:7001?as=b",
          status: "usable",
          statusText: "当前可用",
        },
      ],
      runtimeUsableShareLinks: [
        {
          alias: "alpha",
          url: "tcp://example.com:7000?as=a",
          status: "usable",
          statusText: "当前可用",
        },
      ],
      selectedShareLinkAlias: "missing",
    });

    expect(facts.selectedShareLink?.alias).toBe("alpha");
    expect(facts.selectedAlias).toBe("alpha");
  });

  it("keeps valid saved links selectable even when runtime-usable list is smaller", () => {
    const facts = buildSimpleSessionFacts({
      allLinks: [
        {
          alias: "alpha",
          url: "tcp://example.com:7000?as=a",
          status: "usable",
          statusText: "当前可用",
        },
        {
          alias: "beta",
          url: "tcp://example.com:7001?as=b",
          status: "usable",
          statusText: "当前可用",
        },
      ],
      availableShareLinks: [
        {
          alias: "alpha",
          url: "tcp://example.com:7000?as=a",
          status: "usable",
          statusText: "当前可用",
        },
        {
          alias: "beta",
          url: "tcp://example.com:7001?as=b",
          status: "usable",
          statusText: "当前可用",
        },
      ],
      runtimeUsableShareLinks: [
        {
          alias: "alpha",
          url: "tcp://example.com:7000?as=a",
          status: "usable",
          statusText: "当前可用",
        },
      ],
      selectedShareLinkAlias: "beta",
    });

    expect(facts.selectedShareLink?.alias).toBe("beta");
    expect(facts.savedNeedsCheckCount).toBe(0);
    expect(facts.mustCreateNewShareLink).toBe(false);
  });

  it("builds short hint text for mixed available and needs-check links", () => {
    expect(
      buildShareLinkHint({
        allLinkCount: 3,
        availableLinkCount: 2,
        mustCreateNewShareLink: false,
        savedNeedsCheckCount: 1,
      }),
    ).toBe("当前使用 1 条连接，另有 1 条待检查。");
  });

  it("builds create hint when no links exist", () => {
    expect(
      buildShareLinkHint({
        allLinkCount: 0,
        availableLinkCount: 0,
        mustCreateNewShareLink: true,
        savedNeedsCheckCount: 0,
      }),
    ).toBe("先输入连接信息，才能继续。");
  });

  it("builds closed rows when no alias is selected", () => {
    const rows = buildShareServiceRows(
      buildSimpleShareServiceRowFacts({
        selectedAlias: null,
        currentConfig: emptyConfig(),
        services: [],
      }),
      null,
    );

    expect(rows).toHaveLength(2);
    expect(rows[0]?.statusText).toBe("请先选择连接信息");
    expect(rows[0]?.primaryActionKind).toBe("open");
  });

  it("keeps local-port row in manage mode after ports are saved", () => {
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
        name: "本地端口 3000",
        listen: "vtcp://relay.demo:3000?authcode=ABC123",
        target: "tcp://127.0.0.1:3000",
      },
    ];

    const rows = buildShareServiceRows(
      buildSimpleShareServiceRowFacts({
        selectedAlias: "demo",
        currentConfig: config,
        services: [],
      }),
      "demo",
    );

    const localPortRow = rows.find((row) => row.key === "local-port");
    expect(localPortRow?.primaryActionKind).toBe("manage");
    expect(localPortRow?.primaryActionLabel).toBe("开放");
    expect(localPortRow?.statusText).toBe("已保存 2 个端口，等待生效");
  });

  it("keeps local-port row in manage mode when only one port is open", () => {
    const config = emptyConfig();
    config.tunnels = [
      {
        id: "1",
        name: "本地端口 8080",
        listen: "vtcp://relay.demo:8080?authcode=ABC123",
        target: "tcp://127.0.0.1:8080",
      },
    ];

    const rows = buildShareServiceRows(
      buildSimpleShareServiceRowFacts({
        selectedAlias: "demo",
        currentConfig: config,
        services: [
          {
            id: 1,
            tunnelId: "1",
            type: "proxy",
            name: "本地端口 8080",
            status: "running",
            listenURL: "vtcp://relay.demo:8080?authcode=ABC123",
            targetURL: "tcp://127.0.0.1:8080",
            actives: 0,
            dones: 0,
          },
        ],
      }),
      "demo",
    );

    const localPortRow = rows.find((row) => row.key === "local-port");
    expect(localPortRow?.primaryActionKind).toBe("manage");
    expect(localPortRow?.primaryActionLabel).toBe("开放");
    expect(localPortRow?.statusText).toBe("已开放");
  });

  it("builds socks5 close action when runtime reports open", () => {
    const config = emptyConfig();
    config.tunnels = [
      {
        id: "s1",
        name: "本地网络",
        listen: "vtcp://share.demo:8080",
        target: "socks5://local",
      },
    ];

    const rows = buildShareServiceRows(
      buildSimpleShareServiceRowFacts({
        selectedAlias: "demo",
        currentConfig: config,
        services: [
          {
            id: 1,
            tunnelId: "s1",
            type: "proxy",
            name: "本地网络",
            status: "running",
            listenURL: "vtcp://share.demo:8080",
            targetURL: "socks5://local",
            actives: 0,
            dones: 0,
          },
        ],
      }),
      "demo",
    );

    const socksRow = rows.find((row) => row.key === "socks5");
    expect(socksRow?.statusText).toBe("已开放");
    expect(socksRow?.primaryActionKind).toBe("close");
    expect(socksRow?.primaryActionLabel).toBe("关闭");
  });
});
