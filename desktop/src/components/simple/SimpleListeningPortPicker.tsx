import { Button } from "@/components/ui/button";
import { Input } from "@/components/ui/input";
import type { ListeningPortDTO } from "@/lib/types";
import { SimpleListeningPortList } from "@/components/simple/SimpleListeningPortList";

export function SimpleListeningPortPicker({
  alias,
  filterText,
  onFilterTextChange,
  listeningPorts,
  filteredListeningPorts,
  selectedPorts,
  disabledPorts,
  isLoading,
  error,
  onTogglePort,
  onSelectFiltered,
  onClearSelection,
}: {
  alias: string | null;
  filterText: string;
  onFilterTextChange: (value: string) => void;
  listeningPorts: ListeningPortDTO[];
  filteredListeningPorts: ListeningPortDTO[];
  selectedPorts: ReadonlySet<number>;
  disabledPorts: ReadonlySet<number>;
  isLoading: boolean;
  error: string | null;
  onTogglePort: (port: number) => void;
  onSelectFiltered: () => void;
  onClearSelection: () => void;
}) {
  const hasPorts = listeningPorts.length > 0;
  const hasFilteredPorts = filteredListeningPorts.length > 0;

  return (
    <div className="space-y-4">
      <div className="space-y-1">
        <div className="flex flex-wrap items-center justify-between gap-2">
          <div>
            <h3 className="text-sm font-medium">监听端口列表</h3>
            <p className="text-xs text-muted-foreground">
              已开放端口会置顶显示；其余端口可继续选择开放。
            </p>
          </div>
          {alias ? (
            <div className="text-xs text-muted-foreground">
              当前连接信息：
              <span className="font-medium text-foreground">{alias}</span>
            </div>
          ) : null}
        </div>
      </div>

      <div className="space-y-2">
        <Input
          value={filterText}
          onChange={(event) => onFilterTextChange(event.target.value)}
          placeholder="搜索端口 / 进程名 / PID"
        />
        <div className="flex flex-wrap items-center justify-between gap-2 text-xs text-muted-foreground">
          <div>
            共 {listeningPorts.length} 个监听端口，已开放 {disabledPorts.size} 个，已选择 {selectedPorts.size} 个
          </div>
          <div className="flex flex-wrap gap-2">
            <Button
              type="button"
              variant="outline"
              size="sm"
              onClick={onSelectFiltered}
              disabled={!hasFilteredPorts}
            >
              全选可开放结果
            </Button>
            <Button
              type="button"
              variant="outline"
              size="sm"
              onClick={onClearSelection}
              disabled={selectedPorts.size === 0}
            >
              清空选择
            </Button>
          </div>
        </div>
      </div>

      {isLoading ? (
        <div className="rounded-md border bg-muted/30 px-3 py-3 text-xs text-muted-foreground">
          正在后台读取本机监听端口，你也可以直接在下方手动输入端口继续。
        </div>
      ) : null}

      {error ? (
        <div className="rounded-md border border-amber-300 bg-amber-50 px-3 py-3 text-xs text-amber-700 dark:border-amber-800 dark:bg-amber-950/30 dark:text-amber-400">
          {error}
        </div>
      ) : null}

      {hasFilteredPorts ? (
        <SimpleListeningPortList
          ports={filteredListeningPorts}
          selectedPorts={selectedPorts}
          disabledPorts={disabledPorts}
          onToggle={onTogglePort}
        />
      ) : hasPorts ? (
        <div className="rounded-lg border border-dashed px-4 py-6 text-center text-sm text-muted-foreground">
          无匹配结果，请调整搜索条件或改用手动输入。
        </div>
      ) : (
        <div className="rounded-lg border border-dashed px-4 py-6 text-center text-sm text-muted-foreground">
          当前未检测到监听中的 TCP 端口，请直接使用手动输入兜底。
        </div>
      )}
    </div>
  );
}
