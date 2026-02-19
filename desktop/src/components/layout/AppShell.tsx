import { ReactNode } from "react";
import { Settings, ArrowLeft } from "lucide-react";
import { Button } from "@/components/ui/button";
import { StatusBar } from "./StatusBar";
import { ProfileSwitcher } from "@/components/profile/ProfileSwitcher";
import { useUIStore } from "@/stores/ui-store";

export function AppShell({ children }: { children: ReactNode }) {
  const { currentPage, setCurrentPage } = useUIStore();

  return (
    <div className="flex flex-col h-screen bg-background">
      {/* Header */}
      <header className="flex items-center justify-between border-b px-4 py-2">
        <div className="flex items-center gap-2">
          {currentPage === "settings" ? (
            <Button
              variant="ghost"
              size="icon"
              className="h-8 w-8"
              onClick={() => setCurrentPage("main")}
            >
              <ArrowLeft className="h-4 w-4" />
            </Button>
          ) : null}
          <h1 className="text-sm font-semibold">
            {currentPage === "settings" ? "设置" : "Remotework"}
          </h1>
        </div>
        <div className="flex items-center gap-1">
          {currentPage === "main" && <ProfileSwitcher />}
          {currentPage === "main" && (
            <Button
              variant="ghost"
              size="icon"
              className="h-8 w-8"
              onClick={() => setCurrentPage("settings")}
            >
              <Settings className="h-4 w-4" />
            </Button>
          )}
        </div>
      </header>

      {/* Content */}
      <main className="flex-1 overflow-y-auto">{children}</main>

      {/* Status Bar */}
      <StatusBar />
    </div>
  );
}
