/**
 * Public surface of the `auth` feature. Other features/components should
 * import from `@/features/auth`, not reach into internal files directly —
 * this keeps the internal folder structure free to change.
 */
export { useCurrentUser } from "./hooks/use-current-user";
export { useLogin } from "./hooks/use-login";
export { useLogout } from "./hooks/use-logout";
export { useAuthStore, selectIsAuthenticated } from "./store/auth-store";
export { GithubLoginButton } from "./components/github-login-button";
export { UserMenu } from "./components/user-menu";
export { AuthGuard } from "./components/auth-guard";
