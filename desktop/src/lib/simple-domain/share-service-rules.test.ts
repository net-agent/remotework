import { describe, expect, it } from "vitest";
import {
  buildLocalPortStatusText,
  buildShareServiceItemState,
  findRuntimeService,
  resolveShareServiceType,
} from "@/lib/simple-domain/share-service-rules";
import type { ServiceStateDTO } from "@/lib/types";

describe("share-service-rules", () => {
  it("detects socks5 and local-port tunnel types", () => {
    expect(
      resolveShareServiceType({
        id: "1",
        name: "socks",
        listen: "vtcp://share.demo:8080",
        target: "socks5://local",
      }),
    ).toBe("socks5");

    expect(
      resolveShareServiceType({
        id: "2",
        name: "port",
        listen: "vtcp://relay.demo:3000",
        target: "tcp://127.0.0.1:3000",
      }),
    ).toBe("local-port");
  });

  it("matches runtime service by tunnel id before name", () => {
    const services: ServiceStateDTO[] = [
      {
        id: 1,
        tunnelId: "tunnel-1",
        type: "proxy",
        name: "other",
        status: "running",
        listenURL: "vtcp://relay.demo:8080",
        targetURL: "tcp://127.0.0.1:8080",
        actives: 0,
        dones: 0,
      },
      {
        id: 2,
        type: "proxy",
        name: "same-name",
        status: "running",
        listenURL: "vtcp://relay.demo:3000",
        targetURL: "tcp://127.0.0.1:3000",
        actives: 0,
        dones: 0,
      },
    ];

    expect(
      findRuntimeService(
        {
          id: "tunnel-1",
          name: "same-name",
          listen: "vtcp://relay.demo:8080",
          target: "tcp://127.0.0.1:8080",
        },
        services,
      )?.id,
    ).toBe(1);
  });

  it("builds local-port status text across saved, opening, open and error states", () => {
    expect(
      buildLocalPortStatusText({
        state: "closed",
        openCount: 0,
        hasRuntime: false,
      }),
    ).toBe("尚未开放");

    expect(
      buildLocalPortStatusText({
        state: "opening",
        openCount: 2,
        hasRuntime: false,
      }),
    ).toBe("已保存 2 个端口，等待生效");

    expect(
      buildLocalPortStatusText({
        state: "opening",
        openCount: 2,
        hasRuntime: true,
      }),
    ).toBe("正在开放 2 个端口");

    expect(
      buildLocalPortStatusText({
        state: "open",
        openCount: 1,
        hasRuntime: true,
      }),
    ).toBe("已开放");

    expect(
      buildLocalPortStatusText({
        state: "error",
        openCount: 1,
        hasRuntime: true,
      }),
    ).toBe("运行异常");
  });

  it("projects runtime item state", () => {
    expect(buildShareServiceItemState(null)).toBe("opening");
    expect(
      buildShareServiceItemState({
        id: 1,
        type: "proxy",
        name: "svc",
        status: "running",
        listenURL: "vtcp://relay.demo:8080",
        targetURL: "tcp://127.0.0.1:8080",
        actives: 0,
        dones: 0,
      }),
    ).toBe("open");
    expect(
      buildShareServiceItemState({
        id: 2,
        type: "proxy",
        name: "svc",
        status: "starting",
        listenURL: "vtcp://relay.demo:8080",
        targetURL: "tcp://127.0.0.1:8080",
        actives: 0,
        dones: 0,
      }),
    ).toBe("opening");
    expect(
      buildShareServiceItemState({
        id: 3,
        type: "proxy",
        name: "svc",
        status: "running",
        lastErr: "boom",
        listenURL: "vtcp://relay.demo:8080",
        targetURL: "tcp://127.0.0.1:8080",
        actives: 0,
        dones: 0,
      }),
    ).toBe("error");
  });
});
