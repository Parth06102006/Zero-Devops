"use client";

import { useState } from "react";
import { Navbar } from "./navbar";
import { HeroSection } from "./hero-section";
import { FeaturesSection } from "./features-section";
import { CtaSection } from "./cta-section";
import { DeployConsole } from "./deploy-console";

export function ZeroDevOpsLanding() {
  const [demo, setDemo] = useState(false);

  return (
    <div className="min-h-dvh overflow-hidden bg-[#050505] text-white">
      {/* Background ambient lighting */}
      <div className="pointer-events-none fixed inset-0">
        <div className="absolute left-[8%] top-[-18rem] size-[38rem] rounded-full bg-fuchsia-500/10 blur-[120px]" />
        <div className="absolute right-[-8rem] top-[15%] size-[34rem] rounded-full bg-cyan-400/10 blur-[120px]" />
        <div className="absolute bottom-[-16rem] left-[35%] size-[34rem] rounded-full bg-emerald-400/8 blur-[120px]" />
      </div>

      <Navbar onOpenDemo={() => setDemo(true)} />

      <main className="relative z-10">
        <HeroSection onOpenDemo={() => setDemo(true)} />
        <FeaturesSection />
        <CtaSection onOpenDemo={() => setDemo(true)} />
      </main>

      <DeployConsole open={demo} onClose={() => setDemo(false)} />
    </div>
  );
}
