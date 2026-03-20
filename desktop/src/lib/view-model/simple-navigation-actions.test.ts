import { describe, expect, it } from "vitest";
import { resolveSelectedShareLinkAlias } from "@/lib/view-model/simple-navigation-actions";

describe("simple-navigation-actions", () => {
  const links = [
    {
      alias: "alpha",
      url: "tcp://example.com:7000?as=a",
      status: "usable" as const,
      statusText: "当前可用",
    },
    {
      alias: "beta",
      url: "tcp://example.com:7001?as=b",
      status: "usable" as const,
      statusText: "当前可用",
    },
  ];

  it("returns null when no links are available", () => {
    expect(resolveSelectedShareLinkAlias([], null)).toBeNull();
  });

  it("keeps current alias when it still exists", () => {
    expect(resolveSelectedShareLinkAlias(links, "beta")).toBe("beta");
  });

  it("falls back to the first available alias when current selection is missing", () => {
    expect(resolveSelectedShareLinkAlias(links, "missing")).toBe("alpha");
  });

  it("falls back to the first available alias when nothing is selected", () => {
    expect(resolveSelectedShareLinkAlias(links, null)).toBe("alpha");
  });
});
