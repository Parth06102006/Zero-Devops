import { ArrowUpRight, Github } from "lucide-react";
import Link from "next/link";

import { Container } from "@/components/shared/container";

export function CtaSection() {
  return <section className="py-20 md:py-28"><Container><div className="relative overflow-hidden rounded-lg border border-primary/30 bg-card p-7 md:p-10"><div className="pointer-events-none absolute -right-20 -top-28 size-72 rounded-full bg-primary/10 blur-3xl" /><div className="relative flex flex-col gap-6 md:flex-row md:items-end md:justify-between"><div className="max-w-xl"><p className="inline-flex items-center gap-2 font-mono text-[11px] uppercase tracking-[0.2em] text-primary"><Github className="size-3.5" /> Start with GitHub</p><h2 className="mt-4 text-3xl font-semibold tracking-[-0.035em] text-foreground">Your next deploy should start with a commit.</h2><p className="mt-3 text-sm leading-6 text-muted-foreground">Sign in with GitHub and get to the point where a repository can become a monitored, rollback-ready deployment.</p></div><Link href="/login" className="inline-flex h-11 shrink-0 items-center justify-center gap-2 rounded-md bg-primary px-5 text-sm font-semibold text-primary-foreground transition-transform hover:-translate-y-0.5">Continue with GitHub <ArrowUpRight className="size-4" /></Link></div></div></Container></section>;
}
