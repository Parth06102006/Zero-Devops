"use client";

import { cn } from "@/lib/utils/cn";
import {
  CheckCircle2,
  AlertCircle,
  Loader2,
  Clock,
  XCircle,
} from "lucide-react";

export type DeploymentStatus = 
  | "pending" 
  | "building" 
  | "success" 
  | "failed" 
  | "canceled" 
  | "queued"
  | string;

interface DeploymentStatusBadgeProps {
  status: DeploymentStatus;
  size?: "sm" | "md" | "lg";
  showIcon?: boolean;
  showLabel?: boolean;
}

type StatusConfig = {
  color: string;
  bg: string;
  border: string;
  icon: React.ComponentType<{ className?: string }>;
  label: string;
};

const STATUS_CONFIG: Record<string, StatusConfig> = {
  pending: { 
    color: "text-amber-300", 
    bg: "bg-amber-400/15", 
    border: "border-amber-400/30", 
    icon: Clock, 
    label: "Pending" 
  },
  queued: { 
    color: "text-amber-300", 
    bg: "bg-amber-400/15", 
    border: "border-amber-400/30", 
    icon: Clock, 
    label: "Queued" 
  },
  building: { 
    color: "text-cyan-300", 
    bg: "bg-cyan-400/15", 
    border: "border-cyan-400/30", 
    icon: Loader2, 
    label: "Building" 
  },
  success: { 
    color: "text-emerald-300", 
    bg: "bg-emerald-400/15", 
    border: "border-emerald-400/30", 
    icon: CheckCircle2, 
    label: "Success" 
  },
  failed: { 
    color: "text-rose-300", 
    bg: "bg-rose-400/15", 
    border: "border-rose-400/30", 
    icon: AlertCircle, 
    label: "Failed" 
  },
  canceled: { 
    color: "text-white/40", 
    bg: "bg-white/5", 
    border: "border-white/10", 
    icon: XCircle, 
    label: "Canceled" 
  },
};

const SIZE_CLASSES = {
  sm: "px-2 py-0.5 text-[10px] gap-1",
  md: "px-2.5 py-1 text-[11px] gap-1.5",
  lg: "px-3 py-1.5 text-sm gap-2",
};

const ICON_SIZES = {
  sm: "size-2.5",
  md: "size-3",
  lg: "size-4",
};

const DEFAULT_CONFIG = STATUS_CONFIG.pending;

function getConfig(status: DeploymentStatus): StatusConfig {
  return (STATUS_CONFIG[status] as StatusConfig) ?? DEFAULT_CONFIG;
}

export function DeploymentStatusBadge({ 
  status, 
  size = "md", 
  showIcon = true, 
  showLabel = true 
}: DeploymentStatusBadgeProps) {
  const config = getConfig(status);
  const Icon = config.icon;

  return (
    <span
      className={cn(
        "inline-flex items-center font-medium rounded-full border uppercase tracking-wider",
        SIZE_CLASSES[size],
        config.color,
        config.bg,
        config.border,
      )}
    >
      {showIcon && <Icon className={cn(ICON_SIZES[size], "shrink-0")} aria-hidden="true" />}
      {showLabel && <span>{config.label}</span>}
    </span>
  );
}

export function DeploymentStatusDot({ status, size = "sm" }: { status: DeploymentStatus; size?: "sm" | "md" | "lg" }) {
  const config = getConfig(status);
  
  const dotSizes = {
    sm: "size-1.5",
    md: "size-2",
    lg: "size-3",
  };

  return (
    <span
      className={cn(
        "inline-block rounded-full",
        dotSizes[size],
        config.color.replace("text-", "bg-"),
      )}
      title={config.label}
    />
  );
}

export function DeploymentStatusPill({ status }: { status: DeploymentStatus }) {
  const config = getConfig(status);
  const Icon = config.icon;

  return (
    <span
      className={cn(
        "inline-flex items-center gap-1.5 rounded-full px-2.5 py-1 text-xs font-medium",
        config.color,
        config.bg,
        config.border,
      )}
    >
      <Icon className="size-3" aria-hidden="true" />
      <span>{config.label}</span>
    </span>
  );
}