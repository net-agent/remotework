import { useEffect } from "react";
import { NetworksTab } from "@/components/tabs/NetworksTab";
import { ServicesTab } from "@/components/tabs/ServicesTab";
import { LogsTab } from "@/components/tabs/LogsTab";
import { ConfigTab } from "@/components/tabs/ConfigTab";
import { ShareMyComputerPanel } from "@/components/simple/ShareMyComputerPanel";
import { ConnectOtherComputerPanel } from "@/components/simple/ConnectOtherComputerPanel";
import { Button } from "@/components/ui/button";
import { useAgentStore } from "@/stores/agent-store";
import { useProfileStore } from "@/stores/profile-store";
import { useUIStore } from "@/stores/ui-store";
import { useSimpleActions } from "@/lib/view-model/simple-actions";
import { buildSimpleSessionVM } from "@/lib/view-model/simple-session-vm";
import { RotateCw, Play, AlertTriangle } from "lucide-react";

function RestartBanner() {
  const { needsRestart } = useProfileStore();
  const actions = useSimpleActions();

  if (!needsRestart) {
    return null;
  }

  return (
    <div className="flex items-center justify-between border-b border-amber-200 bg-amber-50 px-3 py-1.5 text-xs text-amber-700 dark:border-amber-800 dark:bg-amber-950/30 dark:text-amber-400 shrink-0">
      <span>配置已修改，重启后生效</span>
      <Button
        variant="ghost"
        size="sm"
        className="h-5 px-1.5 text-xs text-amber-700 hover:text-amber-900 dark:text-amber-400"
        onClick={() => void actions.restart()}
      >
        <RotateCw className="mr-1 h-3 w-3" />
        重启
      </Button>
    </div>
  );
}

// sidecar 配置解析失败进入降级模式（/status mode=degraded）时的引导横幅
function DegradedBanner() {
  const { status } = useAgentStore();
  const { setUIMode, setAdvancedTab } = useUIStore();

  if (status?.mode !== "degraded") {
    return null;
  }

  return (
    <div className="flex items-center justify-between gap-3 border-b border-red-200 bg-red-50 px-3 py-1.5 text-xs text-red-700 dark:border-red-800 dark:bg-red-950/30 dark:text-red-400 shrink-0">
      <div className="flex min-w-0 items-start gap-2">
        <AlertTriangle className="mt-0.5 h-3.5 w-3.5 shrink-0" />
        <div className="min-w-0">
          <span className="font-medium">配置文件解析失败，Agent 以降级模式运行</span>
          {status.configError ? (
            <p className="mt-0.5 break-all whitespace-pre-wrap text-red-600 dark:text-red-400/80">
              {status.configError}
            </p>
          ) : null}
        </div>
      </div>
      <Button
        variant="ghost"
        size="sm"
        className="h-5 shrink-0 px-1.5 text-xs text-red-700 hover:text-red-900 dark:text-red-400"
        onClick={() => {
          setUIMode("advanced");
          setAdvancedTab("config");
        }}
      >
        前往配置页
      </Button>
    </div>
  );
}

function AdvancedMainPage() {
  const { advancedTab } = useUIStore();

  return (
    <div className="flex h-full flex-col">
      <DegradedBanner />
      <RestartBanner />
      <div className="min-h-0 flex-1">
        {advancedTab === "networks" && <NetworksTab />}
        {advancedTab === "services" && <ServicesTab />}
        {advancedTab === "logs" && <LogsTab />}
        {advancedTab === "config" && <ConfigTab />}
      </div>
    </div>
  );
}

function SimpleMainPage() {
  const {
    simpleTask,
    selectedSimpleShareLinkAlias,
    setSelectedSimpleShareLinkAlias,
  } = useUIStore();
  const { agentRunning, wsConnected, networks, services, streams, startError } =
    useAgentStore();
  const { currentConfig, needsRestart, activeProfile } = useProfileStore();
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
  const resolvedSelectedAlias = session.selectedShareLink?.alias ?? null;

  useEffect(() => {
    if (resolvedSelectedAlias !== selectedSimpleShareLinkAlias) {
      setSelectedSimpleShareLinkAlias(resolvedSelectedAlias);
    }
  }, [
    resolvedSelectedAlias,
    selectedSimpleShareLinkAlias,
    setSelectedSimpleShareLinkAlias,
  ]);

  return (
    <div className="flex h-full flex-col">
      {startError ? (
        <div className="shrink-0 border-b border-red-200 bg-red-50 px-3 py-2 text-xs dark:border-red-800 dark:bg-red-950/30">
          <div className="flex items-start gap-2">
            <AlertTriangle className="mt-0.5 h-3.5 w-3.5 shrink-0 text-red-500" />
            <div className="min-w-0">
              <p className="font-medium text-red-700 dark:text-red-400">
                服务启动失败
              </p>
              <pre className="mt-1 break-all whitespace-pre-wrap text-red-600 dark:text-red-400/80">
                {startError}
              </pre>
            </div>
          </div>
        </div>
      ) : null}

      <DegradedBanner />

      <RestartBanner />

      {!agentRunning && activeProfile ? (
        <div className="flex shrink-0 items-center justify-center gap-3 border-b bg-card px-4 py-3">
          <span className="text-xs text-muted-foreground">
            当前配置方案未运行，启动后才能进入共享或连接流程
          </span>
          <Button
            size="sm"
            className="h-7 text-xs"
            onClick={() => void actions.start()}
          >
            <Play className="mr-1 h-3 w-3" />
            启动
          </Button>
        </div>
      ) : null}

      {!activeProfile ? (
        <div className="flex flex-1 items-center justify-center px-8 text-center text-sm text-muted-foreground">
          请先创建或选择一个配置方案。
        </div>
      ) : (
        <div className="min-h-0 flex-1 overflow-auto px-4 py-4">
          <div className="mx-auto flex max-w-5xl flex-col gap-3">
            <div className="min-w-0">
              {simpleTask === "share" ? (
                <ShareMyComputerPanel session={session} />
              ) : (
                <ConnectOtherComputerPanel session={session} />
              )}
            </div>
          </div>
        </div>
      )}
    </div>
  );
}

export function MainPage() {
  const { uiMode } = useUIStore();

  return uiMode === "advanced" ? <AdvancedMainPage /> : <SimpleMainPage />;
}
