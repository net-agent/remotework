import type { PlatformAdapter } from "./types";
import { TauriAdapter } from "./tauri";
import { MockAdapter } from "./mock";

const isTauri =
  typeof window !== "undefined" && "__TAURI_INTERNALS__" in window;

export const platform: PlatformAdapter = isTauri
  ? new TauriAdapter()
  : new MockAdapter();

export type {
  PlatformAdapter,
  SidecarLogPayload,
  SidecarLogLevel,
  SidecarExitedPayload,
  ApiEndpoint,
  SaveDialogOptions,
  OpenDialogOptions,
} from "./types";
