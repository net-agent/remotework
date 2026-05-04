import { invoke } from "@tauri-apps/api/core";
import { emit, listen } from "@tauri-apps/api/event";
import { save, open } from "@tauri-apps/plugin-dialog";
import type { AgentConfig, ProfilesIndex } from "@/lib/config-types";
import type {
  PlatformAdapter,
  SidecarLogPayload,
  SaveDialogOptions,
  OpenDialogOptions,
} from "./types";

export class TauriAdapter implements PlatformAdapter {
  // Sidecar management
  startAgent(profileName: string): Promise<number> {
    return invoke<number>("start_agent", { profileName });
  }

  async stopAgent(): Promise<void> {
    await invoke("stop_agent");
  }

  restartAgent(profileName: string): Promise<number> {
    return invoke<number>("restart_agent", { profileName });
  }

  agentRunning(): Promise<boolean> {
    return invoke<boolean>("agent_running");
  }

  getAgentPort(): Promise<number> {
    return invoke<number>("get_agent_port");
  }

  // Profile management
  listProfiles(): Promise<ProfilesIndex> {
    return invoke<ProfilesIndex>("list_profiles");
  }

  getProfile(name: string): Promise<Partial<AgentConfig>> {
    return invoke<Partial<AgentConfig>>("get_profile", { name });
  }

  async saveProfile(name: string, config: AgentConfig): Promise<void> {
    await invoke("save_profile", { name, config });
  }

  async deleteProfile(name: string): Promise<void> {
    await invoke("delete_profile", { name });
  }

  async renameProfile(oldName: string, newName: string): Promise<void> {
    await invoke("rename_profile", { oldName, newName });
  }

  async setActiveProfile(name: string): Promise<void> {
    await invoke("set_active_profile", { name });
  }

  async importProfile(name: string, content: string): Promise<void> {
    await invoke("import_profile", { name, content });
  }

  exportProfile(name: string): Promise<string> {
    return invoke<string>("export_profile", { name });
  }

  // System info
  getNetworkInterfaces(): Promise<string[]> {
    return invoke<string[]>("get_network_interfaces");
  }

  // Events
  onSidecarLog(handler: (payload: SidecarLogPayload) => void): () => void {
    let cancelled = false;
    const unlistenPromise = listen<SidecarLogPayload>(
      "sidecar-log",
      (event) => {
        if (!cancelled) handler(event.payload);
      },
    );
    return () => {
      cancelled = true;
      unlistenPromise.then((fn) => fn());
    };
  }

  onTrayToggleUIMode(handler: () => void): () => void {
    let cancelled = false;
    const unlistenPromise = listen("tray-toggle-ui-mode", () => {
      if (!cancelled) handler();
    });
    return () => {
      cancelled = true;
      unlistenPromise.then((fn) => fn());
    };
  }

  emitUIModeChanged(mode: string): void {
    void emit("ui-mode-changed", mode);
  }

  // Native dialogs
  async showSaveDialog(options: SaveDialogOptions): Promise<string | null> {
    const path = await save({
      defaultPath: options.defaultPath,
      filters: options.filters,
    });
    return path ?? null;
  }

  async showOpenDialog(options: OpenDialogOptions): Promise<string | null> {
    const file = await open({
      filters: options.filters,
      multiple: options.multiple ?? false,
    });
    if (file === null) return null;
    // open() returns string | string[] depending on `multiple`
    return Array.isArray(file) ? file[0] ?? null : file;
  }
}
