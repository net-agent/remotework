import { ReactNode } from "react";
import { Settings, ArrowLeft } from "lucide-react";
import { Button } from "@/components/ui/button";
import { StatusBar } from "./StatusBar";
import { ProfileSwitcher } from "@/components/profile/ProfileSwitcher";
import { useUIStore } from "@/stores/ui-store";

const simpleTasks = [
  { value: "share", label: "共享我的电脑" },
  { value: "connect", label: "连接他人的电脑" },
] as const;

const advancedTabs = [
  { value: "networks", label: "网络" },
  { value: "services", label: "服务" },
  { value: "logs", label: "日志" },
  { value: "config", label: "配置" },
] as const;

export function AppShell({ children }: { children: ReactNode }) {
  const {
    currentPage,
    setCurrentPage,
    uiMode,
    simpleTask,
    setSimpleTask,
    advancedTab,
    setAdvancedTab,
  } = useUIStore();

  return (
    <div className="flex flex-col h-screen bg-background">
      <header className="flex items-center justify-between px-3 py-1.5 shrink-0 bg-card border-b gap-3">
        <div className="flex items-center gap-1.5 min-w-0">
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
          <h1 className="text-sm font-semibold truncate">
            {currentPage === "settings" ? (
              "设置"
            ) : (
              <span>
                Remote<span className="text-primary">work</span>
              </span>
            )}
          </h1>
        </div>

        {currentPage === "main" ? (
          <div className="flex items-center gap-2 min-w-0 flex-1 justify-center">
            <div className="flex items-center gap-0.5 bg-muted rounded-md p-0.5 overflow-x-auto">
              {uiMode === "simple"
                ? simpleTasks.map((task) => (
                    <button
                      key={task.value}
                      className={`px-3 py-0.5 text-xs font-medium rounded transition-colors whitespace-nowrap ${
                        simpleTask === task.value
                          ? "bg-background text-foreground shadow-sm"
                          : "text-muted-foreground hover:text-foreground"
                      }`}
                      onClick={() => setSimpleTask(task.value)}
                    >
                      {task.label}
                    </button>
                  ))
                : advancedTabs.map((tab) => (
                    <button
                      key={tab.value}
                      className={`px-3 py-0.5 text-xs font-medium rounded transition-colors whitespace-nowrap ${
                        advancedTab === tab.value
                          ? "bg-background text-foreground shadow-sm"
                          : "text-muted-foreground hover:text-foreground"
                      }`}
                      onClick={() => setAdvancedTab(tab.value)}
                    >
                      {tab.label}
                    </button>
                  ))}
            </div>
          </div>
        ) : (
          <div className="flex-1" />
        )}

        <div className="flex items-center gap-0.5 shrink-0">
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

      <main className="flex-1 overflow-y-auto">{children}</main>
      <StatusBar />
    </div>
  );
}
