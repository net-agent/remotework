import { useEffect } from "react";
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

function App() {
  const { currentPage } = useUIStore();
  const { boot } = useStartup();

  useWebSocket();
  useSidecarLogs();

  useEffect(() => {
    boot();
  }, [boot]);

  return (
    <TooltipProvider>
      <AppShell>
        {currentPage === "main" && <MainPage />}
        {currentPage === "settings" && <SettingsPage />}
      </AppShell>
      <NetworkForm />
      <ServiceForm />
      <Toaster position="bottom-center" />
    </TooltipProvider>
  );
}

export default App;
