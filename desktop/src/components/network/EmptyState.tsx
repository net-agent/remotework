import { Network } from "lucide-react";
import { Button } from "@/components/ui/button";
import { useUIStore } from "@/stores/ui-store";

export function EmptyState() {
  const { openNetworkForm, openServiceForm } = useUIStore();

  return (
    <div className="flex flex-col items-center justify-center h-full gap-3 text-center px-8">
      <div className="rounded-full bg-accent/60 p-3">
        <Network className="h-6 w-6 text-muted-foreground" />
      </div>
      <div>
        <p className="text-sm font-medium mb-0.5">还没有网络</p>
        <p className="text-xs text-muted-foreground">
          添加网络连接来开始使用
        </p>
      </div>
      <div className="flex gap-2">
        <Button size="sm" onClick={() => openNetworkForm()}>添加网络</Button>
        <Button size="sm" variant="ghost" onClick={() => openServiceForm()}>添加服务</Button>
      </div>
    </div>
  );
}
