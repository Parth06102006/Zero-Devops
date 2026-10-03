import {
  Boxes,
  GitBranch,
  Github,
  Rocket,
  Server,
  Terminal,
  Zap,
  type LucideIcon,
} from "lucide-react";

export interface LandingStep {
  icon: LucideIcon;
  title: string;
  subtitle: string;
}

export interface DemoStep {
  icon: LucideIcon;
  number: string;
  title: string;
}

export interface ValueProp {
  number: string;
  title: string;
  description: string;
}

export const heroSteps: LandingStep[] = [
  { icon: Github, title: "GitHub", subtitle: "Connect your source" },
  { icon: GitBranch, title: "Push", subtitle: "Webhook trigger" },
  { icon: Boxes, title: "Build", subtitle: "Queue + worker" },
  { icon: Server, title: "Deploy", subtitle: "Immutable output" },
];

export const demoSteps: DemoStep[] = [
  { icon: Github, number: "01", title: "Connect GitHub" },
  { icon: GitBranch, number: "02", title: "Select repository + branch" },
  { icon: Zap, number: "03", title: "Webhook creates a build" },
  { icon: Terminal, number: "04", title: "Worker executes approved configuration" },
  { icon: Rocket, number: "05", title: "Deployment status returns to dashboard" },
];

export const valueProps: ValueProp[] = [
  {
    number: "01",
    title: "One project model",
    description: "Repositories, branches and build configuration stay attached to an explicit project.",
  },
  {
    number: "02",
    title: "Visible build state",
    description: "Pending, building, success and failed states come from the real backend.",
  },
  {
    number: "03",
    title: "Less platform work",
    description: "The interface keeps the deployment path understandable instead of hiding it behind magic.",
  },
];
