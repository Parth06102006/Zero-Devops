"use client";

import { useEffect, useState } from "react";
import { useRouter, useSearchParams } from "next/navigation";
import { installGithubApp } from "@/features/github";
import { Loader2, AlertCircle, RefreshCw } from "lucide-react";
import { Button } from "@/components/ui/button";

export default function GithubInstallCallback() {
  const router = useRouter();
  const searchParams = useSearchParams();
  const code = searchParams.get("code");
  const installationId = searchParams.get("installation_id");
  const setupAction = searchParams.get("setup_action");
  const returnTo = searchParams.get("return_to") ?? "/dashboard";
  const [error, setError] = useState<string | null>(null);
  const [installing, setInstalling] = useState(true);

  useEffect(() => {
    // Handle installation update flow (repo selection changed)
    if (installationId && setupAction === "update") {
      // Just redirect to dashboard - installation already exists, just repos updated
      (router.push as (url: string) => void)(returnTo);
      return;
    }

    // Handle initial OAuth flow
    if (!code) {
      router.push("/github");
      return;
    }

    const doInstall = async () => {
      setInstalling(true);
      setError(null);
      try {
        await installGithubApp(code);
        // Poll for installation to be available
        let attempts = 0;
        while (attempts < 10) {
          await new Promise(r => setTimeout(r, 1000));
          try {
            const res = await fetch("/integrations/scm/github/installation", { credentials: "include" });
            if (res.ok) {
              (router.push as (url: string) => void)(returnTo);
              return;
            }
          } catch {}
          attempts++;
        }
        throw new Error("Installation not detected after retry");
      } catch (err) {
        setError(err instanceof Error ? err.message : "Failed to install GitHub App");
        setInstalling(false);
      }
    };

    doInstall();
  }, [code, installationId, setupAction, returnTo, router]);

  if (installing) {
    return (
      <div className="flex min-h-dvh items-center justify-center bg-[#050505] text-white px-4">
        <div className="text-center">
          <Loader2 className="mx-auto size-8 animate-spin text-cyan-300 mb-4" />
          <p className="text-sm text-white/60">Completing GitHub App installation…</p>
        </div>
      </div>
    );
  }

  return (
    <div className="flex min-h-dvh items-center justify-center bg-[#050505] text-white px-4">
      <div className="max-w-md text-center">
        <AlertCircle className="mx-auto size-12 text-rose-400 mb-4" />
        <h2 className="text-lg font-medium mb-2">Installation Failed</h2>
        <p className="text-sm text-white/60 mb-6">{error}</p>
        <div className="flex gap-2 justify-center">
          <Button
            variant="default"
            className="gap-2"
            onClick={() => window.location.assign(`/github/install/callback?code=${code}&return_to=${encodeURIComponent(returnTo)}`)}
          >
            <RefreshCw className="size-3.5" /> Retry
          </Button>
          <Button
            variant="outline"
            onClick={() => router.push("/github")}
          >
            Back to GitHub Settings
          </Button>
        </div>
      </div>
    </div>
  );
}