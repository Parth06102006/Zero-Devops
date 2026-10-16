import { endpoints } from "@/lib/api/endpoints";
import { httpClient } from "@/lib/api/http-client";
import { env } from "@/lib/config/env";
import type { ApiSuccess } from "@/types/api";
import type { GithubConnectionStatus, GithubInstallation, GithubRepositoryList } from "@/types/domain";

export function getGithubAppInstallUrl(returnTo = "/dashboard", _userId?: string): string {
  return `${env.NEXT_PUBLIC_API_URL}${endpoints.github.startInstall(returnTo)}`;
}

export async function getGithubInstallation(signal?: AbortSignal): Promise<GithubInstallation> {
  const { data } = await httpClient.get<ApiSuccess<GithubInstallation>>(endpoints.github.installation, signal ? { signal } : undefined);
  return data.data;
}

export async function listGithubInstallations(signal?: AbortSignal): Promise<GithubInstallation[]> {
  const { data } = await httpClient.get<ApiSuccess<GithubInstallation[]>>(endpoints.github.installations, signal ? { signal } : undefined);
  return data.data ?? [];
}

export async function updateGithubConnectionStatus(
  connectionStatus: GithubConnectionStatus,
  id?: string
): Promise<GithubInstallation> {
  const payload: { connection_status: GithubConnectionStatus; id?: string } = {
    connection_status: connectionStatus,
  };
  if (id) {
    payload.id = id;
  }
  const { data } = await httpClient.patch<ApiSuccess<GithubInstallation>>(endpoints.github.installation, payload);
  return data.data;
}

export async function listGithubRepositories(query = "", signal?: AbortSignal): Promise<GithubRepositoryList> {
  const { data } = await httpClient.get<ApiSuccess<GithubRepositoryList>>(endpoints.github.repositories, { params: { query: query || undefined, per_page: 100 }, ...(signal ? { signal } : {}) });
  return data.data;
}

export async function listRepositoryDirectories(repositoryId: number, branch?: string, signal?: AbortSignal): Promise<string[]> {
  const { data } = await httpClient.get<ApiSuccess<string[]>>(
    endpoints.github.directories(repositoryId, branch),
    signal ? { signal } : undefined
  );
  return data.data ?? [];
}

export async function installGithubApp(code: string): Promise<void> {
  try {
    await httpClient.post(endpoints.github.install(code));
  } catch (err: unknown) {
    const error = err as { response?: { data?: { message?: string } }; message?: string };
    const message = error?.response?.data?.message || error?.message || "Failed to install GitHub App";
    throw new Error(message);
  }
}

export async function deleteGithubInstallation(): Promise<void> {
  await httpClient.delete(endpoints.github.deleteInstallation);
}

