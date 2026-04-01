import { useEffect, useMemo, useState } from "react";
import { toast } from "sonner";
import { Plus, List } from "lucide-react";
import { Button } from "@/components/ui/button";
import { Input } from "@/components/ui/input";
import { Label } from "@/components/ui/label";
import {
  Select,
  SelectContent,
  SelectItem,
  SelectTrigger,
  SelectValue,
} from "@/components/ui/select";
import { FeatureCard } from "@/components/simple/FeatureCard";
import { ConnectMappingTable } from "@/components/simple/ConnectMappingTable";
import { useProfileStore } from "@/stores/profile-store";
import { useAgentStore } from "@/stores/agent-store";
import { saveTunnelConfig, removeTunnelConfig } from "@/lib/tunnel-save";
import type { SimpleSessionVM } from "@/lib/view-model/simple-session-vm";
import {
  analyzeConnectForm,
  buildConnectTunnel,
  buildConnectTunnelName,
  isConnectTunnel,
  validateConnectForm,
} from "@/lib/simple-domain/connect-tunnel-rules";
import type { TunnelInfo } from "@/lib/config-types";

export function ConnectOtherComputerPanel({
  session,
}: {
  session: SimpleSessionVM;
}) {
  const {
    currentConfig,
    activeProfile,
    saveConfig,
    setCurrentConfig,
    setNeedsRestart,
  } = useProfileStore();
  const { services, agentRunning } = useAgentStore();

  const [targetUrl, setTargetUrl] = useState("");
  const [localPort, setLocalPort] = useState("");
  const [selectedAlias, setSelectedAlias] = useState<string | null>(null);
  const [name, setName] = useState("");
  const [saving, setSaving] = useState(false);

  const availableLinks = session.config.allLinks;

  const analysis = useMemo(
    () => analyzeConnectForm(targetUrl, availableLinks),
    [targetUrl, availableLinks],
  );

  useEffect(() => {
    if (analysis.portFromUrl && !localPort) {
      setLocalPort(analysis.portFromUrl);
    }
  }, [analysis.portFromUrl, localPort]);

  useEffect(() => {
    setSelectedAlias(analysis.resolvedAlias);
  }, [analysis.resolvedAlias]);

  const effectiveAlias = analysis.needsAliasPicker
    ? selectedAlias
    : analysis.resolvedAlias;

  const formError = validateConnectForm({
    targetVtcpUrl: targetUrl,
    localPort,
    localAlias: effectiveAlias,
  });

  const connectTunnels = useMemo(
    () => (currentConfig.tunnels ?? []).filter(isConnectTunnel),
    [currentConfig.tunnels],
  );

  const tunnelSaveContext = useMemo(
    () => ({
      currentConfig,
      setCurrentConfig,
      saveConfig,
      activeProfile,
      setNeedsRestart,
    }),
    [currentConfig, setCurrentConfig, saveConfig, activeProfile, setNeedsRestart],
  );

  const handleSave = async () => {
    if (formError || !effectiveAlias) return;
    const resolvedName =
      name.trim() || buildConnectTunnelName(targetUrl, localPort);
    const tunnel = buildConnectTunnel({
      targetVtcpUrl: targetUrl,
      localAlias: effectiveAlias,
      localPort,
      name: resolvedName,
    });
    if (!tunnel) {
      toast.error("目标地址格式无效");
      return;
    }
    setSaving(true);
    try {
      await saveTunnelConfig({ tunnel, context: tunnelSaveContext });
      setTargetUrl("");
      setLocalPort("");
      setName("");
    } finally {
      setSaving(false);
    }
  };

  const handleRemove = async (tunnel: TunnelInfo) => {
    const fullIndex = (currentConfig.tunnels ?? []).findIndex(
      (t) => t.id === tunnel.id,
    );
    await removeTunnelConfig({
      context: tunnelSaveContext,
      tunnelId: tunnel.id || undefined,
      configIndex: fullIndex >= 0 ? fullIndex : undefined,
    });
  };

  const handleCopy = async (url: string) => {
    try {
      await navigator.clipboard.writeText(url);
      toast.success("已复制地址");
    } catch {
      toast.error("复制失败");
    }
  };

  const connectCount = connectTunnels.length;

  return (
    <div className="grid items-start gap-4 sm:grid-cols-[360px_minmax(0,1fr)]">
      {/* 左侧：添加连接入口 */}
      <FeatureCard
        theme="emerald"
        icon={<Plus className="h-4.5 w-4.5" />}
        title="添加连接入口"
        description="填写对方开放的虚拟地址，设置本地端口后添加。"
        footerAction={
          <Button
            size="sm"
            className="w-full bg-emerald-600 text-white hover:bg-emerald-500 dark:bg-emerald-600 dark:hover:bg-emerald-500"
            disabled={!!formError || saving || availableLinks.length === 0}
            onClick={() => void handleSave()}
          >
            {saving ? "添加中…" : "添加入口"}
          </Button>
        }
      >
        <div className="space-y-3">
          <div className="space-y-1.5">
            <Label className="text-xs">目标虚拟地址</Label>
            <Input
              className="h-8 font-mono text-xs"
              placeholder="vtcp://service.alias:3389"
              value={targetUrl}
              onChange={(e) => {
                setTargetUrl(e.target.value);
                setLocalPort("");
              }}
            />
            {analysis.error ? (
              <p className="text-xs text-destructive">{analysis.error}</p>
            ) : analysis.extractedAlias && !analysis.needsAliasPicker ? (
              <p className="text-xs text-muted-foreground">
                将使用本地别名{" "}
                <span className="font-medium text-foreground">
                  {analysis.resolvedAlias}
                </span>{" "}
                连接
              </p>
            ) : null}
          </div>

          {analysis.needsAliasPicker ? (
            <div className="space-y-1.5">
              <Label className="text-xs">
                本地别名
                <span className="ml-1 text-muted-foreground">
                  （未找到 &ldquo;{analysis.extractedAlias}&rdquo;，请选择）
                </span>
              </Label>
              <Select
                value={selectedAlias ?? ""}
                onValueChange={setSelectedAlias}
              >
                <SelectTrigger className="h-8 text-xs">
                  <SelectValue placeholder="选择本地连接别名" />
                </SelectTrigger>
                <SelectContent>
                  {availableLinks.map((link) => (
                    <SelectItem key={link.alias} value={link.alias}>
                      {link.alias}
                    </SelectItem>
                  ))}
                </SelectContent>
              </Select>
            </div>
          ) : null}

          <div className="space-y-1.5">
            <Label className="text-xs">本地端口</Label>
            <Input
              className="h-8 font-mono text-xs"
              placeholder="3389"
              value={localPort}
              onChange={(e) => setLocalPort(e.target.value)}
            />
          </div>

          <div className="space-y-1.5">
            <Label className="text-xs">
              名称
              <span className="ml-1 text-muted-foreground">（可选）</span>
            </Label>
            <Input
              className="h-8 text-xs"
              placeholder="如：同事的远程桌面"
              value={name}
              onChange={(e) => setName(e.target.value)}
            />
          </div>

          {availableLinks.length === 0 ? (
            <p className="text-center text-xs text-muted-foreground">
              请先在设置中添加连接别名
            </p>
          ) : null}
        </div>
      </FeatureCard>

      {/* 右侧：已创建的映射清单 */}
      <FeatureCard
        theme="emerald"
        icon={<List className="h-4.5 w-4.5" />}
        title="已创建的入口"
        description="通过本地端口访问对方开放的虚拟服务"
        badge={
          connectCount > 0
            ? { text: `${connectCount} 个`, state: "open" as const }
            : null
        }
        emptyText="还没有连接入口。在左侧填写目标地址和本地端口，点击添加。"
      >
        {connectTunnels.length > 0 ? (
          <ConnectMappingTable
            tunnels={connectTunnels}
            services={services}
            agentRunning={agentRunning}
            onCopy={(url) => void handleCopy(url)}
            onRemove={(tunnel) => void handleRemove(tunnel)}
          />
        ) : null}
      </FeatureCard>
    </div>
  );
}
