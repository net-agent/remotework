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
import { Select, SelectContent, SelectItem, SelectTrigger, SelectValue } from "@/components/ui/select";
import { Label } from "@/components/ui/label";
import { useUIStore } from "@/stores/ui-store";
import { useProfileStore } from "@/stores/profile-store";
import { useAgentStore } from "@/stores/agent-store";
import { saveTunnelConfig, saveTunnelConfigsBatch } from "@/lib/tunnel-save";
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
  buildPresetTunnel,
  DEFAULT_LOCAL_PORT,
  ensureLinkAuthcode,
  getDefaultName,
  getPresetMeta,
  hasPartialCredentials,
  type SimpleSharePresetType,
  validatePortInput,
} from "@/lib/simple-domain/share-preset-rules";
import { sortListeningPorts } from "@/lib/simple-domain/local-port-rules";
import { Socks5PresetFields } from "@/components/simple/share-preset/Socks5PresetFields";
import { LocalPortPresetFields } from "@/components/simple/share-preset/LocalPortPresetFields";

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
      const timeoutId = window.setTimeout(() => {
        if (cancelled) {
          return;
        }
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

  const saveLocalPortPreset = async (selectedAlias: string) => {
    let combinedPorts: number[];
    try {
      ({ combinedPorts } = buildCombinedPorts({
        selectedPortList,
        manualPortsText,
      }));
    } catch (error) {
      toast.error(String(error));
      return;
    }

    if (combinedPorts.length === 0) {
      toast.error("请至少选择一个端口，或在手动输入中填写端口");
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

  const saveSocks5Preset = async (
    selectedAlias: string,
    currentPreset: SimpleSharePresetType,
  ) => {
    const trimmedName = name.trim();
    if (!trimmedName) {
      toast.error("请填写开放名称");
      return;
    }

    const trimmedUsername = username.trim();
    if (hasPartialCredentials(trimmedUsername, password)) {
      toast.error("用户名和密码需要同时填写，或同时留空以匿名开放");
      return;
    }

    const portError = validatePortInput(port);
    if (portError) {
      toast.error(portError);
      return;
    }

    const tunnel = buildPresetTunnel({
      preset: currentPreset,
      alias: selectedAlias,
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

    await saveSocks5Preset(resolvedAlias, preset);
  };

  return (
    <Dialog
      open={simpleShareDialogOpen}
      onOpenChange={(open) => !open && closeSimpleShareDialog()}
    >
      <DialogContent className={preset === "local-port" ? "max-w-2xl" : "max-w-sm"}>
        <DialogHeader>
          <DialogTitle>{meta?.title ?? "开放方式"}</DialogTitle>
          <DialogDescription>
            {meta?.description ?? "按普通模式引导填写必要信息。"}
          </DialogDescription>
        </DialogHeader>

        <div className="space-y-4">
          {preset === "local-port" ? (
            <LocalPortPresetFields
              resolvedAlias={resolvedAlias}
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
            />
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
          <Button onClick={handleSave} disabled={requiresExplicitAliasSelection}>
            {preset === "local-port"
              ? buildLocalPortSaveLabel({
                  selectedPortCount: selectedPortList.length,
                  manualPortsText,
                })
              : (meta?.saveLabel ?? "保存")}
          </Button>
        </DialogFooter>
      </DialogContent>
    </Dialog>
  );
}
