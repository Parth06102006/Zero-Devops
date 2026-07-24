import { GitCommitHorizontal } from "lucide-react";
import type { Metadata } from "next";

import { EmptyState } from "@/components/shared/empty-state";

export const metadata: Metadata = { title: "Deployments" };

export default function DeploymentsPage() {
  return (
    <div className="flex flex-col gap-8">
      <div>
        <h1 className="text-2xl font-semibold text-foreground">Deployments</h1>
        <p className="mt-1 text-sm text-muted-foreground">
          Every build and release, across every environment, will be listed here.
        </p>
      </div>

      <EmptyState
        icon={GitCommitHorizontal}
        title="No deployments yet"
        description="Once a project is connected, every push will show up here with its build and release status."
      />
    </div>
  );
}
