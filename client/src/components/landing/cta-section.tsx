"use client";

import { ArrowRight } from "lucide-react";

interface CtaSectionProps {
  onOpenDemo: () => void;
}

export function CtaSection({ onOpenDemo }: CtaSectionProps) {
  return (
    <section className="mx-auto max-w-7xl px-5 py-20 lg:px-8">
      <div className="flex flex-col gap-5 rounded-3xl border border-white/[0.08] bg-gradient-to-br from-cyan-400/[0.05] via-fuchsia-500/[0.035] to-emerald-400/[0.05] p-8 sm:p-12 lg:flex-row lg:items-center lg:justify-between">
        <div>
          <p className="text-xs uppercase tracking-[.18em] text-white/30">Zero-DevOps</p>
          <h2 className="mt-3 max-w-2xl text-3xl font-semibold tracking-[-.04em] sm:text-4xl">
            Your repository is the starting point. The platform handles the path.
          </h2>
        </div>
        <button
          type="button"
          onClick={onOpenDemo}
          className="inline-flex shrink-0 items-center gap-2 text-sm text-white/60 hover:text-white"
        >
          Explore the flow <ArrowRight className="size-4" />
        </button>
      </div>
    </section>
  );
}
