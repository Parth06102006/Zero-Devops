"use client";

import { LayoutDashboard, Menu, Rocket, Settings, X } from "lucide-react";
import Link from "next/link";
import { usePathname } from "next/navigation";
import { useState, type ReactNode } from "react";

import { Container } from "@/components/shared/container";
import { Logo } from "@/components/shared/logo";
import { UserMenu } from "@/features/auth/components/user-menu";
import { cn } from "@/lib/utils/cn";

const navItems = [
  { href: "/dashboard", label: "Overview", icon: LayoutDashboard },
  { href: "/deployments", label: "Deployments", icon: Rocket },
  { href: "/settings", label: "Settings", icon: Settings },
] as const;

export function AppShell({ children }: { children: ReactNode }) {
  const pathname = usePathname();
  const [mobileOpen, setMobileOpen] = useState(false);
  const pageLabel = navItems.find((item) => pathname === item.href || pathname.startsWith(`${item.href}/`))?.label ?? "Workspace";

  return <div className="min-h-dvh bg-background text-foreground"><aside className={cn("fixed inset-y-0 left-0 z-50 flex w-64 flex-col border-r border-border bg-card px-4 py-5 transition-transform duration-300 lg:translate-x-0", mobileOpen ? "translate-x-0" : "-translate-x-full")}><div className="flex items-center justify-between px-2"><Logo /><button className="rounded-md p-2 text-muted-foreground hover:bg-surface hover:text-foreground lg:hidden" onClick={() => setMobileOpen(false)} aria-label="Close navigation"><X className="size-4" /></button></div><div className="mt-10 px-2"><p className="font-mono text-[10px] uppercase tracking-[0.2em] text-muted-foreground">Workspace</p><nav className="mt-3 grid gap-1">{navItems.map(({ href, label, icon: Icon }) => { const active = pathname === href || pathname.startsWith(`${href}/`); return <Link key={href} href={href} onClick={() => setMobileOpen(false)} className={cn("group flex items-center gap-3 rounded-md px-3 py-2.5 text-sm transition-colors", active ? "bg-primary/10 font-medium text-primary" : "text-muted-foreground hover:bg-surface hover:text-foreground")}><Icon className="size-4" />{label}{active ? <span className="ml-auto size-1.5 rounded-full bg-primary" /> : null}</Link>; })}</nav></div><div className="mt-auto rounded-md border border-border bg-surface p-4"><p className="font-mono text-[10px] uppercase tracking-[0.16em] text-primary">Ship without overhead</p><p className="mt-2 text-xs leading-5 text-muted-foreground">Connect GitHub and turn your next commit into a monitored release.</p></div></aside><div className="lg:pl-64"><header className="sticky top-0 z-40 border-b border-border/80 bg-background/85 backdrop-blur-xl"><Container className="flex h-16 items-center justify-between"><div className="flex items-center gap-3"><button className="rounded-md border border-border p-2 text-muted-foreground hover:bg-surface hover:text-foreground lg:hidden" onClick={() => setMobileOpen(true)} aria-label="Open navigation"><Menu className="size-4" /></button><div><p className="font-mono text-[10px] uppercase tracking-[0.18em] text-muted-foreground">Zero DevOps / {pageLabel}</p><p className="mt-0.5 text-sm font-medium text-foreground">Workspace overview</p></div></div><UserMenu /></Container></header><main><Container className="py-8 md:py-10">{children}</Container></main></div>{mobileOpen ? <button aria-label="Close navigation overlay" onClick={() => setMobileOpen(false)} className="fixed inset-0 z-40 bg-background/60 backdrop-blur-sm lg:hidden" /> : null}</div>;
}
