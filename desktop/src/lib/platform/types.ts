import type { AgentConfig, ProfilesIndex } from "@/lib/config-types";

export interface SidecarLogPayload {
  source: "stdout" | "stderr";
  line: string;
}

export interface SaveDialogOptions {
  defaultPath?: string;
  filters?: { name: string; extensions: string[] }[];
}

export interface OpenDialogOptions {
  filters?: { name: string; extensions: string[] }[];
  multiple?: boolean;
}

export interface PlatformAdapter {
  // Sidecar management
  startAgent(profileName: string): Promise<number>;
  stopAgent(): Promise<void>;
  restartAgent(profileName: string): Promise<number>;
  agentRunning(): Promise<boolean>;
  getAgentPort(): Promise<number>;

  // Profile management
  listProfiles(): Promise<ProfilesIndex>;
  getProfile(name: string): Promise<Partial<AgentConfig>>;
  saveProfile(name: string, config: AgentConfig): Promise<void>;
  deleteProfile(name: string): Promise<void>;
  renameProfile(oldName: string, newName: string): Promise<void>;
  setActiveProfile(name: string): Promise<void>;
  importProfile(name: string, content: string): Promise<void>;
  exportProfile(name: string): Promise<string>;

  // System info
  getNetworkInterfaces(): Promise<string[]>;

  // Events — return an unsubscribe function
  onSidecarLog(handler: (payload: SidecarLogPayload) => void): () => void;
  onTrayToggleUIMode(handler: () => void): () => void;
  emitUIModeChanged(mode: string): void;

  // Native dialogs
  showSaveDialog(options: SaveDialogOptions): Promise<string | null>;
  showOpenDialog(options: OpenDialogOptions): Promise<string | null>;
}
