"use client";

import { usePathname } from "next/navigation";
import Link from "next/link";
import { useState, type ReactNode } from "react";
import {
  BarChart3, ChevronDown, CircleHelp, FileText, FolderKanban, Github,
  LayoutDashboard, Menu, Rocket, Search, Settings, SlidersHorizontal, Sparkles, X,
} from "lucide-react";
import { cn } from "@/lib/utils/cn";
import { Logo } from "@/components/shared/logo";
import { UserMenu } from "@/features/auth";
import { NewDeploymentDialog } from "@/features/projects";

import type { ComponentProps } from "react";

type LinkHref = ComponentProps<typeof Link>["href"];

interface NavItem {
  href: LinkHref;
  label: string;
  icon: React.ComponentType<{ className?: string }>;
  soon?: boolean;
}

const nav: readonly NavItem[] = [
  { href: "/dashboard", label: "Overview", icon: LayoutDashboard },
  { href: "/projects", label: "Projects", icon: FolderKanban },
  { href: "/deployments", label: "Deployments", icon: Rocket },
  { href: "/logs", label: "Logs", icon: FileText },
  { href: "/environment-variables", label: "Environment Variables", icon: SlidersHorizontal, soon: true },
  { href: "/github", label: "GitHub", icon: Github },
  { href: "/settings", label: "Settings", icon: Settings },
];

export function AppShell({ children }: { children: ReactNode }) {
  const pathname = usePathname();
  const [mobileOpen, setMobileOpen] = useState(false);
  const [newDeploymentOpen, setNewDeploymentOpen] = useState(false);
  const active = nav.find(
    (item) => pathname === String(item.href) || pathname.startsWith(`${String(item.href)}/`),
  );

  return (
    <div className="min-h-dvh bg-[#050505] text-white">
      <aside className={cn("fixed inset-y-0 left-0 z-50 flex w-[258px] flex-col border-r border-white/[0.08] bg-[#060606]/95 px-3 py-4 backdrop-blur-2xl transition-transform lg:translate-x-0", mobileOpen ? "translate-x-0" : "-translate-x-full")}>
        <div className="flex items-center justify-between px-2 pb-5">
          <Logo />
          <button onClick={() => setMobileOpen(false)} className="rounded-lg p-2 text-white/45 hover:bg-white/[0.06] hover:text-white lg:hidden" aria-label="Close sidebar"><X className="size-4" /></button>
        </div>
        <button className="mx-1 flex h-10 items-center gap-3 rounded-lg border border-white/[0.09] bg-white/[0.025] px-3 text-left text-sm text-white/50 hover:border-white/15 hover:text-white">
          <Search className="size-4" /><span className="flex-1">Find</span><kbd className="rounded border border-white/10 px-1.5 py-0.5 text-[10px] text-white/35">F</kbd>
        </button>
        <div className="mt-5 px-1">
          <p className="px-2 text-[10px] font-semibold uppercase tracking-[0.18em] text-white/25">Workspace</p>
          <nav className="mt-2 space-y-0.5">
            {nav.map(({ href, label, icon: Icon, soon }) => {
              const hrefStr = String(href);
              const isActive = pathname === hrefStr || pathname.startsWith(`${hrefStr}/`);
              return <Link key={hrefStr} href={href} onClick={() => setMobileOpen(false)} className={cn("group flex items-center gap-3 rounded-lg px-3 py-2.5 text-[13px] transition", isActive ? "bg-white/[0.09] text-white shadow-[inset_2px_0_0_rgba(255,255,255,.8)]" : "text-white/55 hover:bg-white/[0.045] hover:text-white")}>
                <Icon className={cn("size-4", isActive ? "text-white" : "text-white/40 group-hover:text-white/70")} />
                <span className="flex-1">{label}</span>{soon ? <span className="rounded-full border border-white/10 px-1.5 py-0.5 text-[8px] uppercase tracking-wider text-white/25">Soon</span> : null}
              </Link>;
            })}
          </nav>
        </div>
        <div className="mt-auto space-y-3">
          <div className="overflow-hidden rounded-xl border border-white/[0.08] bg-gradient-to-br from-fuchsia-500/[0.08] via-cyan-400/[0.04] to-emerald-400/[0.08] p-4">
            <div className="flex items-center gap-2 text-xs font-medium"><Sparkles className="size-3.5 text-fuchsia-300" /> Zero-DevOps</div>
            <p className="mt-2 text-[11px] leading-5 text-white/40">From repository to reproducible builds without managing the platform.</p>
          </div>
          <div className="flex items-center justify-between border-t border-white/[0.07] px-2 pt-3"><Link href="/settings" className="text-white/35 hover:text-white"><CircleHelp className="size-4" /></Link><UserMenu /></div>
        </div>
      </aside>

      <div className="lg:pl-[258px]">
        <header className="sticky top-0 z-40 border-b border-white/[0.07] bg-[#050505]/80 backdrop-blur-2xl">
          <div className="flex h-[66px] items-center gap-3 px-4 sm:px-6">
            <button onClick={() => setMobileOpen(true)} className="rounded-lg border border-white/10 p-2 text-white/60 lg:hidden" aria-label="Open sidebar"><Menu className="size-4" /></button>
            <div className="flex min-w-0 items-center gap-2 text-sm"><button className="flex items-center gap-2 rounded-lg px-2 py-1.5 text-white/70 hover:bg-white/[0.05] hover:text-white"><span className="truncate font-medium">All Projects</span><ChevronDown className="size-3.5 text-white/35" /></button><span className="text-white/15">/</span><span className="truncate text-white/35">{active?.label ?? "Workspace"}</span></div>
            <div className="ml-auto flex items-center gap-2"><Link href="/github" className="hidden items-center gap-2 rounded-lg px-3 py-2 text-xs text-white/50 hover:bg-white/[0.05] hover:text-white sm:flex"><Github className="size-3.5" /> GitHub</Link><Link href="/deployments" className="hidden items-center gap-2 rounded-lg px-3 py-2 text-xs text-white/50 hover:bg-white/[0.05] hover:text-white md:flex"><BarChart3 className="size-3.5" /> Activity</Link><button onClick={() => setNewDeploymentOpen(true)} className="inline-flex h-9 items-center gap-2 rounded-lg bg-white px-3.5 text-xs font-semibold text-black shadow-[0_8px_30px_rgba(255,255,255,.08)] transition hover:bg-white/90"><Rocket className="size-3.5" /> New Deployment</button></div>
          </div>
        </header>
        <main className="relative overflow-hidden">
          <div className="pointer-events-none fixed inset-0 -z-10 bg-[radial-gradient(circle_at_75%_0%,rgba(103,232,249,.07),transparent_25%),radial-gradient(circle_at_95%_35%,rgba(217,70,239,.06),transparent_22%),radial-gradient(circle_at_45%_100%,rgba(52,211,153,.04),transparent_28%)]" />
          {children}
        </main>
      </div>
      {mobileOpen ? <button aria-label="Close sidebar overlay" onClick={() => setMobileOpen(false)} className="fixed inset-0 z-40 bg-black/70 backdrop-blur-sm lg:hidden" /> : null}
      <NewDeploymentDialog open={newDeploymentOpen} onOpenChange={setNewDeploymentOpen} />
    </div>
  );
}
