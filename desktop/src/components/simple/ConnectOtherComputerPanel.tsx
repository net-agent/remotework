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
    <div className="p-4 space-y-4">
      <SimpleStatusCard
        title={connectText.title}
        description={connectText.description}
        tone={getConnectTone(session.connectState)}
      />

      <SimpleActionCard
        title="下一步"
        description={
          session.selectedShareLink
            ? `当前将使用“${session.selectedShareLink.alias}”作为连接信息。请先补全访问入口，再连接他人的电脑。`
            : "请先填写连接信息，再补全访问入口并开始连接。"
        }
        primaryLabel={session.selectedShareLink ? "配置访问入口" : "填写连接信息"}
        secondaryLabel={
          session.runtime.overallState === "stopped" ? "启动服务" : "查看高级状态"
        }
        onPrimaryClick={
          session.selectedShareLink ? actions.configureConnect : actions.createShareLink
        }
        onSecondaryClick={
          session.runtime.overallState === "stopped"
            ? actions.start
            : actions.openAdvancedServices
        }
      />

      {session.userFacingHints.length > 0 ? (
        <div className="rounded-xl border bg-card p-4 shadow-sm">
          <div className="flex items-center justify-between gap-3">
            <div>
              <h3 className="text-sm font-medium">连接提示</h3>
              <p className="mt-1 text-xs text-muted-foreground">
                若无法连接，优先检查当前连接信息、访问入口以及当前服务状态。
              </p>
            </div>
            <Button
              size="sm"
              variant="ghost"
              onClick={actions.openAdvancedServices}
            >
              查看详细信息
            </Button>
          </div>
          <ul className="mt-4 list-disc space-y-2 pl-4 text-xs text-muted-foreground">
            {session.userFacingHints.map((hint) => (
              <li key={hint}>{hint}</li>
            ))}
          </ul>
        </div>
      ) : null}
    </div>
  );
}
