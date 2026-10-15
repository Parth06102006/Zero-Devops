"use client";

import { useMutation, useQuery, useQueryClient } from "@tanstack/react-query";
import { queryKeys } from "@/lib/api/query-keys";
import type { GithubConnectionStatus } from "@/types/domain";
import {
  getGithubInstallation,
  listGithubInstallations,
  listGithubRepositories,
  listRepositoryDirectories,
  updateGithubConnectionStatus,
} from "../api/github-api";

export function useGithubInstallation() {
  return useQuery({ queryKey: queryKeys.github.installation(), queryFn: ({ signal }) => getGithubInstallation(signal), retry: false });
}

export function useGithubInstallations() {
  return useQuery({
    queryKey: queryKeys.github.installations(),
    queryFn: ({ signal }) => listGithubInstallations(signal),
    retry: false,
  });
}

export function useUpdateGithubConnectionStatus() {
  const client = useQueryClient();
  return useMutation({
    mutationFn: ({ connectionStatus, id }: { connectionStatus: GithubConnectionStatus; id?: string }) =>
      updateGithubConnectionStatus(connectionStatus, id),
    onSuccess: () => client.invalidateQueries({ queryKey: queryKeys.github.all }),
  });
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

export function useGithubDirectories(repositoryId?: number, branch?: string, enabled = true) {
  return useQuery({
    queryKey: queryKeys.github.directories(repositoryId ?? 0, branch),
    queryFn: ({ signal }) => (repositoryId ? listRepositoryDirectories(repositoryId, branch, signal) : Promise.resolve([])),
    staleTime: 60_000,
    enabled: enabled && Boolean(repositoryId),
    retry: false,
  });
}

export function useDeleteGithubInstallation() {
  const client = useQueryClient();
  return useMutation({
    mutationFn: (id?: string | void) => updateGithubConnectionStatus("disconnected", id ?? undefined),
    onSuccess: () => client.invalidateQueries({ queryKey: queryKeys.github.all }),
  });
}


