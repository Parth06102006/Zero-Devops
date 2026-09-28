import { Check, Clock3, GitBranch, Rocket, ShieldCheck } from "lucide-react";

import { cn } from "@/lib/utils/cn";

const events = [
  {
    icon: GitBranch,
    title: "GitHub repo connected",
    detail: "main branch watched for every push",
    tone: "success",
  },
  {
    icon: Check,
    title: "Framework detected",
    detail: "Next.js 15 with app router",
    tone: "success",
  },
  {
    icon: Rocket,
    title: "Runtime provisioned",
    detail: "autoscaled across 3 regions",
    tone: "primary",
  },
  {
    icon: ShieldCheck,
    title: "TLS and monitoring ready",
    detail: "live URL, logs, and rollback enabled",
    tone: "primary",
  },
] as const;

export function DeploymentTimeline({ compact = false }: { compact?: boolean }) {
  return (
    <ol className={cn("flex flex-col", compact ? "gap-3" : "gap-4")}>
      {events.map(({ icon: Icon, title, detail, tone }, index) => (
        <li key={title} className="relative flex gap-3">
          {index < events.length - 1 ? (
            <span
              aria-hidden
              className="absolute left-[0.6875rem] top-7 h-[calc(100%-0.5rem)] w-px bg-border"
            />
          ) : null}
          <span
            className={cn(
              "relative z-10 mt-0.5 flex size-6 shrink-0 items-center justify-center rounded-full border bg-surface",
              tone === "success"
                ? "border-success/30 text-success"
                : "border-primary/35 text-primary",
            )}
          >
            <Icon className="size-3.5" />
          </span>
          <span className="flex min-w-0 flex-col gap-0.5">
            <span className="text-sm font-medium text-foreground">{title}</span>
            <span className="text-sm text-muted-foreground">{detail}</span>
          </span>
        </li>
      ))}
      <li className="flex items-center gap-3 pl-9 font-mono text-xs text-muted-foreground">
        <Clock3 className="size-3.5 text-primary" />
        Production URL ready in 41s
      </li>
    </ol>
  );
}
