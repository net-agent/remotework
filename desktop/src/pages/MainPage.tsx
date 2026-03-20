import { NetworksTab } from "@/components/tabs/NetworksTab";
import { ServicesTab } from "@/components/tabs/ServicesTab";
import { LogsTab } from "@/components/tabs/LogsTab";
import { ConfigTab } from "@/components/tabs/ConfigTab";
import { ShareMyComputerPanel } from "@/components/simple/ShareMyComputerPanel";
import { ConnectOtherComputerPanel } from "@/components/simple/ConnectOtherComputerPanel";
import { SimpleVirtualNetworkPicker } from "@/components/simple/SimpleVirtualNetworkPicker";
import { Button } from "@/components/ui/button";
import { useAgentStore } from "@/stores/agent-store";
import { useProfileStore } from "@/stores/profile-store";
import { useUIStore } from "@/stores/ui-store";
import { useSidecar } from "@/hooks/use-sidecar";
import { useSimpleActions } from "@/lib/view-model/simple-actions";
import { buildSimpleSessionVM } from "@/lib/view-model/simple-session-vm";
import { RotateCw, Play, AlertTriangle } from "lucide-react";
import { toast } from "sonner";

function RestartBanner() {
  const { needsRestart, activeProfile, clearNeedsRestart } = useProfileStore();
  const { restartAgent } = useSidecar();

  const handleRestart = async () => {
    if (!activeProfile) return;
    try {
      await restartAgent(activeProfile);
      clearNeedsRestart();
      toast.success("Agent 已重启");
    } catch (error) {
      toast.error(`重启失败: ${error}`);
    }
  };

  if (!needsRestart) {
    return null;
  }

  return (
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
  );
}

function AdvancedMainPage() {
  const { advancedTab } = useUIStore();

  return (
    <div className="flex flex-col h-full">
      <RestartBanner />
      <div className="flex-1 min-h-0">
        {advancedTab === "networks" && <NetworksTab />}
        {advancedTab === "services" && <ServicesTab />}
        {advancedTab === "logs" && <LogsTab />}
        {advancedTab === "config" && <ConfigTab />}
      </div>
    </div>
  );
}

function SimpleMainPage() {
  const { simpleTask, selectedSimpleShareLinkAlias } = useUIStore();
  const { agentRunning, wsConnected, networks, services, streams, startError } =
    useAgentStore();
  const { currentConfig, needsRestart, activeProfile } = useProfileStore();
  const { startAgent } = useSidecar();
  const actions = useSimpleActions();

  const session = buildSimpleSessionVM({
    agentRunning,
    wsConnected,
    networks,
    services,
    streams,
    currentConfig,
    needsRestart,
    selectedShareLinkAlias: selectedSimpleShareLinkAlias,
  });

  const handleStart = async () => {
    if (!activeProfile) return;
    try {
      await startAgent(activeProfile);
      toast.success("Agent 已启动");
    } catch (error) {
      toast.error(`启动失败: ${error}`);
    }
  };

  return (
    <div className="flex flex-col h-full">
      {startError ? (
        <div className="px-3 py-2 bg-red-50 dark:bg-red-950/30 border-b border-red-200 dark:border-red-800 text-xs shrink-0">
          <div className="flex items-start gap-2">
            <AlertTriangle className="h-3.5 w-3.5 text-red-500 shrink-0 mt-0.5" />
            <div className="min-w-0">
              <p className="font-medium text-red-700 dark:text-red-400">
                服务启动失败
              </p>
              <pre className="mt-1 whitespace-pre-wrap text-red-600 dark:text-red-400/80 break-all">
                {startError}
              </pre>
            </div>
          </div>
        </div>
      ) : null}

      <RestartBanner />

      {!agentRunning && activeProfile ? (
        <div className="flex items-center justify-center gap-3 px-4 py-3 border-b shrink-0 bg-card">
          <span className="text-xs text-muted-foreground">
            当前配置方案未运行，启动后才能进入共享或连接流程
          </span>
          <Button size="sm" className="h-7 text-xs" onClick={handleStart}>
            <Play className="h-3 w-3 mr-1" />
            启动
          </Button>
        </div>
      ) : null}

      {!activeProfile ? (
        <div className="flex-1 flex items-center justify-center px-8 text-center text-sm text-muted-foreground">
          请先创建或选择一个配置方案。
        </div>
      ) : (
        <>
          <div className="px-4 pt-4">
            <SimpleVirtualNetworkPicker
              links={session.availableShareLinks}
              selectedAlias={session.selectedShareLink?.alias ?? null}
              hint={session.shareLinkHint}
              onSelect={actions.selectShareLink}
              onCreate={actions.createShareLink}
              onEdit={actions.editShareLink}
            />
          </div>
          {simpleTask === "share" ? (
            <ShareMyComputerPanel session={session} />
          ) : (
            <ConnectOtherComputerPanel session={session} />
          )}
        </>
      )}
    </div>
  );
}

export function MainPage() {
  const { uiMode } = useUIStore();

  return uiMode === "advanced" ? <AdvancedMainPage /> : <SimpleMainPage />;
}
