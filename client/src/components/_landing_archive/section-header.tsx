import type { ReactNode } from "react";

import { cn } from "@/lib/utils/cn";

interface SectionHeaderProps {
  eyebrow?: string;
  title: string;
  description?: string;
  align?: "left" | "center";
  children?: ReactNode;
}

export function SectionHeader({
  eyebrow,
  title,
  description,
  align = "left",
  children,
}: SectionHeaderProps) {
  return (
    <div
      className={cn(
        "flex max-w-2xl flex-col gap-4",
        align === "center" && "mx-auto items-center text-center",
      )}
    >
      {eyebrow ? (
        <span className="font-mono text-xs font-medium uppercase tracking-[0.18em] text-primary">
          {eyebrow}
        </span>
      ) : null}
      <div className="flex flex-col gap-3">
        <h2 className="text-balance text-3xl font-semibold tracking-tight text-foreground sm:text-4xl">
          {title}
        </h2>
        {description ? (
          <p className="text-base leading-7 text-muted-foreground">{description}</p>
        ) : null}
      </div>
      {children}
    </div>
  );
}
