import { MutationCache, QueryClient, isServer } from "@tanstack/react-query";

import { toast } from "@/stores/toast-store";
import { ApiError } from "@/types/api";

function makeQueryClient(): QueryClient {
  return new QueryClient({
    mutationCache: new MutationCache({
      onError: (error, _variables, _context, mutation) => {
        // Mutations can opt out of the global toast (e.g. to show inline
        // field errors instead) via `meta: { suppressErrorToast: true }`.
        if (mutation.meta?.suppressErrorToast) return;

        toast({
          title: "Something went wrong",
          description: ApiError.isApiError(error) ? error.message : "Please try again.",
          variant: "destructive",
        });
      },
    }),
    defaultOptions: {
      queries: {
        staleTime: 30_000,
        gcTime: 5 * 60_000,
        refetchOnWindowFocus: false,
        retry: (failureCount, error) => {
          if (ApiError.isApiError(error) && error.status >= 400 && error.status < 500) {
            return false;
          }
          return failureCount < 2;
        },
      },
      mutations: {
        retry: false,
      },
    },
  });
}

let browserQueryClient: QueryClient | undefined;

/**
 * On the server we always create a fresh QueryClient per request (avoids
 * leaking data between requests/users). In the browser we reuse a single
 * instance across the app's lifetime.
 */
export function getQueryClient(): QueryClient {
  if (isServer) {
    return makeQueryClient();
  }

  browserQueryClient ??= makeQueryClient();
  return browserQueryClient;
}
