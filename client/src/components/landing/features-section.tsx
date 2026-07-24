import { Activity, GitPullRequest, Lock, Radar, Server, Timer } from "lucide-react";

import { Container } from "@/components/shared/container";

const features = [
  {
    icon: Server,
    title: "No servers to provision",
    description:
      "Every push builds and runs on infrastructure that scales itself. There's no cluster to size and no capacity to plan.",
  },
  {
    icon: GitPullRequest,
    title: "Preview every pull request",
    description:
      "Each PR gets its own isolated environment with a shareable URL, torn down automatically when it's merged or closed.",
  },
  {
    icon: Radar,
    title: "Built-in monitoring",
    description:
      "Error rates, latency, and logs are collected from the first deploy — no agent to install, no dashboard to configure.",
  },
  {
    icon: Lock,
    title: "TLS and secrets handled",
    description:
      "Certificates renew themselves and environment secrets stay encrypted at rest, scoped per environment automatically.",
  },
  {
    icon: Timer,
    title: "Rollback in one click",
    description:
      "Every deploy is a restore point. If something regresses, go back to the last known-good build immediately.",
  },
  {
    icon: Activity,
    title: "Autoscaling by default",
    description:
      "Traffic spikes are absorbed automatically and scaled back down after, so you never pay for idle capacity.",
  },
];

export function FeaturesSection() {
  return (
    <section id="product" className="border-t border-border/60 py-20 md:py-28">
      <Container>
        <div className="max-w-xl">
          <h2 className="text-balance text-3xl font-semibold tracking-tight text-foreground sm:text-4xl">
            Everything a DevOps team would set up, done for you.
          </h2>
          <p className="mt-4 text-muted-foreground">
            You get the outcomes of a dedicated platform team without hiring
            one, configuring one, or being paged by one at 2am.
          </p>
        </div>

        <div className="mt-14 grid gap-px overflow-hidden rounded-xl border border-border bg-border sm:grid-cols-2 lg:grid-cols-3">
          {features.map(({ icon: Icon, title, description }) => (
            <div key={title} className="flex flex-col gap-3 bg-surface p-6">
              <Icon className="size-5 text-primary" />
              <h3 className="text-sm font-medium text-foreground">{title}</h3>
              <p className="text-sm text-muted-foreground">{description}</p>
            </div>
          ))}
        </div>
      </Container>
    </section>
  );
}
