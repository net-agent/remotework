import { useMemo } from "react";
import { useProfileStore } from "@/stores/profile-store";
import { saveTunnelConfig } from "@/lib/tunnel-save";
import type { TunnelSaveContext } from "@/lib/tunnel-save";
import { toast } from "sonner";
import {
  buildPresetTunnel,
  DEFAULT_LOCAL_PORT,
  ensureLinkAuthcode,
  getPresetMeta,
  hasPartialCredentials,
} from "@/lib/simple-domain/share-preset-rules";

export interface SaveSocks5PresetInput {
  selectedAlias: string;
  name?: string;
  username?: string;
  password?: string;
  authcode?: string;
  localAddress?: string;
  context: TunnelSaveContext;
  onSaved?: () => void;
}

export async function saveSocks5Preset(input: SaveSocks5PresetInput) {
  const trimmedName =
    (input.name ?? "").trim() || getPresetMeta("socks5").defaultName;
  const trimmedUsername = (input.username ?? "").trim();
  const password = input.password ?? "";

  if (hasPartialCredentials(trimmedUsername, password)) {
    toast.error("用户名和密码需要同时填写，或同时留空以匿名开放");
    return false;
  }

  let asValue: string;
  try {
    const { asValue: extracted } = ensureLinkAuthcode({
      alias: input.selectedAlias,
      currentConfig: input.context.currentConfig,
    });
    asValue = extracted;
  } catch (err) {
    toast.error(String(err));
    return false;
  }

  const tunnel = buildPresetTunnel({
    preset: "socks5",
    alias: input.selectedAlias,
    asValue,
    name: trimmedName,
    port: DEFAULT_LOCAL_PORT,
    localAddress: input.localAddress ?? "127.0.0.1",
    username: trimmedUsername,
    password,
    authcode: input.authcode,
  });

  const saved = await saveTunnelConfig({
    tunnel,
    context: input.context,
  });

  if (saved) {
    input.onSaved?.();
  }

  return saved;
}

export function useTunnelSaveContext(): TunnelSaveContext {
  const {
    currentConfig,
    setCurrentConfig,
    saveConfig,
    activeProfile,
    setNeedsRestart,
  } = useProfileStore();

  return useMemo(
    () => ({
      currentConfig,
      setCurrentConfig,
      saveConfig,
      activeProfile,
      setNeedsRestart,
    }),
    [
      activeProfile,
      currentConfig,
      saveConfig,
      setCurrentConfig,
      setNeedsRestart,
    ],
  );
}
