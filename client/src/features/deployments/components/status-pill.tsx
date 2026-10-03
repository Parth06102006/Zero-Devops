import { cn } from "@/lib/utils/cn";
import type { DeploymentStatus } from "@/types/domain";

export function StatusPill({ status }: { status: DeploymentStatus }) {
  const normalized = status.toLowerCase();

  const tone =
    normalized === "success"
      ? "bg-emerald-400/10 text-emerald-300 border-emerald-400/20"
      : normalized === "failed"
        ? "bg-rose-400/10 text-rose-300 border-rose-400/20"
        : normalized === "building"
          ? "bg-cyan-400/10 text-cyan-300 border-cyan-400/20"
          : "bg-amber-400/10 text-amber-300 border-amber-400/20";

  const dotTone =
    normalized === "success"
      ? "bg-emerald-300"
      : normalized === "failed"
        ? "bg-rose-300"
        : normalized === "building"
          ? "animate-pulse bg-cyan-300"
          : "bg-amber-300";

  return (
    <span
      className={cn(
        "inline-flex items-center gap-1.5 rounded-full border px-2 py-1 text-[10px] font-medium capitalize",
        tone,
      )}
    >
      <span className={cn("size-1.5 rounded-full", dotTone)} />
      {normalized}
    </span>
  );
}
