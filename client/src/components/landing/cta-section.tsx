import Link from "next/link";
import { ArrowRight, Check, Github } from "lucide-react";

import { Button } from "@/components/ui/button";
import { Container } from "@/components/shared/container";

const checks = ["Connect with GitHub", "Pick a repository", "Push to deploy"] as const;

export function CtaSection() {
  return (
    <section className="border-t border-border/60 py-20 md:py-28">
      <Container>
        <div className="grid gap-8 rounded-xl border border-border bg-card p-6 shadow-2xl shadow-black/10 md:grid-cols-[1fr_0.8fr] md:p-8">
          <div className="flex flex-col justify-center gap-5">
            <span className="inline-flex w-fit items-center gap-2 rounded-full border border-primary/25 bg-primary/10 px-3 py-1.5 text-xs font-medium text-primary">
              <Github className="size-3.5" />
              No credit card required
            </span>
            <div className="flex flex-col gap-3">
              <h2 className="max-w-xl text-balance text-3xl font-semibold tracking-tight text-foreground sm:text-4xl">
                Your next deploy should start with a commit, not an infrastructure checklist.
              </h2>
              <p className="max-w-lg text-base leading-7 text-muted-foreground">
                Sign in with GitHub and get to the point where a repository can become a monitored,
                rollback-ready deployment.
              </p>
            </div>
            <Button asChild size="lg" className="w-full sm:w-fit">
              <Link href="/login">
                Start deploying <ArrowRight />
              </Link>
            </Button>
          </div>

          <div className="rounded-lg border border-border bg-surface p-5">
            <p className="text-sm font-medium text-foreground">First-session checklist</p>
            <ul className="mt-5 grid gap-4">
              {checks.map((check) => (
                <li key={check} className="flex items-center gap-3 text-sm text-muted-foreground">
                  <span className="flex size-6 shrink-0 items-center justify-center rounded-full bg-success/15 text-success">
                    <Check className="size-3.5" />
                  </span>
                  {check}
                </li>
              ))}
            </ul>
            <div className="mt-6 rounded-md border border-border bg-background/70 p-3 font-mono text-xs text-muted-foreground">
              zero deploy --from github/main
            </div>
          </div>
        </div>
      </Container>
    </section>
  );
}
