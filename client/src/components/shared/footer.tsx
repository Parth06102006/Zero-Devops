import Link from "next/link";

import { Container } from "@/components/shared/container";
import { Logo } from "@/components/shared/logo";
import { siteConfig } from "@/lib/config/site";

export function Footer() {
  return <footer className="border-t border-border/70 py-10"><Container className="flex flex-col gap-8 sm:flex-row sm:items-end sm:justify-between"><div><Logo /><p className="mt-3 max-w-xs text-sm leading-6 text-muted-foreground">{siteConfig.tagline}</p></div><div className="flex gap-5 text-sm text-muted-foreground"><Link href="#product" className="hover:text-foreground">Capabilities</Link><Link href="#workflow" className="hover:text-foreground">Workflow</Link><Link href="/login" className="hover:text-foreground">Log in</Link><a href={siteConfig.github} target="_blank" rel="noopener noreferrer" className="hover:text-foreground">GitHub</a></div></Container><Container className="mt-8 border-t border-border/70 pt-5"><p className="font-mono text-[11px] uppercase tracking-[0.16em] text-muted-foreground">© {new Date().getFullYear()} {siteConfig.name}</p></Container></footer>;
}
