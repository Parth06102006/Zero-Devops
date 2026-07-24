import axios, { type AxiosError, type InternalAxiosRequestConfig } from "axios";

import { env } from "@/lib/config/env";
import { ApiError, type ApiErrorBody } from "@/types/api";

export const httpClient = axios.create({
  baseURL: env.NEXT_PUBLIC_API_URL,
  timeout: 15_000,
  withCredentials: true,
  headers: {
    "Content-Type": "application/json",
    Accept: "application/json",
  },
});

interface RetryableRequestConfig extends InternalAxiosRequestConfig {
  _retriedAfterRefresh?: boolean;
}

let refreshPromise: Promise<void> | null = null;

async function refreshSession(): Promise<void> {
  await httpClient.post("/auth/refresh");
}

httpClient.interceptors.response.use(
  (response) => response,
  async (error: AxiosError<ApiErrorBody>) => {
    const originalRequest = error.config as RetryableRequestConfig | undefined;

    const isUnauthorized = error.response?.status === 401;
    const isAuthEndpoint = originalRequest?.url?.startsWith("/auth");

    if (isUnauthorized && originalRequest && !isAuthEndpoint && !originalRequest._retriedAfterRefresh) {
      originalRequest._retriedAfterRefresh = true;

      try {
        // Coalesce concurrent 401s into a single refresh call.
        refreshPromise ??= refreshSession().finally(() => {
          refreshPromise = null;
        });
        await refreshPromise;

        return httpClient(originalRequest);
      } catch {
        return Promise.reject(normalizeError(error));
      }
    }

    return Promise.reject(normalizeError(error));
  },
);

function normalizeError(error: AxiosError<ApiErrorBody>): ApiError {
  const isCancelled: boolean = axios.isCancel(error);

  if (isCancelled) {
    return new ApiError({
      message: "Request was cancelled.",
      code: "REQUEST_CANCELLED",
      status: 0,
    });
  }

  if (!error.response) {
    return new ApiError({
      message: "Network error. Check your connection and try again.",
      code: "NETWORK_ERROR",
      status: 0,
    });
  }

  const body = error.response.data;

  return new ApiError({
    message: body?.message ?? "Something went wrong. Please try again.",
    code: body?.code ?? "UNKNOWN_ERROR",
    status: error.response.status,
    fieldErrors: body?.fieldErrors,
  });
}

/** Convenience helper for creating a cancellable request's AbortController. */
export function createAbortController(): AbortController {
  return new AbortController();
}
