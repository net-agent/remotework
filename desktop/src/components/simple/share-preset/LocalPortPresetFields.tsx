import { Input } from "@/components/ui/input";
import { Label } from "@/components/ui/label";
import { SimpleListeningPortPicker } from "@/components/simple/SimpleListeningPortPicker";
import type { ListeningPortDTO } from "@/lib/types";

export function LocalPortPresetFields({
  resolvedAlias,
  requiresExplicitAliasSelection,
  filterText,
  onFilterTextChange,
  listeningPorts,
  filteredListeningPorts,
  selectedPorts,
  openPortSet,
  isLoadingListeningPorts,
  listeningPortsError,
  onToggleSelectedPort,
  onSelectFilteredPorts,
  onClearSelectedPorts,
  manualPortsText,
  onManualPortsTextChange,
}: {
  resolvedAlias: string | null;
  requiresExplicitAliasSelection: boolean;
  filterText: string;
  onFilterTextChange: (value: string) => void;
  listeningPorts: ListeningPortDTO[];
  filteredListeningPorts: ListeningPortDTO[];
  selectedPorts: Set<number>;
  openPortSet: Set<number>;
  isLoadingListeningPorts: boolean;
  listeningPortsError: string | null;
  onToggleSelectedPort: (port: number) => void;
  onSelectFilteredPorts: () => void;
  onClearSelectedPorts: () => void;
  manualPortsText: string;
  onManualPortsTextChange: (value: string) => void;
}) {
  return (
    <>
      {requiresExplicitAliasSelection ? (
        <div className="rounded-md border border-amber-300 bg-amber-50 px-3 py-3 text-xs text-amber-700 dark:border-amber-800 dark:bg-amber-950/30 dark:text-amber-400">
          当前存在多个连接信息，请先在主界面明确选择一个连接信息，再继续开放端口。
        </div>
      ) : null}

      <SimpleListeningPortPicker
        alias={resolvedAlias}
        filterText={filterText}
        onFilterTextChange={onFilterTextChange}
        listeningPorts={listeningPorts}
        filteredListeningPorts={filteredListeningPorts}
        selectedPorts={selectedPorts}
        disabledPorts={openPortSet}
        isLoading={isLoadingListeningPorts}
        error={listeningPortsError}
        onTogglePort={onToggleSelectedPort}
        onSelectFiltered={onSelectFilteredPorts}
        onClearSelection={onClearSelectedPorts}
      />

      <div className="space-y-2 rounded-lg border bg-muted/20 px-3 py-3">
        <Label htmlFor="simple-share-manual-ports">手动输入端口</Label>
        <Input
          id="simple-share-manual-ports"
          value={manualPortsText}
          onChange={(event) => onManualPortsTextChange(event.target.value)}
          placeholder="例如：8080, 3000 9222"
        />
        <div className="text-xs text-muted-foreground">
          也可直接输入多个端口，使用空格或逗号分隔。
        </div>
      </div>
    </>
  );
}
