import type { AgentConfig, ProfilesIndex } from "@/lib/config-types";

export type SidecarLogLevel = "debug" | "info" | "warn" | "error";

export interface SidecarLogPayload {
  source: "stdout" | "stderr";
  level: SidecarLogLevel;
  module?: string;
  msg?: string;
  line: string; // 原始行文本，msg 缺失时回退展示
}

export interface SidecarExitedPayload {
  code: number | null;
  intended: boolean; // false 且 code !== 0 表示异常退出
}

export interface ApiEndpoint {
  port: number;
  token: string | null;
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
  getApiEndpoint(): Promise<ApiEndpoint>;

  // Profile management
  listProfiles(): Promise<ProfilesIndex>;
  getProfile(name: string): Promise<Partial<AgentConfig>>;
  saveProfile(name: string, config: AgentConfig): Promise<void>;
  deleteProfile(name: string): Promise<void>;
  renameProfile(oldName: string, newName: string): Promise<void>;
  setActiveProfile(name: string): Promise<void>;
  importProfile(name: string, content: string): Promise<void>;
  exportProfile(name: string): Promise<string>;
  readProfileFile(path: string): Promise<string>;

  // System info
  getNetworkInterfaces(): Promise<string[]>;

  // Events — return an unsubscribe function
  onSidecarLog(handler: (payload: SidecarLogPayload) => void): () => void;
  onSidecarExited(handler: (payload: SidecarExitedPayload) => void): () => void;
  onTrayToggleUIMode(handler: () => void): () => void;
  emitUIModeChanged(mode: string): void;

  // Native dialogs
  showSaveDialog(options: SaveDialogOptions): Promise<string | null>;
  showOpenDialog(options: OpenDialogOptions): Promise<string | null>;
}
