import { Button } from "@/components/ui/button";
import { Input } from "@/components/ui/input";
import type { ListeningPortDTO } from "@/lib/types";
import { ListeningPortList } from "@/components/simple/ListeningPortList";

export function ListeningPortPicker({
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
    <div className="space-y-3">
      <div className="rounded-lg border bg-background">
        <div className="border-b px-3 py-3">
          <Input
            value={filterText}
            onChange={(event) => onFilterTextChange(event.target.value)}
            placeholder="搜索端口、应用名或 PID"
          />
          <div className="mt-2 flex flex-wrap items-center justify-between gap-2 text-xs text-muted-foreground">
            <div>
              可共享 {listeningPorts.length} 个，已共享 {disabledPorts.size}{" "}
              个，已选择 {selectedPorts.size} 个
            </div>
            <div className="flex flex-wrap gap-2">
              <Button
                type="button"
                variant="outline"
                size="sm"
                onClick={onSelectFiltered}
                disabled={!hasFilteredPorts}
              >
                全选结果
              </Button>
              <Button
                type="button"
                variant="outline"
                size="sm"
                onClick={onClearSelection}
                disabled={selectedPorts.size === 0}
              >
                清空
              </Button>
            </div>
          </div>
        </div>

        <div className="p-3 pt-2">
          {isLoading ? (
            <div className="rounded-md border bg-muted/30 px-3 py-3 text-xs text-muted-foreground">
              正在读取本机监听端口…
            </div>
          ) : null}

          {error ? (
            <div className="rounded-md border border-amber-300 bg-amber-50 px-3 py-3 text-xs text-amber-700 dark:border-amber-800 dark:bg-amber-950/30 dark:text-amber-400">
              {error}
            </div>
          ) : null}

          {hasFilteredPorts ? (
            <ListeningPortList
              ports={filteredListeningPorts}
              selectedPorts={selectedPorts}
              disabledPorts={disabledPorts}
              onToggle={onTogglePort}
            />
          ) : hasPorts ? (
            <div className="rounded-lg border border-dashed px-4 py-6 text-center text-sm text-muted-foreground">
              无匹配结果，请调整搜索条件。
            </div>
          ) : (
            <div className="rounded-lg border border-dashed px-4 py-6 text-center text-sm text-muted-foreground">
              当前未检测到监听中的 TCP 端口。
            </div>
          )}
        </div>
      </div>
    </div>
  );
}
