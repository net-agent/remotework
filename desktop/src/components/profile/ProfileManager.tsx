import { useState } from "react";
import { Card, CardContent, CardHeader, CardTitle } from "@/components/ui/card";
import { Button } from "@/components/ui/button";
import { Input } from "@/components/ui/input";
import { ConfirmDialog } from "@/components/shared/ConfirmDialog";
import { useProfileStore } from "@/stores/profile-store";
import { invoke } from "@tauri-apps/api/core";
import { save, open } from "@tauri-apps/plugin-dialog";
import { Pencil, Trash2, Download, Upload } from "lucide-react";
import { toast } from "sonner";

export function ProfileManager() {
  const { profiles, activeProfile, deleteProfile, renameProfile, loadProfiles } =
    useProfileStore();
  const [renamingProfile, setRenamingProfile] = useState<string | null>(null);
  const [renameValue, setRenameValue] = useState("");
  const [deleteTarget, setDeleteTarget] = useState<string | null>(null);

  const handleRename = async () => {
    if (!renamingProfile || !renameValue.trim()) return;
    try {
      await renameProfile(renamingProfile, renameValue.trim());
      setRenamingProfile(null);
      toast.success("已重命名");
    } catch (e) {
      toast.error(`重命名失败: ${e}`);
    }
  };

  const handleDelete = async () => {
    if (!deleteTarget) return;
    try {
      await deleteProfile(deleteTarget);
      setDeleteTarget(null);
      toast.success("已删除");
    } catch (e) {
      toast.error(`删除失败: ${e}`);
    }
  };

  const handleExport = async (name: string) => {
    try {
      const content = await invoke<string>("export_profile", { name });
      const path = await save({
        defaultPath: `${name}.json`,
        filters: [{ name: "JSON", extensions: ["json"] }],
      });
      if (path) {
        // Write via Rust — we can use the dialog result
        // For simplicity, copy to clipboard or use a different approach
        // Actually, we need to write the file. Let's use the shell or a Tauri command.
        // For now, use the Blob approach via the browser API
        const blob = new Blob([content], { type: "application/json" });
        const url = URL.createObjectURL(blob);
        const a = document.createElement("a");
        a.href = url;
        a.download = `${name}.json`;
        a.click();
        URL.revokeObjectURL(url);
        toast.success("已导出");
      }
    } catch (e) {
      toast.error(`导出失败: ${e}`);
    }
  };

  const handleImport = async () => {
    try {
      const file = await open({
        filters: [{ name: "Config", extensions: ["json", "toml"] }],
        multiple: false,
      });
      if (!file) return;

      // Read file content via fetch (file:// protocol)
      const resp = await fetch(`asset://localhost/${file}`);
      const content = await resp.text();

      // Use filename without extension as profile name
      const fileName = file.split("/").pop()?.split("\\").pop() ?? "imported";
      const name = fileName.replace(/\.(json|toml)$/, "");

      await invoke("import_profile", { name, content });
      await loadProfiles();
      toast.success(`已导入 "${name}"`);
    } catch (e) {
      toast.error(`导入失败: ${e}`);
    }
  };

  return (
    <Card>
      <CardHeader className="pb-3">
        <div className="flex items-center justify-between">
          <CardTitle className="text-sm">Profile 管理</CardTitle>
          <Button variant="ghost" size="sm" className="h-7 text-xs" onClick={handleImport}>
            <Upload className="h-3 w-3 mr-1" />
            导入
          </Button>
        </div>
      </CardHeader>
      <CardContent className="space-y-2">
        {profiles.length === 0 && (
          <p className="text-xs text-muted-foreground text-center py-4">
            暂无 Profile
          </p>
        )}
        {profiles.map((p) => (
          <div
            key={p.name}
            className="flex items-center gap-2 py-1.5 px-2 rounded hover:bg-accent/50"
          >
            {renamingProfile === p.name ? (
              <Input
                value={renameValue}
                onChange={(e) => setRenameValue(e.target.value)}
                onKeyDown={(e) => {
                  if (e.key === "Enter") handleRename();
                  if (e.key === "Escape") setRenamingProfile(null);
                }}
                onBlur={handleRename}
                className="h-7 text-sm"
                autoFocus
              />
            ) : (
              <>
                <span className="flex-1 text-sm truncate">
                  {p.name}
                  {p.name === activeProfile && (
                    <span className="ml-2 text-xs text-muted-foreground">
                      (当前)
                    </span>
                  )}
                </span>
                <Button
                  variant="ghost"
                  size="icon"
                  className="h-6 w-6"
                  onClick={() => handleExport(p.name)}
                >
                  <Download className="h-3 w-3" />
                </Button>
                <Button
                  variant="ghost"
                  size="icon"
                  className="h-6 w-6"
                  onClick={() => {
                    setRenamingProfile(p.name);
                    setRenameValue(p.name);
                  }}
                >
                  <Pencil className="h-3 w-3" />
                </Button>
                <Button
                  variant="ghost"
                  size="icon"
                  className="h-6 w-6"
                  disabled={p.name === activeProfile}
                  onClick={() => setDeleteTarget(p.name)}
                >
                  <Trash2 className="h-3 w-3" />
                </Button>
              </>
            )}
          </div>
        ))}
      </CardContent>

      <ConfirmDialog
        open={!!deleteTarget}
        onOpenChange={(open) => !open && setDeleteTarget(null)}
        title="删除 Profile"
        description={`确定要删除 "${deleteTarget}" 吗？此操作不可撤销。`}
        confirmLabel="删除"
        variant="destructive"
        onConfirm={handleDelete}
      />
    </Card>
  );
}
