export interface FrameworkPreset {
  name: string;
  displayName: string;
  icon: string;
  color: string;
  buildCommand: string;
  outputDirectory: string;
  installCommand: string;
  devCommand?: string;
  envExample?: Record<string, string>;
}

export const FRAMEWORK_PRESETS: Record<string, FrameworkPreset> = {
  nextjs: {
    name: "nextjs",
    displayName: "Next.js",
    icon: "▲",
    color: "text-white bg-black",
    buildCommand: "npm run build",
    outputDirectory: ".next",
    installCommand: "npm install",
    devCommand: "npm run dev",
  },
  vite: {
    name: "vite",
    displayName: "Vite",
    icon: "⚡",
    color: "text-black bg-amber-300",
    buildCommand: "npm run build",
    outputDirectory: "dist",
    installCommand: "npm install",
    devCommand: "npm run dev",
  },
  react: {
    name: "react",
    displayName: "Create React App",
    icon: "⚛",
    color: "text-white bg-cyan-400",
    buildCommand: "npm run build",
    outputDirectory: "build",
    installCommand: "npm install",
    devCommand: "npm start",
  },
  remix: {
    name: "remix",
    displayName: "Remix",
    icon: "◈",
    color: "text-white bg-fuchsia-500",
    buildCommand: "npm run build",
    outputDirectory: "build",
    installCommand: "npm install",
    devCommand: "npm run dev",
  },
  astro: {
    name: "astro",
    displayName: "Astro",
    icon: "🚀",
    color: "text-white bg-orange-500",
    buildCommand: "npm run build",
    outputDirectory: "dist",
    installCommand: "npm install",
    devCommand: "npm run dev",
  },
  sveltekit: {
    name: "sveltekit",
    displayName: "SvelteKit",
    icon: "🧡",
    color: "text-white bg-orange-400",
    buildCommand: "npm run build",
    outputDirectory: "build",
    installCommand: "npm install",
    devCommand: "npm run dev",
  },
  nuxt: {
    name: "nuxt",
    displayName: "Nuxt",
    icon: "▲",
    color: "text-white bg-emerald-400",
    buildCommand: "npm run build",
    outputDirectory: ".output",
    installCommand: "npm install",
    devCommand: "npm run dev",
  },
  gatsby: {
    name: "gatsby",
    displayName: "Gatsby",
    icon: "🔥",
    color: "text-white bg-violet-500",
    buildCommand: "npm run build",
    outputDirectory: "public",
    installCommand: "npm install",
    devCommand: "npm run develop",
  },
  angular: {
    name: "angular",
    displayName: "Angular",
    icon: "🅰",
    color: "text-white bg-rose-500",
    buildCommand: "npm run build",
    outputDirectory: "dist",
    installCommand: "npm install",
    devCommand: "npm run start",
  },
  static: {
    name: "static",
    displayName: "Static Site",
    icon: "📄",
    color: "text-white bg-gray-500",
    buildCommand: "echo 'No build command'",
    outputDirectory: ".",
    installCommand: "",
    devCommand: "",
  },
  docker: {
    name: "docker",
    displayName: "Docker",
    icon: "🐳",
    color: "text-white bg-blue-500",
    buildCommand: "docker build -t app .",
    outputDirectory: ".",
    installCommand: "",
    devCommand: "",
  },
  python: {
    name: "python",
    displayName: "Python (FastAPI/Django/Flask)",
    icon: "🐍",
    color: "text-white bg-blue-600",
    buildCommand: "pip install -r requirements.txt",
    outputDirectory: ".",
    installCommand: "pip install -r requirements.txt",
    devCommand: "python main.py",
  },
  go: {
    name: "go",
    displayName: "Go",
    icon: "🐹",
    color: "text-white bg-cyan-500",
    buildCommand: "go build -o app .",
    outputDirectory: ".",
    installCommand: "go mod tidy",
    devCommand: "go run main.go",
  },
  node: {
    name: "node",
    displayName: "Node.js",
    icon: "🟢",
    color: "text-white bg-emerald-500",
    buildCommand: "npm run build",
    outputDirectory: "dist",
    installCommand: "npm install",
    devCommand: "npm run dev",
  },
};

export const FRAMEWORK_LIST = Object.values(FRAMEWORK_PRESETS);

export function detectFramework(repoName: string, repoDescription: string | null): FrameworkPreset | null {
  const text = `${repoName} ${repoDescription ?? ""}`.toLowerCase();

  const patterns: Array<{ keywords: string[]; framework: keyof typeof FRAMEWORK_PRESETS }> = [
    { keywords: ["nextjs", "next-js", "next.js"], framework: "nextjs" },
    { keywords: ["vite"], framework: "vite" },
    { keywords: ["create-react-app", "cra", "react-app"], framework: "react" },
    { keywords: ["remix"], framework: "remix" },
    { keywords: ["astro"], framework: "astro" },
    { keywords: ["sveltekit", "svelte-kit"], framework: "sveltekit" },
    { keywords: ["nuxt", "nuxt3"], framework: "nuxt" },
    { keywords: ["gatsby"], framework: "gatsby" },
    { keywords: ["angular"], framework: "angular" },
    { keywords: ["django"], framework: "python" },
    { keywords: ["fastapi", "flask"], framework: "python" },
    { keywords: ["gin", "echo", "fiber", "go-"], framework: "go" },
    { keywords: ["docker", "dockerfile"], framework: "docker" },
  ];

  for (const { keywords, framework } of patterns) {
    if (keywords.some((k) => text.includes(k))) {
      return FRAMEWORK_PRESETS[framework] ?? null;
    }
  }

  if (text.includes("react")) return FRAMEWORK_PRESETS.react ?? null;
  if (text.includes("vue")) return FRAMEWORK_PRESETS.vite ?? null;
  if (text.includes("node") || text.includes("express")) return FRAMEWORK_PRESETS.node ?? null;

  return null;
}

export function getFrameworkPreset(name: string): FrameworkPreset | undefined {
  return FRAMEWORK_PRESETS[name];
}