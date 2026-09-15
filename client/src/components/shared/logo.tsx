import Link from "next/link";
import Image from "next/image";

import { cn } from "@/lib/utils/cn";

export function Logo({ className }: { className?: string }) {
  return (
    <Link href="/" className={cn("group flex items-center gap-2.5 font-semibold text-foreground", className)}>
      <span className="flex size-8 items-center justify-center rounded-md border border-primary/50 bg-primary/10 text-primary transition-transform duration-200 group-hover:rotate-6">
        <Image src="/logo.svg" alt="" width={22} height={22} className="size-5 shrink-0" aria-hidden />
      </span>
      <span className="text-[17px] tracking-[-0.02em]">ghost</span>
    </Link>
  );
}
