import { invoke } from "@tauri-apps/api/core";
import { emit, listen } from "@tauri-apps/api/event";
import { save, open } from "@tauri-apps/plugin-dialog";
import type { AgentConfig, ProfilesIndex } from "@/lib/config-types";
import type {
  PlatformAdapter,
  SidecarLogPayload,
  SidecarExitedPayload,
  ApiEndpoint,
  SaveDialogOptions,
  OpenDialogOptions,
} from "./types";

function subscribe<T>(
  event: string,
  handler: (payload: T) => void,
): () => void {
  let cancelled = false;
  const unlistenPromise = listen<T>(event, (e) => {
    if (!cancelled) handler(e.payload);
  });
  return () => {
    cancelled = true;
    unlistenPromise.then((fn) => fn());
  };
}

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

  getApiEndpoint(): Promise<ApiEndpoint> {
    return invoke<ApiEndpoint>("get_api_endpoint");
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

  readProfileFile(path: string): Promise<string> {
    return invoke<string>("read_profile_file", { path });
  }

  // System info
  getNetworkInterfaces(): Promise<string[]> {
    return invoke<string[]>("get_network_interfaces");
  }

  // Events
  onSidecarLog(handler: (payload: SidecarLogPayload) => void): () => void {
    return subscribe<SidecarLogPayload>("sidecar-log", handler);
  }

  onSidecarExited(
    handler: (payload: SidecarExitedPayload) => void,
  ): () => void {
    return subscribe<SidecarExitedPayload>("sidecar-exited", handler);
  }

  onTrayToggleUIMode(handler: () => void): () => void {
    return subscribe<void>("tray-toggle-ui-mode", handler);
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
