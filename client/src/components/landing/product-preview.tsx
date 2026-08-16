import { Activity, Check, Copy, GitCommitHorizontal, Globe2, Terminal } from "lucide-react";

import { Button } from "@/components/ui/button";
import { DeploymentTimeline } from "@/components/landing/deployment-timeline";

const logs = [
  "clone github.com/acme/web",
  "detected package manager: pnpm",
  "building optimized production bundle",
  "upload complete: 18.4 MB",
] as const;

export function ProductPreview() {
  return (
    <div className="relative w-full max-w-xl">
      <div
        aria-hidden
        className="absolute -inset-6 -z-10 rounded-[2rem] bg-primary/20 blur-3xl"
      />
      <div className="overflow-hidden rounded-xl border border-border bg-card shadow-2xl shadow-black/20">
        <div className="flex flex-wrap items-center justify-between gap-3 border-b border-border bg-surface px-4 py-3">
          <div className="flex items-center gap-2">
            <span className="flex size-8 items-center justify-center rounded-md bg-primary/15 text-primary">
              <Terminal className="size-4" />
            </span>
            <div>
              <p className="text-sm font-medium text-foreground">acme/web</p>
              <p className="font-mono text-xs text-muted-foreground">main / 8f42c9a</p>
            </div>
          </div>
          <span className="inline-flex items-center gap-2 rounded-full border border-success/25 bg-success/10 px-3 py-1 text-xs font-medium text-success">
            <span className="size-1.5 rounded-full bg-success" />
            Live
          </span>
        </div>

        <div className="grid gap-px bg-border md:grid-cols-[1fr_0.9fr]">
          <div className="bg-card p-5">
            <div className="mb-5 grid grid-cols-2 gap-3">
              <div className="rounded-lg border border-border bg-surface p-3">
                <p className="text-xs text-muted-foreground">Deploy time</p>
                <p className="mt-1 font-mono text-lg font-semibold text-foreground">41s</p>
              </div>
              <div className="rounded-lg border border-border bg-surface p-3">
                <p className="text-xs text-muted-foreground">Regions</p>
                <p className="mt-1 font-mono text-lg font-semibold text-foreground">3</p>
              </div>
            </div>

            <DeploymentTimeline />
          </div>

          <div className="flex flex-col bg-card">
            <div className="border-b border-border p-4">
              <div className="flex items-center justify-between gap-3">
                <div className="flex items-center gap-2">
                  <Globe2 className="size-4 text-primary" />
                  <span className="text-sm font-medium text-foreground">Preview URL</span>
                </div>
                <Button variant="ghost" size="icon" aria-label="Copy preview URL">
                  <Copy />
                </Button>
              </div>
              <p className="mt-2 truncate font-mono text-xs text-muted-foreground">
                https://acme-web.zero.dev
              </p>
            </div>

            <div className="flex-1 p-4">
              <div className="mb-3 flex items-center gap-2">
                <Activity className="size-4 text-primary" />
                <span className="text-sm font-medium text-foreground">Build log</span>
              </div>
              <div className="space-y-2 rounded-lg border border-border bg-background/60 p-3 font-mono text-xs text-muted-foreground">
                {logs.map((log) => (
                  <p key={log} className="flex items-start gap-2">
                    <Check className="mt-0.5 size-3 shrink-0 text-success" />
                    <span>{log}</span>
                  </p>
                ))}
              </div>
            </div>

            <div className="border-t border-border p-4">
              <div className="flex items-center gap-2 text-xs text-muted-foreground">
                <GitCommitHorizontal className="size-3.5 text-primary" />
                Rollback point saved automatically
              </div>
            </div>
          </div>
        </div>
      </div>
    </div>
  );
}
