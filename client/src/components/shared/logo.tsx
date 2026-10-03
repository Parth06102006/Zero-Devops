import Image from "next/image";
import Link from "next/link";
import { cn } from "@/lib/utils/cn";

export function Logo({ className }: { className?: string }) {
  return (
    <Link href="/" className={cn("group flex items-center gap-2.5 font-semibold text-foreground", className)}>
      <span className="relative flex size-8 items-center justify-center overflow-hidden rounded-lg border border-white/15 bg-white/[0.04]">
        <span className="absolute inset-0 bg-gradient-to-br from-fuchsia-500/40 via-cyan-400/20 to-emerald-400/30 opacity-80" />
        <Image src="/logo.svg" alt="" width={22} height={22} className="relative size-5 shrink-0" aria-hidden />
      </span>
      <span className="text-[16px] tracking-[-0.025em]">Zero-DevOps</span>
    </Link>
  );
}
