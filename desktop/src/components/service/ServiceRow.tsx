import { ListItem } from "@/components/shared/ListItem";
import { useUIStore } from "@/stores/ui-store";
import { ArrowLeftRight, Route, Plug } from "lucide-react";
import type { ServiceStateDTO } from "@/lib/types";

const serviceIcons: Record<string, React.ReactNode> = {
  portproxy: <ArrowLeftRight className="h-3.5 w-3.5" />,
  socks5: <Route className="h-3.5 w-3.5" />,
};

const defaultIcon = <Plug className="h-3.5 w-3.5" />;

export function ServiceRow({ service }: { service: ServiceStateDTO }) {
  const { selectedService, setSelectedService } = useUIStore();

  return (
    <ListItem
      state={service.status}
      icon={serviceIcons[service.type] ?? defaultIcon}
      name={service.name}
      secondary={service.type}
      selected={selectedService === service.id}
      onClick={() => setSelectedService(service.id)}
    />
  );
}
