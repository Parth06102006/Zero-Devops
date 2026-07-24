/**
 * Generic infrastructure types for talking to the Zero DevOps backend.
 *
 * The backend contract is not finalized. These types intentionally describe
 * *shapes*, not specific endpoints, so that when real response envelopes are
 * confirmed we only touch `lib/api/http-client.ts` and each feature's
 * `api/*-mapper.ts`, never the components or hooks that consume them.
 */

/** Standard success envelope, assumed until the backend spec says otherwise. */
export interface ApiSuccess<TData> {
  data: TData;
  meta?: Record<string, unknown>;
}

/** Standard error envelope returned by the backend on 4xx/5xx responses. */
export interface ApiErrorBody {
  message: string;
  code: string;
  fieldErrors?: Record<string, string[]>;
}

/** Normalized error shape used throughout the app, regardless of transport. */
export class ApiError extends Error {
  readonly code: string;
  readonly status: number;
  readonly fieldErrors: Record<string, string[]> | undefined;

  constructor(params: {
    message: string;
    code: string;
    status: number;
    fieldErrors?: Record<string, string[]> | undefined;
  }) {
    super(params.message);
    this.name = "ApiError";
    this.code = params.code;
    this.status = params.status;
    this.fieldErrors = params.fieldErrors;
  }

  static isApiError(error: unknown): error is ApiError {
    return error instanceof ApiError;
  }
}

/** Cursor-paginated list envelope, for future list endpoints (deployments, projects, etc.). */
export interface PaginatedResult<TItem> {
  items: TItem[];
  nextCursor: string | null;
  hasMore: boolean;
}
