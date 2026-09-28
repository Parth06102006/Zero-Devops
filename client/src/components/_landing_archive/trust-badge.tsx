import type { LucideIcon } from "lucide-react";

interface TrustBadgeProps {
  icon: LucideIcon;
  label: string;
}

export function TrustBadge({ icon: Icon, label }: TrustBadgeProps) {
  return (
    <span className="inline-flex items-center gap-2 rounded-full border border-border bg-surface px-3 py-1.5 text-xs font-medium text-muted-foreground shadow-sm shadow-black/5">
      <Icon className="size-3.5 text-primary" />
      {label}
    </span>
  );
}
