"use client";

import { useEffect } from "react";
import { useRouter, useSearchParams } from "next/navigation";
// import { installGithubApp } from "@/features/github";
import { Loader2 } from "lucide-react";

export default function GithubInstallCallback() {
  const router = useRouter();
  const searchParams = useSearchParams();
  const returnTo = searchParams.get("return_to") ?? "/dashboard";

  // Client-side callback exchange logic commented out in favor of server-side callback:
  /*
  const code = searchParams.get("code");
  const installationId = searchParams.get("installation_id");
  const setupAction = searchParams.get("setup_action");
  const [error, setError] = useState<string | null>(null);
  const [installing, setInstalling] = useState(true);

  useEffect(() => {
    if (installationId && setupAction === "update") {
      (router.push as (url: string) => void)(returnTo);
      return;
    }
    if (!code) {
      router.push("/github");
      return;
    }
    const doInstall = async () => {
      setInstalling(true);
      setError(null);
      try {
        await installGithubApp(code);
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
  */

  useEffect(() => {
    // With server-side setup, the server completes the installation and redirects to returnTo.
    // If the frontend callback route is accessed directly, route to destination.
    (router.push as (url: string) => void)(returnTo);
  }, [returnTo, router]);

  return (
    <div className="flex min-h-dvh items-center justify-center bg-[#050505] text-white px-4">
      <div className="text-center">
        <Loader2 className="mx-auto size-8 animate-spin text-cyan-300 mb-4" />
        <p className="text-sm text-white/60">Redirecting…</p>
      </div>
    </div>
  );
}