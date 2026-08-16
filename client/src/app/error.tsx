"use client";

import { useEffect } from "react";

import { Button } from "@/components/ui/button";

export default function RootError({
  error,
  reset,
}: {
  error: Error & { digest?: string };
  reset: () => void;
}) {
  useEffect(() => {
    // BACKEND INTEGRATION POINT: wire this into an error-reporting service
    // (Sentry, etc.) once one is chosen.
    console.error(error);
  }, [error]);

  return (
    <div className="flex min-h-dvh flex-col items-center justify-center gap-4 bg-background px-6 text-center">
      <p className="font-mono text-sm text-destructive">Error</p>
      <h1 className="text-2xl font-semibold text-foreground">Something broke on our end</h1>
      <p className="max-w-sm text-sm text-muted-foreground">
        Try again, and if it keeps happening, let us know what you were doing.
      </p>
      <Button onClick={reset} className="mt-2">
        Try again
      </Button>
    </div>
  );
}
