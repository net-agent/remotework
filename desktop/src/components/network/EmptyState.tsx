import { Network } from "lucide-react";
import { Button } from "@/components/ui/button";
import { useUIStore } from "@/stores/ui-store";

export function EmptyState() {
  const { openNetworkForm } = useUIStore();

  return (
    <div className="flex flex-col items-center justify-center h-full gap-4 text-center px-8 py-16">
      <div className="rounded-full bg-muted p-4">
        <Network className="h-8 w-8 text-muted-foreground" />
      </div>
      <div>
        <h3 className="font-medium mb-1">还没有网络</h3>
        <p className="text-sm text-muted-foreground">
          添加你的第一个网络连接来开始使用
        </p>
      </div>
      <Button onClick={() => openNetworkForm()}>添加网络</Button>
    </div>
  );
}
