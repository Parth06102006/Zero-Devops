"use client";

import { AnimatePresence, motion } from "framer-motion";
import { Menu, X } from "lucide-react";
import Link from "next/link";
import { useEffect, useState } from "react";

import { Button } from "@/components/ui/button";
import { Container } from "@/components/shared/container";
import { Logo } from "@/components/shared/logo";
import { ThemeToggle } from "@/components/shared/theme-toggle";

const links = [
  { href: "#product", label: "Capabilities" },
  { href: "#workflow", label: "Workflow" },
] as const;

export function Navbar() {
  const [scrolled, setScrolled] = useState(false);
  const [open, setOpen] = useState(false);

  useEffect(() => {
    const handleScroll = () => setScrolled(window.scrollY > 20);
    handleScroll();
    window.addEventListener("scroll", handleScroll, { passive: true });
    return () => window.removeEventListener("scroll", handleScroll);
  }, []);

  return (
    <>
      <motion.header initial={{ y: -24, opacity: 0 }} animate={{ y: 0, opacity: 1 }} className={`sticky top-0 z-50 transition-all duration-300 ${scrolled ? "border-b border-border/80 bg-background/85 shadow-lg shadow-black/10 backdrop-blur-xl" : "bg-transparent"}`}>
        <Container className="flex h-[4.5rem] items-center justify-between">
          <Logo />
          <nav className="hidden items-center gap-8 md:flex">
            {links.map((link) => <Link key={link.href} href={link.href} className="text-sm text-muted-foreground transition-colors hover:text-foreground">{link.label}</Link>)}
          </nav>
          <div className="flex items-center gap-2">
            <ThemeToggle />
            <Button asChild variant="ghost" size="sm" className="hidden sm:inline-flex"><Link href="/login">Log in</Link></Button>
            <Button asChild size="sm" className="hidden sm:inline-flex"><Link href="/login">Start free <span aria-hidden>↗</span></Link></Button>
            <Button variant="ghost" size="icon" className="md:hidden" onClick={() => setOpen((value) => !value)} aria-label={open ? "Close menu" : "Open menu"}>{open ? <X /> : <Menu />}</Button>
          </div>
        </Container>
      </motion.header>
      <AnimatePresence>
        {open ? <motion.nav initial={{ opacity: 0, height: 0 }} animate={{ opacity: 1, height: "auto" }} exit={{ opacity: 0, height: 0 }} className="sticky top-[4.5rem] z-40 overflow-hidden border-b border-border bg-background/95 px-6 py-4 backdrop-blur-xl md:hidden"><div className="mx-auto flex max-w-6xl flex-col gap-1">{links.map((link) => <Link key={link.href} href={link.href} onClick={() => setOpen(false)} className="rounded-md px-3 py-3 text-sm text-muted-foreground hover:bg-surface hover:text-foreground">{link.label}</Link>)}<Link href="/login" onClick={() => setOpen(false)} className="mt-2 rounded-md bg-primary px-3 py-3 text-center text-sm font-medium text-primary-foreground">Continue with GitHub</Link></div></motion.nav> : null}
      </AnimatePresence>
    </>
  );
}
