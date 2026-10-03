import { NextResponse, type NextRequest } from "next/server";

const SESSION_COOKIE_NAME = "access_token";
const PROTECTED_PREFIXES = ["/dashboard", "/projects", "/deployments", "/logs", "/environment-variables", "/github", "/settings"];
const AUTH_ONLY_PREFIXES = ["/login"];

function matchesPrefix(pathname: string, prefixes: string[]): boolean {
  return prefixes.some((prefix) => pathname === prefix || pathname.startsWith(`${prefix}/`));
}

export function middleware(request: NextRequest) {
  const hasSessionCookie = request.cookies.has(SESSION_COOKIE_NAME);
  const { pathname } = request.nextUrl;
  if (matchesPrefix(pathname, PROTECTED_PREFIXES) && !hasSessionCookie) {
    const loginUrl = new URL("/login", request.url);
    loginUrl.searchParams.set("return_to", pathname);
    return NextResponse.redirect(loginUrl);
  }
  if (matchesPrefix(pathname, AUTH_ONLY_PREFIXES) && hasSessionCookie) return NextResponse.redirect(new URL("/dashboard", request.url));
  return NextResponse.next();
}

export const config = { matcher: ["/dashboard/:path*", "/projects/:path*", "/deployments/:path*", "/logs/:path*", "/environment-variables/:path*", "/github/:path*", "/settings/:path*", "/login"] };
