import { endpoints } from "@/lib/api/endpoints";
import { httpClient } from "@/lib/api/http-client";
import { env } from "@/lib/config/env";
import type { ApiSuccess } from "@/types/api";
import type { CurrentUser } from "@/types/auth";

import { mapCurrentUser } from "./auth-mapper";
import type { CurrentUserDto } from "../types/dto";

export function getGithubLoginUrl(returnTo = "/dashboard"): string {
  const url = new URL(endpoints.auth.githubLogin, env.NEXT_PUBLIC_API_URL);
  url.searchParams.set("return_to", returnTo);
  return url.toString();
}

export async function fetchCurrentUser(signal?: AbortSignal): Promise<CurrentUser> {
  const { data } = await httpClient.get<ApiSuccess<CurrentUserDto>>(endpoints.auth.me, signal ? { signal } : undefined);
  return mapCurrentUser(data.data);
}

export async function logout(): Promise<void> {
  await httpClient.post(endpoints.auth.logout);
}
