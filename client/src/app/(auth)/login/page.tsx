import Link from "next/link";
import { AlertCircle, ArrowLeft, Github, ShieldCheck } from "lucide-react";
import type { Metadata } from "next";
import { GithubLoginButton } from "@/features/auth";
import { Logo } from "@/components/shared/logo";
import { AuthContextPanel } from "@/components/login/auth-context-panel";

export const metadata: Metadata = {
  title: "Log in - Zero-DevOps",
};

interface LoginPageProps {
  searchParams: Promise<{
    return_to?: string;
    error?: string;
    message?: string;
  }>;
}

export default async function LoginPage({ searchParams }: LoginPageProps) {
  const { return_to: returnTo, error, message } = await searchParams;

  return (
    <main className="relative flex min-h-dvh items-center justify-center overflow-hidden bg-[#050505] px-4 py-8 text-white">
      {/* Background ambient gradient */}
      <div className="pointer-events-none absolute left-1/2 top-[-20rem] size-[42rem] -translate-x-1/2 rounded-full bg-gradient-to-r from-cyan-400/10 via-fuchsia-500/10 to-emerald-400/10 blur-[110px]" />

      <div className="relative grid w-full max-w-5xl overflow-hidden rounded-3xl border border-white/[0.08] bg-[#080808]/90 shadow-2xl shadow-black/60 backdrop-blur-xl lg:grid-cols-[.8fr_1.2fr]">
        <section className="flex flex-col justify-between border-b border-white/[0.07] p-7 lg:border-b-0 lg:border-r">
          <div className="flex items-center justify-between">
            <Logo />
            <Link
              href="/"
              className="inline-flex items-center gap-2 text-xs text-white/35 hover:text-white"
            >
              <ArrowLeft className="size-3.5" /> Home
            </Link>
          </div>

          <div className="mx-auto w-full max-w-sm py-16">
            <div className="inline-flex items-center gap-2 rounded-full border border-white/10 bg-white/[0.03] px-3 py-1.5 text-[10px] uppercase tracking-[.18em] text-white/40">
              <Github className="size-3.5" /> GitHub OAuth
            </div>

            <h1 className="mt-6 text-4xl font-semibold tracking-[-.05em]">
              Welcome to Zero-DevOps.
            </h1>
            <p className="mt-3 text-sm leading-6 text-white/40">
              Authenticate with GitHub to connect repositories and open your deployment workspace.
            </p>

            {error || message ? (
              <div className="mt-6 flex gap-3 rounded-xl border border-rose-400/15 bg-rose-400/5 p-4 text-xs text-rose-200/70">
                <AlertCircle className="mt-0.5 size-4 shrink-0" />
                {message ?? "GitHub sign-in could not be completed."}
              </div>
            ) : null}

            <div className="mt-6 rounded-2xl border border-white/[0.08] bg-white/[0.025] p-4">
              <GithubLoginButton
                returnTo={returnTo}
                label="Continue with GitHub"
                className="w-full bg-white text-black hover:bg-white/90"
              />
              <p className="mt-4 flex gap-2 text-[11px] leading-5 text-white/30">
                <ShieldCheck className="mt-0.5 size-3.5 shrink-0 text-cyan-200/60" />
                Authentication and session cookies remain managed by the existing backend.
              </p>
            </div>
          </div>

          <p className="font-mono text-[10px] text-white/20">© 2026 Zero-DevOps</p>
        </section>

        <AuthContextPanel />
      </div>
    </main>
  );
}
