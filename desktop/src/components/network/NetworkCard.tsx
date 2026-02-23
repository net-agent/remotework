import { ListItem } from "@/components/shared/ListItem";
import { useUIStore } from "@/stores/ui-store";
import { Network } from "lucide-react";
import type { NetworkStateDTO } from "@/lib/types";

export function NetworkCard({ network }: { network: NetworkStateDTO }) {
  const { selectedNetwork, setSelectedNetwork } = useUIStore();

  return (
    <ListItem
      state={network.state}
      icon={<Network className="h-3.5 w-3.5" />}
      name={network.name}
      secondary={network.domain}
      selected={selectedNetwork === network.name}
      onClick={() => setSelectedNetwork(network.name)}
    />
  );
}
