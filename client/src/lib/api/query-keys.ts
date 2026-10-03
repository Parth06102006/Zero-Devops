export const queryKeys = {
  auth: {
    all: ["auth"] as const,
    currentUser: () => [...queryKeys.auth.all, "current-user"] as const,
  },
  projects: {
    all: ["projects"] as const,
    list: () => [...queryKeys.projects.all, "list"] as const,
    detail: (id: string) => [...queryKeys.projects.all, "detail", id] as const,
    builds: (id: string) => [...queryKeys.projects.all, "builds", id] as const,
  },
  deployments: {
    all: ["deployments"] as const,
    list: (ids: string[]) => [...queryKeys.deployments.all, "all", ...ids] as const,
    detail: (id: string) => [...queryKeys.deployments.all, "detail", id] as const,
  },
  github: {
    all: ["github"] as const,
    installation: () => [...queryKeys.github.all, "installation"] as const,
    repositories: (query: string) => [...queryKeys.github.all, "repositories", query] as const,
  },
} as const;
