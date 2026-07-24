import { create } from "zustand";

import type { AuthStatus, AuthUser } from "@/types/auth";

interface AuthState {
  status: AuthStatus;
  user: AuthUser | null;
}

interface AuthActions {
  setAuthenticated: (user: AuthUser) => void;
  setUnauthenticated: () => void;
  setLoading: () => void;
  reset: () => void;
}

/**
 * TanStack Query owns the *fetching* of the current user (caching, retries,
 * refetch-on-focus, etc). This store exists so synchronous, non-component
 * code (route guards, event handlers, other stores) can read "am I logged
 * in right now" without subscribing to a query. It is kept in sync by
 * `useCurrentUser` and must never be written to from anywhere else.
 */
export const useAuthStore = create<AuthState & AuthActions>((set) => ({
  status: "idle",
  user: null,

  setAuthenticated: (user) => set({ status: "authenticated", user }),
  setUnauthenticated: () => set({ status: "unauthenticated", user: null }),
  setLoading: () => set({ status: "loading" }),
  reset: () => set({ status: "idle", user: null }),
}));

export const selectIsAuthenticated = (state: AuthState): boolean =>
  state.status === "authenticated";
