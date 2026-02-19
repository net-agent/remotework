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
import { Tabs, TabsContent, TabsList, TabsTrigger } from "@/components/ui/tabs";
import { UrlField } from "@/components/shared/UrlField";
import { useUIStore } from "@/stores/ui-store";
import { useProfileStore } from "@/stores/profile-store";
import { useAgentStore } from "@/stores/agent-store";
import {
  emptyPortproxy,
  emptySocks5,
  emptyRDP,
  type PortproxyInfo,
  type Socks5Info,
  type RDPInfo,
} from "@/lib/config-types";
import { toast } from "sonner";

type ServiceType = "portproxy" | "socks5" | "rdp";

const LOCAL_SCHEMES = ["tcp"];

export function ServiceForm() {
  const { serviceFormOpen, closeServiceForm, editingServiceType, editingServiceIndex } =
    useUIStore();
  const { currentConfig, setCurrentConfig } = useProfileStore();
  const runtimeNetworks = useAgentStore((s) => s.networks);
  const isEditing = editingServiceIndex !== null && editingServiceType !== null;

  const [tab, setTab] = useState<ServiceType>("portproxy");
  const [portproxy, setPortproxy] = useState<PortproxyInfo>(emptyPortproxy());
  const [socks5, setSocks5] = useState<Socks5Info>(emptySocks5());
  const [rdp, setRdp] = useState<RDPInfo>(emptyRDP());
  const [localAddresses, setLocalAddresses] = useState<string[]>([]);

  // Build networks list: local schemes + virtual network names from config agents + runtime networks
  const virtualNames = new Set<string>();
  for (const a of currentConfig.agents) {
    if (a.name && !LOCAL_SCHEMES.includes(a.name)) virtualNames.add(a.name);
  }
  for (const n of runtimeNetworks) {
    if (n.name && !LOCAL_SCHEMES.includes(n.name)) virtualNames.add(n.name);
  }
  const networks = [...LOCAL_SCHEMES, ...virtualNames];

  useEffect(() => {
    if (serviceFormOpen) {
      invoke<string[]>("get_network_interfaces")
        .then(setLocalAddresses)
        .catch(() => setLocalAddresses(["0.0.0.0", "127.0.0.1"]));
    }
  }, [serviceFormOpen]);

  useEffect(() => {
    if (serviceFormOpen) {
      if (isEditing && editingServiceType) {
        setTab(editingServiceType);
        switch (editingServiceType) {
          case "portproxy":
            setPortproxy({ ...currentConfig.portproxy[editingServiceIndex!] });
            break;
          case "socks5":
            setSocks5({ ...currentConfig.socks5[editingServiceIndex!] });
            break;
          case "rdp":
            setRdp({ ...currentConfig.rdp[editingServiceIndex!] });
            break;
        }
      } else {
        setTab("portproxy");
        setPortproxy(emptyPortproxy());
        setSocks5(emptySocks5());
        setRdp(emptyRDP());
      }
    }
  }, [serviceFormOpen, isEditing, editingServiceType, editingServiceIndex, currentConfig]);

  const handleSave = () => {
    const config = { ...currentConfig };
    switch (tab) {
      case "portproxy": {
        if (!portproxy.listen || !portproxy.target) {
          toast.error("请填写监听和目标地址");
          return;
        }
        const list = [...config.portproxy];
        if (isEditing && editingServiceType === "portproxy") {
          list[editingServiceIndex!] = portproxy;
        } else {
          list.push(portproxy);
        }
        config.portproxy = list;
        break;
      }
      case "socks5": {
        if (!socks5.listen) {
          toast.error("请填写监听地址");
          return;
        }
        const list = [...config.socks5];
        if (isEditing && editingServiceType === "socks5") {
          list[editingServiceIndex!] = socks5;
        } else {
          list.push(socks5);
        }
        config.socks5 = list;
        break;
      }
      case "rdp": {
        if (!rdp.listen) {
          toast.error("请填写监听地址");
          return;
        }
        const list = [...config.rdp];
        if (isEditing && editingServiceType === "rdp") {
          list[editingServiceIndex!] = rdp;
        } else {
          list.push(rdp);
        }
        config.rdp = list;
        break;
      }
    }
    setCurrentConfig(config);
    closeServiceForm();
    toast.success(isEditing ? "服务已更新" : "服务已添加");
  };

  const handleDelete = () => {
    if (!isEditing || !editingServiceType) return;
    const config = { ...currentConfig };
    switch (editingServiceType) {
      case "portproxy":
        config.portproxy = config.portproxy.filter((_, i) => i !== editingServiceIndex);
        break;
      case "socks5":
        config.socks5 = config.socks5.filter((_, i) => i !== editingServiceIndex);
        break;
      case "rdp":
        config.rdp = config.rdp.filter((_, i) => i !== editingServiceIndex);
        break;
    }
    setCurrentConfig(config);
    closeServiceForm();
    toast.success("服务已删除");
  };

  return (
    <Dialog open={serviceFormOpen} onOpenChange={(open) => !open && closeServiceForm()}>
      <DialogContent className="max-w-sm">
        <DialogHeader>
          <DialogTitle>{isEditing ? "编辑服务" : "添加服务"}</DialogTitle>
        </DialogHeader>

        <Tabs value={tab} onValueChange={(v) => setTab(v as ServiceType)}>
          <TabsList className="w-full">
            <TabsTrigger value="portproxy" className="flex-1">端口转发</TabsTrigger>
            <TabsTrigger value="socks5" className="flex-1">SOCKS5</TabsTrigger>
            <TabsTrigger value="rdp" className="flex-1">RDP</TabsTrigger>
          </TabsList>

          <TabsContent value="portproxy" className="space-y-4 mt-4">
            <div>
              <Label htmlFor="pp-log">名称</Label>
              <Input
                id="pp-log"
                value={portproxy.log}
                onChange={(e) => setPortproxy({ ...portproxy, log: e.target.value })}
                placeholder="例如：ssh-tunnel"
                className="mt-1.5"
              />
            </div>
            <UrlField
              label="监听地址"
              value={portproxy.listen}
              onChange={(v) => setPortproxy({ ...portproxy, listen: v })}
              networks={networks}
              localAddresses={localAddresses}
              isListen
            />
            <UrlField
              label="目标地址"
              value={portproxy.target}
              onChange={(v) => setPortproxy({ ...portproxy, target: v })}
              networks={networks}
            />
          </TabsContent>

          <TabsContent value="socks5" className="space-y-4 mt-4">
            <div>
              <Label htmlFor="s5-log">名称</Label>
              <Input
                id="s5-log"
                value={socks5.log}
                onChange={(e) => setSocks5({ ...socks5, log: e.target.value })}
                placeholder="例如：proxy"
                className="mt-1.5"
              />
            </div>
            <UrlField
              label="监听地址"
              value={socks5.listen}
              onChange={(v) => setSocks5({ ...socks5, listen: v })}
              networks={networks}
              localAddresses={localAddresses}
              isListen
            />
            <div>
              <Label htmlFor="s5-user">用户名</Label>
              <Input
                id="s5-user"
                value={socks5.username}
                onChange={(e) => setSocks5({ ...socks5, username: e.target.value })}
                placeholder="可选"
                className="mt-1.5"
              />
            </div>
            <div>
              <Label htmlFor="s5-pass">密码</Label>
              <Input
                id="s5-pass"
                type="password"
                value={socks5.password}
                onChange={(e) => setSocks5({ ...socks5, password: e.target.value })}
                placeholder="可选"
                className="mt-1.5"
              />
            </div>
          </TabsContent>

          <TabsContent value="rdp" className="space-y-4 mt-4">
            <div>
              <Label htmlFor="rdp-log">名称</Label>
              <Input
                id="rdp-log"
                value={rdp.log}
                onChange={(e) => setRdp({ ...rdp, log: e.target.value })}
                placeholder="例如：remote-desktop"
                className="mt-1.5"
              />
            </div>
            <UrlField
              label="监听地址"
              value={rdp.listen}
              onChange={(v) => setRdp({ ...rdp, listen: v })}
              networks={networks}
              localAddresses={localAddresses}
              isListen
            />
            <p className="text-sm text-muted-foreground">
              RDP 服务会自动将流量转发到 tcp://localhost:3389
            </p>
          </TabsContent>
        </Tabs>

        <DialogFooter>
          {isEditing && (
            <Button variant="destructive" onClick={handleDelete} className="mr-auto">
              删除
            </Button>
          )}
          <Button variant="outline" onClick={closeServiceForm}>
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
