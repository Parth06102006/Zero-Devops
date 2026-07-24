/**
 * Centralized query key factories. Every feature appends its own namespace
 * here instead of hand-writing key arrays inline, so invalidation stays
 * consistent as the app grows (e.g. `queryClient.invalidateQueries({ queryKey: queryKeys.auth.all })`).
 */
export const queryKeys = {
  auth: {
    all: ["auth"] as const,
    currentUser: () => [...queryKeys.auth.all, "current-user"] as const,
  },
} as const;
