"use client";

import { motion } from "framer-motion";
import { GitBranch, LockKeyhole, Radar, Rocket } from "lucide-react";

const details = [
  { icon: GitBranch, label: "Connect repositories with GitHub OAuth" },
  { icon: Rocket, label: "Deploy every push without YAML or cluster setup" },
  { icon: Radar, label: "Get logs, health signals, and rollback points" },
  { icon: LockKeyhole, label: "TLS and deployment secrets stay managed" },
] as const;

export function AuthContextPanel() {
  return <aside className="relative hidden min-h-[680px] flex-col justify-between overflow-hidden bg-surface p-10 lg:flex"><div className="pointer-events-none absolute -right-28 -top-28 size-96 rounded-full bg-primary/10 blur-3xl" /><div className="relative"><p className="font-mono text-[11px] uppercase tracking-[0.2em] text-primary">Developer platform / 01</p><h2 className="mt-8 max-w-sm text-4xl font-semibold leading-[1.02] tracking-[-0.05em] text-foreground">One login. Every repository.</h2><p className="mt-5 max-w-sm text-sm leading-6 text-muted-foreground">ghost uses GitHub to discover repositories, watch branches, and start the deployment flow after each push.</p></div><div className="relative space-y-3"><div className="rounded-md border border-border bg-card/80 p-4 font-mono text-xs text-muted-foreground"><div className="mb-4 flex items-center justify-between border-b border-border pb-3"><span>ghost / deployment</span><span className="text-success">● live</span></div><p><span className="text-primary">$</span> connect github</p><p className="mt-2 text-foreground">authorization accepted</p><p className="mt-2">next: choose a repository</p></div><ul className="grid gap-3 pt-3">{details.map(({ icon: Icon, label }) => <motion.li whileHover={{ x: 4 }} key={label} className="flex items-center gap-3 text-sm text-muted-foreground"><span className="flex size-8 shrink-0 items-center justify-center rounded-md border border-primary/25 bg-primary/10 text-primary"><Icon className="size-4" /></span>{label}</motion.li>)}</ul></div></aside>;
}
