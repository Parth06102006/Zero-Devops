import { GitCommitHorizontal, Rocket } from "lucide-react";
import type { Metadata } from "next";

import { EmptyState } from "@/components/shared/empty-state";

export const metadata: Metadata = { title: "Deployments" };

export default function DeploymentsPage() {
  return <div className="flex flex-col gap-8"><div className="flex flex-col gap-2 border-b border-border pb-6 sm:flex-row sm:items-end sm:justify-between"><div><p className="font-mono text-[10px] uppercase tracking-[0.2em] text-primary">Release history</p><h1 className="mt-2 text-3xl font-semibold tracking-[-0.04em] text-foreground">Deployments</h1><p className="mt-2 text-sm text-muted-foreground">Every build and release, across every environment, will be listed here.</p></div><span className="inline-flex w-fit items-center gap-2 rounded-full border border-border bg-card px-3 py-1.5 font-mono text-[10px] uppercase tracking-[0.15em] text-muted-foreground"><Rocket className="size-3.5 text-primary" /> No releases</span></div><div className="flex flex-wrap items-center gap-2"><span className="rounded-md bg-primary/10 px-3 py-2 text-xs font-medium text-primary">All deployments</span><span className="rounded-md border border-border px-3 py-2 text-xs text-muted-foreground">Live</span><span className="rounded-md border border-border px-3 py-2 text-xs text-muted-foreground">Building</span><span className="rounded-md border border-border px-3 py-2 text-xs text-muted-foreground">Failed</span></div><EmptyState icon={GitCommitHorizontal} title="No deployments yet" description="Once a project is connected, every push will show up here with its build and release status." /></div>;
}
