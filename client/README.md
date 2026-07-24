# Zero DevOps — Frontend

## Stack

Next.js 15 · React 19 · TypeScript (strictest config) · Tailwind CSS ·
shadcn/ui (New York) · Radix UI · Zustand · TanStack Query · React Hook Form ·
Zod · Axios · next-themes · Framer Motion · Lucide React · CVA

## Getting started

```bash
cp .env.example .env.local
npm install
npm run dev
```

Open http://localhost:3000.

`npm run build`, `npm run lint`, and `npm run typecheck` are all expected to
pass clean on a fresh checkout.

## Architecture

```
src/
  app/                      Routes only. No business logic lives here.
    (app)/                  Authenticated route group: dashboard, deployments, settings
    login/                  GitHub OAuth login page
    layout.tsx              Root layout: fonts, metadata, AppProviders
    globals.css             Design tokens (CSS variables) + base styles

  features/
    auth/                   Everything auth-related, self-contained
      api/                  HTTP calls + DTO -> domain mappers (backend seam)
      components/           Feature-specific UI (login button, user menu, guard)
      hooks/                TanStack Query wrappers (useCurrentUser, useLogin, useLogout)
      store/                Zustand slice for synchronous auth state
      types/                Raw backend DTOs — the ONLY types that change
                             when the backend contract is finalized
      index.ts               Public barrel export — import from here, not internals

  components/
    ui/                     shadcn/Radix primitives (Button, Dialog, Toast, ...)
    shared/                 Cross-feature components (Logo, Container, AppShell, ...)
    landing/                Marketing page sections

  lib/
    api/                    Axios instance, endpoint constants, query keys/client
    config/                 env.ts (Zod-validated), fonts.ts, site.ts
    providers/              Theme / Query / composed AppProviders
    utils/                  cn() and other pure helpers

  stores/                   App-wide Zustand stores not owned by a single feature
  types/                    Cross-cutting domain + API envelope types
  hooks/                    Cross-feature hooks (currently empty — add as needed)

middleware.ts                Coarse, cookie-presence route protection
```

### Why feature-first

Everything auth-related — API calls, hooks, Zustand state, components — lives
in `features/auth`. Nothing outside that folder imports its internals; they
import from `features/auth` (the barrel `index.ts`). This means the auth
feature can be modified, tested, or even lifted into a shared package without
touching the rest of the app, and the same pattern is what you'd replicate for
`features/deployments`, `features/projects`, etc. as the product grows.

### The backend-contract seam

The backend's real response shapes aren't finalized yet. Rather than let that
uncertainty leak into components, there's a strict boundary:

```
Backend response  →  DTO (features/*/types/dto.ts)
                   →  mapper (features/*/api/*-mapper.ts)
                   →  Domain type (types/*.ts)  ←  everything else depends on this
```

When the backend team finalizes a contract, only the DTO + its mapper need to
change. Components, hooks, and stores never see raw backend shapes.

### Auth model

- **GitHub OAuth only.** `useLogin` redirects the full page to the backend's
  OAuth entrypoint; there is no email/password form and none should be added.
- **Cookies, not tokens.** The backend is assumed to own HttpOnly, SameSite
  session cookies. The frontend never reads, stores, or forwards a bearer
  token — `axios` is configured with `withCredentials: true` and nothing
  else.
- **Two layers of route protection:**
  1. `middleware.ts` does a fast, cookie-*presence* check to avoid flashing
     protected UI before a redirect. This is a UX optimization, not a
     security boundary.
  2. `AuthGuard` (client) calls `/auth/me` via `useCurrentUser` and redirects
     if the session turns out to be invalid. The backend must independently
     reject unauthenticated API requests — the frontend cannot enforce this.
- **401 handling:** the Axios response interceptor coalesces concurrent 401s
  into a single `/auth/refresh` call and retries the original request once.

### State boundaries

- **TanStack Query** owns anything that comes from the network (current user,
  future: projects, deployments, logs). It is the cache and the source of
  truth for server data.
- **Zustand** owns synchronous, client-only state: `auth-store` mirrors
  "am I logged in" for code that can't use a hook (event handlers, other
  stores), `ui-store` holds things like sidebar/nav open state, and
  `toast-store` is an imperative toast queue any layer can push to.

