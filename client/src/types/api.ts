export interface ApiSuccess<TData> {
  success: true;
  message?: string;
  data: TData;
  reqID: string;
}

export interface ApiErrorBody {
  success: false;
  error: {
    code: number;
    message: string;
    debug?: { raw_error?: string; stack?: string; reason?: string; query?: string };
  };
  reqID: string;
}

export class ApiError extends Error {
  readonly code: string;
  readonly status: number;
  readonly fieldErrors: Record<string, string[]> | undefined;

  constructor(params: { message: string; code: string; status: number; fieldErrors?: Record<string, string[]> }) {
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

export interface PaginatedResult<TItem> {
  items: TItem[];
  nextCursor: string | null;
  hasMore: boolean;
}
