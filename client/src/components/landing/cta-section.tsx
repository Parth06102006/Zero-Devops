import Link from "next/link";
import { ArrowRight } from "lucide-react";

import { Button } from "@/components/ui/button";
import { Container } from "@/components/shared/container";

export function CtaSection() {
  return (
    <section className="border-t border-border/60 py-20 md:py-28">
      <Container className="flex flex-col items-center gap-6 text-center">
        <h2 className="max-w-lg text-balance text-3xl font-semibold tracking-tight text-foreground sm:text-4xl">
          Your next deploy doesn&apos;t need a runbook.
        </h2>
        <p className="max-w-md text-muted-foreground">
          Connect a repository and see it live in under a minute. No credit
          card required to start.
        </p>
        <Button asChild size="lg">
          <Link href="/login">
            Start deploying <ArrowRight />
          </Link>
        </Button>
      </Container>
    </section>
  );
}
