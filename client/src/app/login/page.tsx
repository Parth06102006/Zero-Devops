import Link from "next/link";
import { AlertCircle, ArrowLeft, Github, ShieldCheck } from "lucide-react";
import type { Metadata } from "next";

import { GithubLoginButton } from "@/features/auth/components/github-login-button";
import { AuthContextPanel } from "@/components/login/auth-context-panel";
import { Logo } from "@/components/shared/logo";

export const metadata: Metadata = { title: "Log in - ghost" };

interface LoginPageProps {
  searchParams: Promise<{ return_to?: string; error?: string; message?: string }>;
}

export default async function LoginPage({ searchParams }: LoginPageProps) {
  const { return_to: returnTo, error, message } = await searchParams;
  const hasError = Boolean(error || message);

  return (
    <main className="relative flex min-h-dvh items-center overflow-hidden px-4 py-8 text-foreground sm:px-6">
      <div className="pointer-events-none absolute left-1/2 top-0 size-[40rem] -translate-x-1/2 rounded-full bg-primary/10 blur-3xl" />
      <div className="relative mx-auto grid w-full max-w-5xl overflow-hidden rounded-lg border border-border/80 bg-card/90 shadow-2xl shadow-black/30 backdrop-blur-xl lg:grid-cols-[0.82fr_1.18fr]">
        <section className="flex flex-col justify-between border-b border-border p-6 sm:p-10 lg:border-b-0 lg:border-r">
          <div className="flex items-center justify-between"><Logo /><Link href="/" className="inline-flex items-center gap-2 text-xs text-muted-foreground transition-colors hover:text-foreground"><ArrowLeft className="size-3.5" /> Back home</Link></div>
          <div className="reveal-up mx-auto w-full max-w-sm py-14 lg:py-20"><div className="mb-8 space-y-4"><span className="inline-flex items-center gap-2 rounded-full border border-primary/30 bg-primary/10 px-3 py-1.5 font-mono text-[11px] uppercase tracking-[0.16em] text-primary"><Github className="size-3.5" /> GitHub OAuth</span><div><h1 className="text-4xl font-semibold tracking-[-0.045em] text-foreground">Welcome back.</h1><p className="mt-3 text-sm leading-6 text-muted-foreground">Sign in to connect repositories, watch pushes, and start deployment flows from GitHub.</p></div></div>
            {hasError ? <div role="alert" className="mb-5 flex gap-3 rounded-md border border-destructive/30 bg-destructive/10 p-4 text-sm text-destructive"><AlertCircle className="mt-0.5 size-4 shrink-0" /><div><p className="font-medium">GitHub sign-in could not be completed.</p><p className="mt-1">{message ?? "Try again, or return home and start the sign-in flow again."}</p></div></div> : null}
            <div className="rounded-md border border-border bg-background/50 p-5"><GithubLoginButton returnTo={returnTo} className="w-full" label="Continue with GitHub" /><p className="mt-4 flex items-start gap-2 text-xs leading-5 text-muted-foreground"><ShieldCheck className="mt-0.5 size-3.5 shrink-0 text-primary" /> GitHub handles authorization securely, then returns you to your workspace.</p></div>
            <p className="mt-6 text-center font-mono text-[10px] uppercase tracking-[0.14em] text-muted-foreground">OAuth session / encrypted cookies / ready</p>
          </div>
          <p className="font-mono text-[11px] text-muted-foreground">© 2026 ghost</p>
        </section>
        <AuthContextPanel />
      </div>
    </main>
  );
}
