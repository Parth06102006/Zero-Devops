export const endpoints = {
  auth: {
    githubLogin: "/auth/github/login",
    login: (returnTo?: string) =>
      returnTo ? `/auth/github/login?return_to=${encodeURIComponent(returnTo)}` : "/auth/github/login",
    logout: "/auth/logout",
    refresh: "/auth/refresh",
    me: "/auth/user/me",
  },
  projects: {
    list: "/projects",
    create: "/projects",
    byId: (id: string) => `/projects/${id}`,
    update: (id: string) => `/projects/${id}`,
    delete: (id: string) => `/projects/${id}`,
    builds: (id: string) => `/projects/${id}/builds`,
    triggerBuild: (id: string) => `/projects/${id}/builds`,
  },
  builds: {
    byId: (id: string) => `/builds/${id}`,
  },
  health: {
    live: "/health/live",
    ready: "/health/ready",
    status: "/health",
  },
  github: {
    repositories: "/integrations/scm/github/repositories",
    repositoriesQuery: (params?: { query?: string; cursor?: string; per_page?: number }) => {
      const search = new URLSearchParams();
      if (params?.query) search.set("query", params.query);
      if (params?.cursor) search.set("cursor", params.cursor);
      if (params?.per_page) search.set("per_page", String(params.per_page));
      const qs = search.toString();
      return qs ? `/integrations/scm/github/repositories?${qs}` : "/integrations/scm/github/repositories";
    },
    installation: "/integrations/scm/github/installation",
    installations: "/integrations/scm/github/installations",
    installCallback: "/github/install/callback",
    startInstall: (returnTo?: string) =>
      returnTo ? `/integrations/scm/github/install?return_to=${encodeURIComponent(returnTo)}` : "/integrations/scm/github/install",
    install: (code?: string) =>
      code ? `/integrations/scm/github/installation?code=${encodeURIComponent(code)}` : "/integrations/scm/github/installation",
    directories: (repoId: number, branch?: string) => {
      const qs = branch ? `?branch=${encodeURIComponent(branch)}` : "";
      return `/integrations/scm/github/repositories/${repoId}/directories${qs}`;
    },
    deleteInstallation: "/integrations/scm/github/installation",
  },
} as const;
