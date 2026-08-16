import Link from "next/link";
import Image from "next/image";

import { cn } from "@/lib/utils/cn";

export function Logo({ className }: { className?: string }) {
  return (
    <Link href="/" className={cn("flex items-center font-semibold text-foreground", className)}>
      <Image src="/logo.svg" alt="" width={32} height={32} className="size-14 shrink-0" aria-hidden />
      <span className="text-[17px] tracking-tight">ghost</span>
    </Link>
  );
}
