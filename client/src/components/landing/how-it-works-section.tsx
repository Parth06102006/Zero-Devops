"use client";

import { motion } from "framer-motion";
import { GitBranch, MonitorCheck, PackageCheck, Rocket } from "lucide-react";

import { Container } from "@/components/shared/container";

const steps = [
  { icon: GitBranch, number: "01", title: "Connect a repository", description: "Authorize GitHub and choose the repo that should become a live service." },
  { icon: PackageCheck, number: "02", title: "Let ghost detect it", description: "The platform reads the framework, package manager, build command, and runtime needs." },
  { icon: Rocket, number: "03", title: "Ship on every push", description: "Builds produce immutable releases with live URLs, TLS, regions, and rollback points." },
  { icon: MonitorCheck, number: "04", title: "Operate from one place", description: "Watch status, logs, health, and deployment history without wiring separate tools." },
] as const;

export function HowItWorksSection() {
  return (
    <section id="workflow" className="border-b border-border/70 py-20 md:py-28">
      <Container>
        <div className="flex flex-col gap-4 md:flex-row md:items-end md:justify-between"><div><p className="font-mono text-[11px] uppercase tracking-[0.2em] text-primary">Workflow / 04</p><h2 className="mt-4 max-w-xl text-3xl font-semibold leading-tight tracking-[-0.035em] text-foreground">A production path that follows how developers already work.</h2></div><p className="max-w-xs text-sm leading-6 text-muted-foreground">Connect GitHub, deploy on push, and keep release state visible.</p></div>
        <div className="relative mt-12 grid gap-4 lg:grid-cols-4">{steps.map(({ icon: Icon, number, title, description }, index) => <motion.article key={number} initial={{ opacity: 0, y: 20 }} whileInView={{ opacity: 1, y: 0 }} viewport={{ once: true, margin: "-80px" }} transition={{ delay: index * 0.1, duration: 0.5 }} className="relative flex min-h-56 flex-col justify-between rounded-lg border border-border bg-card p-5"><div className="flex items-center justify-between"><span className="font-mono text-xs text-primary">{number}</span><span className="flex size-9 items-center justify-center rounded-md border border-primary/30 bg-primary/10 text-primary"><Icon className="size-4" /></span></div><div><h3 className="text-sm font-medium text-foreground">{title}</h3><p className="mt-2 text-sm leading-6 text-muted-foreground">{description}</p></div></motion.article>)}</div>
      </Container>
    </section>
  );
}
