import { Button } from "@/components/ui/button";
import { Plus, Network, Plug } from "lucide-react";
import { useProfileStore } from "@/stores/profile-store";
import { useUIStore } from "@/stores/ui-store";

/**
 * ConfigTab displays the profile config for editing when the agent is NOT running.
 * This allows users to view and fix configuration errors that prevent startup.
 */
export function ConfigTab() {
  const { currentConfig } = useProfileStore();
  const { openLinkForm, openServiceForm } = useUIStore();

  const linkEntries = Object.entries(currentConfig.links ?? {});
  const tunnels = currentConfig.tunnels ?? [];

  return (
    <div className="p-4 space-y-6 overflow-auto h-full">
      {/* Links Section */}
      <section>
        <div className="flex items-center justify-between mb-2">
          <h3 className="text-xs font-medium text-muted-foreground uppercase tracking-wider flex items-center gap-1.5">
            <Network className="h-3.5 w-3.5" />
            网络 ({linkEntries.length})
          </h3>
          <Button
            variant="ghost"
            size="sm"
            className="h-6 text-xs px-1.5"
            onClick={() => openLinkForm()}
          >
            <Plus className="h-3.5 w-3.5 mr-0.5" />
            添加
          </Button>
        </div>
        {linkEntries.length === 0 ? (
          <p className="text-xs text-muted-foreground">暂无网络配置</p>
        ) : (
          <div className="space-y-1">
            {linkEntries.map(([alias, url]) => (
              <div
                key={alias}
                className="flex items-center justify-between px-3 py-2 rounded-md border bg-card hover:bg-accent/50 cursor-pointer"
                onClick={() => openLinkForm(alias)}
              >
                <span className="text-sm font-medium">{alias}</span>
                <span className="text-xs text-muted-foreground font-mono truncate ml-4 max-w-[60%]">
                  {url}
                </span>
              </div>
            ))}
          </div>
        )}
      </section>

      {/* Tunnels Section */}
      <section>
        <div className="flex items-center justify-between mb-2">
          <h3 className="text-xs font-medium text-muted-foreground uppercase tracking-wider flex items-center gap-1.5">
            <Plug className="h-3.5 w-3.5" />
            隧道 ({tunnels.length})
          </h3>
          <Button
            variant="ghost"
            size="sm"
            className="h-6 text-xs px-1.5"
            onClick={() => openServiceForm()}
          >
            <Plus className="h-3.5 w-3.5 mr-0.5" />
            添加
          </Button>
        </div>
        {tunnels.length === 0 ? (
          <p className="text-xs text-muted-foreground">暂无隧道配置</p>
        ) : (
          <div className="space-y-1">
            {tunnels.map((tunnel, index) => (
              <div
                key={tunnel.id}
                className="flex items-center justify-between px-3 py-2 rounded-md border bg-card hover:bg-accent/50 cursor-pointer"
                onClick={() => openServiceForm(index)}
              >
                <span className="text-sm font-medium">{tunnel.name || "(未命名)"}</span>
                <div className="text-xs text-muted-foreground font-mono text-right">
                  <div className="truncate max-w-[200px]">{tunnel.listen}</div>
                  <div className="truncate max-w-[200px]">{tunnel.target}</div>
                </div>
              </div>
            ))}
          </div>
        )}
      </section>
    </div>
  );
}
