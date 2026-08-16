import { Activity, GitPullRequest, Lock, Radar, RotateCcw, Server, Timer } from "lucide-react";

import { Container } from "@/components/shared/container";
import { SectionHeader } from "@/components/landing/section-header";

const features = [
  {
    icon: Server,
    title: "Runtime without provisioning",
    description:
      "Framework detection, builds, regions, and scaling happen behind one deployment flow.",
  },
  {
    icon: GitPullRequest,
    title: "Preview every pull request",
    description:
      "Each branch can become an isolated environment with a shareable URL and clean teardown.",
  },
  {
    icon: Radar,
    title: "Operational signals built in",
    description:
      "Logs, health checks, latency, and release status are part of the deploy surface.",
  },
  {
    icon: Lock,
    title: "TLS and secrets handled",
    description:
      "Certificates renew automatically and secrets stay scoped to the environments that need them.",
  },
  {
    icon: RotateCcw,
    title: "Rollback as a first-class action",
    description:
      "Every successful deploy becomes a restore point, so recovery does not need a runbook.",
  },
  {
    icon: Timer,
    title: "Fast path for small teams",
    description:
      "Ship the product before you spend a week designing CI, runtime, monitoring, and domains.",
  },
] as const;

export function FeaturesSection() {
  return (
    <section id="product" className="border-t border-border/60 py-20 md:py-28">
      <Container>
        <SectionHeader
          eyebrow="Product"
          title="The deployment platform layer, compressed into one workflow."
          description="The page is built for developers who want production outcomes without becoming infrastructure operators."
        />

        <div className="mt-12 grid gap-px overflow-hidden rounded-xl border border-border bg-border sm:grid-cols-2 lg:grid-cols-3">
          {features.map(({ icon: Icon, title, description }) => (
            <article key={title} className="flex min-h-52 flex-col justify-between bg-card p-6">
              <div className="flex size-10 items-center justify-center rounded-md bg-primary/10 text-primary">
                <Icon className="size-5" />
              </div>
              <div className="mt-8 flex flex-col gap-2">
                <h3 className="text-base font-medium text-foreground">{title}</h3>
                <p className="text-sm leading-6 text-muted-foreground">{description}</p>
              </div>
            </article>
          ))}
        </div>

        <div className="mt-8 flex flex-col gap-3 rounded-xl border border-border bg-surface p-5 sm:flex-row sm:items-center sm:justify-between">
          <div className="flex items-start gap-3">
            <span className="flex size-9 shrink-0 items-center justify-center rounded-md bg-primary text-primary-foreground">
              <Activity className="size-4" />
            </span>
            <div>
              <p className="text-sm font-medium text-foreground">Designed around deployment state</p>
              <p className="mt-1 text-sm text-muted-foreground">
                Every UI pattern points users back to builds, releases, health, and recovery.
              </p>
            </div>
          </div>
          <p className="font-mono text-xs text-muted-foreground">push / build / deploy / monitor</p>
        </div>
      </Container>
    </section>
  );
}
