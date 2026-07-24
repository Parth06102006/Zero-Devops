import Link from "next/link";
import { ArrowRight } from "lucide-react";

import { Button } from "@/components/ui/button";
import { Container } from "@/components/shared/container";
import { DeployConsole } from "@/components/landing/deploy-console";

export function HeroSection() {
  return (
    <section className="relative overflow-hidden">
      <div
        aria-hidden
        className="bg-grid mask-fade-b pointer-events-none absolute inset-0 -z-10 opacity-60"
      />
      <Container className="grid items-center gap-16 py-20 md:py-28 lg:grid-cols-[1.1fr_1fr] lg:gap-12">
        <div className="flex flex-col items-start gap-6">
          <span className="inline-flex items-center gap-2 rounded-full border border-border bg-surface px-3 py-1 font-mono text-xs text-muted-foreground">
            <span className="size-1.5 rounded-full bg-success" />
            No infra to manage, ever
          </span>

          <h1 className="max-w-xl text-balance text-4xl font-semibold tracking-tight text-foreground sm:text-5xl md:text-6xl">
            Push code.
            <br />
            <span className="text-muted-foreground">Skip the DevOps.</span>
          </h1>

          <p className="max-w-md text-balance text-lg text-muted-foreground">
            Zero DevOps turns a git push into a running, monitored, autoscaled
            deployment. No YAML, no clusters, no on-call rotation.
          </p>

          <div className="mt-2 flex flex-col gap-3 sm:flex-row">
            <Button asChild size="lg">
              <Link href="/login">
                Start deploying <ArrowRight />
              </Link>
            </Button>
            <Button asChild size="lg" variant="outline">
              <Link href="#how-it-works">See how it works</Link>
            </Button>
          </div>

          <p className="font-mono text-xs text-muted-foreground">
            Connect a GitHub repo → live in under a minute.
          </p>
        </div>

        <div className="flex justify-center lg:justify-end">
          <DeployConsole />
        </div>
      </Container>
    </section>
  );
}
