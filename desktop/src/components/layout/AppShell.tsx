import { ReactNode } from "react";
import { Settings, ArrowLeft } from "lucide-react";
import { Button } from "@/components/ui/button";
import { StatusBar } from "./StatusBar";
import { ProfileSwitcher } from "@/components/profile/ProfileSwitcher";
import { useUIStore } from "@/stores/ui-store";
import { useAgentStore } from "@/stores/agent-store";

const tabs = [
  { value: "networks", label: "网络" },
  { value: "services", label: "服务" },
  { value: "logs", label: "日志" },
] as const;

export function AppShell({ children }: { children: ReactNode }) {
  const { currentPage, setCurrentPage, activeTab, setActiveTab } = useUIStore();
  const { agentRunning } = useAgentStore();

  return (
    <div className="flex flex-col h-screen bg-background">
      {/* Header */}
      <header className="flex items-center justify-between px-3 py-1.5 shrink-0 bg-card border-b">
        <div className="flex items-center gap-1.5">
          {currentPage === "settings" ? (
            <Button
              variant="ghost"
              size="icon"
              className="h-7 w-7"
              onClick={() => setCurrentPage("main")}
            >
              <ArrowLeft className="h-3.5 w-3.5" />
            </Button>
          ) : null}
          <h1 className="text-sm font-semibold">
            {currentPage === "settings" ? "设置" : (
              <span>Remote<span className="text-primary">work</span></span>
            )}
          </h1>
        </div>

        {/* Center: tab buttons */}
        {currentPage === "main" && agentRunning && (
          <div className="flex items-center gap-0.5 bg-muted rounded-md p-0.5">
            {tabs.map((t) => (
              <button
                key={t.value}
                className={`px-3 py-0.5 text-xs font-medium rounded transition-colors ${
                  activeTab === t.value
                    ? "bg-background text-foreground shadow-sm"
                    : "text-muted-foreground hover:text-foreground"
                }`}
                onClick={() => setActiveTab(t.value)}
              >
                {t.label}
              </button>
            ))}
          </div>
        )}

        <div className="flex items-center gap-0.5">
          {currentPage === "main" && <ProfileSwitcher />}
          {currentPage === "main" && (
            <Button
              variant="ghost"
              size="icon"
              className="h-7 w-7"
              onClick={() => setCurrentPage("settings")}
            >
              <Settings className="h-3.5 w-3.5" />
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
