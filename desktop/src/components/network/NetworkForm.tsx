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
import { Select, SelectContent, SelectItem, SelectTrigger, SelectValue } from "@/components/ui/select";
import { useUIStore } from "@/stores/ui-store";
import { useProfileStore } from "@/stores/profile-store";
import { emptyAgentInfo, type AgentInfo } from "@/lib/config-types";
import { toast } from "sonner";

export function NetworkForm() {
  const { networkFormOpen, closeNetworkForm, editingNetworkIndex } = useUIStore();
  const { currentConfig, setCurrentConfig } = useProfileStore();
  const isEditing = editingNetworkIndex !== null;

  const [form, setForm] = useState<AgentInfo>(emptyAgentInfo());

  useEffect(() => {
    if (networkFormOpen) {
      if (isEditing && currentConfig.agents[editingNetworkIndex!]) {
        setForm({ ...currentConfig.agents[editingNetworkIndex!] });
      } else {
        setForm(emptyAgentInfo());
      }
    }
  }, [networkFormOpen, isEditing, editingNetworkIndex, currentConfig.agents]);

  const buildUrl = () => {
    let url = `${form.protocol || "vtcp"}://`;
    if (form.domain) {
      url += form.domain;
      if (form.password) url += `:${form.password}`;
      url += "@";
    }
    url += form.address;
    if (form.wsPath) url += form.wsPath;
    return url;
  };

  const handleSave = () => {
    if (!form.name.trim()) {
      toast.error("请输入网络名称");
      return;
    }
    if (!form.address.trim()) {
      toast.error("请输入服务器地址");
      return;
    }

    const agent: AgentInfo = {
      ...form,
      url: buildUrl(),
    };

    const agents = [...currentConfig.agents];
    if (isEditing) {
      agents[editingNetworkIndex!] = agent;
    } else {
      agents.push(agent);
    }

    setCurrentConfig({ ...currentConfig, agents });
    closeNetworkForm();
    toast.success(isEditing ? "网络已更新" : "网络已添加");
  };

  const handleDelete = () => {
    if (!isEditing) return;
    const agents = currentConfig.agents.filter((_, i) => i !== editingNetworkIndex);
    setCurrentConfig({ ...currentConfig, agents });
    closeNetworkForm();
    toast.success("网络已删除");
  };

  return (
    <Dialog open={networkFormOpen} onOpenChange={(open) => !open && closeNetworkForm()}>
      <DialogContent className="max-w-sm">
        <DialogHeader>
          <DialogTitle>{isEditing ? "编辑网络" : "添加网络"}</DialogTitle>
        </DialogHeader>
        <div className="space-y-4 py-2">
          <div>
            <Label htmlFor="net-name">名称</Label>
            <Input
              id="net-name"
              value={form.name}
              onChange={(e) => setForm({ ...form, name: e.target.value })}
              placeholder="例如：office-server"
              className="mt-1.5"
            />
          </div>
          <div>
            <Label>协议</Label>
            <Select
              value={form.protocol || "vtcp"}
              onValueChange={(v) => setForm({ ...form, protocol: v })}
            >
              <SelectTrigger className="mt-1.5">
                <SelectValue />
              </SelectTrigger>
              <SelectContent>
                <SelectItem value="vtcp">vtcp (Flex 虚拟网络)</SelectItem>
                <SelectItem value="ws">ws (WebSocket)</SelectItem>
              </SelectContent>
            </Select>
          </div>
          <div>
            <Label htmlFor="net-address">服务器地址</Label>
            <Input
              id="net-address"
              value={form.address}
              onChange={(e) => setForm({ ...form, address: e.target.value })}
              placeholder="example.com:8080"
              className="mt-1.5"
            />
          </div>
          <div>
            <Label htmlFor="net-domain">域名</Label>
            <Input
              id="net-domain"
              value={form.domain}
              onChange={(e) => setForm({ ...form, domain: e.target.value })}
              placeholder="唯一标识，例如：my-pc"
              className="mt-1.5"
            />
          </div>
          <div>
            <Label htmlFor="net-password">密码</Label>
            <Input
              id="net-password"
              type="password"
              value={form.password}
              onChange={(e) => setForm({ ...form, password: e.target.value })}
              placeholder="连接密码（可选）"
              className="mt-1.5"
            />
          </div>
          {form.protocol === "ws" && (
            <div>
              <Label htmlFor="net-wspath">WebSocket 路径</Label>
              <Input
                id="net-wspath"
                value={form.wsPath}
                onChange={(e) => setForm({ ...form, wsPath: e.target.value })}
                placeholder="/ws"
                className="mt-1.5"
              />
            </div>
          )}
        </div>
        <DialogFooter>
          {isEditing && (
            <Button variant="destructive" onClick={handleDelete} className="mr-auto">
              删除
            </Button>
          )}
          <Button variant="outline" onClick={closeNetworkForm}>
            取消
          </Button>
          <Button onClick={handleSave}>
            {isEditing ? "保存" : "添加"}
          </Button>
        </DialogFooter>
      </DialogContent>
    </Dialog>
  );
}
