"use client";

import { useState } from "react";
import Link from "next/link";
import { useParams, useRouter } from "next/navigation";
import {
  ArrowLeft,
  GitBranch,
  Github,
  Play,
  RefreshCw,
  Settings2,
  Terminal,
  Trash2,
  Key,
  Zap,
  Clock,
} from "lucide-react";
import {
  useProject,
  useProjectBuilds,
  useCreateProjectBuild,
  useDeleteProject,
} from "@/features/projects";
import { DeploymentStatusBadge } from "@/features/projects";
import { Button } from "@/components/ui/button";
import { FrameworkBadge } from "@/features/projects";
import {
  Tabs,
  TabsContent,
  TabsList,
  TabsTrigger,
} from "@/components/ui/tabs";

export default function ProjectDetailPage() {
  const params = useParams<{ id: string }>();
  const router = useRouter();
  const id = params.id;
  const project = useProject(id);
  const builds = useProjectBuilds(id);
  const createBuild = useCreateProjectBuild(id);
  const deleteProject = useDeleteProject();
  const p = project.data;
  const [activeTab, setActiveTab] = useState("deployments");

  if (project.isPending) {
    return (
      <div className="mx-auto max-w-[1400px] px-4 py-6 sm:px-6 lg:px-8">
        <div className="h-8 w-64 animate-pulse rounded bg-white/[0.05]" />
        <div className="mt-5 h-52 animate-pulse rounded-2xl bg-white/[0.04]" />
      </div>
    );
  }

  if (project.isError || !p) {
    return (
      <div className="mx-auto max-w-[1400px] px-4 py-6 sm:px-6 lg:px-8">
        <Link href="/projects" className="text-xs text-white/40 hover:text-white">
          ← Projects
        </Link>
        <div className="mt-5 rounded-xl border border-rose-400/15 bg-rose-400/5 p-5 text-sm text-rose-200/70">
          {project.error?.message ?? "Project not found."}
        </div>
      </div>
    );
  }

  const branch = p.configured_branch.replace("refs/heads/", "");
  const latest = builds.data?.[0];

  const handleDelete = () => {
    if (window.confirm("Delete this project?")) {
      deleteProject.mutate(p.id, {
        onSuccess: () => {
          router.push("/projects");
        },
      });
    }
  };

  const formatDate = (dateStr: string) => new Date(dateStr).toLocaleString();
  const formatShortDate = (dateStr: string) => new Date(dateStr).toLocaleDateString();

  return (
    <div className="mx-auto max-w-[1400px] px-4 py-6 sm:px-6 lg:px-8">
      {/* Back button */}
      <Link
        href="/projects"
        className="inline-flex items-center gap-2 text-xs text-white/35 hover:text-white"
      >
        <ArrowLeft className="size-3.5" /> Projects
      </Link>

      {/* Project Header */}
      <div className="mt-5 flex flex-col gap-5 border-b border-white/[0.07] pb-6 lg:flex-row lg:items-end lg:justify-between">
        <div className="min-w-0">
          <div className="flex items-center gap-3">
            <span className="flex size-10 items-center justify-center rounded-xl bg-gradient-to-br from-cyan-400/15 to-fuchsia-500/15">
              <Github className="size-5 text-white/70" />
            </span>
            <div className="min-w-0">
              <h1 className="truncate text-2xl font-semibold tracking-[-0.035em]">
                {p.repository_name}
              </h1>
              <p className="truncate text-sm text-white/35">{p.repository_full_name}</p>
            </div>
          </div>
          <div className="mt-4 flex flex-wrap items-center gap-3 text-xs text-white/35">
            <FrameworkBadge framework={p.build_configuration?.executable ?? "node"} size="sm" />
            <span className="inline-flex items-center gap-1.5">
              <GitBranch className="size-3.5" />
              {branch}
            </span>
            <span>Webhook {p.project_webhook_enabled ? "enabled" : "disabled"}</span>
            <span>Updated {formatShortDate(p.updated_at)}</span>
          </div>
        </div>

        <div className="flex flex-wrap gap-2">
          <Button
            variant="outline"
            onClick={() => createBuild.mutate(branch)}
            disabled={createBuild.isPending}
            className="gap-1.5"
          >
            {createBuild.isPending ? (
              <RefreshCw className="animate-spin" />
            ) : (
              <Play className="size-3.5" />
            )}
            Build now
          </Button>
          <Button
            variant="outline"
            onClick={handleDelete}
            disabled={deleteProject.isPending}
            className="gap-1.5"
          >
            <Trash2 className="size-3.5 text-rose-300" /> Delete
          </Button>
        </div>
      </div>

      {/* Tabs */}
      <div className="mt-6">
        <Tabs value={activeTab} onValueChange={setActiveTab} className="w-full">
          <TabsList className="grid w-full grid-cols-4 bg-white/[0.02] border border-white/[0.08] rounded-xl p-1">
            <TabsTrigger value="deployments" className="gap-2 py-2.5">
              <Terminal className="size-3.5" />
              <span>Deployments</span>
            </TabsTrigger>
            <TabsTrigger value="settings" className="gap-2 py-2.5">
              <Settings2 className="size-3.5" />
              <span>Settings</span>
            </TabsTrigger>
            <TabsTrigger value="environment" className="gap-2 py-2.5">
              <Key className="size-3.5" />
              <span>Environment</span>
            </TabsTrigger>
            <TabsTrigger value="domains" className="gap-2 py-2.5">
              <Zap className="size-3.5" />
              <span>Domains</span>
            </TabsTrigger>
          </TabsList>

          <TabsContent value="deployments" className="mt-4">
            <section className="rounded-2xl border border-white/[0.08] bg-white/[0.02] overflow-hidden">
              <div className="flex items-center justify-between p-5 border-b border-white/[0.07]">
                <div>
                  <p className="text-sm font-medium">Deployments</p>
                  <p className="mt-1 text-xs text-white/35">
                    Builds created by pushes and manual triggers.
                  </p>
                </div>
                <Terminal className="size-4 text-white/25" />
              </div>

              <div className="p-5 space-y-2">
                {builds.isPending ? (
                  [1, 2, 3].map((n) => (
                    <div key={n} className="h-16 animate-pulse rounded-xl bg-white/[0.04]" />
                  ))
                ) : builds.data?.length ? (
                  builds.data.map((b) => (
                    <a
                      key={b.id}
                      href={`/projects/${p.id}/builds/${b.id}`}
                      className="block rounded-xl border border-white/[0.07] bg-black/20 p-4 transition hover:bg-white/[0.03]"
                    >
                      <div className="flex flex-wrap items-center gap-3">
                        <span className="font-mono text-xs text-white/35">
                          #{b.build_number ?? "—"}
                        </span>
                        <DeploymentStatusBadge status={b.status} size="sm" />
                        <span className="text-[10px] text-white/25">
                          {formatDate(b.created_at)}
                        </span>
                        <span className="ml-auto text-[10px] text-white/30">
                          {b.trigger ?? "manual"}
                        </span>
                      </div>
                      <div className="mt-3 flex flex-wrap gap-4 text-[10px] text-white/35">
                        <span>commit {b.commit_sha?.slice(0, 12) ?? "—"}</span>
                        {b.output_url ? (
                          <a
                            href={b.output_url}
                            target="_blank"
                            rel="noreferrer"
                            className="text-cyan-200/70 hover:text-cyan-200"
                          >
                            Open output ↗
                          </a>
                        ) : null}
                      </div>
                      {b.error_message ? (
                        <p className="mt-3 rounded-lg bg-rose-400/5 p-2.5 text-[11px] text-rose-200/70">
                          {b.error_message}
                        </p>
                      ) : null}
                      <div className="mt-4 rounded-lg border border-white/[0.06] bg-[#050505] p-3 font-mono text-[10px] text-white/40">
                        {b.output_url
                          ? `deployment output: ${b.output_url}`
                          : "Build logs are not available yet."}
                      </div>
                    </a>
                  ))
                ) : (
                  <div className="rounded-xl border border-dashed border-white/10 p-12 text-center text-xs text-white/30">
                    No builds yet. Click &ldquo;Build now&rdquo; to create the first one.
                  </div>
                )}
              </div>
            </section>
          </TabsContent>

          <TabsContent value="settings" className="mt-4">
            <div className="grid gap-4 lg:grid-cols-[1.25fr_.75fr]">
              <section className="rounded-2xl border border-white/[0.08] bg-white/[0.02] p-5">
                <div className="flex items-center gap-2 mb-4">
                  <Settings2 className="size-4 text-white/35" />
                  <p className="text-sm font-medium">Build Configuration</p>
                </div>
                <dl className="space-y-4 text-sm">
                  <div className="flex flex-col sm:flex-row sm:items-center sm:justify-between gap-2 p-3 rounded-lg bg-white/[0.02] border border-white/[0.05]">
                    <dt className="text-white/30">Framework</dt>
                    <dd className="text-right">
                      <FrameworkBadge framework={p.build_configuration?.executable ?? "node"} />
                    </dd>
                  </div>
                  <div className="flex flex-col sm:flex-row sm:items-center sm:justify-between gap-2 p-3 rounded-lg bg-white/[0.02] border border-white/[0.05]">
                    <dt className="text-white/30">Build Command</dt>
                    <dd className="font-mono text-right text-white/60 truncate max-w-[60%]">
                      {[p.build_configuration.executable, ...p.build_configuration.args].join(" ")}
                    </dd>
                  </div>
                  <div className="flex flex-col sm:flex-row sm:items-center sm:justify-between gap-2 p-3 rounded-lg bg-white/[0.02] border border-white/[0.05]">
                    <dt className="text-white/30">Output Directory</dt>
                    <dd className="font-mono text-right text-white/60">{p.build_configuration.working_dir}</dd>
                  </div>
                  <div className="flex flex-col sm:flex-row sm:items-center sm:justify-between gap-2 p-3 rounded-lg bg-white/[0.02] border border-white/[0.05]">
                    <dt className="text-white/30">Working Directory</dt>
                    <dd className="font-mono text-right text-white/60">{p.build_configuration.working_dir}</dd>
                  </div>
                  <div className="flex flex-col sm:flex-row sm:items-center sm:justify-between gap-2 p-3 rounded-lg bg-white/[0.02] border border-white/[0.05]">
                    <dt className="text-white/30">Branch</dt>
                    <dd className="font-mono text-right text-white/60">{branch}</dd>
                  </div>
                  <div className="flex flex-col sm:flex-row sm:items-center sm:justify-between gap-2 p-3 rounded-lg bg-white/[0.02] border border-white/[0.05]">
                    <dt className="text-white/30">Webhook Builds</dt>
                    <dd className="text-right">{p.project_webhook_enabled ? "Enabled" : "Disabled"}</dd>
                  </div>
                  <div className="flex flex-col sm:flex-row sm:items-center sm:justify-between gap-2 p-3 rounded-lg bg-white/[0.02] border border-white/[0.05]">
                    <dt className="text-white/30">Scanner Policy</dt>
                    <dd className="text-right">{p.command_policy_version}</dd>
                  </div>
                  <div className="flex flex-col sm:flex-row sm:items-center sm:justify-between gap-2 p-3 rounded-lg bg-white/[0.02] border border-white/[0.05]">
                    <dt className="text-white/30">Scanner Result</dt>
                    <dd className="capitalize text-emerald-300/80">{p.command_scan_result.status}</dd>
                  </div>
                </dl>
              </section>

              <aside className="space-y-4">
                <div className="rounded-2xl border border-white/[0.08] bg-gradient-to-br from-cyan-400/[0.05] via-fuchsia-500/[0.04] to-emerald-400/[0.05] p-5">
                  <p className="text-xs font-medium">Latest build</p>
                  {latest ? (
                    <>
                      <div className="mt-3 flex items-center justify-between">
                        <span className="text-2xl font-semibold">#{latest.build_number ?? "—"}</span>
                        <DeploymentStatusBadge status={latest.status} />
                      </div>
                      <p className="mt-2 text-[10px] text-white/35">
                        {latest.commit_sha?.slice(0, 12) ?? "No commit SHA"}
                      </p>
                    </>
                  ) : (
                    <p className="mt-3 text-xs text-white/30">No build has been created.</p>
                  )}
                </div>

                <div className="rounded-2xl border border-white/[0.08] bg-white/[0.02] p-5">
                  <div className="flex items-center gap-2 mb-4">
                    <Clock className="size-4 text-white/35" />
                    <p className="text-sm font-medium">Repository Info</p>
                  </div>
                  <dl className="space-y-3 text-xs">
                    <div className="flex justify-between gap-4 p-3 rounded-lg bg-white/[0.02] border border-white/[0.05]">
                      <dt className="text-white/30">Repository</dt>
                      <dd className="font-mono text-right text-white/60 truncate max-w-[60%]">{p.repository_full_name}</dd>
                    </div>
                    <div className="flex justify-between gap-4 p-3 rounded-lg bg-white/[0.02] border border-white/[0.05]">
                      <dt className="text-white/30">Owner</dt>
                      <dd className="font-mono text-right text-white/60">{p.repository_owner}</dd>
                    </div>
                    <div className="flex justify-between gap-4 p-3 rounded-lg bg-white/[0.02] border border-white/[0.05]">
                      <dt className="text-white/30">Clone URL</dt>
                      <dd className="font-mono text-right text-white/60 truncate max-w-[60%]">{p.build_configuration?.executable ?? "N/A"}</dd>
                    </div>
                    <div className="flex justify-between gap-4 p-3 rounded-lg bg-white/[0.02] border border-white/[0.05]">
                      <dt className="text-white/30">Repository Available</dt>
                      <dd className="text-right">{p.repository_available ? "Yes" : "No"}</dd>
                    </div>
                    <div className="flex justify-between gap-4 p-3 rounded-lg bg-white/[0.02] border border-white/[0.05]">
                      <dt className="text-white/30">Created</dt>
                      <dd className="text-right">{formatShortDate(p.created_at)}</dd>
                    </div>
                  </dl>
                </div>
              </aside>
            </div>
          </TabsContent>

          <TabsContent value="environment" className="mt-4">
            <div className="rounded-2xl border border-white/[0.08] bg-white/[0.02] p-6">
              <div className="flex items-center justify-between mb-6">
                <div className="flex items-center gap-2">
                  <Key className="size-4 text-white/35" />
                  <p className="text-sm font-medium">Environment Variables</p>
                </div>
              </div>
              <div className="rounded-xl border border-white/[0.08] bg-white/[0.02] p-4 text-center text-white/40">
                <p className="text-sm">Environment variables management coming soon.</p>
                <p className="mt-1 text-xs">Backend support required for secure variable storage.</p>
              </div>
            </div>
          </TabsContent>

          <TabsContent value="domains" className="mt-4">
            <div className="rounded-2xl border border-white/[0.08] bg-white/[0.02] p-6">
              <div className="flex items-center justify-between mb-6">
                <div className="flex items-center gap-2">
                  <Zap className="size-4 text-white/35" />
                  <p className="text-sm font-medium">Custom Domains</p>
                </div>
              </div>
              <div className="rounded-xl border border-white/[0.08] bg-white/[0.02] p-4 text-center text-white/40">
                <p className="text-sm">Custom domains coming soon.</p>
                <p className="mt-1 text-xs">Configure custom domains for your deployments.</p>
              </div>
            </div>
          </TabsContent>
        </Tabs>
      </div>
    </div>
  );
}