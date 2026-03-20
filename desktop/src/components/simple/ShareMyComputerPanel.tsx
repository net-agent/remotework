import { Button } from "@/components/ui/button";
import { SimpleShareServiceRowCard } from "@/components/simple/SimpleShareServiceRow";
import type { SimpleSessionVM } from "@/lib/view-model/simple-session-vm";
import { useSimpleActions } from "@/lib/view-model/simple-actions";
import { cn } from "@/lib/utils";

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

function getSummaryText(state: SimpleSessionVM["shareState"]) {
  switch (state) {
    case "ready":
      return "已基本可共享";
    case "degraded":
      return "共享配置存在异常";
    default:
      return "请选择连接信息并开放服务";
  }
}

export function ShareMyComputerPanel({
  session,
}: {
  session: SimpleSessionVM;
}) {
  const actions = useSimpleActions();
  const selectedAlias = session.selectedShareLink?.alias ?? null;
  const savedNeedsCheckCount =
    session.config.allLinks.length - session.availableShareLinks.length;

  return (
    <div className="space-y-4 p-4">
      <div className="rounded-xl border bg-card px-4 py-3 shadow-sm">
        <div className="flex flex-col gap-3 lg:flex-row lg:items-center lg:justify-between">
          <div className="flex flex-wrap items-center gap-x-4 gap-y-2 text-xs">
            <div className="flex items-center gap-2">
              <span className="text-muted-foreground">当前状态</span>
              <span
                className={cn(
                  "font-medium",
                  getSummaryTone(session.shareState),
                )}
              >
                {getSummaryText(session.shareState)}
              </span>
            </div>
            <div className="flex items-center gap-2">
              <span className="text-muted-foreground">已开放服务</span>
              <span className="font-medium">
                {session.shareServiceRows.filter((row) => row.isOpen).length} 项
              </span>
            </div>
            {session.activeConnectionCount > 0 ? (
              <div className="flex items-center gap-2">
                <span className="text-muted-foreground">当前会话</span>
                <span className="font-medium">
                  {session.activeConnectionCount} 个
                </span>
              </div>
            ) : null}
            {savedNeedsCheckCount > 0 ? (
              <div className="flex items-center gap-2 text-amber-600 dark:text-amber-400">
                <span>另有连接信息需检查，请到设置或高级模式处理</span>
              </div>
            ) : null}
          </div>
          {session.requiresRestart ? (
            <Button size="sm" variant="outline" onClick={actions.restart}>
              重启后生效
            </Button>
          ) : null}
        </div>
      </div>

      <div className="rounded-xl border bg-card p-4 shadow-sm">
        <div className="flex flex-col gap-3 sm:flex-row sm:items-start sm:justify-between">
          <div className="space-y-1">
            <h3 className="text-sm font-medium">核心共享服务</h3>
            <p className="text-xs text-muted-foreground">
              在一个面板里直接管理最常用的 2 类共享能力。
            </p>
          </div>
          <div className="flex flex-wrap gap-2">
            <Button
              size="sm"
              variant="outline"
              onClick={actions.openShareServiceForm}
            >
              完整设置
            </Button>
          </div>
        </div>

        <div className="mt-4 divide-y">
          {session.shareServiceRows.map((row) => (
            <SimpleShareServiceRowCard
              key={row.key}
              row={row}
              disabled={!selectedAlias}
              onOpen={() => actions.openSimpleShareServiceDialog(row.key)}
              onClose={() =>
                row.closeAction === "manage"
                  ? actions.openSimpleShareServiceDialog(row.key)
                  : actions.closeSimpleShareService({
                      tunnelId: row.tunnelId ?? undefined,
                      configIndex: row.configIndex ?? undefined,
                    })
              }
              onCloseItem={(input) => actions.closeSimpleShareService(input)}
            />
          ))}
        </div>
      </div>

      {session.userFacingHints.length > 0 ? (
        <div className="rounded-xl border border-dashed bg-muted/20 px-4 py-3">
          <div className="space-y-1">
            <h3 className="text-sm font-medium">排查提示</h3>
            <p className="text-xs text-muted-foreground">
              共享不稳定时，可先查看这些提示，或
              <button
                type="button"
                className="mx-1 inline font-medium text-foreground underline underline-offset-2"
                onClick={actions.openAdvancedNetworks}
              >
                进入高级模式
              </button>
              处理。
            </p>
            <ul className="mt-2 list-disc space-y-1 pl-4 text-xs text-muted-foreground">
              {session.userFacingHints.slice(0, 3).map((hint) => (
                <li key={hint}>{hint}</li>
              ))}
            </ul>
          </div>
        </div>
      ) : null}
    </div>
  );
}
