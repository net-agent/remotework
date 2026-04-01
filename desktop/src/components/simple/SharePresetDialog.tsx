import { useEffect, useMemo, useState } from "react";
import { platform } from "@/lib/platform";
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
import { Badge } from "@/components/ui/badge";
import {
  Select,
  SelectContent,
  SelectItem,
  SelectTrigger,
  SelectValue,
} from "@/components/ui/select";
import { Label } from "@/components/ui/label";
import { useUIStore } from "@/stores/ui-store";
import { useAgentStore } from "@/stores/agent-store";
import { saveTunnelConfigsBatch } from "@/lib/tunnel-save";
import type { ListeningPortDTO } from "@/lib/types";
import { toast } from "sonner";
import {
  buildCombinedPorts,
  buildFilteredListeningPorts,
  buildLocalPortSaveLabel,
  buildOpenPortSet,
  buildSelectedPortList,
} from "@/lib/simple-domain/local-port-input-rules";
import {
  buildLocalPortTunnel,
  DEFAULT_LOCAL_PORT,
  ensureLinkAuthcode,
  getDefaultName,
  getPresetMeta,
} from "@/lib/simple-domain/share-preset-rules";
import { sortListeningPorts } from "@/lib/simple-domain/local-port-rules";
import { Socks5PresetFields } from "@/components/simple/share-preset/Socks5PresetFields";
import { LocalPortPresetFields } from "@/components/simple/share-preset/LocalPortPresetFields";
import {
  saveSocks5Preset,
  useTunnelSaveContext,
} from "@/components/simple/share-preset/socks5-preset-save";

export function SharePresetDialog() {
  const {
    simpleShareDialogOpen,
    simpleShareDialogType,
    selectedSimpleShareLinkAlias,
    closeSimpleShareDialog,
  } = useUIStore();
  const saveContext = useTunnelSaveContext();
  const {
    currentConfig,
    setCurrentConfig,
    saveConfig,
    activeProfile,
    setNeedsRestart,
  } = saveContext;
  const [name, setName] = useState("");
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
  const [localPortInputMode, setLocalPortInputMode] = useState<
    "list" | "manual"
  >("list");

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

  const openPortSet = useMemo(
    () => buildOpenPortSet(currentConfig, resolvedAlias),
    [currentConfig, resolvedAlias],
  );

  const filteredListeningPorts = useMemo(
    () =>
      buildFilteredListeningPorts({
        listeningPorts,
        filterText,
        openPortSet,
      }),
    [filterText, listeningPorts, openPortSet],
  );

  const selectedPortList = useMemo(
    () => buildSelectedPortList(selectedPorts),
    [selectedPorts],
  );

  const manualPortList = useMemo(() => {
    try {
      return buildCombinedPorts({
        selectedPortList: [],
        manualPortsText,
      }).combinedPorts;
    } catch {
      return [];
    }
  }, [manualPortsText]);

  const portsToOpenPreview = useMemo(() => {
    return localPortInputMode === "manual" ? manualPortList : selectedPortList;
  }, [localPortInputMode, manualPortList, selectedPortList]);

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
    setLocalPortInputMode("list");

    platform
      .getNetworkInterfaces()
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
      const timeoutId = window.setTimeout(() => {
        if (cancelled) {
          return;
        }
        setIsLoadingListeningPorts(false);
        setListeningPortsError(
          "自动读取监听端口超时，请使用手动输入或稍后重试",
        );
      }, 1500);

      setName(getDefaultName(preset, DEFAULT_LOCAL_PORT));
      setIsLoadingListeningPorts(true);

      api
        .getListeningPorts()
        .then((items) => {
          if (cancelled) {
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
          if (cancelled) {
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

    return undefined;
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

  const saveLocalPortPreset = async (selectedAlias: string) => {
    let combinedPorts: number[];
    try {
      if (localPortInputMode === "manual") {
        ({ combinedPorts } = buildCombinedPorts({
          selectedPortList: [],
          manualPortsText,
        }));
      } else {
        ({ combinedPorts } = buildCombinedPorts({
          selectedPortList,
          manualPortsText: "",
        }));
      }
    } catch (error) {
      toast.error(String(error));
      return;
    }

    if (combinedPorts.length === 0) {
      toast.error(
        localPortInputMode === "manual"
          ? "请在手动输入中填写至少一个端口"
          : "请至少选择一个端口",
      );
      return;
    }

    let linkInfo: ReturnType<typeof ensureLinkAuthcode>;
    try {
      linkInfo = ensureLinkAuthcode({
        alias: selectedAlias,
        currentConfig,
      });
    } catch (error) {
      toast.error(String(error));
      return;
    }

    const tunnels = combinedPorts.map((selectedPort) =>
      buildLocalPortTunnel({
        alias: selectedAlias,
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
        toast.message("连接信息已自动补全 authcode，重启后会按新连接信息运行");
        setNeedsRestart();
      }
      closeSimpleShareDialog();
    }
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

    if (preset === "local-port") {
      await saveLocalPortPreset(resolvedAlias);
      return;
    }

    await saveSocks5Preset({
      selectedAlias: resolvedAlias,
      name,
      username,
      password,
      localAddress,
      context: saveContext,
      onSaved: closeSimpleShareDialog,
    });
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
              <LocalPortPresetFields
                requiresExplicitAliasSelection={requiresExplicitAliasSelection}
                filterText={filterText}
                onFilterTextChange={setFilterText}
                listeningPorts={listeningPorts}
                filteredListeningPorts={filteredListeningPorts}
                selectedPorts={selectedPorts}
                openPortSet={openPortSet}
                isLoadingListeningPorts={isLoadingListeningPorts}
                listeningPortsError={listeningPortsError}
                onToggleSelectedPort={toggleSelectedPort}
                onSelectFilteredPorts={selectFilteredPorts}
                onClearSelectedPorts={clearSelectedPorts}
                manualPortsText={manualPortsText}
                onManualPortsTextChange={setManualPortsText}
                localPortInputMode={localPortInputMode}
                onLocalPortInputModeChange={setLocalPortInputMode}
              />

              <div className="rounded-lg border bg-muted/20 px-3 py-2">
                <div className="text-xs font-medium text-muted-foreground">
                  本次开放端口
                </div>
                {portsToOpenPreview.length > 0 ? (
                  <div className="mt-2 flex flex-wrap gap-2">
                    {portsToOpenPreview.map((targetPort) => (
                      <Badge
                        key={targetPort}
                        variant="secondary"
                        className="font-mono"
                      >
                        {targetPort}
                      </Badge>
                    ))}
                  </div>
                ) : (
                  <div className="mt-1 text-xs text-muted-foreground">
                    尚未选择端口
                  </div>
                )}
              </div>
            </>
          ) : preset === "socks5" ? (
            <Socks5PresetFields
              resolvedAlias={resolvedAlias}
              name={name}
              onNameChange={setName}
              username={username}
              onUsernameChange={setUsername}
              password={password}
              onPasswordChange={setPassword}
            />
          ) : (
            <div>
              <Label>当前连接信息</Label>
              <div className="mt-1.5 rounded-md border bg-muted/40 px-3 py-2 text-sm">
                {resolvedAlias ?? "未选择"}
              </div>
              <div className="mt-4">
                <Label>本地地址</Label>
                <Select value={localAddress} onValueChange={setLocalAddress}>
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
            </div>
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
              ? buildLocalPortSaveLabel({
                  selectedPortCount:
                    localPortInputMode === "manual"
                      ? manualPortList.length
                      : selectedPortList.length,
                  manualPortsText:
                    localPortInputMode === "manual" ? manualPortsText : "",
                })
              : (meta?.saveLabel ?? "保存")}
          </Button>
        </DialogFooter>
      </DialogContent>
    </Dialog>
  );
}
