import { JetBrains_Mono, Inter } from "next/font/google";

/**
 * Inter carries the UI and display type — set with tight tracking at large
 * sizes in components rather than here. JetBrains Mono is the "utility"
 * face used for anything that represents machine output: deploy logs,
 * commit SHAs, environment values — a deliberate choice for an infra
 * product, not decoration.
 */
export const fontSans = Inter({
  subsets: ["latin"],
  variable: "--font-sans",
  display: "swap",
});

export const fontMono = JetBrains_Mono({
  subsets: ["latin"],
  variable: "--font-mono",
  display: "swap",
  weight: ["400", "500", "600"],
});
