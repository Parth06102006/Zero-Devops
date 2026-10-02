"use client";

import Link from "next/link";
import {
  Activity,
  ArrowUpRight,
  Boxes,
  CheckCircle2,
  Clock3,
  Plus,
  Rocket,
  Sparkles,
} from "lucide-react";
import { useProjects } from "@/features/projects";
import { useAllDeployments } from "@/features/deployments";
import { useCurrentUser } from "@/features/auth";
import { ProjectCard, DeploymentStatusBadge } from "@/features/projects";
import { Button } from "@/components/ui/button";

export default function DashboardPage() {
  const projects = useProjects();
  const user = useCurrentUser();
  const builds = useAllDeployments(projects.data ?? []);
  const deployments = (builds.data ?? []).sort(
    (a, b) => new Date(b.created_at).getTime() - new Date(a.created_at).getTime(),
  );
  const successful = deployments.filter((d) => d.status === "success").length;
  const active = deployments.filter(
    (d) => d.status === "pending" || d.status === "building",
  ).length;

  const statCards = [
    {
      label: "Projects",
      value: projects.data?.length ?? 0,
      icon: Boxes,
      tone: "from-cyan-400/20 to-blue-500/5",
    },
    {
      label: "Deployments",
      value: deployments.length,
      icon: Rocket,
      tone: "from-fuchsia-500/20 to-purple-500/5",
    },
    {
      label: "Successful builds",
      value: successful,
      icon: CheckCircle2,
      tone: "from-emerald-400/20 to-cyan-400/5",
    },
    {
      label: "Active builds",
      value: active,
      icon: Activity,
      tone: "from-amber-400/20 to-rose-400/5",
    },
  ];

  return (
    <div className="mx-auto max-w-[1500px] px-4 py-6 sm:px-6 lg:px-8">
      {/* Header */}
      <div className="mb-7 flex flex-col gap-4 sm:flex-row sm:items-end sm:justify-between">
        <div>
          <p className="text-xs font-medium text-white/35">Overview</p>
          <h1 className="mt-1 text-2xl font-semibold tracking-[-0.035em]">
            Good to see you
            {user.data?.user.username ? `, ${user.data.user.username}` : ""}.
          </h1>
          <p className="mt-1 text-sm text-white/35">
            Your repositories, builds and deployment activity in one place.
          </p>
        </div>
        <Link
          href="/projects"
          className="inline-flex w-fit items-center gap-2 rounded-lg border border-white/10 px-3.5 py-2 text-xs text-white/65 hover:bg-white/[0.05] hover:text-white"
        >
          Manage projects <ArrowUpRight className="size-3.5" />
        </Link>
      </div>

      {/* Metrics Row */}
      <section className="grid gap-3 sm:grid-cols-2 xl:grid-cols-4">
        {statCards.map(({ label, value, icon: Icon, tone }) => (
          <div
            key={label}
            className="relative overflow-hidden rounded-xl border border-white/[0.08] bg-white/[0.025] p-4"
          >
            <div
              className={`pointer-events-none absolute -right-10 -top-10 size-28 rounded-full bg-gradient-to-br ${tone} blur-2xl`}
            />
            <div className="relative flex items-center justify-between">
              <span className="text-xs text-white/40">{label}</span>
              <Icon className="size-4 text-white/35" />
            </div>
            <p className="relative mt-5 text-2xl font-semibold">{value}</p>
          </div>
        ))}
      </section>

      {/* Projects & Activity Grid */}
      <section className="mt-6 grid gap-4 xl:grid-cols-[1.35fr_.65fr]">
        {/* Projects panel */}
        <div className="rounded-2xl border border-white/[0.08] bg-white/[0.02] p-5">
          <div className="flex items-center justify-between">
            <div>
              <p className="text-sm font-medium">Projects</p>
              <p className="mt-1 text-xs text-white/35">Configured repositories ready to build.</p>
            </div>
            <Link href="/projects" className="text-xs text-white/45 hover:text-white">
              View all
            </Link>
          </div>

          {projects.isPending ? (
            <div className="mt-5 grid gap-3 sm:grid-cols-2 lg:grid-cols-3">
              {[1, 2, 3, 4, 5, 6].map((n) => (
                <div key={n} className="h-40 animate-pulse rounded-xl bg-white/[0.04]" />
              ))}
            </div>
          ) : projects.isError ? (
            <div className="mt-5 rounded-xl border border-rose-400/15 bg-rose-400/5 p-4 text-xs text-rose-200/70">
              {projects.error.message}
            </div>
          ) : projects.data?.length ? (
            <div className="mt-5 grid gap-3 sm:grid-cols-2 lg:grid-cols-3">
              {projects.data.slice(0, 6).map((project) => (
                <ProjectCard
                  key={project.id}
                  project={project}
                  latestDeployment={(builds.data ?? []).find(d => d.project_id === project.id) ?? null}
                  variant="dashboard"
                  showActions
                  onClick={() => {}}
                />
              ))}
            </div>
          ) : (
            <div className="mt-5 rounded-xl border border-dashed border-white/10 p-10 text-center">
              <Boxes className="mx-auto size-6 text-white/20" />
              <p className="mt-3 text-sm text-white/50">No projects yet.</p>
              <p className="mt-1 text-xs text-white/25">
                Import a GitHub repository to get started.
              </p>
              <Button
                variant="default"
                className="mt-4 gap-2"
                onClick={() => {
                  // Trigger the import dialog - we'll need to handle this
                }}
              >
                <Plus className="size-3.5" /> Import Repository
              </Button>
            </div>
          )}
        </div>

        {/* Recent activity panel */}
        <div className="rounded-2xl border border-white/[0.08] bg-white/[0.02] p-5">
          <div className="flex items-center justify-between">
            <div>
              <p className="text-sm font-medium">Recent activity</p>
              <p className="mt-1 text-xs text-white/35">Latest builds across your projects.</p>
            </div>
            <Activity className="size-4 text-white/25" />
          </div>
          <div className="mt-5 space-y-2">
            {deployments.slice(0, 6).map((deployment) => (
              <Link
                key={deployment.id}
                href={
                  deployment.project_id
                    ? `/projects/${deployment.project_id}`
                    : "/deployments"
                }
                className="flex items-center gap-3 rounded-xl border border-white/[0.06] bg-black/20 p-3 hover:bg-white/[0.04]"
              >
                <span className="flex size-8 items-center justify-center rounded-lg bg-white/[0.04]">
                  <Rocket className="size-3.5 text-white/40" />
                </span>
                <span className="min-w-0 flex-1">
                  <span className="block truncate text-xs text-white/70">
                    Build #{deployment.build_number ?? "—"}
                  </span>
                  <span className="mt-0.5 block truncate text-[10px] text-white/30">
                    {deployment.commit_sha?.slice(0, 7) ??
                      deployment.requested_ref ??
                      "No revision"}
                  </span>
                </span>
                <DeploymentStatusBadge status={deployment.status} size="sm" />
              </Link>
            ))}
            {!deployments.length ? (
              <div className="rounded-xl border border-dashed border-white/10 p-8 text-center">
                <Clock3 className="mx-auto size-5 text-white/20" />
                <p className="mt-2 text-xs text-white/35">No deployment activity yet.</p>
              </div>
            ) : null}
          </div>
        </div>
      </section>

      {/* Deployment flow explainer banner */}
      <section className="mt-4 rounded-2xl border border-white/[0.08] bg-gradient-to-r from-cyan-400/[0.05] via-fuchsia-500/[0.035] to-emerald-400/[0.05] p-5">
        <div className="flex flex-col gap-4 sm:flex-row sm:items-center">
          <div className="flex size-10 items-center justify-center rounded-xl bg-white/[0.06]">
            <Sparkles className="size-4 text-cyan-200" />
          </div>
          <div className="flex-1">
            <p className="text-sm font-medium">Zero-DevOps deployment flow</p>
            <p className="mt-1 text-xs leading-5 text-white/35">
              GitHub push → webhook → build queue → worker → immutable build → deployment.
            </p>
          </div>
          <Link
            href="/deployments"
            className="inline-flex items-center gap-2 text-xs text-white/55 hover:text-white"
          >
            Open activity <ArrowUpRight className="size-3.5" />
          </Link>
        </div>
      </section>
    </div>
  );
}
