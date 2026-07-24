import { endpoints } from "@/lib/api/endpoints";
import { httpClient } from "@/lib/api/http-client";
import { env } from "@/lib/config/env";
import type { ApiSuccess } from "@/types/api";
import type { CurrentUser } from "@/types/auth";

import { mapCurrentUser } from "./auth-mapper";
import type { CurrentUserDto } from "../types/dto";

/**
 * Builds the URL the browser should be sent to in order to start the GitHub
 * OAuth handshake. This is a full page navigation (not an XHR/fetch call)
 * because the backend needs to redirect the browser to GitHub and back.
 *
 * BACKEND INTEGRATION POINT: adjust the query param name/shape once the
 * backend's OAuth entrypoint contract is finalized (e.g. it may want a
 * `redirect_to` or a signed `state` param instead of `return_to`).
 */
export function getGithubLoginUrl(returnTo = "/dashboard"): string {
  const url = new URL(endpoints.auth.githubLogin, env.NEXT_PUBLIC_API_URL);
  url.searchParams.set("return_to", new URL(returnTo, env.NEXT_PUBLIC_APP_URL).toString());
  return url.toString();
}

export async function fetchCurrentUser(signal?: AbortSignal): Promise<CurrentUser> {
  const { data } = await httpClient.get<ApiSuccess<CurrentUserDto>>(endpoints.auth.me, {
    ...(signal ? { signal } : {}),
  });
  return mapCurrentUser(data.data);
}

export async function logout(): Promise<void> {
  await httpClient.post(endpoints.auth.logout);
}
