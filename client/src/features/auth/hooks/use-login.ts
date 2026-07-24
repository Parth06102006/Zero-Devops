"use client";

import { useCallback, useState } from "react";

import { getGithubLoginUrl } from "../api/auth-api";

/**
 * GitHub OAuth is a full-page redirect handshake, not an async mutation —
 * there's no request/response cycle to model with TanStack Query here. This
 * hook just centralizes the redirect so components never build the URL
 * themselves, and tracks a `isRedirecting` flag for button loading states.
 */
export function useLogin() {
  const [isRedirecting, setIsRedirecting] = useState(false);

  const loginWithGithub = useCallback((returnTo?: string) => {
    setIsRedirecting(true);
    window.location.assign(getGithubLoginUrl(returnTo));
  }, []);

  return { loginWithGithub, isRedirecting };
}
