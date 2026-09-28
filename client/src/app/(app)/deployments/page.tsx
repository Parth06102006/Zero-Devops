"use client";

import Link from "next/link";
import { Activity, ArrowUpRight, GitCommitHorizontal, Rocket } from "lucide-react";
import { useMemo, useState } from "react";
import { useProjects } from "@/features/projects";
import { useAllDeployments, StatusPill } from "@/features/deployments";

export default function DeploymentsPage() {
  const projects = useProjects();
  const queries = useAllDeployments(projects.data ?? []);
  const [filter, setFilter] = useState("all");

  const projectMap = useMemo(
    () => new Map((projects.data ?? []).map((p) => [p.id, p])),
    [projects.data],
  );

  const deployments = (queries.data ?? [])
    .map((d) => ({ ...d, project: projectMap.get(d.project_id ?? "") }))
    .sort((a, b) => new Date(b.created_at).getTime() - new Date(a.created_at).getTime());

  const filtered = filter === "all" ? deployments : deployments.filter((d) => d.status === filter);

  const filterOptions = ["all", "success", "building", "pending", "failed"] as const;

  return (
    <div className="mx-auto max-w-[1400px] px-4 py-6 sm:px-6 lg:px-8">
      {/* Header */}
      <div className="flex flex-col gap-4 border-b border-white/[0.07] pb-6 sm:flex-row sm:items-end sm:justify-between">
        <div>
          <p className="text-xs text-white/35">Release activity</p>
          <h1 className="mt-1 text-2xl font-semibold tracking-[-0.035em]">Deployments</h1>
          <p className="mt-1 text-sm text-white/35">
            Aggregated from each project&apos;s real build history.
          </p>
        </div>
        <div className="flex items-center gap-2 rounded-lg border border-white/[0.08] bg-white/[0.02] px-3 py-2 text-xs text-white/45">
          <Activity className="size-3.5" /> {deployments.length} builds
        </div>
      </div>

      {/* Filter Tabs */}
      <div className="mt-5 flex flex-wrap gap-2">
        {filterOptions.map((v) => (
          <button
            key={v}
            type="button"
            onClick={() => setFilter(v)}
            className={`rounded-lg border px-3 py-2 text-xs capitalize ${
              filter === v
                ? "border-white/15 bg-white/[0.07] text-white"
                : "border-white/[0.08] text-white/35 hover:text-white"
            }`}
          >
            {v}
          </button>
        ))}
      </div>

      {/* Deployments List */}
      <div className="mt-4 overflow-hidden rounded-2xl border border-white/[0.08] bg-white/[0.02]">
        {projects.isPending ? (
          <div className="p-10 text-center text-xs text-white/30">
            Loading deployment activity…
          </div>
        ) : filtered.length ? (
          <div className="divide-y divide-white/[0.06]">
            {filtered.map((d) => (
              <Link
                key={d.id}
                href={d.project_id ? `/projects/${d.project_id}` : "/deployments"}
                className="flex flex-col gap-3 p-4 transition hover:bg-white/[0.025] sm:flex-row sm:items-center"
              >
                <span className="flex size-9 shrink-0 items-center justify-center rounded-lg bg-white/[0.04]">
                  <Rocket className="size-4 text-white/45" />
                </span>
                <span className="min-w-0 flex-1">
                  <span className="block truncate text-sm font-medium text-white/75">
                    {d.project?.repository_name ?? "Project"} · build #{d.build_number ?? "—"}
                  </span>
                  <span className="mt-1 flex flex-wrap items-center gap-2 text-[10px] text-white/30">
                    <GitCommitHorizontal className="size-3" />
                    {d.commit_sha?.slice(0, 12) ?? d.requested_ref ?? "No revision"}
                    <span>·</span>
                    {new Date(d.created_at).toLocaleString()}
                  </span>
                </span>
                <StatusPill status={d.status} />
                <ArrowUpRight className="hidden size-4 text-white/20 sm:block" />
              </Link>
            ))}
          </div>
        ) : (
          <div className="p-16 text-center">
            <Rocket className="mx-auto size-7 text-white/20" />
            <p className="mt-3 text-sm text-white/45">No deployments found.</p>
            <p className="mt-1 text-xs text-white/25">Create a project and trigger its first build.</p>
          </div>
        )}
      </div>
    </div>
  );
}
