import { NextResponse, type NextRequest } from "next/server";

/**
 * Must match the HttpOnly access token cookie the backend sets after a
 * successful GitHub OAuth callback (see auth_handler.go). The refresh token
 * lives in a sibling `refresh_token` cookie; the presence of the access
 * token is the coarse "logged in" signal this middleware relies on.
 */
const SESSION_COOKIE_NAME = "access_token";

const PROTECTED_PREFIXES = ["/dashboard", "/deployments", "/settings"];
const AUTH_ONLY_PREFIXES = ["/login"];

function matchesPrefix(pathname: string, prefixes: string[]): boolean {
  return prefixes.some((prefix) => pathname === prefix || pathname.startsWith(`${prefix}/`));
}

/**
 * Middleware only performs a coarse, cookie-presence check — it cannot
 * validate the session (that requires a network round-trip to the backend,
 * which the client does via `useCurrentUser`). This is a UX optimization to
 * avoid flashing protected content before the client redirects, not a
 * security boundary; the backend must independently reject unauthenticated
 * API requests.
 */
export function middleware(request: NextRequest) {
  
  const hasSessionCookie = request.cookies.has(SESSION_COOKIE_NAME);
  const { pathname } = request.nextUrl;

  if (matchesPrefix(pathname, PROTECTED_PREFIXES) && !hasSessionCookie) {
    const loginUrl = new URL("/login", request.url);
    loginUrl.searchParams.set("return_to", pathname);
    return NextResponse.redirect(loginUrl);
  }

  if (matchesPrefix(pathname, AUTH_ONLY_PREFIXES) && hasSessionCookie) {
    return NextResponse.redirect(new URL("/dashboard", request.url));
  }

  return NextResponse.next();
}

export const config = {
  matcher: ["/dashboard/:path*", "/deployments/:path*", "/settings/:path*", "/login"],
};
