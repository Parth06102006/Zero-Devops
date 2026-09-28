import type { ReactNode } from "react";

import { AppShell } from "@/components/shared/app-shell";
import { AuthGuard } from "@/features/auth";

export default function AppLayout({ children }: { children: ReactNode }) {
  return (
    <AuthGuard>
      <AppShell>{children}</AppShell>
    </AuthGuard>
  );
}
