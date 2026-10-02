"use client";

import { Github, Lock, Clock, ExternalLink, GitBranch } from "lucide-react";
import { cn } from "@/lib/utils/cn";
import { FrameworkBadge } from "./framework-badge";
import { DeploymentStatusBadge } from "./deployment-status-badge";
import type { Project } from "@/types/domain";

interface ProjectCardProps {
  project: Project;
  latestDeployment?: {
    status: string;
    created_at: string;
    build_number?: number;
  } | null;
  onClick?: () => void;
  variant?: "dashboard" | "list" | "import";
  showActions?: boolean;
}

export function ProjectCard({
  project,
  latestDeployment,
  onClick,
  variant = "dashboard",
  showActions = false,
}: ProjectCardProps) {
  const branch = project.configured_branch.replace("refs/heads/", "");
  const isPrivate = !project.repository_available;

  const baseClasses = "group relative overflow-hidden rounded-2xl border transition-all hover:-translate-y-0.5 hover:border-white/[0.15]";
  const variantClasses = {
    dashboard: "bg-white/[0.02] p-5 border-white/[0.08] hover:bg-white/[0.04]",
    list: "bg-white/[0.02] p-4 border-white/[0.08] flex items-center gap-4 hover:bg-white/[0.04]",
    import: "bg-white/[0.02] p-4 border-white/[0.08] cursor-pointer hover:bg-white/[0.05] hover:border-cyan-300/30",
  };

  const handleClick = () => onClick?.();

  if (variant === "import") {
    return (
      <button
        type="button"
        onClick={handleClick}
        className={cn(baseClasses, variantClasses.import, "w-full text-left")}
      >
        <div className="flex items-start gap-4">
          <div className="flex size-12 shrink-0 items-center justify-center rounded-xl bg-gradient-to-br from-cyan-400/15 to-fuchsia-500/15">
            <Github className="size-5 text-white/60" />
          </div>
          <div className="min-w-0 flex-1">
            <div className="flex items-center gap-2">
              <h3 className="truncate font-medium text-white group-hover:text-white">{project.repository_name}</h3>
              {isPrivate && <Lock className="size-3.5 text-amber-300/70" aria-label="Private" />}
            </div>
            <p className="mt-1 truncate text-xs text-white/35">{project.repository_full_name}</p>
            <div className="mt-3 flex flex-wrap items-center gap-3 text-[10px] text-white/35">
              <span className="inline-flex items-center gap-1">
                <GitBranch className="size-3" />
                {branch}
              </span>
              {project.updated_at && (
                <span>Updated {new Date(project.updated_at).toLocaleDateString()}</span>
              )}
            </div>
          </div>
          <div className="flex items-center justify-center px-3 text-cyan-300/60">
            <ExternalLink className="size-5" />
          </div>
        </div>
      </button>
    );
  }

  return (
    <article className={cn(baseClasses, variantClasses[variant])} onClick={handleClick}>
      <div className="flex items-start justify-between">
        <div className="flex items-center gap-3">
          <span className="flex size-10 items-center justify-center rounded-lg bg-gradient-to-br from-cyan-400/15 to-fuchsia-500/15">
            <Github className="size-4.5 text-white/65" />
          </span>
          <div className="min-w-0">
            <h3 className="truncate font-medium text-white group-hover:text-white">{project.repository_name}</h3>
            <p className="truncate text-xs text-white/35">{project.repository_full_name}</p>
          </div>
        </div>
        {isPrivate && (
          <span className="flex shrink-0 items-center gap-1 rounded-full border border-amber-300/30 bg-amber-300/10 px-2 py-0.5 text-[10px] text-amber-200/80">
            <Lock className="size-2.5" /> Private
          </span>
        )}
      </div>

      <div className="mt-4 flex flex-wrap items-center gap-3 text-[10px] text-white/35">
        <FrameworkBadge framework={project.build_configuration?.executable ?? "node"} />
        <span className="inline-flex items-center gap-1">
          <Clock className="size-3" />
          {branch}
        </span>
        {latestDeployment && (
          <DeploymentStatusBadge status={latestDeployment.status} size="sm" />
        )}
        {project.updated_at && (
          <span>Updated {new Date(project.updated_at).toLocaleDateString()}</span>
        )}
      </div>

      {showActions && variant === "dashboard" && latestDeployment && (
        <div className="mt-4 pt-4 border-t border-white/[0.06] flex items-center justify-between">
          <a
            href={`/projects/${project.id}`}
            onClick={(e) => e.stopPropagation()}
            className="text-xs text-cyan-300/70 hover:text-cyan-300"
          >
            View deployments →
          </a>
          <span className="text-[10px] text-white/30">#{latestDeployment.build_number}</span>
        </div>
      )}
    </article>
  );
}