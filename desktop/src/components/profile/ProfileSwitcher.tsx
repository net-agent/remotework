import { useState } from "react";
import {
  DropdownMenu,
  DropdownMenuContent,
  DropdownMenuItem,
  DropdownMenuSeparator,
  DropdownMenuTrigger,
} from "@/components/ui/dropdown-menu";
import { Button } from "@/components/ui/button";
import { ChevronDown, Plus } from "lucide-react";
import {
  Dialog,
  DialogContent,
  DialogFooter,
  DialogHeader,
  DialogTitle,
} from "@/components/ui/dialog";
import { Input } from "@/components/ui/input";
import { Textarea } from "@/components/ui/textarea";
import { Label } from "@/components/ui/label";
import { invoke } from "@tauri-apps/api/core";
import { useProfileStore } from "@/stores/profile-store";
import { useAgentStore } from "@/stores/agent-store";
import { useSidecar } from "@/hooks/use-sidecar";
import { toast } from "sonner";

export function ProfileSwitcher() {
  const { profiles, activeProfile, setActiveProfile, createProfile, loadConfig, loadProfiles } =
    useProfileStore();
  const { agentRunning } = useAgentStore();
  const { startAgent, stopAgent } = useSidecar();
  const [newDialogOpen, setNewDialogOpen] = useState(false);
  const [newName, setNewName] = useState("");
  const [newConfig, setNewConfig] = useState("");

  const handleSwitch = async (name: string) => {
    try {
      if (agentRunning) {
        await stopAgent();
      }
      await setActiveProfile(name);
      await loadConfig(name);
      await startAgent(name);
    } catch (e) {
      toast.error(`切换失败: ${e}`);
    }
  };

  const handleCreate = async () => {
    const name = newName.trim();
    if (!name) return;
    try {
      if (newConfig.trim()) {
        await invoke("import_profile", { name, content: newConfig });
        await loadProfiles();
      } else {
        await createProfile(name);
      }
      await setActiveProfile(name);
      setNewDialogOpen(false);
      setNewName("");
      setNewConfig("");
    } catch (e) {
      toast.error(`创建失败: ${e}`);
    }
  };

  return (
    <>
      <DropdownMenu modal={false}>
        <DropdownMenuTrigger asChild>
          <Button variant="ghost" size="sm" className="h-7 gap-1 text-sm px-2">
            {activeProfile || "选择 Profile"}
            <ChevronDown className="h-3 w-3" />
          </Button>
        </DropdownMenuTrigger>
        <DropdownMenuContent align="end">
          {profiles.map((p) => (
            <DropdownMenuItem
              key={p.name}
              onSelect={() => handleSwitch(p.name)}
              className={p.name === activeProfile ? "font-medium" : ""}
            >
              {p.name}
              {p.name === activeProfile && (
                <span className="ml-auto text-xs text-primary font-medium">当前</span>
              )}
            </DropdownMenuItem>
          ))}
          {profiles.length > 0 && <DropdownMenuSeparator />}
          <DropdownMenuItem onSelect={() => setNewDialogOpen(true)}>
            <Plus className="h-3.5 w-3.5 mr-1.5" />
            新建 Profile
          </DropdownMenuItem>
        </DropdownMenuContent>
      </DropdownMenu>

      <Dialog open={newDialogOpen} onOpenChange={(open) => {
        setNewDialogOpen(open);
        if (!open) { setNewName(""); setNewConfig(""); }
      }}>
        <DialogContent className="max-w-md">
          <DialogHeader>
            <DialogTitle>新建 Profile</DialogTitle>
          </DialogHeader>
          <div className="py-2 space-y-3">
            <div>
              <Label htmlFor="profile-name">名称</Label>
              <Input
                id="profile-name"
                value={newName}
                onChange={(e) => setNewName(e.target.value)}
                placeholder="例如：办公室"
                className="mt-1.5"
                onKeyDown={(e) => e.key === "Enter" && !newConfig && handleCreate()}
              />
            </div>
            <div>
              <Label htmlFor="profile-config">配置（可选）</Label>
              <Textarea
                id="profile-config"
                value={newConfig}
                onChange={(e) => setNewConfig(e.target.value)}
                placeholder="粘贴 JSON 配置"
                className="mt-1.5 font-mono text-xs min-h-24 max-h-64 overflow-y-auto"
              />
            </div>
          </div>
          <DialogFooter>
            <Button variant="outline" onClick={() => setNewDialogOpen(false)}>
              取消
            </Button>
            <Button onClick={handleCreate} disabled={!newName.trim()}>
              创建
            </Button>
          </DialogFooter>
        </DialogContent>
      </Dialog>
    </>
  );
}
