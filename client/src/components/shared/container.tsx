import type { ReactNode } from "react";

import { cn } from "@/lib/utils/cn";

export function Container({
  children,
  className,
  as: Tag = "div",
}: {
  children: ReactNode;
  className?: string;
  as?: "div" | "section" | "header" | "footer";
}) {
  return <Tag className={cn("mx-auto w-full max-w-6xl px-6", className)}>{children}</Tag>;
}
