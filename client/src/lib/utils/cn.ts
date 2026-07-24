import { clsx, type ClassValue } from "clsx";
import { twMerge } from "tailwind-merge";

/**
 * Merges conditional class names and resolves conflicting Tailwind utility
 * classes (e.g. "p-2" vs "p-4") deterministically. Use this instead of
 * template-string concatenation anywhere className composition happens.
 */
export function cn(...inputs: ClassValue[]): string {
  return twMerge(clsx(inputs));
}
