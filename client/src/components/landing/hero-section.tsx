"use client";

import { motion } from "framer-motion";
import { CircleDot, Code2, Play, Sparkles, Zap } from "lucide-react";
import { GithubLoginButton } from "@/features/auth";
import { heroSteps } from "./data/landing-content";

interface HeroSectionProps {
  onOpenDemo: () => void;
}

export function HeroSection({ onOpenDemo }: HeroSectionProps) {
  return (
    <section className="mx-auto max-w-7xl px-5 pb-20 pt-20 lg:px-8 lg:pb-28 lg:pt-28">
      <div className="grid items-center gap-16 lg:grid-cols-[.9fr_1.1fr]">
        <motion.div
          initial={{ opacity: 0, y: 18 }}
          animate={{ opacity: 1, y: 0 }}
          transition={{ duration: 0.7 }}
        >
          <div className="inline-flex items-center gap-2 rounded-full border border-white/10 bg-white/[0.035] px-3 py-1.5 text-[10px] uppercase tracking-[.18em] text-white/45">
            <Sparkles className="size-3.5 text-fuchsia-300" /> Git push to production
          </div>
          <h1 className="mt-7 max-w-3xl text-balance text-5xl font-semibold leading-[.98] tracking-[-.06em] sm:text-6xl lg:text-[76px]">
            Deploy without becoming a <span className="gradient-text">DevOps team.</span>
          </h1>
          <p className="mt-6 max-w-xl text-base leading-7 text-white/40 sm:text-lg">
            Zero-DevOps turns a configured GitHub repository into a reproducible build and deployment
            workflow, while keeping the infrastructure visible when you need it.
          </p>
          <div className="mt-8 flex flex-col gap-3 sm:flex-row">
            <GithubLoginButton
              label="Continue with GitHub"
              size="lg"
              className="h-12 rounded-xl bg-white px-6 font-semibold text-black hover:bg-white/90"
            />
            <button
              type="button"
              onClick={onOpenDemo}
              className="inline-flex h-12 items-center justify-center gap-2 rounded-xl border border-white/10 bg-white/[0.025] px-6 text-sm text-white/70 hover:bg-white/[0.06] hover:text-white"
            >
              <Play className="size-4" /> See how it works
            </button>
          </div>
          <div className="mt-8 flex flex-wrap gap-x-5 gap-y-2 text-[11px] text-white/30">
            <span className="inline-flex items-center gap-1.5">
              <CircleDot className="size-3 text-emerald-300" /> GitHub OAuth
            </span>
            <span className="inline-flex items-center gap-1.5">
              <Zap className="size-3 text-cyan-300" /> Reproducible builds
            </span>
            <span className="inline-flex items-center gap-1.5">
              <Code2 className="size-3 text-fuchsia-300" /> Backend unchanged
            </span>
          </div>
        </motion.div>

        <motion.div
          initial={{ opacity: 0, x: 24 }}
          animate={{ opacity: 1, x: 0 }}
          transition={{ duration: 0.8, delay: 0.15 }}
          className="relative"
        >
          <div className="absolute -inset-10 bg-gradient-to-r from-cyan-400/8 via-fuchsia-500/8 to-emerald-400/8 blur-3xl" />
          <div className="relative overflow-hidden rounded-3xl border border-white/[0.1] bg-[#080808]/90 shadow-2xl shadow-black/50">
            <div className="flex items-center gap-2 border-b border-white/[0.07] px-5 py-4">
              <span className="size-2 rounded-full bg-rose-400/70" />
              <span className="size-2 rounded-full bg-amber-300/70" />
              <span className="size-2 rounded-full bg-emerald-300/70" />
              <span className="ml-3 font-mono text-[10px] text-white/25">
                zero-devops / deployment-flow
              </span>
            </div>
            <div className="relative p-5 sm:p-8">
              <div className="grid grid-cols-2 gap-3 sm:grid-cols-4">
                {heroSteps.map(({ icon: Icon, title, subtitle }, index) => (
                  <motion.div
                    key={title}
                    animate={{ y: [0, -5, 0] }}
                    transition={{ duration: 3, repeat: Infinity, delay: index * 0.25 }}
                    className="rounded-2xl border border-white/[0.08] bg-white/[0.025] p-4"
                  >
                    <div
                      className={`flex size-9 items-center justify-center rounded-xl bg-gradient-to-br ${
                        index % 2 === 0
                          ? "from-cyan-400/15 to-blue-500/10"
                          : "from-fuchsia-400/15 to-violet-500/10"
                      }`}
                    >
                      <Icon className="size-4 text-white/70" />
                    </div>
                    <p className="mt-4 text-xs font-medium">{title}</p>
                    <p className="mt-1 text-[10px] leading-4 text-white/30">{subtitle}</p>
                  </motion.div>
                ))}
              </div>
              <div className="my-5 h-px bg-gradient-to-r from-cyan-300/0 via-white/15 to-fuchsia-300/0" />
              <div className="rounded-2xl border border-white/[0.07] bg-black/50 p-4 font-mono text-[10px] leading-6 text-white/35">
                <p>
                  <span className="text-emerald-300/80">$</span> git push origin main
                </p>
                <p>
                  <span className="text-cyan-300/80">→</span> webhook accepted · build queued
                </p>
                <p>
                  <span className="text-fuchsia-300/80">→</span> worker receives immutable build job
                </p>
                <p>
                  <span className="text-amber-200/70">→</span> output is published when the backend reports success
                </p>
              </div>
            </div>
          </div>
        </motion.div>
      </div>
    </section>
  );
}
