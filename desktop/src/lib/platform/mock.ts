import { emptyConfig } from "@/lib/config-types";
import type { AgentConfig, ProfilesIndex, ProfileMeta } from "@/lib/config-types";
import type {
  PlatformAdapter,
  SidecarLogPayload,
  SaveDialogOptions,
  OpenDialogOptions,
} from "./types";

const STORAGE_KEY = "mock-profiles";
const MOCK_PORT = 19195;

interface StoredProfiles {
  active: string;
  profiles: { meta: ProfileMeta; config: AgentConfig }[];
}

function loadStorage(): StoredProfiles {
  try {
    const raw = localStorage.getItem(STORAGE_KEY);
    if (raw) return JSON.parse(raw) as StoredProfiles;
  } catch {
    // ignore
  }
  return { active: "", profiles: [] };
}

function saveStorage(data: StoredProfiles): void {
  localStorage.setItem(STORAGE_KEY, JSON.stringify(data));
}

export class MockAdapter implements PlatformAdapter {
  // Sidecar — assume agent is either manually started or not needed
  async startAgent(_profileName: string): Promise<number> {
    return MOCK_PORT;
  }

  async stopAgent(): Promise<void> {
    // no-op
  }

  async restartAgent(_profileName: string): Promise<number> {
    return MOCK_PORT;
  }

  async agentRunning(): Promise<boolean> {
    return false;
  }

  async getAgentPort(): Promise<number> {
    return MOCK_PORT;
  }

  // Profile management backed by localStorage
  async listProfiles(): Promise<ProfilesIndex> {
    const data = loadStorage();
    return {
      active: data.active,
      profiles: data.profiles.map((p) => p.meta),
    };
  }

  async getProfile(name: string): Promise<Partial<AgentConfig>> {
    const data = loadStorage();
    const found = data.profiles.find((p) => p.meta.name === name);
    return found?.config ?? {};
  }

  async saveProfile(name: string, config: AgentConfig): Promise<void> {
    const data = loadStorage();
    const idx = data.profiles.findIndex((p) => p.meta.name === name);
    const meta: ProfileMeta = { name, filename: `${name}.json` };
    if (idx >= 0) {
      data.profiles = data.profiles.map((p, i) =>
        i === idx ? { meta, config } : p,
      );
    } else {
      data.profiles = [...data.profiles, { meta, config }];
    }
    saveStorage(data);
  }

  async deleteProfile(name: string): Promise<void> {
    const data = loadStorage();
    data.profiles = data.profiles.filter((p) => p.meta.name !== name);
    if (data.active === name) data.active = "";
    saveStorage(data);
  }

  async renameProfile(oldName: string, newName: string): Promise<void> {
    const data = loadStorage();
    data.profiles = data.profiles.map((p) =>
      p.meta.name === oldName
        ? { ...p, meta: { name: newName, filename: `${newName}.json` } }
        : p,
    );
    if (data.active === oldName) data.active = newName;
    saveStorage(data);
  }

  async setActiveProfile(name: string): Promise<void> {
    const data = loadStorage();
    data.active = name;
    saveStorage(data);
  }

  async importProfile(name: string, content: string): Promise<void> {
    const config = JSON.parse(content) as AgentConfig;
    await this.saveProfile(name, config);
  }

  async exportProfile(name: string): Promise<string> {
    const config = await this.getProfile(name);
    return JSON.stringify({ ...emptyConfig(), ...config }, null, 2);
  }

  // System info
  async getNetworkInterfaces(): Promise<string[]> {
    return ["0.0.0.0", "127.0.0.1", "192.168.1.100"];
  }

  // Events — no-op in browser
  onSidecarLog(_handler: (payload: SidecarLogPayload) => void): () => void {
    return () => {};
  }

  onTrayToggleUIMode(_handler: () => void): () => void {
    return () => {};
  }

  emitUIModeChanged(_mode: string): void {
    // no-op
  }

  // Dialogs — browser fallbacks
  async showSaveDialog(options: SaveDialogOptions): Promise<string | null> {
    // In browser mode, save dialog is not meaningful for file paths.
    // Return the defaultPath as a hint — callers use Blob download anyway.
    return options.defaultPath ?? null;
  }

  async showOpenDialog(_options: OpenDialogOptions): Promise<string | null> {
    // Use a hidden file input to let user pick a file, return content via data URL
    return new Promise<string | null>((resolve) => {
      const input = document.createElement("input");
      input.type = "file";
      if (_options.filters?.length) {
        input.accept = _options.filters
          .flatMap((f) => f.extensions.map((ext) => `.${ext}`))
          .join(",");
      }
      input.onchange = () => {
        const file = input.files?.[0];
        if (!file) {
          resolve(null);
          return;
        }
        const reader = new FileReader();
        reader.onload = () => resolve(reader.result as string);
        reader.onerror = () => resolve(null);
        reader.readAsText(file);
      };
      // If the dialog is cancelled, resolve null
      // (there's no reliable cancel event, but the change event won't fire)
      input.click();
      // Fallback: if no selection after 60s, resolve null
      const timeout = window.setTimeout(() => resolve(null), 60000);
      const origOnChange = input.onchange;
      input.onchange = (e) => {
        window.clearTimeout(timeout);
        (origOnChange as EventListener)?.(e);
      };
    });
  }
}
