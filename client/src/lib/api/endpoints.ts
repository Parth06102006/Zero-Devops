
export const endpoints = {
  auth: {
    githubLogin: "/auth/github/login",
    logout: "/auth/logout",
    refresh: "/auth/refresh",
    me: "/auth/me",
  },
} as const;
