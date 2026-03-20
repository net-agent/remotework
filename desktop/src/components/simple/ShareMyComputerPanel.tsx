import { Button } from "@/components/ui/button";
import { SimpleShareServiceRowCard } from "@/components/simple/SimpleShareServiceRow";
import type { SimpleSessionVM } from "@/lib/view-model/simple-session-vm";
import { useSimpleActions } from "@/lib/view-model/simple-actions";
import { cn } from "@/lib/utils";
import { toast } from "sonner";

function getSummaryTone(state: SimpleSessionVM["shareState"]) {
  switch (state) {
    case "ready":
      return "text-emerald-600 dark:text-emerald-400";
    case "degraded":
      return "text-amber-600 dark:text-amber-400";
    default:
      return "text-muted-foreground";
  }
}

function getSummaryText(input: {
  shareState: SimpleSessionVM["shareState"];
  openCount: number;
  activeConnectionCount: number;
}) {
  if (input.shareState === "ready") {
    return input.activeConnectionCount > 0
      ? `当前可共享，已开放 ${input.openCount} 项服务，当前 ${input.activeConnectionCount} 个会话`
      : `当前可共享，已开放 ${input.openCount} 项服务`;
  }

  if (input.shareState === "degraded") {
    return "共享配置存在异常";
  }

  return "请选择连接信息并开放服务";
}

async function copyAddress(listenURL: string) {
  try {
    await navigator.clipboard.writeText(listenURL);
    toast.success("已复制虚拟地址");
  } catch (error) {
    toast.error(`复制失败: ${String(error)}`);
  }
}

export function ShareMyComputerPanel({
  session,
}: {
  session: SimpleSessionVM;
}) {
  const actions = useSimpleActions();
  const selectedAlias = session.selectedShareLink?.alias ?? null;
  const openCount = session.shareServiceRows.filter((row) => row.isOpen).length;

  return (
    <div className="space-y-4">
      <div className="rounded-xl border bg-card p-4 shadow-sm">
        <div className="flex flex-col gap-3 border-b pb-3 sm:flex-row sm:items-start sm:justify-between">
          <div className="space-y-1">
            <div className="text-xs">
              <span
                className={cn(
                  "font-medium",
                  getSummaryTone(session.shareState),
                )}
              >
                {getSummaryText({
                  shareState: session.shareState,
                  openCount,
                  activeConnectionCount: session.activeConnectionCount,
                })}
              </span>
            </div>
          </div>
          <div className="flex flex-wrap gap-2">
            {!selectedAlias ? (
              <Button size="sm" onClick={actions.createShareLink}>
                输入连接信息
              </Button>
            ) : null}
            {session.requiresRestart ? (
              <Button size="sm" variant="outline" onClick={actions.restart}>
                重启后生效
              </Button>
            ) : null}
            <Button
              size="sm"
              variant="outline"
              onClick={actions.openShareServiceForm}
            >
              完整设置
            </Button>
          </div>
        </div>

        {!selectedAlias ? (
          <div className="pt-3 text-xs text-muted-foreground">
            先选择连接信息，再开放或管理共享服务。
          </div>
        ) : null}

        <div className="mt-1 divide-y pt-3">
          {session.shareServiceRows.map((row) => (
            <SimpleShareServiceRowCard
              key={row.key}
              row={row}
              disabled={!selectedAlias}
              onOpen={() => actions.openSimpleShareServiceDialog(row.key)}
              onClose={() =>
                row.primaryActionKind === "manage"
                  ? actions.openSimpleShareServiceDialog(row.key)
                  : actions.closeSimpleShareService({
                      tunnelId: row.tunnelId ?? undefined,
                      configIndex: row.configIndex ?? undefined,
                    })
              }
              onCopyAddress={(listenURL) => void copyAddress(listenURL)}
              onCloseItem={(input) => actions.closeSimpleShareService(input)}
            />
          ))}
        </div>
      </div>

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
              type="button"
              variant="ghost"
              onClick={actions.openAdvancedNetworks}
            >
              进入高级模式
            </Button>
          </div>
        </div>
      ) : null}
    </div>
  );
}
