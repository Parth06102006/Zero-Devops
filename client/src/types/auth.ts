export type AuthProvider = "github";

export interface AuthUser {
  id: string;
  displayName: string;
  username: string;
  email: string | null;
  avatarUrl: string | null;
  provider: AuthProvider;
}

export interface CurrentUser {
  user: AuthUser;
}

export type AuthStatus = "idle" | "authenticated" | "unauthenticated" | "loading";
