"use client";

import { useMemo, useState } from "react";
import { Check, ChevronDown, GitBranch, Github, Loader2, Rocket, Search } from "lucide-react";
import {
  Dialog,
  DialogContent,
  DialogDescription,
  DialogHeader,
  DialogTitle,
} from "@/components/ui/dialog";
import { Input } from "@/components/ui/input";
import { Button } from "@/components/ui/button";
import { useGithubRepositories } from "@/features/github";
import { useCreateProject } from "../hooks/use-projects";
import { cn } from "@/lib/utils/cn";

export function NewDeploymentDialog({
  open,
  onOpenChange,
}: {
  open: boolean;
  onOpenChange: (open: boolean) => void;
}) {
  const [search, setSearch] = useState("");
  const [selectedId, setSelectedId] = useState<number | null>(null);
  const [branch, setBranch] = useState("");
  const [command, setCommand] = useState("npm run build");
  const [workingDir, setWorkingDir] = useState(".");
  const repos = useGithubRepositories(search);
  const create = useCreateProject();
  const selected = useMemo(
    () => repos.data?.repositories.find((repo) => repo.id === selectedId),
    [repos.data, selectedId],
  );

  const choose = (id: number, defaultBranch: string) => {
    setSelectedId(id);
    setBranch(defaultBranch);
  };

  const submit = async () => {
    if (!selected || !branch.trim()) return;
    await create.mutateAsync({
      repository_id: selected.id,
      configured_branch: branch.startsWith("refs/heads/") ? branch : `refs/heads/${branch}`,
      project_webhook_enabled: true,
      build_configuration: {
        executable: command.trim().split(/\s+/)[0] ?? "npm",
        args: command.trim().split(/\s+/).slice(1),
        working_dir: workingDir.trim() || ".",
        scanner_policy_version: "v1",
      },
    });
    setSelectedId(null);
    setBranch("");
    setCommand("npm run build");
    setWorkingDir(".");
    onOpenChange(false);
  };

  return (
    <Dialog open={open} onOpenChange={onOpenChange}>
      <DialogContent className="max-w-2xl border-white/10 bg-[#0a0a0a] text-white shadow-2xl">
        <DialogHeader>
          <DialogTitle className="flex items-center gap-2 text-xl">
            <span className="flex size-8 items-center justify-center rounded-lg bg-gradient-to-br from-cyan-400/20 to-fuchsia-500/20">
              <Rocket className="size-4 text-cyan-200" />
            </span>{" "}
            New Deployment
          </DialogTitle>
          <DialogDescription className="text-white/45">
            Select a GitHub repository and configure its first reproducible build.
          </DialogDescription>
        </DialogHeader>

        <div className="grid gap-5">
          <div>
            <label className="mb-2 block text-xs font-medium text-white/60">Repository</label>
            <div className="relative">
              <Search className="pointer-events-none absolute left-3 top-3 size-4 text-white/25" />
              <Input
                value={search}
                onChange={(e) => setSearch(e.target.value)}
                placeholder="Search repositories"
                className="border-white/10 bg-white/[0.03] pl-9 text-white placeholder:text-white/25"
              />
            </div>
            <div className="mt-2 max-h-40 overflow-auto rounded-xl border border-white/[0.08] bg-white/[0.02]">
              {repos.isPending ? (
                <div className="flex items-center gap-2 p-4 text-xs text-white/40">
                  <Loader2 className="size-4 animate-spin" /> Loading repositories…
                </div>
              ) : repos.isError ? (
                <div className="p-4 text-xs leading-5 text-amber-200/70">
                  GitHub repositories are unavailable. Make sure your GitHub App installation is connected.
                </div>
              ) : repos.data?.repositories.length ? (
                repos.data.repositories.map((repo) => (
                  <button
                    key={repo.id}
                    type="button"
                    onClick={() => choose(repo.id, repo.default_branch)}
                    className={cn(
                      "flex w-full items-center gap-3 border-b border-white/[0.06] px-3 py-2.5 text-left last:border-0 hover:bg-white/[0.05]",
                      selectedId === repo.id && "bg-white/[0.07]",
                    )}
                  >
                    <Github className="size-4 text-white/40" />
                    <span className="min-w-0 flex-1">
                      <span className="block truncate text-sm text-white">{repo.full_name}</span>
                      <span className="text-[11px] text-white/35">
                        {repo.private ? "Private" : "Public"} · {repo.default_branch}
                      </span>
                    </span>
                    {selectedId === repo.id ? (
                      <Check className="size-4 text-emerald-300" />
                    ) : (
                      <ChevronDown className="size-3.5 rotate-[-90deg] text-white/20" />
                    )}
                  </button>
                ))
              ) : (
                <div className="p-4 text-xs text-white/35">No repositories found.</div>
              )}
            </div>
          </div>

          <div className="grid gap-4 sm:grid-cols-2">
            <div>
              <label className="mb-2 block text-xs font-medium text-white/60">Branch</label>
              <div className="relative">
                <GitBranch className="pointer-events-none absolute left-3 top-3 size-4 text-white/25" />
                <Input
                  value={branch}
                  onChange={(e) => setBranch(e.target.value)}
                  placeholder="main"
                  className="border-white/10 bg-white/[0.03] pl-9 text-white placeholder:text-white/25"
                />
              </div>
            </div>
            <div>
              <label className="mb-2 block text-xs font-medium text-white/60">Working directory</label>
              <Input
                value={workingDir}
                onChange={(e) => setWorkingDir(e.target.value)}
                className="border-white/10 bg-white/[0.03] text-white"
              />
            </div>
          </div>

          <div>
            <label className="mb-2 block text-xs font-medium text-white/60">Build command</label>
            <Input
              value={command}
              onChange={(e) => setCommand(e.target.value)}
              className="border-white/10 bg-white/[0.03] font-mono text-xs text-white"
            />
          </div>

          <div className="flex items-center justify-end gap-2 border-t border-white/[0.07] pt-4">
            <Button variant="ghost" onClick={() => onOpenChange(false)}>
              Cancel
            </Button>
            <Button
              onClick={submit}
              disabled={!selected || !branch.trim() || create.isPending}
            >
              {create.isPending ? <Loader2 className="animate-spin" /> : <Rocket />} Create project
            </Button>
          </div>
          {create.isError ? (
            <p className="text-xs text-rose-300">{create.error.message}</p>
          ) : null}
        </div>
      </DialogContent>
    </Dialog>
  );
}
