"use client";

import { useRouter } from "next/navigation";
import { type ReactNode, useEffect } from "react";

import { Skeleton } from "@/components/ui/skeleton";

import { useCurrentUser } from "../hooks/use-current-user";

/**
 * Middleware handles the fast, cookie-presence redirect. This guard is the
 * second layer: once the client actually confirms (via `/auth/me`) that the
 * session is invalid or expired, it redirects too. Wrap any protected
 * layout in this component.
 */
export function AuthGuard({ children }: { children: ReactNode }) {
  const router = useRouter();
  const { data, isPending, isError } = useCurrentUser();

  useEffect(() => {
    if (isError) {
      router.replace("/login");
    }
  }, [isError, router]);

  if (isPending) {
    return (
      <div className="flex min-h-dvh flex-col gap-4 p-8">
        <Skeleton className="h-8 w-48" />
        <Skeleton className="h-64 w-full" />
      </div>
    );
  }

  if (!data) {
    return null;
  }

  return <>{children}</>;
}
