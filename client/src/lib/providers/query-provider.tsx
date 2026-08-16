"use client";

import { QueryClientProvider } from "@tanstack/react-query";
import { ReactQueryDevtools } from "@tanstack/react-query-devtools";
import { type ReactNode, useState } from "react";

import { getQueryClient } from "@/lib/api/query-client";

export function QueryProvider({ children }: { children: ReactNode }) {
  // useState ensures the client is created once per component instance
  // (and once per request on the server, thanks to getQueryClient's
  // isServer branch), never re-created on re-renders.
  const [queryClient] = useState(getQueryClient);

  return (
    <QueryClientProvider client={queryClient}>
      {children}
      {process.env.NODE_ENV === "development" ? (
        <ReactQueryDevtools initialIsOpen={false} buttonPosition="bottom-left" />
      ) : null}
    </QueryClientProvider>
  );
}
