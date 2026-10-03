"use client";

import { useMutation, useQuery, useQueryClient } from "@tanstack/react-query";
import { queryKeys } from "@/lib/api/query-keys";
import { createProject, deleteProject, getProject, listProjects, type CreateProjectInput } from "../api/projects-api";
import { createProjectBuild, listProjectBuilds } from "../api/builds-api";

export function useProjects() {
  return useQuery({ queryKey: queryKeys.projects.list(), queryFn: ({ signal }) => listProjects(signal) });
}

export function useProject(id: string) {
  return useQuery({ queryKey: queryKeys.projects.detail(id), queryFn: ({ signal }) => getProject(id, signal), enabled: Boolean(id) });
}

export function useProjectBuilds(id: string) {
  return useQuery({
    queryKey: queryKeys.projects.builds(id),
    queryFn: ({ signal }) => listProjectBuilds(id, signal),
    enabled: Boolean(id),
    refetchInterval: (query) => query.state.data?.some((b) => b.status === "pending" || b.status === "building") ? 3000 : false,
  });
}

export function useCreateProject() {
  const client = useQueryClient();
  return useMutation({ mutationFn: (input: CreateProjectInput) => createProject(input), onSuccess: () => client.invalidateQueries({ queryKey: queryKeys.projects.list() }) });
}

export function useDeleteProject() {
  const client = useQueryClient();
  return useMutation({ mutationFn: deleteProject, onSuccess: () => client.invalidateQueries({ queryKey: queryKeys.projects.list() }) });
}

export function useCreateProjectBuild(projectId: string) {
  const client = useQueryClient();
  return useMutation({ mutationFn: (shaOrRef: string) => createProjectBuild(projectId, shaOrRef), onSuccess: () => client.invalidateQueries({ queryKey: queryKeys.projects.builds(projectId) }) });
}
