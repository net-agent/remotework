import { useEffect } from "react";
import { platform } from "@/lib/platform";
import { Toaster } from "@/components/ui/sonner";
import { TooltipProvider } from "@/components/ui/tooltip";
import { AppShell } from "@/components/layout/AppShell";
import { MainPage } from "@/pages/MainPage";
import { SettingsPage } from "@/pages/SettingsPage";
import { useUIStore } from "@/stores/ui-store";
import { useWebSocket } from "@/hooks/use-websocket";
import { useStartup } from "@/hooks/use-sidecar";
import { useSidecarLogs } from "@/hooks/use-sidecar-logs";
import { NetworkForm } from "@/components/network/NetworkForm";
import { ServiceForm } from "@/components/service/ServiceForm";
import { SharePresetDialog } from "@/components/simple/SharePresetDialog";

function App() {
  const { currentPage, uiMode } = useUIStore();
  const { boot } = useStartup();

  useWebSocket();
  useSidecarLogs();

  useEffect(() => {
    boot();
  }, [boot]);

  useEffect(() => {
    const unsubscribe = platform.onTrayToggleUIMode(() => {
      const { uiMode: currentMode, setUIMode } = useUIStore.getState();
      setUIMode(currentMode === "advanced" ? "simple" : "advanced");
    });

    return unsubscribe;
  }, []);

  useEffect(() => {
    void platform.emitUIModeChanged(uiMode);
  }, [uiMode]);

  return (
    <TooltipProvider>
      <AppShell>
        {currentPage === "main" && <MainPage />}
        {currentPage === "settings" && <SettingsPage />}
      </AppShell>
      <NetworkForm />
      <ServiceForm />
      <SharePresetDialog />
      <Toaster position="bottom-center" />
    </TooltipProvider>
  );
}

export default App;
