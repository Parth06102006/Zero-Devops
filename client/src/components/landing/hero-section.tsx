"use client";

import { motion } from "framer-motion";
import { ArrowUpRight, CheckCircle2, Github, ShieldCheck, Sparkles } from "lucide-react";
import Link from "next/link";

import { Container } from "@/components/shared/container";
import { ProductPreview } from "@/components/landing/product-preview";

export function HeroSection() {
  return (
    <section className="relative overflow-hidden border-b border-border/70">
      <div className="pointer-events-none absolute inset-0 bg-grid mask-fade-b opacity-50" />
      <div className="pointer-events-none absolute -left-48 top-20 size-[34rem] rounded-full bg-primary/10 blur-3xl" />
      <Container className="relative grid items-center gap-14 py-20 md:py-28 lg:grid-cols-[0.85fr_1.15fr] lg:gap-10 lg:py-32">
        <div className="reveal-up flex flex-col items-start gap-7">
          <span className="inline-flex items-center gap-2 rounded-full border border-primary/30 bg-primary/10 px-3 py-1.5 font-mono text-[11px] uppercase tracking-[0.16em] text-primary"><Sparkles className="size-3.5" /> Git push to production</span>
          <div className="space-y-5">
            <h1 className="max-w-2xl text-balance text-5xl font-semibold leading-[0.98] tracking-[-0.055em] text-foreground sm:text-6xl lg:text-7xl">Ship the product.<span className="block text-muted-foreground">Skip the platform queue.</span></h1>
            <p className="max-w-xl text-base leading-7 text-muted-foreground sm:text-lg">ghost detects your app, builds it, provisions the runtime, and ships a monitored deployment with TLS, logs, autoscaling, and rollback points already wired.</p>
          </div>
          <div className="flex w-full flex-col gap-3 sm:w-auto sm:flex-row"><Link href="/login" className="inline-flex h-12 items-center justify-center gap-2 rounded-md bg-primary px-6 text-sm font-semibold text-primary-foreground shadow-[0_12px_30px_hsl(var(--primary)/0.22)] transition-transform hover:-translate-y-0.5">Continue with GitHub <ArrowUpRight className="size-4" /></Link><Link href="#workflow" className="inline-flex h-12 items-center justify-center rounded-md border border-border bg-surface/50 px-6 text-sm font-medium text-foreground transition-colors hover:bg-surface">See the workflow</Link></div>
          <div className="flex flex-wrap gap-x-5 gap-y-2 text-xs text-muted-foreground"><span className="inline-flex items-center gap-1.5"><Github className="size-3.5 text-primary" /> GitHub OAuth</span><span className="inline-flex items-center gap-1.5"><CheckCircle2 className="size-3.5 text-success" /> No credit card to start</span><span className="inline-flex items-center gap-1.5"><ShieldCheck className="size-3.5 text-accent" /> TLS and rollback included</span></div>
        </div>
        <motion.div initial={{ opacity: 0, x: 28 }} animate={{ opacity: 1, x: 0 }} transition={{ duration: 0.8, delay: 0.2, ease: [0.16, 1, 0.3, 1] }} className="lg:pl-4"><ProductPreview /></motion.div>
      </Container>
      <div className="mx-auto flex max-w-6xl items-center gap-3 px-6 pb-5 font-mono text-[10px] uppercase tracking-[0.2em] text-muted-foreground/70"><span className="size-1.5 rounded-full bg-success" /> Systems online <span className="h-px flex-1 bg-border" /> zero-deploy / 01</div>
    </section>
  );
}
