"use client";

import { useQuery } from "@tanstack/react-query";
import { useEffect } from "react";

import { queryKeys } from "@/lib/api/query-keys";
import { ApiError } from "@/types/api";

import { fetchCurrentUser } from "../api/auth-api";
import { useAuthStore } from "../store/auth-store";

/**
 * Source of truth for "who is logged in". Fetches once, caches, and
 * revalidates in the background per TanStack Query's normal lifecycle.
 * A 401 here means "unauthenticated", not an error to surface to the user.
 */
export function useCurrentUser() {
  const setAuthenticated = useAuthStore((state) => state.setAuthenticated);
  const setUnauthenticated = useAuthStore((state) => state.setUnauthenticated);
  const setLoading = useAuthStore((state) => state.setLoading);

  const query = useQuery({
    queryKey: queryKeys.auth.currentUser(),
    queryFn: ({ signal }) => fetchCurrentUser(signal),
    retry: (failureCount, error) => {
      if (ApiError.isApiError(error) && error.status === 401) return false;
      return failureCount < 2;
    },
  });

  useEffect(() => {
    if (query.isPending) {
      setLoading();
      return;
    }

    if (query.isSuccess) {
      setAuthenticated(query.data.user);
      return;
    }

    if (query.isError) {
      setUnauthenticated();
    }
  }, [query.isPending, query.isSuccess, query.isError, query.data, setAuthenticated, setUnauthenticated, setLoading]);

  return query;
}
