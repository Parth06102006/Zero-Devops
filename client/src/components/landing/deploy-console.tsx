"use client";

import { motion } from "framer-motion";
import { Check, GitBranch } from "lucide-react";

const steps = [
  { label: "Detected framework", detail: "Next.js 15 · App Router" },
  { label: "Provisioned runtime", detail: "no Dockerfile, no cluster" },
  { label: "Configured TLS + domain", detail: "app.zero-devops.dev" },
  { label: "Deployed", detail: "3 regions · autoscaled" },
];

const container = {
  hidden: {},
  show: {
    transition: { staggerChildren: 0.35, delayChildren: 0.3 },
  },
};

const item = {
  hidden: { opacity: 0, y: 8 },
  show: { opacity: 1, y: 0, transition: { duration: 0.4, ease: [0.16, 1, 0.3, 1] } },
};

export function DeployConsole() {
  return (
    <div className="relative">
      <div
        aria-hidden
        className="absolute -inset-6 -z-10 rounded-[2rem] bg-primary/20 blur-3xl"
      />
      <div className="w-full max-w-md overflow-hidden rounded-xl border border-border bg-surface shadow-2xl shadow-black/40">
        <div className="flex items-center gap-1.5 border-b border-border px-4 py-3">
          <span className="size-2.5 rounded-full bg-destructive/70" />
          <span className="size-2.5 rounded-full bg-warning/70" />
          <span className="size-2.5 rounded-full bg-success/70" />
          <span className="ml-3 flex items-center gap-1.5 font-mono text-xs text-muted-foreground">
            <GitBranch className="size-3.5" />
            main
          </span>
        </div>

        <motion.ul
          variants={container}
          initial="hidden"
          animate="show"
          className="flex flex-col gap-4 p-5 font-mono text-[13px]"
        >
          {steps.map((step) => (
            <motion.li key={step.label} variants={item} className="flex items-start gap-3">
              <span className="mt-0.5 flex size-4 shrink-0 items-center justify-center rounded-full bg-success/15 text-success">
                <Check className="size-3" strokeWidth={3} />
              </span>
              <span className="flex flex-col gap-0.5">
                <span className="text-foreground">{step.label}</span>
                <span className="text-muted-foreground">{step.detail}</span>
              </span>
            </motion.li>
          ))}
          <motion.li
            variants={item}
            className="flex items-center gap-2 pl-7 text-muted-foreground"
          >
            <span className="inline-block h-3.5 w-1.5 animate-pulse bg-primary" aria-hidden />
            live in 41s
          </motion.li>
        </motion.ul>
      </div>
    </div>
  );
}
