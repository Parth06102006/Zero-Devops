import { z } from "zod";

const publicEnvSchema = z.object({
  NEXT_PUBLIC_API_URL: z.string().url(),
  NEXT_PUBLIC_APP_URL: z.string().url(),
  NEXT_PUBLIC_GITHUB_APP_CLIENT_ID: z.string().default("Iv23ligOZC6N1vG6xobG"),
  NEXT_PUBLIC_GITHUB_APP_SLUG: z.string().default("zero-devoops"),
});
const PUBLIC_API_URL = "https://localhost:8080";
const NEXT_PUBLIC_APP_URL = "http://localhost:3000";

const parsed = publicEnvSchema.safeParse({
  NEXT_PUBLIC_API_URL: process.env.NEXT_PUBLIC_API_URL || PUBLIC_API_URL,
  NEXT_PUBLIC_APP_URL: process.env.NEXT_PUBLIC_APP_URL || NEXT_PUBLIC_APP_URL,
  NEXT_PUBLIC_GITHUB_APP_CLIENT_ID: process.env.NEXT_PUBLIC_GITHUB_APP_CLIENT_ID,
  NEXT_PUBLIC_GITHUB_APP_SLUG: process.env.NEXT_PUBLIC_GITHUB_APP_SLUG,
});

if (!parsed.success) {

  console.error(
    "❌ Invalid environment variables:",
    parsed.error.flatten().fieldErrors,
  );
  throw new Error("Invalid environment variables. See console output above.");
}

export const env = parsed.data;
