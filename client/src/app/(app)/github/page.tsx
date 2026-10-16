"use client";

import {
  Github,
  Link2,
  Link2Off,
  Loader2,
  RefreshCw,
  Plus,
  CheckCircle2,
  AlertCircle,
  ShieldCheck,
  Building2,
  User,
} from "lucide-react";
import {
  useGithubInstallations,
  useUpdateGithubConnectionStatus,
  getGithubAppInstallUrl,
} from "@/features/github";
import { Button } from "@/components/ui/button";
import { useAuthStore } from "@/features/auth/store/auth-store";
import type { GithubConnectionStatus, GithubInstallationStatus } from "@/types/domain";

function getStatusBadge(status: GithubInstallationStatus) {
  switch (status) {
    case "active":
      return (
        <span className="inline-flex items-center gap-1.5 rounded-full border border-emerald-500/20 bg-emerald-500/10 px-2.5 py-0.5 text-xs font-medium text-emerald-400">
          <span className="size-1.5 rounded-full bg-emerald-400" />
          Active
        </span>
      );
    case "suspended":
      return (
        <span className="inline-flex items-center gap-1.5 rounded-full border border-amber-500/20 bg-amber-500/10 px-2.5 py-0.5 text-xs font-medium text-amber-400">
          <span className="size-1.5 rounded-full bg-amber-400" />
          Suspended
        </span>
      );
    case "uninstalled":
      return (
        <span className="inline-flex items-center gap-1.5 rounded-full border border-rose-500/20 bg-rose-500/10 px-2.5 py-0.5 text-xs font-medium text-rose-400">
          <span className="size-1.5 rounded-full bg-rose-400" />
          Uninstalled
        </span>
      );
    default:
      return (
        <span className="inline-flex items-center gap-1.5 rounded-full border border-white/10 bg-white/5 px-2.5 py-0.5 text-xs font-medium text-white/50">
          {status}
        </span>
      );
  }
}

function getConnectionBadge(status?: GithubConnectionStatus) {
  if (status === "connected") {
    return (
      <span className="inline-flex items-center gap-1.5 rounded-full border border-cyan-400/30 bg-cyan-400/10 px-2.5 py-0.5 text-xs font-medium text-cyan-300">
        <span className="size-1.5 rounded-full bg-cyan-400 animate-pulse" />
        Connected
      </span>
    );
  }
  return (
    <span className="inline-flex items-center gap-1.5 rounded-full border border-white/10 bg-white/5 px-2.5 py-0.5 text-xs font-medium text-white/40">
      <span className="size-1.5 rounded-full bg-white/30" />
      Disconnected
    </span>
  );
}

export default function GithubPage() {
  const allInstallations = useGithubInstallations();
  const updateStatus = useUpdateGithubConnectionStatus();
  const user = useAuthStore((state) => state.user);

  // The list includes connection_status, so it is the single source of truth for this page.
  const activeData = allInstallations.data?.find(
    (installation) => installation.connection_status === "connected",
  );
  const isCurrentlyConnected = activeData?.status === "active";

  const handleDisconnect = (id: string) => {
    if (window.confirm("Disconnect this GitHub account? The installation will remain saved and can be reconnected at any time.")) {
      updateStatus.mutate({ connectionStatus: "disconnected", id });
    }
  };

  const handleConnect = (id: string) => {
    updateStatus.mutate({ connectionStatus: "connected", id });
  };

  const handleRefresh = () => {
    allInstallations.refetch();
  };

  return (
    <div className="mx-auto max-w-[1100px] px-4 py-6 sm:px-6 lg:px-8">
      {/* Header */}
      <div className="flex flex-col gap-4 border-b border-white/[0.07] pb-6 sm:flex-row sm:items-center sm:justify-between">
        <div>
          <p className="text-xs text-white/35">Integrations</p>
          <h1 className="mt-1 text-2xl font-semibold">GitHub Accounts</h1>
          <p className="mt-1 text-sm text-white/35">
            Manage your connected GitHub App installations and select which account ShipZero uses for repository access.
          </p>
        </div>
        <div className="flex items-center gap-3">
          <Button
            variant="outline"
            size="sm"
            onClick={handleRefresh}
            className="gap-2 text-xs"
            disabled={allInstallations.isFetching}
          >
            <RefreshCw className={`size-3.5 ${allInstallations.isFetching ? "animate-spin" : ""}`} />
            Refresh
          </Button>
          <Button
            variant="default"
            size="sm"
            className="gap-2 text-xs"
            onClick={() => window.location.assign(getGithubAppInstallUrl("/github", user?.id))}
          >
            <Plus className="size-3.5" />
            Add Account
          </Button>
        </div>
      </div>

      {/* Currently Selected Installation Card */}
      <div className="mt-6 rounded-2xl border border-white/[0.08] bg-white/[0.02] p-6">
        <div className="flex items-center justify-between pb-4 border-b border-white/[0.06]">
          <div className="flex items-center gap-2 text-xs font-medium uppercase tracking-wider text-white/40">
            <ShieldCheck className="size-4 text-cyan-400" />
            Active SCM Connection
          </div>
          {activeData && getConnectionBadge(activeData.connection_status)}
        </div>

        <div className="mt-5 flex flex-col gap-5 sm:flex-row sm:items-center">
          <span className="flex size-14 items-center justify-center rounded-xl bg-gradient-to-br from-cyan-400/15 to-fuchsia-500/15">
            <Github className="size-7 text-white/80" />
          </span>
          <div className="flex-1">
            <div className="flex items-center gap-2">
              <p className="text-base font-semibold">{activeData?.account_login ?? "No account connected"}</p>
              {activeData && getStatusBadge(activeData.status)}
            </div>
            <p className="mt-1 text-xs text-white/40">
              {activeData ? (
                <>
                  {activeData.account_type === "Organization" ? (
                    <Building2 className="mr-1 inline-block size-3.5 align-text-bottom" />
                  ) : (
                    <User className="mr-1 inline-block size-3.5 align-text-bottom" />
                  )}
                  {activeData.account_type} account · GitHub Installation ID #{activeData.installation_id}
                </>
              ) : allInstallations.isPending ? (
                "Checking connection status…"
              ) : (
                "Connect a GitHub account below to grant ShipZero repository access."
              )}
            </p>
          </div>
          {activeData?.connection_status === "connected" ? (
            <Button
              variant="outline"
              onClick={() => handleDisconnect(activeData.id)}
              disabled={updateStatus.isPending}
              className="gap-2 text-xs text-rose-300 hover:text-rose-200 hover:border-rose-500/40"
            >
              <Link2Off className="size-3.5" /> Disconnect
            </Button>
          ) : (
            <Button
              variant="default"
              className="gap-2 text-xs"
              onClick={() => window.location.assign(getGithubAppInstallUrl("/github", user?.id))}
            >
              <Plus className="size-3.5" /> Connect GitHub App
            </Button>
          )}
        </div>

        {/* Status Grid */}
        <div className="mt-6 grid gap-3 sm:grid-cols-2">
          <div className="rounded-xl border border-white/[0.07] bg-black/20 p-4">
            <p className="text-xs text-white/30">Installation status</p>
            <div className="mt-2 flex items-center gap-2">
              {allInstallations.isPending ? (
                <Loader2 className="size-4 animate-spin text-white/50" />
              ) : activeData ? (
                <>
                  {getStatusBadge(activeData.status)}
                  <span className="text-xs text-white/50">via GitHub</span>
                </>
              ) : (
                <span className="text-xs text-white/40">Not connected</span>
              )}
            </div>
          </div>
          <div className="rounded-xl border border-white/[0.07] bg-black/20 p-4">
            <p className="text-xs text-white/30">Repository access</p>
            <div className="mt-2 flex items-center gap-2">
              {isCurrentlyConnected ? (
                <>
                  <CheckCircle2 className="size-4 text-emerald-400" />
                  <span className="text-xs text-emerald-300">Available through GitHub App</span>
                </>
              ) : (
                <>
                  <AlertCircle className="size-4 text-amber-400" />
                  <span className="text-xs text-white/50">
                    {activeData && activeData.status !== "active"
                      ? "Unavailable (App is suspended or uninstalled on GitHub)"
                      : "Connect an active account to enable repository access"}
                  </span>
                </>
              )}
            </div>
          </div>
        </div>
      </div>

      {/* Saved Installations List */}
      <div className="mt-8">
        <div className="flex items-center justify-between pb-3">
          <div>
            <h2 className="text-lg font-medium">Saved GitHub Installations</h2>
            <p className="text-xs text-white/35">
              Switch between connected organizations or user accounts without losing configuration.
            </p>
          </div>
          <span className="text-xs text-white/30">
            {allInstallations.data?.length ?? 0} saved
          </span>
        </div>

        <div className="mt-3 divide-y divide-white/[0.06] rounded-2xl border border-white/[0.08] bg-white/[0.02]">
          {allInstallations.isPending ? (
            <div className="flex items-center justify-center p-8 text-white/40">
              <Loader2 className="size-5 animate-spin mr-2" />
              Loading saved installations…
            </div>
          ) : !allInstallations.data || allInstallations.data.length === 0 ? (
            <div className="p-8 text-center">
              <Github className="mx-auto size-8 text-white/20" />
              <p className="mt-2 text-sm font-medium text-white/60">No saved GitHub installations</p>
              <p className="mt-1 text-xs text-white/30">
                Install the ShipZero GitHub App on your personal account or organization to get started.
              </p>
              <Button
                variant="outline"
                size="sm"
                className="mt-4 gap-2 text-xs"
                onClick={() => window.location.assign(getGithubAppInstallUrl("/github", user?.id))}
              >
                <Plus className="size-3.5" /> Install GitHub App
              </Button>
            </div>
          ) : (
            allInstallations.data.map((inst) => {
              const isSelected = inst.connection_status === "connected";
              const canConnect = inst.status === "active";

              return (
                <div
                  key={inst.id}
                  className="flex flex-col gap-4 p-5 sm:flex-row sm:items-center sm:justify-between transition-colors hover:bg-white/[0.01]"
                >
                  <div className="flex items-center gap-3.5">
                    <span className="flex size-10 items-center justify-center rounded-xl bg-white/[0.04] text-white/70">
                      {inst.account_type === "Organization" ? (
                        <Building2 className="size-5" />
                      ) : (
                        <User className="size-5" />
                      )}
                    </span>
                    <div>
                      <div className="flex items-center gap-2">
                        <span className="text-sm font-medium text-white">{inst.account_login}</span>
                        {getStatusBadge(inst.status)}
                        {getConnectionBadge(inst.connection_status)}
                      </div>
                      <p className="mt-0.5 text-xs text-white/35">
                        {inst.account_type} · ID #{inst.installation_id}
                      </p>
                    </div>
                  </div>

                  <div className="flex items-center gap-3">
                    {isSelected ? (
                      <Button
                        variant="outline"
                        size="sm"
                        onClick={() => handleDisconnect(inst.id)}
                        disabled={updateStatus.isPending}
                        className="gap-1.5 text-xs text-rose-300 hover:text-rose-200 hover:border-rose-500/40"
                      >
                        <Link2Off className="size-3.5" />
                        Disconnect
                      </Button>
                    ) : canConnect ? (
                      <Button
                        variant="outline"
                        size="sm"
                        onClick={() => handleConnect(inst.id)}
                        disabled={updateStatus.isPending}
                        className="gap-1.5 text-xs border-cyan-400/30 text-cyan-300 hover:bg-cyan-400/10"
                      >
                        <Link2 className="size-3.5" />
                        Connect / Switch
                      </Button>
                    ) : (
                      <Button
                        variant="outline"
                        size="sm"
                        disabled
                        className="text-xs text-white/30 border-white/10"
                      >
                        Unavailable ({inst.status})
                      </Button>
                    )}
                  </div>
                </div>
              );
            })
          )}
        </div>
      </div>

      {/* Information Banner */}
      <div className="mt-8 rounded-2xl border border-cyan-400/20 bg-cyan-400/5 p-5">
        <h3 className="text-xs font-semibold uppercase tracking-wider text-cyan-300">
          How Multi-Account Connections Work
        </h3>
        <p className="mt-1 text-xs text-cyan-200/70 leading-relaxed">
          You can install the GitHub App across multiple personal accounts and organizations. ShipZero maintains all your installations in history, with at most one account connected at any time. When you disconnect an account, the GitHub App installation remains intact on GitHub so you can switch back instantly without reinstalling.
        </p>
      </div>
    </div>
  );
}
