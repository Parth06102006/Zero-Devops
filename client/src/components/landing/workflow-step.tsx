import type { LucideIcon } from "lucide-react";

interface WorkflowStepProps {
  icon: LucideIcon;
  number: string;
  title: string;
  description: string;
}

export function WorkflowStep({ icon: Icon, number, title, description }: WorkflowStepProps) {
  return (
    <div className="group relative flex min-h-48 flex-col justify-between rounded-lg border border-border bg-card p-5 shadow-sm shadow-black/5 transition-colors hover:border-primary/45">
      <div className="flex items-start justify-between gap-4">
        <span className="font-mono text-xs text-primary">{number}</span>
        <span className="flex size-9 items-center justify-center rounded-md bg-primary/10 text-primary transition-colors group-hover:bg-primary group-hover:text-primary-foreground">
          <Icon className="size-4" />
        </span>
      </div>
      <div className="mt-8 flex flex-col gap-2">
        <h3 className="text-base font-medium text-foreground">{title}</h3>
        <p className="text-sm leading-6 text-muted-foreground">{description}</p>
      </div>
    </div>
  );
}
