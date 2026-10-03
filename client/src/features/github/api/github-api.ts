import { endpoints } from "@/lib/api/endpoints";
import { httpClient } from "@/lib/api/http-client";
import { env } from "@/lib/config/env";
import type { ApiSuccess } from "@/types/api";
import type { GithubInstallation, GithubRepositoryList } from "@/types/domain";

export function getGithubAppInstallUrl(returnTo = "/dashboard", userId?: string): string {
  const appSlug = env.NEXT_PUBLIC_GITHUB_APP_SLUG;
  const baseCallbackUrl = `${env.NEXT_PUBLIC_APP_URL}/github/install/callback?return_to=${encodeURIComponent(returnTo)}`;
  const callbackUrl = userId ? `${baseCallbackUrl}&user_id=${encodeURIComponent(userId)}` : baseCallbackUrl;
  return `https://github.com/apps/${appSlug}/installations/new?return_to=${encodeURIComponent(callbackUrl)}`;
}

export async function getGithubInstallation(signal?: AbortSignal): Promise<GithubInstallation> {
  const { data } = await httpClient.get<ApiSuccess<GithubInstallation>>(endpoints.github.installation, signal ? { signal } : undefined);
  return data.data;
}

export async function listGithubRepositories(query = "", signal?: AbortSignal): Promise<GithubRepositoryList> {
  const { data } = await httpClient.get<ApiSuccess<GithubRepositoryList>>(endpoints.github.repositories, { params: { query: query || undefined, per_page: 100 }, ...(signal ? { signal } : {}) });
  return data.data;
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
