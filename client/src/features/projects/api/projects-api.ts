import { endpoints } from "@/lib/api/endpoints";
import { httpClient } from "@/lib/api/http-client";
import type { ApiSuccess } from "@/types/api";
import type { BuildConfiguration, Project } from "@/types/domain";

export interface CreateProjectInput {
  repository_id: number;
  configured_branch: string;
  project_webhook_enabled: boolean;
  build_configuration: BuildConfiguration;
}

export async function listProjects(signal?: AbortSignal): Promise<Project[]> {
  const { data } = await httpClient.get<ApiSuccess<Project[]>>(endpoints.projects.list, signal ? { signal } : undefined);
  return data.data;
}

export async function getProject(id: string, signal?: AbortSignal): Promise<Project> {
  const { data } = await httpClient.get<ApiSuccess<Project>>(endpoints.projects.byId(id), signal ? { signal } : undefined);
  return data.data;
}

export async function createProject(input: CreateProjectInput): Promise<Project> {
  const { data } = await httpClient.post<ApiSuccess<Project>>(endpoints.projects.create, input);
  return data.data;
}

export async function deleteProject(id: string): Promise<void> {
  await httpClient.delete(endpoints.projects.byId(id));
}
