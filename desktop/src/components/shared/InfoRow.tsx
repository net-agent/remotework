export function InfoRow({ label, value, mono }: { label: string; value: string; mono?: boolean }) {
  return (
    <div className="flex items-center justify-between py-1.5 border-b border-border/40 last:border-0">
      <span className="text-muted-foreground">{label}</span>
      <span className={`text-right truncate ml-4 ${mono ? "font-mono" : ""}`}>{value}</span>
    </div>
  );
}
