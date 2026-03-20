import { useState } from "react";
import { Card, CardContent, CardHeader, CardTitle } from "@/components/ui/card";
import { Label } from "@/components/ui/label";
import {
  Select,
  SelectContent,
  SelectItem,
  SelectTrigger,
  SelectValue,
} from "@/components/ui/select";
import { Button } from "@/components/ui/button";
import { Separator } from "@/components/ui/separator";
import { ProfileManager } from "@/components/profile/ProfileManager";
import { useAgentApi } from "@/hooks/use-agent-api";
import { useAgentStore } from "@/stores/agent-store";
import { useProfileStore } from "@/stores/profile-store";
import { useSidecar } from "@/hooks/use-sidecar";
import { useUIStore } from "@/stores/ui-store";
import { toast } from "sonner";

export function SettingsPage() {
  const [logLevel, setLogLevel] = useState("info");
  const { setLogLevel: apiSetLogLevel } = useAgentApi();
  const { agentRunning } = useAgentStore();
  const { activeProfile, dirty, needsRestart, clearNeedsRestart } =
    useProfileStore();
  const { uiMode, setUIMode } = useUIStore();
  const { restartAgent } = useSidecar();

  const handleLogLevelChange = async (level: string) => {
    try {
      await apiSetLogLevel(level);
      setLogLevel(level);
      toast.success(`日志级别已设为 ${level}`);
    } catch (error) {
      toast.error(`设置失败: ${error}`);
    }
  };

  const handleRestart = async () => {
    if (!activeProfile) return;
    try {
      await restartAgent(activeProfile);
      clearNeedsRestart();
      toast.success("Agent 已重启");
    } catch (error) {
      toast.error(`重启失败: ${error}`);
    }
  };

  return (
    <div className="p-4 space-y-4">
      <Card>
        <CardHeader className="pb-3">
          <CardTitle className="text-sm">使用模式</CardTitle>
        </CardHeader>
        <CardContent className="space-y-4">
          <div className="flex items-center justify-between gap-4">
            <div className="space-y-1">
              <p className="text-sm font-medium">默认推荐普通模式</p>
              <p className="text-xs text-muted-foreground">
                普通模式围绕共享与连接组织界面；高级模式保留完整配置、日志与诊断能力。
              </p>
            </div>
            <Select value={uiMode} onValueChange={(value) => setUIMode(value as "simple" | "advanced")}>
              <SelectTrigger className="w-32 h-8">
                <SelectValue />
              </SelectTrigger>
              <SelectContent>
                <SelectItem value="simple">普通模式</SelectItem>
                <SelectItem value="advanced">高级模式</SelectItem>
              </SelectContent>
            </Select>
          </div>
        </CardContent>
      </Card>

      <Card>
        <CardHeader className="pb-3">
          <CardTitle className="text-sm">Agent</CardTitle>
        </CardHeader>
        <CardContent className="space-y-4">
          <div className="flex items-center justify-between">
            <Label>日志级别</Label>
            <Select
              value={logLevel}
              onValueChange={handleLogLevelChange}
              disabled={!agentRunning}
            >
              <SelectTrigger className="w-28 h-8">
                <SelectValue />
              </SelectTrigger>
              <SelectContent>
                <SelectItem value="debug">debug</SelectItem>
                <SelectItem value="info">info</SelectItem>
                <SelectItem value="warn">warn</SelectItem>
                <SelectItem value="error">error</SelectItem>
              </SelectContent>
            </Select>
          </div>
          {(dirty || needsRestart) && (
            <div className="flex items-center justify-between">
              <span className="text-sm text-amber-600">配置已修改，需要重启</span>
              <Button size="sm" variant="outline" onClick={handleRestart}>
                重启 Agent
              </Button>
            </div>
          )}
        </CardContent>
      </Card>

      <Separator />
      <ProfileManager />
      <Separator />

      <Card>
        <CardHeader className="pb-3">
          <CardTitle className="text-sm">关于</CardTitle>
        </CardHeader>
        <CardContent>
          <p className="text-sm text-muted-foreground">Remotework Desktop v0.1.0</p>
        </CardContent>
      </Card>
    </div>
  );
}
