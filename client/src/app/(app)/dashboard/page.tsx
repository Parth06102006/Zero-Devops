import { Rocket } from "lucide-react";
import type { Metadata } from "next";

import { Button } from "@/components/ui/button";
import { EmptyState } from "@/components/shared/empty-state";

export const metadata: Metadata = { title: "Overview" };

export default function DashboardPage() {
  return (
    <div className="flex flex-col gap-8">
      <div>
        <h1 className="text-2xl font-semibold text-foreground">Overview</h1>
        <p className="mt-1 text-sm text-muted-foreground">
          A summary of your projects and recent deployments will live here.
        </p>
      </div>

      <EmptyState
        icon={Rocket}
        title="No projects yet"
        description="Connect a GitHub repository to see your first deployment appear here."
        action={<Button>Connect a repository</Button>}
      />
    </div>
  );
}
