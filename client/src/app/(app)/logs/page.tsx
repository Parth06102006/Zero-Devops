"use client";

import { FileText, Terminal } from "lucide-react";
import { useProjects } from "@/features/projects";
import { useAllDeployments } from "@/features/deployments";

export default function LogsPage() {
  const projects = useProjects();
  const queries = useAllDeployments(projects.data ?? []);
  const builds = (queries.data ?? []).sort(
    (a, b) => new Date(b.created_at).getTime() - new Date(a.created_at).getTime(),
  );

  return (
    <div className="mx-auto max-w-[1400px] px-4 py-6 sm:px-6 lg:px-8">
      {/* Header */}
      <div className="border-b border-white/[0.07] pb-6">
        <p className="text-xs text-white/35">Observability</p>
        <h1 className="mt-1 text-2xl font-semibold">Logs</h1>
        <p className="mt-1 text-sm text-white/35">Only real backend build output is shown here.</p>
      </div>

      {/* Logs Grid */}
      <div className="mt-5 grid gap-3 lg:grid-cols-2">
        {builds.length ? (
          builds.map((b) => (
            <div
              key={b.id}
              className="overflow-hidden rounded-2xl border border-white/[0.08] bg-[#070707]"
            >
              <div className="flex items-center gap-2 border-b border-white/[0.07] px-4 py-3">
                <Terminal className="size-3.5 text-white/30" />
                <span className="font-mono text-[10px] text-white/35">
                  build-{b.build_number ?? "unknown"}
                </span>
                <span className="ml-auto text-[10px] text-white/25">{b.status}</span>
              </div>
              <pre className="min-h-28 whitespace-pre-wrap p-4 font-mono text-[10px] leading-5 text-white/40">
                {b.error_message
                  ? b.error_message
                  : b.output_url
                    ? `deployment output: ${b.output_url}`
                    : "Build logs are not available yet."}
              </pre>
            </div>
          ))
        ) : (
          <div className="rounded-2xl border border-dashed border-white/10 p-16 text-center lg:col-span-2">
            <FileText className="mx-auto size-7 text-white/20" />
            <p className="mt-3 text-sm text-white/45">No build logs available.</p>
            <p className="mt-1 text-xs text-white/25">
              Logs will appear when the backend exposes build output.
            </p>
          </div>
        )}
      </div>
    </div>
  );
}
