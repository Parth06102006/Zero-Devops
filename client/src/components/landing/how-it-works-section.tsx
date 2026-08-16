import { GitBranch, MonitorCheck, PackageCheck, Rocket } from "lucide-react";

import { Container } from "@/components/shared/container";
import { SectionHeader } from "@/components/landing/section-header";
import { WorkflowStep } from "@/components/landing/workflow-step";

const steps = [
  {
    icon: GitBranch,
    number: "01",
    title: "Connect a repository",
    description: "Authorize GitHub and choose the repo that should become a live service.",
  },
  {
    icon: PackageCheck,
    number: "02",
    title: "Let ghost detect it",
    description: "The platform reads the framework, package manager, build command, and runtime needs.",
  },
  {
    icon: Rocket,
    number: "03",
    title: "Ship on every push",
    description: "Builds produce immutable releases with live URLs, TLS, regions, and rollback points.",
  },
  {
    icon: MonitorCheck,
    number: "04",
    title: "Operate from one place",
    description: "Watch status, logs, health, and deployment history without wiring separate tools.",
  },
] as const;

export function HowItWorksSection() {
  return (
    <section id="workflow" className="border-t border-border/60 py-20 md:py-28">
      <Container>
        <SectionHeader
          eyebrow="Workflow"
          title="A production path that follows how developers already work."
          description="The interface should make the deployment lifecycle obvious before users connect their first repository."
        />

        <div className="mt-12 grid gap-4 sm:grid-cols-2 lg:grid-cols-4">
          {steps.map((step) => (
            <WorkflowStep key={step.number} {...step} />
          ))}
        </div>
      </Container>
    </section>
  );
}
