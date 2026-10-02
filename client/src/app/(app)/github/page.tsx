"use client";

import { useEffect } from "react";
import { Github, Link2Off, Loader2, RefreshCw, Plus } from "lucide-react";
import { useGithubInstallation, useDeleteGithubInstallation } from "@/features/github";
import { getGithubAppInstallUrl } from "@/features/github";
import { Button } from "@/components/ui/button";

export default function GithubPage() {
  const installation = useGithubInstallation();
  const remove = useDeleteGithubInstallation();
  const data = installation.data;

  // Force refetch on mount to pick up new installation
  useEffect(() => {
    installation.refetch();
  }, [installation]);

  return (
    <div className="mx-auto max-w-[1100px] px-4 py-6 sm:px-6 lg:px-8">
      {/* Header */}
      <div className="border-b border-white/[0.07] pb-6">
        <p className="text-xs text-white/35">Integrations</p>
        <h1 className="mt-1 text-2xl font-semibold">GitHub Account</h1>
        <p className="mt-1 text-sm text-white/35">
          Manage the GitHub App connection used for repository access.
        </p>
      </div>

      {/* Main card */}
      <div className="mt-5 rounded-2xl border border-white/[0.08] bg-white/[0.02] p-6">
        <div className="flex flex-col gap-5 sm:flex-row sm:items-center">
          <span className="flex size-12 items-center justify-center rounded-xl bg-gradient-to-br from-cyan-400/15 to-fuchsia-500/15">
            <Github className="size-6 text-white/70" />
          </span>
          <div className="flex-1">
            <p className="text-sm font-medium">{data?.account_login ?? "GitHub App"}</p>
            <p className="mt-1 text-xs text-white/35">
              {data
                ? `${data.account_type} account · ${data.status}`
                : installation.isError
                  ? "No GitHub App installation found"
                  : "Checking connection…"}
            </p>
          </div>
          {data ? (
            <Button
              variant="outline"
              onClick={() => {
                if (window.confirm("Disconnect GitHub App?")) remove.mutate();
              }}
              disabled={remove.isPending}
            >
              <Link2Off /> Disconnect
            </Button>
          ) : (
            <Button
              variant="default"
              className="gap-2"
              onClick={() => window.location.assign(getGithubAppInstallUrl("/github"))}
            >
              <Plus className="size-3.5" /> Connect GitHub App
            </Button>
          )}
        </div>

        {/* Status Grid */}
        <div className="mt-6 grid gap-3 sm:grid-cols-2">
          <div className="rounded-xl border border-white/[0.07] bg-black/20 p-4">
            <p className="text-xs text-white/30">Installation status</p>
            <p className="mt-2 text-sm text-white/70">
              {installation.isPending ? (
                <Loader2 className="size-4 animate-spin" />
              ) : (
                data?.status ?? "Not connected"
              )}
            </p>
          </div>
          <div className="rounded-xl border border-white/[0.07] bg-black/20 p-4">
            <p className="text-xs text-white/30">Repository access</p>
            <p className="mt-2 text-sm text-white/70">
              {data
                ? "Available through GitHub App"
                : "Connect the GitHub App to select repositories"}
            </p>
          </div>
        </div>

        {!data && (
          <div className="mt-6 rounded-xl border border-cyan-400/20 bg-cyan-400/5 p-4">
            <p className="text-xs text-cyan-200/70">
              Install the Zero-DevOps GitHub App to access your repositories and create deployments.
            </p>
          </div>
        )}

        <div className="mt-5 flex items-center gap-4">
          <button
            type="button"
            onClick={() => installation.refetch()}
            className="inline-flex items-center gap-2 text-xs text-white/35 hover:text-white"
          >
            <RefreshCw className="size-3.5" /> Refresh status
          </button>
          {!data && (
            <Button
              variant="default"
              className="gap-2"
              onClick={() => window.location.assign(getGithubAppInstallUrl("/github"))}
            >
              <Plus className="size-3.5" /> Connect GitHub App
            </Button>
          )}
        </div>
      </div>
    </div>
  );
}
