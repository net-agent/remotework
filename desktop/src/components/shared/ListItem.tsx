import type { ReactNode } from "react";
import { StatusDot } from "@/components/shared/StatusDot";

export function ListItem({
  state,
  icon,
  name,
  secondary,
  selected,
  onClick,
}: {
  state: string;
  icon?: ReactNode;
  name: string;
  secondary?: string;
  selected: boolean;
  onClick: () => void;
}) {
  return (
    <div
      className={`flex items-center gap-2 px-2.5 py-2 rounded-lg cursor-pointer select-none transition-colors ${
        selected ? "bg-primary/15 ring-1 ring-primary/40" : "hover:bg-accent/50"
      }`}
      onClick={onClick}
    >
      <StatusDot state={state} />
      {icon && <span className="text-muted-foreground shrink-0">{icon}</span>}
      <div className="min-w-0 flex-1">
        <div className="text-sm font-medium leading-tight">{name}</div>
        {secondary && (
          <div className="text-xs text-muted-foreground leading-tight mt-0.5 truncate">
            {secondary}
          </div>
        )}
      </div>
    </div>
  );
}
