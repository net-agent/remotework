import { useState, useEffect } from "react";
import { platform } from "@/lib/platform";
import {
  Dialog,
  DialogContent,
  DialogFooter,
  DialogHeader,
  DialogTitle,
} from "@/components/ui/dialog";
import { Button } from "@/components/ui/button";
import { Input } from "@/components/ui/input";
import { Label } from "@/components/ui/label";
import { UrlField } from "@/components/shared/UrlField";
import { useUIStore } from "@/stores/ui-store";
import { useProfileStore } from "@/stores/profile-store";
import { useAgentStore } from "@/stores/agent-store";
import { emptyTunnel, type TunnelInfo } from "@/lib/config-types";
import { toast } from "sonner";
import { saveTunnelConfig, removeTunnelConfig } from "@/lib/tunnel-save";

const LISTEN_SCHEMES = ["tcp", "vtcp"];
const TARGET_SCHEMES = ["tcp", "vtcp", "socks5"];

export function ServiceForm() {
  const { serviceFormOpen, closeServiceForm, editingServiceIndex } =
    useUIStore();
  const {
    currentConfig,
    setCurrentConfig,
    saveConfig,
    activeProfile,
    setNeedsRestart,
  } = useProfileStore();
  const runtimeNetworks = useAgentStore((s) => s.networks);
  const isEditing = editingServiceIndex !== null;

  const [tunnel, setTunnel] = useState<TunnelInfo>(emptyTunnel());
  const [localAddresses, setLocalAddresses] = useState<string[]>([]);

  const configLinks = currentConfig.links ?? {};
  const linkDomains: Record<string, string> = {};

  for (const [alias, url] of Object.entries(configLinks)) {
    if (!alias) continue;
    try {
      const asParam = new URL(url).searchParams.get("as");
      if (asParam) linkDomains[alias] = asParam;
    } catch {
      // ignore invalid URLs
    }
  }

  for (const n of runtimeNetworks) {
    if (n.name && n.domain && n.kind === "virtual") {
      linkDomains[n.name] = n.domain;
    }
  }

  const linkAliases = Object.keys(linkDomains);

  useEffect(() => {
    if (serviceFormOpen) {
      platform
        .getNetworkInterfaces()
        .then(setLocalAddresses)
        .catch(() => setLocalAddresses(["0.0.0.0", "127.0.0.1"]));
    }
  }, [serviceFormOpen]);

  useEffect(() => {
    if (serviceFormOpen) {
      if (isEditing && editingServiceIndex !== null) {
        const tunnels = currentConfig.tunnels ?? [];
        if (editingServiceIndex < tunnels.length) {
          setTunnel({ ...tunnels[editingServiceIndex] });
        }
      } else {
        setTunnel(emptyTunnel());
      }
    }
  }, [serviceFormOpen, isEditing, editingServiceIndex, currentConfig]);

  const handleSave = async () => {
    if (!tunnel.listen || !tunnel.target) {
      toast.error("请填写监听和目标地址");
      return;
    }
    if (!tunnel.name) {
      toast.error("请填写隧道名称");
      return;
    }

    const saved = await saveTunnelConfig({
      tunnel,
      context: {
        currentConfig,
        setCurrentConfig,
        saveConfig,
        activeProfile,
        setNeedsRestart,
      },
      editingServiceIndex,
    });

    if (saved) {
      closeServiceForm();
    }
  };

  const handleDelete = async () => {
    if (!isEditing || editingServiceIndex === null) return;
    const removed = await removeTunnelConfig({
      context: {
        currentConfig,
        setCurrentConfig,
        saveConfig,
        activeProfile,
        setNeedsRestart,
      },
      configIndex: editingServiceIndex,
    });
    if (removed) {
      closeServiceForm();
    }
  };

  return (
    <Dialog
      open={serviceFormOpen}
      onOpenChange={(open) => !open && closeServiceForm()}
    >
      <DialogContent className="max-w-sm">
        <DialogHeader>
          <DialogTitle>{isEditing ? "编辑隧道" : "添加隧道"}</DialogTitle>
        </DialogHeader>

        <div className="space-y-4">
          <div>
            <Label htmlFor="tunnel-name">名称</Label>
            <Input
              id="tunnel-name"
              value={tunnel.name}
              onChange={(e) => setTunnel({ ...tunnel, name: e.target.value })}
              placeholder="例如：access-corp-db"
              className="mt-1.5"
            />
          </div>

          <UrlField
            label="监听地址"
            value={tunnel.listen}
            onChange={(v) => setTunnel({ ...tunnel, listen: v })}
            networks={LISTEN_SCHEMES}
            localAddresses={localAddresses}
            linkAliases={linkAliases}
            linkDomains={linkDomains}
            isListen
          />

          <UrlField
            label="目标地址"
            value={tunnel.target}
            onChange={(v) => setTunnel({ ...tunnel, target: v })}
            networks={TARGET_SCHEMES}
            linkAliases={linkAliases}
          />

          <p className="text-xs text-muted-foreground font-mono">
            ID: {tunnel.id}
          </p>
        </div>

        <DialogFooter>
          {isEditing && (
            <Button
              variant="destructive"
              onClick={handleDelete}
              className="mr-auto"
            >
              删除
            </Button>
          )}
          <Button variant="outline" onClick={closeServiceForm}>
            取消
          </Button>
          <Button onClick={handleSave}>{isEditing ? "保存" : "添加"}</Button>
        </DialogFooter>
      </DialogContent>
    </Dialog>
  );
}
