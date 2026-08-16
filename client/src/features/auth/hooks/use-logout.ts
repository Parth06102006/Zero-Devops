"use client";

import { useMutation, useQueryClient } from "@tanstack/react-query";
import { useRouter } from "next/navigation";

import { queryKeys } from "@/lib/api/query-keys";

import { logout } from "../api/auth-api";
import { useAuthStore } from "../store/auth-store";

export function useLogout() {
  const queryClient = useQueryClient();
  const resetAuthStore = useAuthStore((state) => state.reset);
  const router = useRouter();

  return useMutation({
    mutationFn: logout,
    onSuccess: async () => {
      resetAuthStore();
      queryClient.removeQueries({ queryKey: queryKeys.auth.all });
      router.push("/login");
      router.refresh();
    },
  });
}
