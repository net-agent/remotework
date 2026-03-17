import { useState, useEffect } from "react";
import { invoke } from "@tauri-apps/api/core";
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
import * as api from "@/lib/api";
import { validateTunnel } from "@/lib/config-validation";
import { toast } from "sonner";

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

  // Build available link aliases for vtcp scheme hints
  const linkAliases = Object.keys(currentConfig.links ?? {});
  for (const n of runtimeNetworks) {
    if (n.name && !linkAliases.includes(n.name)) linkAliases.push(n.name);
  }

  useEffect(() => {
    if (serviceFormOpen) {
      invoke<string[]>("get_network_interfaces")
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

    const tunnelErr = validateTunnel(tunnel, linkAliases);
    if (tunnelErr) {
      toast.error(tunnelErr);
      return;
    }

    const config = { ...currentConfig };
    const tunnels = [...(config.tunnels ?? [])];
    let isNew = false;

    if (isEditing && editingServiceIndex !== null) {
      tunnels[editingServiceIndex] = tunnel;
    } else {
      tunnels.push(tunnel);
      isNew = true;
    }
    config.tunnels = tunnels;

    setCurrentConfig(config);
    if (activeProfile) saveConfig(activeProfile, config);

    // Dynamically add to running agent (new tunnels only)
    const agentRunning = useAgentStore.getState().agentRunning;
    if (isNew && agentRunning) {
      try {
        await api.addTunnel(tunnel);
        toast.success("隧道已添加并启动");
      } catch (e) {
        toast.warning(`已保存配置，但动态添加失败: ${e}`);
        setNeedsRestart();
      }
    } else if (isEditing && agentRunning) {
      toast.success("隧道已更新，重启后生效");
      setNeedsRestart();
    } else {
      toast.success(isEditing ? "隧道已更新" : "隧道已添加");
    }

    closeServiceForm();
  };

  const handleDelete = () => {
    if (!isEditing || editingServiceIndex === null) return;
    const config = { ...currentConfig };
    config.tunnels = (config.tunnels ?? []).filter(
      (_, i) => i !== editingServiceIndex,
    );
    setCurrentConfig(config);
    if (activeProfile) saveConfig(activeProfile, config);
    const agentRunning = useAgentStore.getState().agentRunning;
    if (agentRunning) setNeedsRestart();
    closeServiceForm();
    toast.success(agentRunning ? "隧道已删除，重启后生效" : "隧道已删除");
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
            isListen
          />

          <UrlField
            label="目标地址"
            value={tunnel.target}
            onChange={(v) => setTunnel({ ...tunnel, target: v })}
            networks={TARGET_SCHEMES}
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
