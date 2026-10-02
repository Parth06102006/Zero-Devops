"use client";

import { useState, useMemo, useEffect } from "react";
import { Search, Github, Loader2, ChevronRight, Filter, X, Plus } from "lucide-react";
import {
  Dialog,
  DialogContent,
  DialogHeader,
  DialogTitle,
  DialogDescription,
} from "@/components/ui/dialog";
import { Input } from "@/components/ui/input";
import { Button } from "@/components/ui/button";
import { useGithubRepositories, useGithubInstallation } from "@/features/github";
import { getGithubAppInstallUrl } from "@/features/github";
import { ProjectCard } from "./ui";

interface ImportRepositoryDialogProps {
  open: boolean;
  onOpenChange: (open: boolean) => void;
  onSelectRepository: (repo: {
    id: number;
    full_name: string;
    name: string;
    owner: string;
    default_branch: string;
    clone_url: string;
    private: boolean;
  }) => void;
}

export function ImportRepositoryDialog({
  open,
  onOpenChange,
  onSelectRepository,
}: ImportRepositoryDialogProps) {
  const [search, setSearch] = useState("");
  const [filter, setFilter] = useState<"all" | "public" | "private">("all");
  const [selectedRepo, setSelectedRepo] = useState<number | null>(null);
  const [showFilter, setShowFilter] = useState(false);

  const installation = useGithubInstallation();
  const repos = useGithubRepositories(search, !!installation.data);

  const filteredRepos = useMemo(() => {
    if (!repos.data) return [];
    return repos.data.repositories.filter((repo) => {
      if (filter === "public") return !repo.private;
      if (filter === "private") return repo.private;
      return true;
    });
  }, [repos.data, filter]);

  const handleSelect = (repo: typeof filteredRepos[0]) => {
    setSelectedRepo(repo.id);
    onSelectRepository(repo);
  };

  useEffect(() => {
    setSelectedRepo(null);
    setSearch("");
  }, [open]);

  const isNotInstalled = !installation.isPending && !installation.data;

  return (
    <Dialog open={open} onOpenChange={onOpenChange}>
      <DialogContent className="max-w-4xl max-h-[90vh] border-white/10 bg-[#0a0a0a] text-white shadow-2xl overflow-hidden">
        <DialogHeader>
          <DialogTitle className="flex items-center gap-2 text-xl">
            <span className="flex size-8 items-center justify-center rounded-lg bg-gradient-to-br from-cyan-400/20 to-fuchsia-500/20">
              <Github className="size-4 text-cyan-200" />
            </span>
            Import Repository
          </DialogTitle>
          <DialogDescription className="text-white/45">
            Select a GitHub repository to deploy. Configure build settings in the next step.
          </DialogDescription>
        </DialogHeader>

        <div className="grid gap-5">
          {isNotInstalled ? (
            <div className="rounded-xl border border-cyan-400/20 bg-cyan-400/5 p-6 text-center">
              <div className="flex items-center justify-center gap-2 mb-3">
                <span className="flex size-12 items-center justify-center rounded-xl bg-cyan-400/15">
                  <Github className="size-6 text-cyan-300" />
                </span>
              </div>
              <p className="text-sm text-white/70 mb-1">GitHub App not connected</p>
              <p className="text-xs text-white/40 mb-4">Install the Zero-DevOps GitHub App to access your repositories</p>
              <Button
                variant="default"
                className="gap-2"
                onClick={() => window.location.assign(getGithubAppInstallUrl("/projects"))}
              >
                <Plus className="size-3.5" /> Connect GitHub App
              </Button>
            </div>
          ) : (
            <>
              <div className="flex flex-col sm:flex-row gap-4">
                <div className="relative flex-1">
                  <Search className="pointer-events-none absolute left-3 top-3 size-4 text-white/25" />
                  <Input
                    value={search}
                    onChange={(e) => setSearch(e.target.value)}
                    placeholder="Search repositories by name..."
                    className="border-white/10 bg-white/[0.03] pl-9 text-white placeholder:text-white/25"
                  />
                </div>
                <div className="flex items-center gap-2">
                  <Button
                    variant="outline"
                    size="sm"
                    onClick={() => setShowFilter(!showFilter)}
                    className="gap-1.5"
                  >
                    <Filter className="size-3.5" />
                    <span>Filter</span>
                  </Button>
                </div>
              </div>

              {showFilter && (
                <div className="flex items-center gap-3 p-3 rounded-lg bg-white/[0.02] border border-white/[0.05]">
                  <span className="text-xs text-white/40">Visibility:</span>
                  <div className="flex gap-2">
                    {(["all", "public", "private"] as const).map((f) => (
                      <Button
                        key={f}
                        variant={filter === f ? "default" : "outline"}
                        size="sm"
                        onClick={() => setFilter(f)}
                        className="gap-1.5"
                      >
                        {f === "all" && <Github className="size-3.5" />}
                        {f === "public" && <span className="text-xs">Public</span>}
                        {f === "private" && <span className="text-xs">Private</span>}
                      </Button>
                    ))}
                  </div>
                </div>
              )}

              <div className="max-h-[60vh] overflow-auto">
                {repos.isPending ? (
                  <div className="flex items-center justify-center gap-2 p-8 text-xs text-white/40">
                    <Loader2 className="size-5 animate-spin" /> Loading repositories…
                  </div>
                ) : repos.isError ? (
                  <div className="p-8 text-center">
                    <Github className="mx-auto size-12 text-white/20 mb-4" />
                    <p className="text-sm text-white/50">Unable to load repositories</p>
                    <p className="mt-1 text-xs text-white/30">
                      Make sure GitHub App is installed and has repository access.
                    </p>
                    <Button
                      variant="outline"
                      className="mt-3"
                      onClick={() => repos.refetch()}
                    >
                      Retry
                    </Button>
                  </div>
                ) : filteredRepos.length === 0 ? (
                  <div className="p-8 text-center">
                    <Github className="mx-auto size-12 text-white/20 mb-4" />
                    <p className="text-sm text-white/50">
                      {search ? "No repositories match your search." : "No repositories found."}
                    </p>
                    {search && (
                      <Button
                        variant="ghost"
                        size="sm"
                        onClick={() => setSearch("")}
                        className="mt-2"
                      >
                        <X className="size-3.5 mr-1.5" /> Clear search
                      </Button>
                    )}
                  </div>
                ) : (
                  <div className="grid gap-3 sm:grid-cols-2 lg:grid-cols-3 p-1">
                    {filteredRepos.map((repo) => (
                      <ProjectCard
                        key={repo.id}
                        project={{
                          id: String(repo.id),
                          user_id: "",
                          installation_id: "",
                          github_repository_id: repo.id,
                          repository_owner: repo.owner,
                          repository_name: repo.name,
                          repository_full_name: repo.full_name,
                          configured_branch: `refs/heads/${repo.default_branch}`,
                          project_webhook_enabled: true,
                          repository_available: true,
                          desired_revision_generation: 0,
                          build_configuration: {
                            executable: "npm",
                            args: ["run", "build"],
                            working_dir: ".",
                            scanner_policy_version: "v1",
                          },
                          configuration_version: 0,
                          command_policy_version: "v1",
                          command_scan_result: {
                            status: "pass",
                            policy_version: "v1",
                            source: "auto",
                          },
                          created_at: new Date().toISOString(),
                          updated_at: new Date().toISOString(),
                        }}
                        variant="import"
                        onClick={() => handleSelect(repo)}
                      />
                    ))}
                  </div>
                )}
              </div>

              {selectedRepo && (
                <div className="flex items-center justify-end gap-2 border-t border-white/[0.07] pt-4">
                  <Button variant="ghost" onClick={() => onOpenChange(false)}>
                    Cancel
                  </Button>
                  <Button
                    onClick={() => onOpenChange(false)}
                    className="gap-2"
                  >
                    Configure & Deploy
                    <ChevronRight className="size-3.5" />
                  </Button>
                </div>
              )}
            </>
          )}
        </div>
      </DialogContent>
    </Dialog>
  );
}