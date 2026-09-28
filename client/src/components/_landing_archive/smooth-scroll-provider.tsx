"use client";

import { useEffect, type ReactNode } from "react";

/**
 * Archived Reference Implementation:
 * Originally used the 'lenis' package for smooth scrolling.
 * To use:
 *   pnpm add lenis
 *   import Lenis from "lenis";
 */
export function SmoothScrollProvider({ children }: { children: ReactNode }) {
  useEffect(() => {
    // Reference code:
    // const lenis = new Lenis({ lerp: 0.08, duration: 1.2 });
    // function raf(time: number) {
    //   lenis.raf(time);
    //   requestAnimationFrame(raf);
    // }
    // requestAnimationFrame(raf);
    // return () => lenis.destroy();
  }, []);

  return <>{children}</>;
}
