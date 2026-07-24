import Link from "next/link";
import type { Metadata } from "next";

import { GithubLoginButton } from "@/features/auth/components/github-login-button";
import { Logo } from "@/components/shared/logo";

export const metadata: Metadata = {
  title: "Log in",
};

interface LoginPageProps {
  searchParams: Promise<{ return_to?: string }>;
}

export default async function LoginPage({ searchParams }: LoginPageProps) {
  const { return_to: returnTo } = await searchParams;

  return (
    <div className="flex min-h-dvh flex-col items-center justify-center gap-10 bg-background px-6">
      <Logo />

      <div className="w-full max-w-sm rounded-xl border border-border bg-surface p-8 shadow-2xl shadow-black/30">
        <div className="flex flex-col items-center gap-2 text-center">
          <h1 className="text-xl font-semibold text-foreground">Welcome to Zero DevOps</h1>
          <p className="text-sm text-muted-foreground">
            Sign in with GitHub to connect a repo and start deploying.
          </p>
        </div>

        <GithubLoginButton returnTo={returnTo} className="mt-8 w-full" />

        <p className="mt-6 text-center text-xs text-muted-foreground">
          By continuing, you agree to Zero DevOps&apos;s{" "}
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

      <Link href="/" className="text-sm text-muted-foreground hover:text-foreground">
        ← Back to home
      </Link>
    </div>
  );
}
