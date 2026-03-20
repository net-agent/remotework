import { useEffect, useMemo, useState } from "react";
import { invoke } from "@tauri-apps/api/core";
import * as api from "@/lib/api";
import {
  Dialog,
  DialogContent,
  DialogDescription,
  DialogFooter,
  DialogHeader,
  DialogTitle,
} from "@/components/ui/dialog";
import { Button } from "@/components/ui/button";
import { Input } from "@/components/ui/input";
import { Label } from "@/components/ui/label";
import {
  Select,
  SelectContent,
  SelectItem,
  SelectTrigger,
  SelectValue,
} from "@/components/ui/select";
import { useUIStore } from "@/stores/ui-store";
import { useProfileStore } from "@/stores/profile-store";
import { useAgentStore } from "@/stores/agent-store";
import {
  emptyTunnel,
  type AgentConfig,
  type TunnelInfo,
} from "@/lib/config-types";
import { saveTunnelConfig, saveTunnelConfigsBatch } from "@/lib/tunnel-save";
import type { ListeningPortDTO } from "@/lib/types";
import { toast } from "sonner";
import { SimpleListeningPortPicker } from "@/components/simple/SimpleListeningPortPicker";

const DEFAULT_LOCAL_PORT = "8080";
const AUTHCODE_LENGTH = 6;
const AUTHCODE_ALPHABET = "ABCDEFGHJKLMNPQRSTUVWXYZ23456789";

function buildShareListen(alias: string, port: string) {
  return `vtcp://share.${alias}:${port}`;
}

function buildLocalPortListen(input: {
  alias: string;
  asValue: string;
  port: number;
  authcode: string;
}) {
  return `vtcp://${input.asValue}.${input.alias}:${input.port}?authcode=${input.authcode}`;
}

function buildSocks5Target(username: string, password: string) {
  if (!username && !password) {
    return "socks5://local";
  }

  const encodedUser = encodeURIComponent(username);
  const encodedPassword = encodeURIComponent(password);
  return `socks5://${encodedUser}:${encodedPassword}@local`;
}

function buildPresetTunnel(input: {
  preset: "socks5" | "local-port";
  alias: string;
  name: string;
  port: string;
  localAddress: string;
  username: string;
  password: string;
}): TunnelInfo {
  const { preset, alias, name, port, localAddress, username, password } = input;
  const tunnel = emptyTunnel();

  if (preset === "socks5") {
    return {
      ...tunnel,
      name,
      listen: buildShareListen(alias, DEFAULT_LOCAL_PORT),
      target: buildSocks5Target(username, password),
    };
  }

  return {
    ...tunnel,
    name,
    listen: buildShareListen(alias, port),
    target: `tcp://${localAddress}:${port}`,
  };
}

function buildLocalPortTunnel(input: {
  alias: string;
  asValue: string;
  authcode: string;
  port: number;
  localAddress: string;
}) {
  const tunnel = emptyTunnel();
  return {
    ...tunnel,
    name: `本地端口 ${input.port}`,
    listen: buildLocalPortListen({
      alias: input.alias,
      asValue: input.asValue,
      port: input.port,
      authcode: input.authcode,
    }),
    target: `tcp://${input.localAddress}:${input.port}`,
  } satisfies TunnelInfo;
}

function getPresetMeta(preset: "socks5" | "local-port") {
  switch (preset) {
    case "socks5":
      return {
        title: "开放本地网络",
        description: "让对方通过这台电脑访问你的本地网络资源。",
        defaultName: "本地网络",
        saveLabel: "开放本地网络",
      };
    default:
      return {
        title: "开放本地端口",
        description: "筛选本机监听端口，支持多选并批量开放。",
        defaultName: "本地端口",
        saveLabel: "开放选中端口",
      };
  }
}

function getDefaultName(preset: "socks5" | "local-port", port: string) {
  if (preset === "local-port" && port) {
    return `本地端口 ${port}`;
  }

  return getPresetMeta(preset).defaultName;
}

function sortListeningPorts(items: ListeningPortDTO[]) {
  return [...items].sort((left, right) => {
    if (left.port === 3389 && right.port !== 3389) {
      return -1;
    }
    if (left.port !== 3389 && right.port === 3389) {
      return 1;
    }
    return left.port - right.port;
  });
}

function hasPartialCredentials(username: string, password: string) {
  return (
    (username.length > 0 && password.length === 0) ||
    (username.length === 0 && password.length > 0)
  );
}

function matchesListeningPort(item: ListeningPortDTO, keyword: string) {
  const normalized = keyword.trim().toLowerCase();
  if (!normalized) {
    return true;
  }

  return [
    String(item.port),
    item.processName?.toLowerCase() ?? "",
    item.pid !== undefined ? String(item.pid) : "",
  ].some((part) => part.includes(normalized));
}

function parseManualPorts(text: string) {
  const tokens = text
    .split(/[，,\s]+/)
    .map((item) => item.trim())
    .filter(Boolean);

  if (tokens.length === 0) {
    return [];
  }

  const values = tokens.map((token) => {
    const value = Number(token);
    if (!Number.isInteger(value) || value < 1 || value > 65535) {
      throw new Error(`端口 ${token} 不合法，范围应为 1-65535`);
    }
    return value;
  });

  return Array.from(new Set(values)).sort((left, right) => left - right);
}

function extractAliasFromListen(listen: string) {
  try {
    const parsed = new URL(listen);
    if (parsed.protocol !== "vtcp:") {
      return parsed.protocol.replace(":", "") || null;
    }
    const hostname = parsed.hostname;
    const lastDot = hostname.lastIndexOf(".");
    if (lastDot < 0) {
      return null;
    }
    return hostname.slice(lastDot + 1) || null;
  } catch {
    return null;
  }
}

function isManagedLocalPortTunnel(tunnel: TunnelInfo, alias: string) {
  return (
    extractAliasFromListen(tunnel.listen) === alias &&
    tunnel.target.startsWith("tcp://")
  );
}

function generateRandomAuthcode() {
  return Array.from({ length: AUTHCODE_LENGTH }, () => {
    const index = Math.floor(Math.random() * AUTHCODE_ALPHABET.length);
    return AUTHCODE_ALPHABET[index] ?? "A";
  }).join("");
}

function ensureLinkAuthcode(input: {
  alias: string;
  currentConfig: AgentConfig;
}): {
  asValue: string;
  authcode: string;
  nextConfig: AgentConfig;
  updated: boolean;
} {
  const rawLink = input.currentConfig.links?.[input.alias];
  if (!rawLink) {
    throw new Error("未找到当前连接信息");
  }

  let parsed: URL;
  try {
    parsed = new URL(rawLink);
  } catch {
    throw new Error("当前连接信息格式无效");
  }

  const asValue = parsed.searchParams.get("as")?.trim() ?? "";
  if (!asValue) {
    throw new Error("当前连接信息缺少 as 参数");
  }

  let authcode = parsed.searchParams.get("authcode")?.trim() ?? "";
  let updated = false;

  if (!authcode) {
    authcode = generateRandomAuthcode();
    parsed.searchParams.set("authcode", authcode);
    updated = true;
  }

  if (!updated) {
    return {
      asValue,
      authcode,
      nextConfig: input.currentConfig,
      updated: false,
    };
  }

  return {
    asValue,
    authcode,
    updated: true,
    nextConfig: {
      ...input.currentConfig,
      links: {
        ...(input.currentConfig.links ?? {}),
        [input.alias]: parsed.toString(),
      },
    },
  };
}

export function SimpleSharePresetForm() {
  const {
    simpleShareDialogOpen,
    simpleShareDialogType,
    selectedSimpleShareLinkAlias,
    closeSimpleShareDialog,
  } = useUIStore();
  const {
    currentConfig,
    setCurrentConfig,
    saveConfig,
    activeProfile,
    setNeedsRestart,
  } = useProfileStore();
  const [name, setName] = useState("");
  const [port, setPort] = useState(DEFAULT_LOCAL_PORT);
  const [localAddresses, setLocalAddresses] = useState<string[]>([]);
  const [localAddress, setLocalAddress] = useState("127.0.0.1");
  const [username, setUsername] = useState("");
  const [password, setPassword] = useState("");
  const [listeningPorts, setListeningPorts] = useState<ListeningPortDTO[]>([]);
  const [listeningPortsError, setListeningPortsError] = useState<string | null>(
    null,
  );
  const [isLoadingListeningPorts, setIsLoadingListeningPorts] = useState(false);
  const [filterText, setFilterText] = useState("");
  const [selectedPorts, setSelectedPorts] = useState<Set<number>>(new Set());
  const [manualPortsText, setManualPortsText] = useState("");

  const preset =
    simpleShareDialogType === "socks5" || simpleShareDialogType === "local-port"
      ? simpleShareDialogType
      : null;

  const meta = useMemo(() => (preset ? getPresetMeta(preset) : null), [preset]);

  const resolvedAlias = useMemo(() => {
    if (selectedSimpleShareLinkAlias) {
      return selectedSimpleShareLinkAlias;
    }

    const savedAliases = Object.keys(currentConfig.links ?? {});
    return savedAliases.length === 1 ? (savedAliases[0] ?? null) : null;
  }, [currentConfig.links, selectedSimpleShareLinkAlias]);

  const requiresExplicitAliasSelection = useMemo(() => {
    return Object.keys(currentConfig.links ?? {}).length > 1 && !resolvedAlias;
  }, [currentConfig.links, resolvedAlias]);

  const openPortSet = useMemo(() => {
    if (!resolvedAlias) {
      return new Set<number>();
    }

    return new Set(
      (currentConfig.tunnels ?? [])
        .filter((tunnel) => isManagedLocalPortTunnel(tunnel, resolvedAlias))
        .map((tunnel) => {
          try {
            return Number(new URL(tunnel.target).port);
          } catch {
            return NaN;
          }
        })
        .filter((value) => Number.isInteger(value) && value > 0),
    );
  }, [currentConfig.tunnels, resolvedAlias]);

  const filteredListeningPorts = useMemo(() => {
    const matched = listeningPorts.filter((item) =>
      matchesListeningPort(item, filterText),
    );
    return [...matched].sort((left, right) => {
      const leftOpen = openPortSet.has(left.port);
      const rightOpen = openPortSet.has(right.port);
      if (leftOpen !== rightOpen) {
        return leftOpen ? -1 : 1;
      }
      if (left.port === 3389 && right.port !== 3389) {
        return -1;
      }
      if (left.port !== 3389 && right.port === 3389) {
        return 1;
      }
      return left.port - right.port;
    });
  }, [filterText, listeningPorts, openPortSet]);

  const selectedPortList = useMemo(
    () => Array.from(selectedPorts).sort((left, right) => left - right),
    [selectedPorts],
  );

  useEffect(() => {
    if (!simpleShareDialogOpen || !preset || !meta) {
      return;
    }

    setName(meta.defaultName);
    setUsername("");
    setPassword("");
    setListeningPorts([]);
    setListeningPortsError(null);
    setIsLoadingListeningPorts(false);
    setFilterText("");
    setSelectedPorts(new Set());
    setManualPortsText("");

    invoke<string[]>("get_network_interfaces")
      .then((addresses) => {
        const nextAddresses =
          addresses.length > 0 ? addresses : ["0.0.0.0", "127.0.0.1"];
        setLocalAddresses(nextAddresses);
        setLocalAddress(
          nextAddresses.includes("127.0.0.1")
            ? "127.0.0.1"
            : (nextAddresses[0] ?? "127.0.0.1"),
        );
      })
      .catch(() => {
        const fallback = ["0.0.0.0", "127.0.0.1"];
        setLocalAddresses(fallback);
        setLocalAddress("127.0.0.1");
      });

    if (preset === "local-port") {
      let cancelled = false;
      let timedOut = false;
      const timeoutId = window.setTimeout(() => {
        if (cancelled) {
          return;
        }
        timedOut = true;
        setIsLoadingListeningPorts(false);
        setListeningPortsError(
          "自动读取监听端口超时，请使用手动输入或稍后重试",
        );
      }, 1500);

      setPort(DEFAULT_LOCAL_PORT);
      setName(getDefaultName(preset, DEFAULT_LOCAL_PORT));
      setIsLoadingListeningPorts(true);

      api
        .getListeningPorts()
        .then((items) => {
          if (cancelled || timedOut) {
            return;
          }
          window.clearTimeout(timeoutId);
          setIsLoadingListeningPorts(false);
          const sorted = sortListeningPorts(items).filter(
            (item: ListeningPortDTO) => item.protocol.toLowerCase() === "tcp",
          );
          setListeningPorts(sorted);
          if (sorted.length === 0) {
            setListeningPortsError(
              "未检测到正在监听的 TCP 端口，请直接手动输入",
            );
            return;
          }
          setListeningPortsError(null);
        })
        .catch((error) => {
          if (cancelled || timedOut) {
            return;
          }
          window.clearTimeout(timeoutId);
          setIsLoadingListeningPorts(false);
          setListeningPorts([]);
          setListeningPortsError(
            `读取本机监听端口失败，请改用手动输入：${String(error)}`,
          );
        });

      return () => {
        cancelled = true;
        window.clearTimeout(timeoutId);
      };
    }

    setPort(DEFAULT_LOCAL_PORT);
  }, [meta, preset, simpleShareDialogOpen]);

  const toggleSelectedPort = (nextPort: number) => {
    setSelectedPorts((current) => {
      const next = new Set(current);
      if (next.has(nextPort)) {
        next.delete(nextPort);
      } else {
        next.add(nextPort);
      }
      return next;
    });
  };

  const selectFilteredPorts = () => {
    setSelectedPorts((current) => {
      const next = new Set(current);
      filteredListeningPorts.forEach((item) => {
        if (!openPortSet.has(item.port)) {
          next.add(item.port);
        }
      });
      return next;
    });
  };

  const clearSelectedPorts = () => {
    setSelectedPorts(new Set());
  };

  const handleSave = async () => {
    if (!preset || !meta) {
      toast.error("请选择开放方式");
      return;
    }
    if (!resolvedAlias) {
      toast.error("请先选择连接信息");
      return;
    }

    const trimmedName = name.trim();
    const trimmedUsername = username.trim();

    if (preset === "local-port") {
      let manualPorts: number[] = [];
      try {
        manualPorts = parseManualPorts(manualPortsText);
      } catch (error) {
        toast.error(String(error));
        return;
      }

      const combinedPorts = Array.from(
        new Set([...selectedPortList, ...manualPorts]),
      ).sort((left, right) => left - right);

      if (combinedPorts.length === 0) {
        toast.error("请至少选择一个端口，或在手动输入中填写端口");
        return;
      }

      let linkInfo: ReturnType<typeof ensureLinkAuthcode>;
      try {
        linkInfo = ensureLinkAuthcode({
          alias: resolvedAlias,
          currentConfig,
        });
      } catch (error) {
        toast.error(String(error));
        return;
      }

      const tunnels = combinedPorts.map((selectedPort) =>
        buildLocalPortTunnel({
          alias: resolvedAlias,
          asValue: linkInfo.asValue,
          authcode: linkInfo.authcode,
          port: selectedPort,
          localAddress,
        }),
      );

      const saved = await saveTunnelConfigsBatch({
        tunnels,
        context: {
          currentConfig: linkInfo.nextConfig,
          setCurrentConfig,
          saveConfig,
          activeProfile,
          setNeedsRestart,
        },
        successLabel: "端口",
      });

      if (saved.ok) {
        if (linkInfo.updated && useAgentStore.getState().agentRunning) {
          toast.message(
            "连接信息已自动补全 authcode，重启后会按新连接信息运行",
          );
          setNeedsRestart();
        }
        closeSimpleShareDialog();
      }
      return;
    }

    if (!trimmedName) {
      toast.error("请填写开放名称");
      return;
    }

    if (
      preset === "socks5" &&
      hasPartialCredentials(trimmedUsername, password)
    ) {
      toast.error("用户名和密码需要同时填写，或同时留空以匿名开放");
      return;
    }

    const trimmedPort = port.trim();
    if (!trimmedPort) {
      toast.error("请填写本地端口");
      return;
    }
    const portNumber = Number(trimmedPort);
    if (!Number.isInteger(portNumber) || portNumber < 1 || portNumber > 65535) {
      toast.error("端口范围应为 1-65535");
      return;
    }

    const tunnel = buildPresetTunnel({
      preset,
      alias: resolvedAlias,
      name: trimmedName,
      port: port.trim() || DEFAULT_LOCAL_PORT,
      localAddress,
      username: trimmedUsername,
      password,
    });

    const saved = await saveTunnelConfig({
      tunnel,
      context: {
        currentConfig,
        setCurrentConfig,
        saveConfig,
        activeProfile,
        setNeedsRestart,
      },
    });

    if (saved) {
      closeSimpleShareDialog();
    }
  };

  return (
    <Dialog
      open={simpleShareDialogOpen}
      onOpenChange={(open) => !open && closeSimpleShareDialog()}
    >
      <DialogContent
        className={preset === "local-port" ? "max-w-2xl" : "max-w-sm"}
      >
        <DialogHeader>
          <DialogTitle>{meta?.title ?? "开放方式"}</DialogTitle>
          <DialogDescription>
            {meta?.description ?? "按普通模式引导填写必要信息。"}
          </DialogDescription>
        </DialogHeader>

        <div className="space-y-4">
          {preset === "local-port" ? (
            <>
              {requiresExplicitAliasSelection ? (
                <div className="rounded-md border border-amber-300 bg-amber-50 px-3 py-3 text-xs text-amber-700 dark:border-amber-800 dark:bg-amber-950/30 dark:text-amber-400">
                  当前存在多个连接信息，请先在主界面明确选择一个连接信息，再继续开放端口。
                </div>
              ) : null}

              <SimpleListeningPortPicker
                alias={resolvedAlias}
                filterText={filterText}
                onFilterTextChange={setFilterText}
                listeningPorts={listeningPorts}
                filteredListeningPorts={filteredListeningPorts}
                selectedPorts={selectedPorts}
                disabledPorts={openPortSet}
                isLoading={isLoadingListeningPorts}
                error={listeningPortsError}
                onTogglePort={toggleSelectedPort}
                onSelectFiltered={selectFilteredPorts}
                onClearSelection={clearSelectedPorts}
              />

              <div className="space-y-2 rounded-lg border bg-muted/20 px-3 py-3">
                <Label htmlFor="simple-share-manual-ports">手动输入端口</Label>
                <Input
                  id="simple-share-manual-ports"
                  value={manualPortsText}
                  onChange={(event) => setManualPortsText(event.target.value)}
                  placeholder="例如：8080, 3000 9222"
                />
                <div className="text-xs text-muted-foreground">
                  也可直接输入多个端口，使用空格或逗号分隔。
                </div>
              </div>
            </>
          ) : (
            <>
              <div>
                <Label htmlFor="simple-share-name">开放名称</Label>
                <Input
                  id="simple-share-name"
                  value={name}
                  onChange={(event) => setName(event.target.value)}
                  placeholder={meta?.defaultName ?? "请输入名称"}
                  className="mt-1.5"
                />
              </div>

              <div>
                <Label>当前连接信息</Label>
                <div className="mt-1.5 rounded-md border bg-muted/40 px-3 py-2 text-sm">
                  {resolvedAlias ?? "未选择"}
                </div>
              </div>

              {preset === "socks5" ? (
                <>
                  <div className="rounded-md border bg-muted/30 px-3 py-3 text-xs text-muted-foreground">
                    用户名和密码都可以留空，此时将以匿名方式开放本地网络。若需要认证，请两项同时填写。
                  </div>
                  <div>
                    <Label htmlFor="simple-share-username">用户名</Label>
                    <Input
                      id="simple-share-username"
                      value={username}
                      onChange={(event) => setUsername(event.target.value)}
                      placeholder="可留空"
                      className="mt-1.5"
                    />
                  </div>
                  <div>
                    <Label htmlFor="simple-share-password">密码</Label>
                    <Input
                      id="simple-share-password"
                      type="password"
                      value={password}
                      onChange={(event) => setPassword(event.target.value)}
                      placeholder="可留空"
                      className="mt-1.5"
                    />
                  </div>
                </>
              ) : (
                <>
                  <div>
                    <Label htmlFor="simple-share-port">本地端口</Label>
                    <Input
                      id="simple-share-port"
                      value={port}
                      onChange={(event) => setPort(event.target.value)}
                      placeholder={DEFAULT_LOCAL_PORT}
                      className="mt-1.5"
                    />
                  </div>

                  <div>
                    <Label>本地地址</Label>
                    <Select
                      value={localAddress}
                      onValueChange={setLocalAddress}
                    >
                      <SelectTrigger className="mt-1.5 w-full">
                        <SelectValue placeholder="选择本地地址" />
                      </SelectTrigger>
                      <SelectContent>
                        {localAddresses.map((address) => (
                          <SelectItem key={address} value={address}>
                            {address}
                          </SelectItem>
                        ))}
                      </SelectContent>
                    </Select>
                  </div>
                </>
              )}
            </>
          )}
        </div>

        <DialogFooter>
          <Button variant="outline" onClick={closeSimpleShareDialog}>
            取消
          </Button>
          <Button
            onClick={handleSave}
            disabled={requiresExplicitAliasSelection}
          >
            {preset === "local-port"
              ? `开放选中端口（${selectedPortList.length}${manualPortsText.trim() ? "+手动" : ""}）`
              : (meta?.saveLabel ?? "保存")}
          </Button>
        </DialogFooter>
      </DialogContent>
    </Dialog>
  );
}
