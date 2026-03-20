import { Button } from "@/components/ui/button";

export function SimpleActionCard({
  title,
  description,
  primaryLabel,
  secondaryLabel,
  onPrimaryClick,
  onSecondaryClick,
  disabled,
}: {
  title: string;
  description: string;
  primaryLabel: string;
  secondaryLabel?: string;
  onPrimaryClick: () => void;
  onSecondaryClick?: () => void;
  disabled?: boolean;
}) {
  return (
    <div className="rounded-xl border bg-card p-4 shadow-sm">
      <div className="space-y-1">
        <h3 className="text-sm font-medium">{title}</h3>
        <p className="text-xs text-muted-foreground">{description}</p>
      </div>
      <div className="mt-4 flex flex-wrap gap-2">
        <Button size="sm" onClick={onPrimaryClick} disabled={disabled}>
          {primaryLabel}
        </Button>
        {secondaryLabel && onSecondaryClick ? (
          <Button size="sm" variant="outline" onClick={onSecondaryClick}>
            {secondaryLabel}
          </Button>
        ) : null}
      </div>
    </div>
  );
}
