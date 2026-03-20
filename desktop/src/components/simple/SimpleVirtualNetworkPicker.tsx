import { Button } from "@/components/ui/button";
import type { SimpleLinkOption } from "@/lib/view-model/simple-config-vm";

export function SimpleVirtualNetworkPicker({
  links,
  selectedAlias,
  hint,
  onCreate,
  onEdit,
}: {
  links: SimpleLinkOption[];
  selectedAlias: string | null;
  hint: string;
  onCreate: () => void;
  onEdit: (alias: string) => void;
}) {
  const currentLink =
    links.find((link) => link.alias === selectedAlias) ?? null;

  return (
    <div className="rounded-xl border bg-card p-4 shadow-sm">
      <div className="flex items-start justify-between gap-3">
        <div className="space-y-1">
          <h3 className="text-sm font-medium">当前连接信息</h3>
          <p className="text-xs text-muted-foreground">{hint}</p>
        </div>
        {!currentLink ? (
          <Button size="sm" onClick={onCreate}>
            输入连接信息
          </Button>
        ) : null}
      </div>

      {currentLink ? (
        <div className="mt-4 rounded-lg border bg-background px-3 py-3">
          <div className="flex flex-wrap items-start justify-between gap-3">
            <div className="min-w-0 space-y-1">
              <div className="text-sm font-medium text-foreground">
                {currentLink.alias}
              </div>
              <div className="truncate text-[11px] font-mono text-muted-foreground">
                {currentLink.url}
              </div>
              <div className="text-[11px] text-emerald-600 dark:text-emerald-400">
                {currentLink.statusText}
              </div>
            </div>
            <div className="flex shrink-0 items-center gap-2">
              <Button
                size="sm"
                type="button"
                variant="outline"
                onClick={() => onEdit(currentLink.alias)}
              >
                修改
              </Button>
            </div>
          </div>

          {links.length > 1 ? (
            <div className="mt-3 text-xs text-muted-foreground">
              已保存 {links.length}{" "}
              条连接信息。普通模式当前只围绕这条连接信息工作；如需切换或管理更多连接信息，请前往设置或高级模式。
            </div>
          ) : null}
        </div>
      ) : (
        <div className="mt-4 rounded-lg border border-dashed bg-background px-3 py-4 text-xs text-muted-foreground">
          当前还没有可用连接信息。
        </div>
      )}
    </div>
  );
}
