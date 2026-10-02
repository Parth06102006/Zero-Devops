"use client";

import { cn } from "@/lib/utils/cn";
import { FRAMEWORK_PRESETS } from "@/features/projects/lib/framework-detector";

interface FrameworkBadgeProps {
  framework: string;
  size?: "sm" | "md" | "lg";
  showIcon?: boolean;
}

const FRAMEWORK_COLORS: Record<string, string> = {
  nextjs: "bg-black text-white border-black/20",
  vite: "bg-amber-400/20 text-amber-300 border-amber-400/30",
  react: "bg-cyan-400/20 text-cyan-300 border-cyan-400/30",
  remix: "bg-fuchsia-500/20 text-fuchsia-400 border-fuchsia-500/30",
  astro: "bg-orange-500/20 text-orange-400 border-orange-500/30",
  sveltekit: "bg-orange-400/20 text-orange-300 border-orange-400/30",
  nuxt: "bg-emerald-400/20 text-emerald-300 border-emerald-400/30",
  gatsby: "bg-violet-500/20 text-violet-400 border-violet-500/30",
  angular: "bg-rose-500/20 text-rose-400 border-rose-500/30",
  static: "bg-gray-500/20 text-gray-300 border-gray-500/30",
  docker: "bg-blue-500/20 text-blue-300 border-blue-500/30",
  python: "bg-blue-600/20 text-blue-300 border-blue-600/30",
  go: "bg-cyan-500/20 text-cyan-300 border-cyan-500/30",
  node: "bg-emerald-500/20 text-emerald-300 border-emerald-500/30",
};

const FRAMEWORK_ICONS: Record<string, string> = {
  nextjs: "▲",
  vite: "⚡",
  react: "⚛",
  remix: "◈",
  astro: "🚀",
  sveltekit: "🧡",
  nuxt: "▲",
  gatsby: "🔥",
  angular: "🅰",
  static: "📄",
  docker: "🐳",
  python: "🐍",
  go: "🐹",
  node: "🟢",
};

const SIZE_CLASSES = {
  sm: "px-1.5 py-0.5 text-[9px]",
  md: "px-2 py-0.5 text-[10px]",
  lg: "px-2.5 py-1 text-[11px]",
};

export function FrameworkBadge({ framework, size = "md", showIcon = true }: FrameworkBadgeProps) {
  const preset = FRAMEWORK_PRESETS[framework] ?? FRAMEWORK_PRESETS.node;
  const colorClass = FRAMEWORK_COLORS[framework] ?? FRAMEWORK_COLORS.node;
  const icon = FRAMEWORK_ICONS[framework] ?? "🟢";
  const displayName = preset?.displayName ?? "Unknown";

  return (
    <span
      className={cn(
        "inline-flex items-center gap-1 rounded-full border font-medium uppercase tracking-wider",
        SIZE_CLASSES[size],
        colorClass,
      )}
    >
      {showIcon && <span aria-hidden="true">{icon}</span>}
      <span className="hidden sm:inline">{displayName}</span>
      <span className="sm:hidden">{icon}</span>
    </span>
  );
}

export function FrameworkSelector({
  value,
  onChange,
  className,
}: {
  value: string;
  onChange: (value: string) => void;
  className?: string;
}) {
  return (
    <select
      value={value}
      onChange={(e) => onChange(e.target.value)}
      className={cn(
        "w-full rounded-lg border border-white/[0.1] bg-white/[0.03] px-3 py-2 text-sm text-white outline-none focus:border-cyan-300/50",
        className,
      )}
    >
      {Object.values(FRAMEWORK_PRESETS).map((preset) => (
        <option key={preset.name} value={preset.name}>
          {preset.displayName}
        </option>
      ))}
    </select>
  );
}