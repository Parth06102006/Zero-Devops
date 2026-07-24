import { Container } from "@/components/shared/container";

const steps = [
  {
    number: "01",
    title: "Connect a repository",
    description: "Authorize GitHub and pick the repo you want running. No config files required.",
  },
  {
    number: "02",
    title: "Push to your branch",
    description: "Every push triggers a build. Zero DevOps detects your framework and builds it correctly.",
  },
  {
    number: "03",
    title: "It's live",
    description: "TLS, a domain, monitoring, and autoscaling are already in place by the time the build finishes.",
  },
];

export function HowItWorksSection() {
  return (
    <section id="how-it-works" className="border-t border-border/60 py-20 md:py-28">
      <Container>
        <h2 className="max-w-xl text-balance text-3xl font-semibold tracking-tight text-foreground sm:text-4xl">
          From push to production, in order.
        </h2>

        <ol className="mt-14 grid gap-10 sm:grid-cols-3 sm:gap-6">
          {steps.map((step, index) => (
            <li key={step.number} className="relative flex flex-col gap-3">
              <span className="font-mono text-sm text-primary">{step.number}</span>
              <h3 className="text-base font-medium text-foreground">{step.title}</h3>
              <p className="text-sm text-muted-foreground">{step.description}</p>
              {index < steps.length - 1 ? (
                <span
                  aria-hidden
                  className="absolute right-[-1.5rem] top-1.5 hidden h-px w-6 bg-border sm:block"
                />
              ) : null}
            </li>
          ))}
        </ol>
      </Container>
    </section>
  );
}
