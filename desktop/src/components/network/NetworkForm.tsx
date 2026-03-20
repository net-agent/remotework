import { useState, useEffect } from "react";
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
import { useUIStore } from "@/stores/ui-store";
import { useProfileStore } from "@/stores/profile-store";
import { useAgentStore } from "@/stores/agent-store";
import * as api from "@/lib/api";
import { validateLinkURL } from "@/lib/config-validation";
import { toast } from "sonner";

export function NetworkForm() {
  const {
    linkFormOpen,
    closeLinkForm,
    editingLinkAlias,
    setSelectedSimpleShareLinkAlias,
  } = useUIStore();
  const {
    currentConfig,
    setCurrentConfig,
    saveConfig,
    activeProfile,
    setNeedsRestart,
  } = useProfileStore();
  const { agentRunning } = useAgentStore();
  const isEditing = editingLinkAlias !== null;

  const [alias, setAlias] = useState("");
  const [url, setUrl] = useState("");

  useEffect(() => {
    if (linkFormOpen) {
      if (isEditing && editingLinkAlias) {
        setAlias(editingLinkAlias);
        setUrl(currentConfig.links[editingLinkAlias] ?? "");
      } else {
        setAlias("");
        setUrl("");
      }
    }
  }, [linkFormOpen, isEditing, editingLinkAlias, currentConfig.links]);

  const handleSave = async () => {
    const trimmedAlias = alias.trim();
    if (!trimmedAlias) {
      toast.error("请输入链路别名");
      return;
    }
    if (!/^[a-zA-Z][a-zA-Z0-9_-]*$/.test(trimmedAlias)) {
      toast.error("别名仅允许字母开头，可包含字母、数字、下划线、短横线");
      return;
    }
    if (!url.trim()) {
      toast.error("请输入注册 URL");
      return;
    }

    const urlErr = validateLinkURL(url.trim());
    if (urlErr) {
      toast.error(urlErr);
      return;
    }

    const newLinks = { ...currentConfig.links };

    // If renaming alias during edit, remove old key
    if (isEditing && editingLinkAlias && editingLinkAlias !== trimmedAlias) {
      delete newLinks[editingLinkAlias];
    }

    newLinks[trimmedAlias] = url.trim();
    const newConfig = { ...currentConfig, links: newLinks };
    setCurrentConfig(newConfig);
    if (activeProfile) {
      await saveConfig(activeProfile, newConfig);
    }
    setSelectedSimpleShareLinkAlias(trimmedAlias);

    // Dynamically add to running agent (new links only)
    if (!isEditing && agentRunning) {
      try {
        await api.addLink(trimmedAlias, url.trim());
        toast.success("链路已添加并启动");
      } catch (e) {
        toast.warning(`已保存配置，但动态添加失败: ${e}`);
        setNeedsRestart();
      }
    } else if (isEditing && agentRunning) {
      toast.success("链路已更新，重启后生效");
      setNeedsRestart();
    } else {
      toast.success(isEditing ? "链路已更新" : "链路已添加");
    }

    closeLinkForm();
  };

  const handleDelete = async () => {
    if (!isEditing || !editingLinkAlias) return;
    const newLinks = { ...currentConfig.links };
    delete newLinks[editingLinkAlias];
    const newConfig = { ...currentConfig, links: newLinks };
    setCurrentConfig(newConfig);
    if (activeProfile) {
      await saveConfig(activeProfile, newConfig);
    }
    setSelectedSimpleShareLinkAlias(null);
    if (agentRunning) setNeedsRestart();
    closeLinkForm();
    toast.success(agentRunning ? "链路已删除，重启后生效" : "链路已删除");
  };

  return (
    <Dialog
      open={linkFormOpen}
      onOpenChange={(open) => !open && closeLinkForm()}
    >
      <DialogContent className="max-w-sm">
        <DialogHeader>
          <DialogTitle>{isEditing ? "编辑链路" : "添加链路"}</DialogTitle>
        </DialogHeader>
        <div className="space-y-4 py-2">
          <div>
            <Label htmlFor="link-alias">别名</Label>
            <Input
              id="link-alias"
              value={alias}
              onChange={(e) => setAlias(e.target.value)}
              placeholder="例如：office-relay"
              className="mt-1.5"
              disabled={isEditing}
            />
            <p className="text-xs text-muted-foreground mt-1">
              链路的本地标识，隧道中 vtcp 端点通过此别名引用
            </p>
          </div>
          <div>
            <Label htmlFor="link-url">注册 URL</Label>
            <Input
              id="link-url"
              value={url}
              onChange={(e) => setUrl(e.target.value)}
              placeholder="tcp://my-pc@relay.example.com:8080"
              className="mt-1.5 font-mono text-xs"
            />
            <p className="text-xs text-muted-foreground mt-1">
              格式：scheme://domain@address?as=hostname&auth=secret
            </p>
          </div>
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
          <Button variant="outline" onClick={closeLinkForm}>
            取消
          </Button>
          <Button onClick={handleSave}>{isEditing ? "保存" : "添加"}</Button>
        </DialogFooter>
      </DialogContent>
    </Dialog>
  );
}
