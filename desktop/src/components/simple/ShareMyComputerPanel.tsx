import { useMemo } from "react";
import { Button } from "@/components/ui/button";
import {
  saveSocks5Preset,
  useTunnelSaveContext,
} from "@/components/simple/share-preset/socks5-preset-save";
import { LocalPortMappingTable } from "@/components/simple/LocalPortMappingTable";
import { FeatureCard } from "@/components/simple/FeatureCard";
import {
  generateRandomAuthcode,
  getPresetMeta,
} from "@/lib/simple-domain/share-preset-rules";
import type { SimpleSessionVM } from "@/lib/view-model/simple-session-vm";
import { useSimpleActions } from "@/lib/view-model/simple-actions";
import { toast } from "sonner";
import { Plus, Globe, Network } from "lucide-react";

async function copyAddress(listenURL: string) {
  try {
    await navigator.clipboard.writeText(listenURL);
    toast.success("已复制虚拟地址");
  } catch (error) {
    toast.error(`复制失败: ${String(error)}`);
  }
}

export function ShareMyComputerPanel({
  session,
}: {
  session: SimpleSessionVM;
}) {
  const actions = useSimpleActions();
  const saveContext = useTunnelSaveContext();
  const selectedAlias = session.selectedShareLink?.alias ?? null;
  const socks5Meta = useMemo(() => getPresetMeta("socks5"), []);

  const socks5Row =
    session.shareServiceRows.find((row) => row.key === "socks5") ?? null;
  const localPortRow =
    session.shareServiceRows.find((row) => row.key === "local-port") ?? null;

  const handleOpenSocks5 = async () => {
    if (!selectedAlias) {
      toast.error("请先选择连接信息");
      return;
    }
    await saveSocks5Preset({
      selectedAlias,
      authcode: generateRandomAuthcode(),
      context: saveContext,
    });
  };

  return (
    <div className="space-y-4">
      <div className="space-y-3">
        <div className="flex flex-wrap items-center justify-between gap-2">
          {!selectedAlias ? (
            <div className="rounded-lg border border-dashed bg-muted/20 px-3 py-2 text-xs text-muted-foreground">
              先选择连接信息，再开放或管理共享服务。
            </div>
          ) : (
            <div />
          )}
          <div className="flex flex-wrap gap-2">
            {!selectedAlias ? (
              <Button size="sm" onClick={actions.createShareLink}>
                输入连接信息
              </Button>
            ) : null}
            {session.requiresRestart ? (
              <Button size="sm" variant="outline" onClick={actions.restart}>
                重启后生效
              </Button>
            ) : null}
          </div>
        </div>

        <div className="grid items-start gap-4 sm:grid-cols-[320px_minmax(0,1fr)]">
          {/* 左侧：开放本地网络 */}
          <FeatureCard
            theme="violet"
            icon={<Globe className="h-4.5 w-4.5" />}
            title={socks5Row?.title ?? socks5Meta.title}
            description="开放本机网络访问权限"
            badge={
              socks5Row
                ? { text: socks5Row.statusText, state: socks5Row.state }
                : null
            }
            errorText={socks5Row?.lastErr}
            addresses={
              socks5Row?.listenURL
                ? [
                    {
                      label: "虚拟服务地址",
                      url: socks5Row.listenURL,
                      onCopy: copyAddress,
                    },
                  ]
                : []
            }
            emptyText="开放后，对方可通过虚拟地址访问你的本地网络"
            footerAction={
              socks5Row?.primaryActionKind === "open" || !socks5Row ? (
                <Button
                  size="sm"
                  className="w-full bg-violet-600 text-white hover:bg-violet-500 dark:bg-violet-600 dark:hover:bg-violet-500"
                  onClick={handleOpenSocks5}
                  disabled={!selectedAlias}
                >
                  启动代理服务
                </Button>
              ) : (
                <Button
                  size="sm"
                  variant="outline"
                  className="w-full"
                  onClick={() =>
                    actions.closeSimpleShareService({
                      tunnelId: socks5Row.tunnelId ?? undefined,
                      configIndex: socks5Row.configIndex ?? undefined,
                    })
                  }
                >
                  关闭代理服务
                </Button>
              )
            }
          />

          {/* 右侧：开放本地端口 */}
          <FeatureCard
            theme="sky"
            icon={<Network className="h-4.5 w-4.5" />}
            title="开放本地端口"
            description="对方可以通过虚拟地址访问本地端口服务"
            badge={
              localPortRow
                ? { text: localPortRow.statusText, state: localPortRow.state }
                : null
            }
            errorText={localPortRow?.lastErr}
            headerAction={undefined}
            footerAction={
              <Button
                size="sm"
                className="w-full bg-sky-600 text-white hover:bg-sky-500 dark:bg-sky-600 dark:hover:bg-sky-500"
                onClick={() =>
                  actions.openSimpleShareServiceDialog("local-port")
                }
                disabled={!selectedAlias}
              >
                <Plus className="mr-1 h-3.5 w-3.5" />
                添加端口
              </Button>
            }
            emptyText="暂未开放任何端口，点击添加端口按钮开始"
          >
            {localPortRow && localPortRow.items.length > 0 ? (
              <LocalPortMappingTable
                items={localPortRow.items}
                isActive={localPortRow.state !== "closed"}
                onCopyAddress={(url) => void copyAddress(url)}
                onCloseItem={(input) => actions.closeSimpleShareService(input)}
              />
            ) : null}
          </FeatureCard>
        </div>
      </div>

      {session.userFacingHints.length > 0 ? (
        <div className="rounded-xl border border-dashed bg-muted/20 px-4 py-3">
          <div className="flex flex-col gap-3 sm:flex-row sm:items-start sm:justify-between">
            <div className="space-y-1">
              <h3 className="text-sm font-medium">处理提示</h3>
              <ul className="list-disc space-y-1 pl-4 text-xs text-muted-foreground">
                {session.userFacingHints.slice(0, 3).map((hint) => (
                  <li key={hint}>{hint}</li>
                ))}
              </ul>
            </div>
            <Button
              size="sm"
              type="button"
              variant="ghost"
              onClick={actions.openAdvancedNetworks}
            >
              进入高级模式
            </Button>
          </div>
        </div>
      ) : null}
    </div>
  );
}
