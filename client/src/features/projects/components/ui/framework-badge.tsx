"use client";

import { Check, ChevronDown, Sparkles } from "lucide-react";
import { cn } from "@/lib/utils/cn";
import { FRAMEWORK_PRESETS } from "@/features/projects/lib/framework-detector";
import {
  DropdownMenu,
  DropdownMenuContent,
  DropdownMenuItem,
  DropdownMenuLabel,
  DropdownMenuSeparator,
  DropdownMenuTrigger,
} from "@/components/ui/dropdown-menu";

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

const POPULAR_FRAMEWORKS = ["nextjs", "vite", "react", "node", "python", "go", "docker"];

export function FrameworkSelector({
  value,
  onChange,
  className,
}: {
  value: string;
  onChange: (value: string) => void;
  className?: string;
}) {
  const currentPreset = FRAMEWORK_PRESETS[value] ?? FRAMEWORK_PRESETS.node;
  const currentIcon = FRAMEWORK_ICONS[value] ?? "📦";

  return (
    <div className={cn("space-y-3", className)}>
      <div className="flex items-center gap-2">
        <DropdownMenu>
          <DropdownMenuTrigger asChild>
            <button
              type="button"
              className="flex w-full items-center justify-between gap-3 rounded-xl border border-white/[0.1] bg-white/[0.03] px-3.5 py-2.5 text-left text-sm text-white transition-all hover:border-cyan-400/40 hover:bg-white/[0.05] focus:outline-none focus:ring-2 focus:ring-cyan-400/20"
            >
              <div className="flex items-center gap-2.5 min-w-0">
                <span className="flex size-7 shrink-0 items-center justify-center rounded-lg bg-white/[0.08] text-base">
                  {currentIcon}
                </span>
                <div className="min-w-0">
                  <div className="flex items-center gap-2">
                    <span className="font-medium text-white truncate">{currentPreset?.displayName ?? "Custom"}</span>
                    <span className="text-[10px] text-white/40 font-mono truncate hidden sm:inline">
                      {currentPreset?.buildCommand ?? "—"}
                    </span>
                  </div>
                </div>
              </div>
              <ChevronDown className="size-4 shrink-0 text-white/40" />
            </button>
          </DropdownMenuTrigger>
          <DropdownMenuContent
            align="start"
            className="w-[calc(100vw-3rem)] sm:w-[420px] max-h-[380px] overflow-y-auto rounded-xl border-white/10 bg-[#121214] p-1.5 shadow-2xl backdrop-blur-xl"
          >
            <DropdownMenuLabel className="flex items-center gap-1.5 px-2 py-1.5 text-xs font-semibold text-white/50">
              <Sparkles className="size-3 text-cyan-300" />
              Available Framework Presets
            </DropdownMenuLabel>
            <DropdownMenuSeparator className="bg-white/[0.08]" />
            <div className="grid grid-cols-1 sm:grid-cols-2 gap-1 py-1">
              {Object.values(FRAMEWORK_PRESETS).map((preset) => {
                const isSelected = preset.name === value;
                const icon = FRAMEWORK_ICONS[preset.name] ?? "📦";
                return (
                  <DropdownMenuItem
                    key={preset.name}
                    onClick={() => onChange(preset.name)}
                    className={cn(
                      "flex items-center justify-between gap-2 rounded-lg px-2.5 py-2 text-xs transition-colors cursor-pointer",
                      isSelected
                        ? "bg-cyan-500/15 text-cyan-200 border border-cyan-400/30"
                        : "text-white/75 hover:bg-white/[0.06] hover:text-white"
                    )}
                  >
                    <div className="flex items-center gap-2 min-w-0">
                      <span className="text-sm shrink-0">{icon}</span>
                      <div className="min-w-0">
                        <p className="font-medium truncate">{preset.displayName}</p>
                        <p className="text-[10px] font-mono text-white/35 truncate">{preset.buildCommand}</p>
                      </div>
                    </div>
                    {isSelected && <Check className="size-3.5 shrink-0 text-cyan-300" />}
                  </DropdownMenuItem>
                );
              })}
            </div>
          </DropdownMenuContent>
        </DropdownMenu>
      </div>

      {/* Quick selection pills for top frameworks */}
      <div className="flex flex-wrap items-center gap-1.5">
        <span className="text-[10px] text-white/40 mr-1">Quick pick:</span>
        {POPULAR_FRAMEWORKS.map((fwKey) => {
          const p = FRAMEWORK_PRESETS[fwKey];
          if (!p) return null;
          const isSelected = p.name === value;
          const icon = FRAMEWORK_ICONS[fwKey] ?? "📦";
          return (
            <button
              key={fwKey}
              type="button"
              onClick={() => onChange(fwKey)}
              className={cn(
                "inline-flex items-center gap-1 rounded-md px-2 py-0.5 text-[11px] font-medium transition-all",
                isSelected
                  ? "bg-cyan-400/20 text-cyan-300 border border-cyan-400/40 shadow-sm"
                  : "bg-white/[0.03] text-white/50 border border-white/[0.06] hover:bg-white/[0.07] hover:text-white/80"
              )}
            >
              <span>{icon}</span>
              <span>{p.displayName}</span>
            </button>
          );
        })}
      </div>
    </div>
  );
}