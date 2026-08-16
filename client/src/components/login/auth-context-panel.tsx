import { GitBranch, LockKeyhole, Radar, Rocket } from "lucide-react";

import { DeploymentTimeline } from "@/components/landing/deployment-timeline";

const details = [
  { icon: GitBranch, label: "Connect repositories with GitHub OAuth" },
  { icon: Rocket, label: "Deploy every push without YAML or cluster setup" },
  { icon: Radar, label: "Get logs, health signals, and rollback points" },
  { icon: LockKeyhole, label: "TLS and deployment secrets stay managed" },
] as const;

export function AuthContextPanel() {
  return (
    <aside className="hidden min-h-[560px] flex-col justify-between border-l border-border bg-surface p-8 lg:flex">
      <div>
        <span className="inline-flex items-center gap-2 rounded-full border border-primary/25 bg-primary/10 px-3 py-1 text-xs font-medium text-primary">
          Developer platform
        </span>
        <h2 className="mt-6 max-w-sm text-balance text-3xl font-semibold tracking-tight text-foreground">
          Sign in once, ship from every repository.
        </h2>
        <p className="mt-4 max-w-sm text-sm leading-6 text-muted-foreground">
          ghost uses GitHub to discover repositories, watch branches, and start the deployment
          flow after each push.
        </p>
      </div>

      <div className="rounded-xl border border-border bg-card p-5 shadow-sm shadow-black/5">
        <DeploymentTimeline compact />
      </div>

      <ul className="grid gap-3 pt-3">
        {details.map(({ icon: Icon, label }) => (
          <li key={label} className="flex items-center gap-3 text-sm text-muted-foreground">
            <span className="flex size-8 shrink-0 items-center justify-center rounded-md bg-primary/10 text-primary">
              <Icon className="size-4" />
            </span>
            {label}
          </li>
        ))}
      </ul>
    </aside>
  );
}
