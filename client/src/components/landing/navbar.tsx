"use client";

import { useState } from "react";
import Link from "next/link";
import { Menu, X } from "lucide-react";
import { Logo } from "@/components/shared/logo";
import { GithubLoginButton } from "@/features/auth";

interface NavbarProps {
  onOpenDemo: () => void;
}

export function Navbar({ onOpenDemo }: NavbarProps) {
  const [menu, setMenu] = useState(false);

  return (
    <header className="relative z-30 border-b border-white/[0.06] bg-[#050505]/65 backdrop-blur-2xl">
      <div className="mx-auto flex h-[72px] max-w-7xl items-center gap-6 px-5 lg:px-8">
        <Logo />
        <div className="hidden flex-1 justify-center md:flex">
          <span className="text-xs text-white/35">
            Infrastructure that disappears behind your code.
          </span>
        </div>
        <nav className="ml-auto hidden items-center gap-2 sm:flex">
          <button
            type="button"
            onClick={onOpenDemo}
            className="rounded-lg px-3 py-2 text-xs text-white/55 hover:bg-white/[0.05] hover:text-white"
          >
            Demo
          </button>
          <Link
            href="/login"
            className="rounded-lg px-3 py-2 text-xs text-white/55 hover:bg-white/[0.05] hover:text-white"
          >
            Login
          </Link>
          <GithubLoginButton
            size="sm"
            label="Sign Up"
            className="h-9 rounded-lg bg-white px-4 text-xs font-semibold text-black hover:bg-white/90"
          />
        </nav>
        <button
          type="button"
          onClick={() => setMenu((value) => !value)}
          className="ml-auto rounded-lg border border-white/10 p-2 sm:hidden"
          aria-label="Menu"
        >
          {menu ? <X className="size-4" /> : <Menu className="size-4" />}
        </button>
      </div>

      {menu ? (
        <div className="border-t border-white/[0.06] px-5 py-4 sm:hidden">
          <div className="flex flex-col gap-2">
            <button
              type="button"
              onClick={() => {
                onOpenDemo();
                setMenu(false);
              }}
              className="rounded-lg px-3 py-3 text-left text-sm text-white/60"
            >
              Demo
            </button>
            <Link
              href="/login"
              className="rounded-lg px-3 py-3 text-sm text-white/60"
            >
              Login
            </Link>
            <GithubLoginButton label="Sign Up with GitHub" className="w-full" />
          </div>
        </div>
      ) : null}
    </header>
  );
}
