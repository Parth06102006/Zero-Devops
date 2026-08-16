import Link from "next/link";
import { AlertCircle, ArrowLeft, Github, ShieldCheck } from "lucide-react";
import type { Metadata } from "next";

import { GithubLoginButton } from "@/features/auth/components/github-login-button";
import { AuthContextPanel } from "@/components/login/auth-context-panel";
import { Logo } from "@/components/shared/logo";

export const metadata: Metadata = {
  title: "Log in - ghost",
};

interface LoginPageProps {
  searchParams: Promise<{ return_to?: string; error?: string; message?: string }>;
}

export default async function LoginPage({ searchParams }: LoginPageProps) {
  const { return_to: returnTo, error, message } = await searchParams;
  const hasError = Boolean(error || message);

  return (
    <main className="relative min-h-dvh bg-background px-4 py-6 text-foreground sm:px-6 lg:px-8">
      <div
        aria-hidden
        className="pointer-events-none absolute inset-0 -z-10"
        style={{
          background:
            "radial-gradient(ellipse 80% 50% at 50% -8%, hsl(var(--primary) / 0.08) 0%, transparent 60%), radial-gradient(ellipse 50% 40% at 80% 20%, hsl(var(--ring) / 0.06) 0%, transparent 50%)",
        }}
      />
      <div className="mx-auto flex min-h-[calc(100dvh-3rem)] w-full max-w-6xl items-center">
        <div className="grid w-full overflow-hidden rounded-2xl border border-border bg-card shadow-2xl shadow-black/10 lg:grid-cols-[0.9fr_1fr]">
          <section className="flex min-h-[560px] flex-col justify-between p-6 sm:p-10">
            <div className="flex items-center justify-between gap-4">
              <Logo />
              <Link
                href="/"
                className="inline-flex items-center gap-2 text-sm text-muted-foreground transition-colors hover:text-foreground"
              >
                <ArrowLeft className="size-4" />
                Home
              </Link>
            </div>

            <div className="mx-auto flex w-full max-w-md flex-col gap-8 py-12">
              <div className="flex flex-col gap-4 text-center sm:text-left">
                <span className="mx-auto inline-flex w-fit items-center gap-2 rounded-full border border-primary/25 bg-primary/10 px-3 py-1 text-xs font-medium text-primary sm:mx-0">
                  <Github className="size-3.5" />
                  GitHub-powered deployments
                </span>
                <div className="flex flex-col gap-3">
                  <h1 className="text-balance text-3xl font-semibold tracking-tight text-foreground sm:text-4xl">
                    Continue to ghost
                  </h1>
                  <p className="text-sm leading-6 text-muted-foreground">
                    Sign in to connect repositories, watch pushes, and start deployment flows from
                    GitHub.
                  </p>
                </div>
              </div>

              {hasError ? (
                <div
                  role="alert"
                  className="flex gap-3 rounded-lg border border-destructive/30 bg-destructive/10 p-4 text-sm text-destructive"
                >
                  <AlertCircle className="mt-0.5 size-4 shrink-0" />
                  <div>
                    <p className="font-medium">GitHub sign-in could not be completed.</p>
                    <p className="mt-1 text-destructive/85">
                      {message ?? "Try again, or return home and start the sign-in flow again."}
                    </p>
                  </div>
                </div>
              ) : null}

              <div className="rounded-xl border border-border bg-surface p-5 shadow-sm shadow-black/5">
                <GithubLoginButton
                  returnTo={returnTo}
                  className="w-full"
                  label="Continue with GitHub"
                />
                <div className="mt-4 flex items-start gap-2 text-xs leading-5 text-muted-foreground">
                  <ShieldCheck className="mt-0.5 size-3.5 shrink-0 text-primary" />
                  ghost redirects through GitHub OAuth. The browser is sent to the backend
                  auth endpoint and then back to your workspace.
                </div>
              </div>

              <p className="text-center text-xs leading-5 text-muted-foreground sm:text-left">
                By continuing, you agree to ghost&apos;s{" "}
                <Link href="#" className="underline underline-offset-2 hover:text-foreground">
                  Terms
                </Link>{" "}
                and{" "}
                <Link href="#" className="underline underline-offset-2 hover:text-foreground">
                  Privacy Policy
                </Link>
                .
              </p>
            </div>

            <p className="font-mono text-xs text-muted-foreground">
              &copy; 2026 ghost
            </p>
          </section>

          <AuthContextPanel />
        </div>
      </div>
    </main>
  );
}
