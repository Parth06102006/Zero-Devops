/**
 * Domain-level auth types. These are what the rest of the application
 * (components, hooks, stores) imports and depends on.
 *
 * They are intentionally decoupled from whatever shape the backend actually
 * returns over the wire — that raw shape lives in
 * `features/auth/types/dto.ts` and is converted into these types by
 * `features/auth/api/auth-mapper.ts`. If the backend contract changes,
 * only the DTO + mapper need to change.
 */

export type AuthProvider = "github";

/** The authenticated person, as the rest of the UI understands them. */
export interface AuthUser {
  id: string;
  displayName: string;
  username: string;
  email: string | null;
  avatarUrl: string | null;
  provider: AuthProvider;
  createdAt: string;
}

/**
 * Token metadata only — never the raw token value. The backend owns secure,
 * HttpOnly, SameSite cookies; the browser (and therefore this codebase)
 * never reads or stores a bearer token. This type exists so the UI can
 * reason about expiry (e.g. "your session expires soon") without ever
 * touching the credential itself.
 */
export interface Tokens {
  expiresAt: string;
  refreshable: boolean;
}

/** A single authenticated session. */
export interface Session {
  id: string;
  createdAt: string;
  tokens: Tokens;
}

/** Response shape after a successful login exchange (OAuth callback). */
export interface AuthResponse {
  user: AuthUser;
  session: Session;
}

/** Response shape of the "who am I" endpoint. */
export interface CurrentUser {
  user: AuthUser;
  session: Session;
}

export type AuthStatus = "idle" | "authenticated" | "unauthenticated" | "loading";
