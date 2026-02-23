import { NetworksTab } from "@/components/tabs/NetworksTab";
import { ServicesTab } from "@/components/tabs/ServicesTab";
import { LogsTab } from "@/components/tabs/LogsTab";
import { Button } from "@/components/ui/button";
import { useAgentStore } from "@/stores/agent-store";
import { useProfileStore } from "@/stores/profile-store";
import { useUIStore } from "@/stores/ui-store";
import { useSidecar } from "@/hooks/use-sidecar";
import { RotateCw } from "lucide-react";
import { toast } from "sonner";

export function MainPage() {
  const { agentRunning } = useAgentStore();
  const { activeTab } = useUIStore();
  const { needsRestart, activeProfile, clearNeedsRestart } = useProfileStore();
  const { restartAgent } = useSidecar();

  if (!agentRunning) {
    return (
      <div className="flex flex-col items-center justify-center h-full gap-2 text-center px-8">
        <p className="text-sm text-muted-foreground">
          Agent 未运行，请选择一个 Profile 启动
        </p>
      </div>
    );
  }

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
