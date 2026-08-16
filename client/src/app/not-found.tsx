import Link from "next/link";

import { Button } from "@/components/ui/button";

export default function NotFound() {
  return (
    <div className="flex min-h-dvh flex-col items-center justify-center gap-4 bg-background px-6 text-center">
      <p className="font-mono text-sm text-muted-foreground">404</p>
      <h1 className="text-2xl font-semibold text-foreground">This page doesn&apos;t exist</h1>
      <p className="max-w-sm text-sm text-muted-foreground">
        The route you followed may be broken, or the page may have moved.
      </p>
      <Button asChild className="mt-2">
        <Link href="/">Back to home</Link>
      </Button>
    </div>
  );
}
