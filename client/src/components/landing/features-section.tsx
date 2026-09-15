"use client";

import { motion } from "framer-motion";
import { Activity, GitPullRequest, Lock, Radar, RotateCcw, Server } from "lucide-react";

import { Container } from "@/components/shared/container";

const features = [
  { icon: Server, index: "01", title: "Runtime without provisioning", description: "Framework detection, builds, regions, and scaling happen behind one deployment flow." },
  { icon: GitPullRequest, index: "02", title: "Preview every pull request", description: "Each branch can become an isolated environment with a shareable URL and clean teardown." },
  { icon: Radar, index: "03", title: "Operational signals built in", description: "Logs, health checks, latency, and release status are part of the deploy surface." },
  { icon: Lock, index: "04", title: "TLS and secrets handled", description: "Certificates renew automatically and secrets stay scoped to the environments that need them." },
  { icon: RotateCcw, index: "05", title: "Rollback as a first-class action", description: "Every successful deploy becomes a restore point, so recovery does not need a runbook." },
  { icon: Activity, index: "06", title: "Fast path for small teams", description: "Ship the product before spending a week designing CI, runtime, monitoring, and domains." },
] as const;

export function FeaturesSection() {
  return (
    <section id="product" className="border-b border-border/70 py-20 md:py-28">
      <Container>
        <div className="grid gap-10 lg:grid-cols-[0.7fr_1.3fr]">
          <div className="max-w-sm"><p className="font-mono text-[11px] uppercase tracking-[0.2em] text-primary">Capabilities / 06</p><h2 className="mt-4 text-3xl font-semibold leading-tight tracking-[-0.035em] text-foreground">The platform layer, compressed into one workflow.</h2><p className="mt-4 text-sm leading-6 text-muted-foreground">Production outcomes without becoming infrastructure operators.</p></div>
          <div className="grid gap-px overflow-hidden rounded-lg border border-border bg-border sm:grid-cols-2">
            {features.map(({ icon: Icon, index, title, description }, i) => <motion.article key={title} initial={{ opacity: 0, y: 16 }} whileInView={{ opacity: 1, y: 0 }} viewport={{ once: true, margin: "-60px" }} transition={{ delay: i * 0.06, duration: 0.45 }} className="group min-h-48 bg-card p-5 transition-colors hover:bg-surface"><div className="flex items-start justify-between"><span className="font-mono text-[11px] text-muted-foreground">{index}</span><Icon className="size-5 text-primary transition-transform duration-300 group-hover:-translate-y-1" /></div><h3 className="mt-10 text-sm font-medium text-foreground">{title}</h3><p className="mt-2 text-sm leading-6 text-muted-foreground">{description}</p></motion.article>)}
          </div>
        </div>
      </Container>
    </section>
  );
}
