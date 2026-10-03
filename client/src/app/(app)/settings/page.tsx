"use client";

import { Github, Palette, UserRound } from "lucide-react";
import { useCurrentUser } from "@/features/auth";

export default function SettingsPage() {
  const { data } = useCurrentUser();

  return (
    <div className="mx-auto max-w-[1100px] px-4 py-6 sm:px-6 lg:px-8">
      {/* Header */}
      <div className="border-b border-white/[0.07] pb-6">
        <p className="text-xs text-white/35">Workspace controls</p>
        <h1 className="mt-1 text-2xl font-semibold">Settings</h1>
        <p className="mt-1 text-sm text-white/35">
          Account and interface settings supported by the current frontend.
        </p>
      </div>

      {/* Settings Grid */}
      <div className="mt-5 grid gap-3 lg:grid-cols-2">
        {/* Account card */}
        <div className="rounded-2xl border border-white/[0.08] bg-white/[0.02] p-5">
          <div className="flex items-center gap-3">
            <span className="flex size-9 items-center justify-center rounded-lg bg-cyan-400/10 text-cyan-200">
              <UserRound className="size-4" />
            </span>
            <div>
              <p className="text-sm font-medium">Account</p>
              <p className="text-xs text-white/30">Authenticated GitHub identity</p>
            </div>
          </div>
          <div className="mt-6 flex items-center gap-3">
            <div className="size-10 overflow-hidden rounded-full border border-white/10 bg-white/[0.05]">
              {data?.user.avatarUrl ? (
                /* eslint-disable-next-line @next/next/no-img-element */
                <img
                  src={data.user.avatarUrl}
                  alt=""
                  className="size-full object-cover"
                />
              ) : null}
            </div>
            <div>
              <p className="text-sm">{data?.user.displayName ?? "—"}</p>
              <p className="text-xs text-white/30">@{data?.user.username ?? "—"}</p>
            </div>
          </div>
        </div>

        {/* Interface card */}
        <div className="rounded-2xl border border-white/[0.08] bg-white/[0.02] p-5">
          <div className="flex items-center gap-3">
            <span className="flex size-9 items-center justify-center rounded-lg bg-fuchsia-400/10 text-fuchsia-200">
              <Palette className="size-4" />
            </span>
            <div>
              <p className="text-sm font-medium">Interface</p>
              <p className="text-xs text-white/30">Zero-DevOps visual system</p>
            </div>
          </div>
          <p className="mt-6 text-sm text-white/50">
            Dark mode and multicolour motion are the default workspace experience.
          </p>
        </div>

        {/* GitHub Authentication info card */}
        <div className="rounded-2xl border border-white/[0.08] bg-white/[0.02] p-5 lg:col-span-2">
          <div className="flex items-center gap-3">
            <span className="flex size-9 items-center justify-center rounded-lg bg-emerald-400/10 text-emerald-200">
              <Github className="size-4" />
            </span>
            <div>
              <p className="text-sm font-medium">GitHub authentication</p>
              <p className="text-xs text-white/30">OAuth session handled by the existing backend</p>
            </div>
          </div>
          <p className="mt-5 text-xs leading-5 text-white/35">
            Zero-DevOps never stores GitHub access tokens in the browser. Session cookies remain owned
            by the backend.
          </p>
        </div>
      </div>
    </div>
  );
}
