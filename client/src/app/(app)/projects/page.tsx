"use client";

import Link from "next/link";
import {
  ArrowUpRight,
  FolderKanban,
  GitBranch,
  Github,
  Grid2X2,
  List,
  Search,
} from "lucide-react";
import { useMemo, useState } from "react";
import { useProjects } from "@/features/projects";

export default function ProjectsPage() {
  const { data, isPending, isError, error } = useProjects();
  const [search, setSearch] = useState("");
  const [list, setList] = useState(false);

  const filtered = useMemo(
    () =>
      (data ?? []).filter((p) =>
        `${p.repository_full_name} ${p.repository_name}`
          .toLowerCase()
          .includes(search.toLowerCase()),
      ),
    [data, search],
  );

  return (
    <div className="mx-auto max-w-[1500px] px-4 py-6 sm:px-6 lg:px-8">
      {/* Header */}
      <div className="flex flex-col gap-4 border-b border-white/[0.07] pb-6 lg:flex-row lg:items-end lg:justify-between">
        <div>
          <p className="text-xs text-white/35">Workspace</p>
          <h1 className="mt-1 text-2xl font-semibold tracking-[-0.035em]">Projects</h1>
          <p className="mt-1 text-sm text-white/35">
            Repositories configured for Zero-DevOps builds.
          </p>
        </div>
        <Link
          href="#new"
          className="hidden rounded-lg border border-white/10 px-3 py-2 text-xs text-white/55 hover:bg-white/[0.05] lg:inline-flex"
        >
          Use the New Deployment button above
        </Link>
      </div>

      {/* Controls: search + view toggle */}
      <div className="mt-5 flex gap-2">
        <div className="relative flex-1">
          <Search className="pointer-events-none absolute left-3 top-3 size-4 text-white/25" />
          <input
            value={search}
            onChange={(e) => setSearch(e.target.value)}
            placeholder="Search Projects"
            className="h-10 w-full rounded-lg border border-white/[0.09] bg-white/[0.025] pl-9 pr-3 text-sm text-white outline-none placeholder:text-white/25 focus:border-white/20"
          />
        </div>
        <button
          type="button"
          onClick={() => setList(false)}
          className={`rounded-lg border px-3 ${
            !list
              ? "border-white/15 bg-white/[0.07] text-white"
              : "border-white/[0.08] text-white/35"
          }`}
          aria-label="Grid view"
        >
          <Grid2X2 className="size-4" />
        </button>
        <button
          type="button"
          onClick={() => setList(true)}
          className={`rounded-lg border px-3 ${
            list
              ? "border-white/15 bg-white/[0.07] text-white"
              : "border-white/[0.08] text-white/35"
          }`}
          aria-label="List view"
        >
          <List className="size-4" />
        </button>
      </div>

      {/* Content */}
      {isPending ? (
        <div className="mt-5 grid gap-3 md:grid-cols-2 xl:grid-cols-3">
          {[1, 2, 3, 4, 5, 6].map((n) => (
            <div key={n} className="h-40 animate-pulse rounded-xl bg-white/[0.04]" />
          ))}
        </div>
      ) : isError ? (
        <div className="mt-5 rounded-xl border border-rose-400/15 bg-rose-400/5 p-5 text-sm text-rose-200/70">
          {error.message}
        </div>
      ) : filtered.length ? (
        <div
          className={`mt-5 ${
            list ? "space-y-2" : "grid gap-3 md:grid-cols-2 xl:grid-cols-3"
          }`}
        >
          {filtered.map((project, index) => (
            <Link
              key={project.id}
              href={`/projects/${project.id}`}
              className={`group overflow-hidden rounded-xl border border-white/[0.08] bg-white/[0.02] transition hover:-translate-y-0.5 hover:border-white/[0.16] ${
                list ? "flex items-center gap-4 p-4" : "p-5"
              }`}
            >
              <div className="flex items-start justify-between">
                <span
                  className={`flex size-9 items-center justify-center rounded-lg bg-gradient-to-br ${
                    index % 3 === 0
                      ? "from-cyan-400/15 to-blue-500/10"
                      : index % 3 === 1
                        ? "from-fuchsia-400/15 to-violet-500/10"
                        : "from-emerald-400/15 to-cyan-500/10"
                  }`}
                >
                  <Github className="size-4 text-white/65" />
                </span>
                {!list && (
                  <ArrowUpRight className="size-4 text-white/20 transition group-hover:-translate-y-0.5 group-hover:translate-x-0.5 group-hover:text-white/60" />
                )}
              </div>
              <div className={list ? "min-w-0 flex-1" : ""}>
                <p className="mt-4 truncate text-sm font-medium group-hover:text-white">
                  {project.repository_name}
                </p>
                <p className="mt-1 truncate text-xs text-white/35">
                  {project.repository_full_name}
                </p>
                <div className="mt-4 flex flex-wrap items-center gap-3 text-[10px] text-white/35">
                  <span className="inline-flex items-center gap-1">
                    <GitBranch className="size-3" />
                    {project.configured_branch.replace("refs/heads/", "")}
                  </span>
                  <span>{new Date(project.updated_at).toLocaleDateString()}</span>
                </div>
              </div>
            </Link>
          ))}
        </div>
      ) : (
        <div className="mt-5 rounded-2xl border border-dashed border-white/10 p-16 text-center">
          <FolderKanban className="mx-auto size-7 text-white/20" />
          <p className="mt-3 text-sm text-white/50">No projects match your search.</p>
        </div>
      )}
    </div>
  );
}
