import { create } from "zustand";
import { invoke } from "@tauri-apps/api/core";
import type {
  AgentConfig,
  ProfileMeta,
  ProfilesIndex,
} from "@/lib/config-types";
import { emptyConfig } from "@/lib/config-types";

interface ProfileState {
  profiles: ProfileMeta[];
  activeProfile: string;
  currentConfig: AgentConfig;
  dirty: boolean;
  needsRestart: boolean;

  loadProfiles: () => Promise<void>;
  loadConfig: (name: string) => Promise<void>;
  saveConfig: (name: string, config: AgentConfig) => Promise<void>;
  createProfile: (name: string) => Promise<void>;
  deleteProfile: (name: string) => Promise<void>;
  renameProfile: (oldName: string, newName: string) => Promise<void>;
  setActiveProfile: (name: string) => Promise<void>;
  setCurrentConfig: (config: AgentConfig) => void;
  setDirty: (dirty: boolean) => void;
  setNeedsRestart: () => void;
  clearNeedsRestart: () => void;
}

export const useProfileStore = create<ProfileState>((set, get) => ({
  profiles: [],
  activeProfile: "",
  currentConfig: emptyConfig(),
  dirty: false,
  needsRestart: false,

  loadProfiles: async () => {
    const index = await invoke<ProfilesIndex>("list_profiles");
    set({ profiles: index.profiles, activeProfile: index.active });
  },

  loadConfig: async (name: string) => {
    const raw = await invoke<Partial<AgentConfig>>("get_profile", { name });
    const defaults = emptyConfig();
    const config: AgentConfig = {
      ...defaults,
      ...raw,
      links: raw.links ?? defaults.links,
      tunnels: raw.tunnels ?? defaults.tunnels,
      pprof: raw.pprof ?? defaults.pprof,
      api: raw.api ?? defaults.api,
    };
    set({ currentConfig: config, dirty: false });
  },

  saveConfig: async (name: string, config: AgentConfig) => {
    await invoke("save_profile", { name, config });
    set({ currentConfig: config, dirty: false });
    await get().loadProfiles();
  },

  createProfile: async (name: string) => {
    const config = emptyConfig();
    await invoke("save_profile", { name, config });
    await get().loadProfiles();
  },

  deleteProfile: async (name: string) => {
    await invoke("delete_profile", { name });
    await get().loadProfiles();
  },

  renameProfile: async (oldName: string, newName: string) => {
    await invoke("rename_profile", { oldName, newName });
    await get().loadProfiles();
  },

  setActiveProfile: async (name: string) => {
    await invoke("set_active_profile", { name });
    set({ activeProfile: name });
  },

  setCurrentConfig: (config) => set({ currentConfig: config, dirty: true }),
  setDirty: (dirty) => set({ dirty }),
  setNeedsRestart: () => set({ needsRestart: true }),
  clearNeedsRestart: () => set({ needsRestart: false }),
}));
