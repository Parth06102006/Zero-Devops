import Link from "next/link";

import { cn } from "@/lib/utils/cn";

export function Logo({ className }: { className?: string }) {
  return (
    <Link href="/" className={cn("flex items-center gap-2 font-medium text-foreground", className)}>
      <span className="relative flex size-6 items-center justify-center rounded-md bg-primary">
        <span className="size-2 rounded-sm bg-primary-foreground" />
      </span>
      <span className="text-[15px] tracking-tight">
        Zero<span className="text-muted-foreground">DevOps</span>
      </span>
    </Link>
  );
}
