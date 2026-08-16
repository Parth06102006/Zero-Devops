import Link from "next/link";
import { ArrowRight, CheckCircle2, Github, ShieldCheck, Sparkles } from "lucide-react";

import { Button } from "@/components/ui/button";
import { Container } from "@/components/shared/container";
import { ProductPreview } from "@/components/landing/product-preview";
import { TrustBadge } from "@/components/landing/trust-badge";

export function HeroSection() {
  return (
    <section className="relative overflow-hidden">
      <div
        aria-hidden
        className="bg-grid mask-fade-b pointer-events-none absolute inset-0 -z-10 opacity-70"
      />
      <div
        aria-hidden
        className="pointer-events-none absolute left-1/2 top-10 -z-10 size-[28rem] -translate-x-1/2 rounded-full bg-primary/20 blur-3xl"
      />

      <Container className="grid items-center gap-14 py-16 md:py-24 lg:grid-cols-[0.95fr_1.05fr] lg:gap-10">
        <div className="flex flex-col items-start gap-7">
          <span className="inline-flex items-center gap-2 rounded-full border border-primary/25 bg-primary/10 px-3 py-1.5 font-mono text-xs text-primary">
            <Sparkles className="size-3.5" />
            Git push to production, without the platform team
          </span>

          <div className="flex flex-col gap-5">
            <h1 className="max-w-2xl text-balance text-4xl font-semibold tracking-tight text-foreground sm:text-5xl md:text-6xl">
              Deploy from GitHub.
              <span className="block text-muted-foreground">Skip the infrastructure queue.</span>
            </h1>
            <p className="max-w-xl text-balance text-lg leading-8 text-muted-foreground">
              ghost detects your app, builds it, provisions the runtime, and ships a
              monitored deployment with TLS, logs, autoscaling, and rollback points already wired.
            </p>
          </div>

          <div className="flex w-full flex-col gap-3 sm:w-auto sm:flex-row">
            <Button asChild size="lg" className="sm:min-w-44">
              <Link href="/login">
                Start deploying <ArrowRight />
              </Link>
            </Button>
            <Button asChild size="lg" variant="outline" className="sm:min-w-44">
              <Link href="#workflow">View workflow</Link>
            </Button>
          </div>

          <div className="flex flex-wrap gap-2">
            <TrustBadge icon={Github} label="GitHub OAuth" />
            <TrustBadge icon={CheckCircle2} label="No credit card to start" />
            <TrustBadge icon={ShieldCheck} label="TLS and rollback included" />
          </div>
        </div>

        <div className="flex justify-center lg:justify-end">
          <ProductPreview />
        </div>
      </Container>
    </section>
  );
}
