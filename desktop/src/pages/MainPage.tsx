import { NetworksTab } from "@/components/tabs/NetworksTab";
import { ServicesTab } from "@/components/tabs/ServicesTab";
import { LogsTab } from "@/components/tabs/LogsTab";
import { ConfigTab } from "@/components/tabs/ConfigTab";
import { Button } from "@/components/ui/button";
import { useAgentStore } from "@/stores/agent-store";
import { useProfileStore } from "@/stores/profile-store";
import { useUIStore } from "@/stores/ui-store";
import { useSidecar } from "@/hooks/use-sidecar";
import { RotateCw, Play, AlertTriangle } from "lucide-react";
import { toast } from "sonner";

export function MainPage() {
  const { agentRunning, startError } = useAgentStore();
  const { activeTab } = useUIStore();
  const { needsRestart, activeProfile, clearNeedsRestart } = useProfileStore();
  const { startAgent, restartAgent } = useSidecar();

  const handleRestart = async () => {
    if (!activeProfile) return;
    try {
      await restartAgent(activeProfile);
      clearNeedsRestart();
      toast.success("Agent 已重启");
    } catch (e) {
      toast.error(`重启失败: ${e}`);
    }
  };

  const handleStart = async () => {
    if (!activeProfile) return;
    try {
      await startAgent(activeProfile);
      toast.success("Agent 已启动");
    } catch (e) {
      toast.error(`启动失败: ${e}`);
    }
  };

  // Agent not running — show config editing UI so user can fix issues
  if (!agentRunning) {
    return (
      <div className="flex flex-col h-full">
        {/* Startup error banner */}
        {startError && (
          <div className="px-3 py-2 bg-red-50 dark:bg-red-950/30 border-b border-red-200 dark:border-red-800 text-xs shrink-0">
            <div className="flex items-start gap-2">
              <AlertTriangle className="h-3.5 w-3.5 text-red-500 shrink-0 mt-0.5" />
              <div className="min-w-0">
                <p className="font-medium text-red-700 dark:text-red-400">
                  Agent 启动失败
                </p>
                <pre className="mt-1 whitespace-pre-wrap text-red-600 dark:text-red-400/80 break-all">
                  {startError}
                </pre>
              </div>
            </div>
          </div>
        )}

        {/* Action bar */}
        <div className="flex items-center justify-center gap-3 px-4 py-3 border-b shrink-0">
          <span className="text-xs text-muted-foreground">
            {activeProfile
              ? `Profile: ${activeProfile} — Agent 未运行`
              : "请先选择一个 Profile"}
          </span>
          {activeProfile && (
            <Button size="sm" className="h-7 text-xs" onClick={handleStart}>
              <Play className="h-3 w-3 mr-1" />
              启动
            </Button>
          )}
        </div>

        {/* Config editing — allow user to view and fix config even when agent is down */}
        {activeProfile && (
          <div className="flex-1 min-h-0">
            <ConfigTab />
          </div>
        )}
      </div>
    );
  }

  return (
    <div className="flex flex-col h-full">
      {needsRestart && (
        <div className="flex items-center justify-between px-3 py-1.5 bg-amber-50 dark:bg-amber-950/30 border-b border-amber-200 dark:border-amber-800 text-xs text-amber-700 dark:text-amber-400 shrink-0">
          <span>配置已修改，重启后生效</span>
          <Button
            variant="ghost"
            size="sm"
            className="h-5 text-xs px-1.5 text-amber-700 dark:text-amber-400 hover:text-amber-900"
            onClick={handleRestart}
          >
            <RotateCw className="h-3 w-3 mr-1" />
            重启
          </Button>
        </div>
      )}
      <div className="flex-1 min-h-0">
        {activeTab === "networks" && <NetworksTab />}
        {activeTab === "services" && <ServicesTab />}
        {activeTab === "logs" && <LogsTab />}
      </div>
    </div>
  );
}
