"use client";

import { AnimatePresence, motion } from "framer-motion";
import { ChevronRight, Terminal, X } from "lucide-react";
import { demoSteps } from "./data/landing-content";

interface DeployConsoleProps {
  open: boolean;
  onClose: () => void;
}

export function DeployConsole({ open, onClose }: DeployConsoleProps) {
  return (
    <AnimatePresence>
      {open ? (
        <motion.div
          initial={{ opacity: 0 }}
          animate={{ opacity: 1 }}
          exit={{ opacity: 0 }}
          className="fixed inset-0 z-[100] flex items-center justify-center bg-black/80 p-4 backdrop-blur-xl"
        >
          <motion.div
            initial={{ opacity: 0, scale: 0.97, y: 12 }}
            animate={{ opacity: 1, scale: 1, y: 0 }}
            exit={{ opacity: 0, scale: 0.97, y: 12 }}
            className="max-h-[90vh] w-full max-w-5xl overflow-auto rounded-3xl border border-white/10 bg-[#090909] shadow-2xl"
          >
            <div className="sticky top-0 z-10 flex items-center justify-between border-b border-white/[0.07] bg-[#090909]/90 px-6 py-5 backdrop-blur">
              <div>
                <p className="text-xs uppercase tracking-[.18em] text-white/30">Interactive demo</p>
                <h3 className="mt-1 text-lg font-semibold">From Git push to deployment</h3>
              </div>
              <button
                type="button"
                onClick={onClose}
                className="rounded-lg border border-white/10 p-2 text-white/45 hover:text-white"
                aria-label="Close demo"
              >
                <X className="size-4" />
              </button>
            </div>
            <div className="grid gap-4 p-6 md:grid-cols-[.9fr_1.1fr]">
              <div className="space-y-2">
                {demoSteps.map(({ icon: Icon, number, title }) => (
                  <div
                    key={number}
                    className="flex items-center gap-4 rounded-2xl border border-white/[0.07] bg-white/[0.02] p-4"
                  >
                    <span className="flex size-9 shrink-0 items-center justify-center rounded-xl bg-white/[0.05]">
                      <Icon className="size-4 text-white/60" />
                    </span>
                    <span>
                      <span className="block text-[10px] text-white/25">{number}</span>
                      <span className="mt-0.5 block text-sm text-white/65">{title}</span>
                    </span>
                    <ChevronRight className="ml-auto size-4 text-white/15" />
                  </div>
                ))}
              </div>
              <div className="overflow-hidden rounded-2xl border border-white/[0.08] bg-[#050505]">
                <div className="flex items-center gap-2 border-b border-white/[0.07] px-4 py-3">
                  <Terminal className="size-3.5 text-white/25" />
                  <span className="font-mono text-[10px] text-white/30">workflow-preview</span>
                </div>
                <div className="p-5 font-mono text-[11px] leading-7 text-white/45">
                  <p>
                    <span className="text-emerald-300">$</span> git push origin main
                  </p>
                  <p className="text-cyan-200/70">✓ webhook received</p>
                  <p className="text-fuchsia-200/70">✓ build queued</p>
                  <p>→ worker receives approved configuration</p>
                  <p>→ build status: pending</p>
                  <p>→ build status: building</p>
                  <p>→ deployment result is read from the backend</p>
                  <p className="mt-4 border-t border-white/[0.06] pt-4 text-white/25">
                    This demo is illustrative. It does not execute these commands.
                  </p>
                </div>
              </div>
            </div>
          </motion.div>
        </motion.div>
      ) : null}
    </AnimatePresence>
  );
}
