"use client";

import { useMutation, useQuery, useQueryClient } from "@tanstack/react-query";
import { queryKeys } from "@/lib/api/query-keys";
import { deleteGithubInstallation, getGithubInstallation, listGithubRepositories } from "../api/github-api";

export function useGithubInstallation() {
  return useQuery({ queryKey: queryKeys.github.installation(), queryFn: ({ signal }) => getGithubInstallation(signal), retry: false });
}

export function useGithubRepositories(search = "", enabled = true) {
  return useQuery({ 
    queryKey: queryKeys.github.repositories(search), 
    queryFn: ({ signal }) => listGithubRepositories(search, signal), 
    staleTime: 30_000,
    enabled,
    retry: false,
  });
}

export function useDeleteGithubInstallation() {
  const client = useQueryClient();
  return useMutation({ mutationFn: deleteGithubInstallation, onSuccess: () => client.invalidateQueries({ queryKey: queryKeys.github.all }) });
}
