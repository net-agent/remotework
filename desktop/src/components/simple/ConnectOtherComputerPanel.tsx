import { Button } from "@/components/ui/button";
import { SimpleActionCard } from "@/components/simple/SimpleActionCard";
import { SimpleStatusCard } from "@/components/simple/SimpleStatusCard";
import type { SimpleSessionVM } from "@/lib/view-model/simple-session-vm";
import { useSimpleActions } from "@/lib/view-model/simple-actions";

function getConnectTone(state: SimpleSessionVM["connectState"]) {
  switch (state) {
    case "connected":
      return "success" as const;
    case "validating":
      return "warning" as const;
    case "error":
      return "danger" as const;
    default:
      return "muted" as const;
  }
}

function getConnectText(state: SimpleSessionVM["connectState"]) {
  switch (state) {
    case "connected":
      return {
        title: "连接已建立",
        description: "当前已经有连接中的会话，可继续使用目标电脑。",
      };
    case "validating":
      return {
        title: "正在尝试连接",
        description: "系统正在建立连接或恢复状态，请稍候。",
      };
    case "error":
      return {
        title: "连接遇到问题",
        description: "连接信息可能有误，或目标暂时不可达。",
      };
    default:
      return {
        title: "尚未开始连接",
        description: "先准备好访问入口，再连接他人的电脑。",
      };
  }
}

export function ConnectOtherComputerPanel({
  session,
}: {
  session: SimpleSessionVM;
}) {
  const actions = useSimpleActions();
  const connectText = getConnectText(session.connectState);

  return (
    <div className="space-y-4">
      <SimpleStatusCard
        title={connectText.title}
        description={connectText.description}
        tone={getConnectTone(session.connectState)}
        extra={
          session.runtime.overallState === "stopped" ? (
            <Button size="sm" variant="outline" onClick={actions.start}>
              启动服务
            </Button>
          ) : undefined
        }
      />

      <SimpleActionCard
        title="连接他人电脑"
        description={
          session.selectedShareLink
            ? `当前使用“${session.selectedShareLink.alias}”作为连接信息。下一步去配置访问入口。`
            : "请先填写连接信息，再继续配置访问入口。"
        }
        primaryLabel={
          session.selectedShareLink ? "配置访问入口" : "填写连接信息"
        }
        secondaryLabel={
          session.runtime.overallState === "stopped"
            ? undefined
            : "查看高级状态"
        }
        onPrimaryClick={
          session.selectedShareLink
            ? actions.configureConnect
            : actions.createShareLink
        }
        onSecondaryClick={
          session.runtime.overallState === "stopped"
            ? undefined
            : actions.openAdvancedServices
        }
      />

      {session.userFacingHints.length > 0 ? (
        <div className="rounded-xl border border-dashed bg-muted/20 px-4 py-3">
          <div className="flex flex-col gap-3 sm:flex-row sm:items-start sm:justify-between">
            <div className="space-y-1">
              <h3 className="text-sm font-medium">处理提示</h3>
              <ul className="list-disc space-y-1 pl-4 text-xs text-muted-foreground">
                {session.userFacingHints.slice(0, 3).map((hint) => (
                  <li key={hint}>{hint}</li>
                ))}
              </ul>
            </div>
            <Button
              size="sm"
              variant="ghost"
              onClick={actions.openAdvancedServices}
            >
              查看详细信息
            </Button>
          </div>
        </div>
      ) : null}
    </div>
  );
}
