"use client";

import { Loader2 } from "lucide-react";

import { Button, type ButtonProps } from "@/components/ui/button";
import { GithubIcon } from "@/components/shared/icons";

import { useLogin } from "../hooks/use-login";

interface GithubLoginButtonProps extends Omit<ButtonProps, "onClick"> {
  returnTo?: string | undefined;
  label?: string;
}

export function GithubLoginButton({
  returnTo,
  label = "Continue with GitHub",
  variant = "default",
  size = "lg",
  className,
  ...props
}: GithubLoginButtonProps) {
  const { loginWithGithub, isRedirecting } = useLogin();

  return (
    <Button
      variant={variant}
      size={size}
      className={className}
      onClick={() => loginWithGithub(returnTo)}
      disabled={isRedirecting}
      {...props}
    >
      {isRedirecting ? <Loader2 className="animate-spin" /> : <GithubIcon />}
      {isRedirecting ? "Redirecting…" : label}
    </Button>
  );
}
