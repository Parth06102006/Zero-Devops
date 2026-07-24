import Link from "next/link";

import { Container } from "@/components/shared/container";
import { Logo } from "@/components/shared/logo";
import { siteConfig } from "@/lib/config/site";

const columns = [
  {
    title: "Product",
    links: [
      { label: "How it works", href: "#how-it-works", external: false },
      { label: "Pricing", href: "#pricing", external: false },
    ],
  },
  {
    title: "Company",
    links: [
      { label: "GitHub", href: siteConfig.github, external: true },
      { label: "Log in", href: "/login", external: false },
    ],
  },
] as const;

export function Footer() {
  return (
    <footer className="border-t border-border/60 py-12">
      <Container className="flex flex-col gap-10 sm:flex-row sm:justify-between">
        <div className="flex flex-col gap-3">
          <Logo />
          <p className="max-w-xs text-sm text-muted-foreground">{siteConfig.tagline}</p>
        </div>

        <div className="flex gap-16">
          {columns.map((column) => (
            <div key={column.title} className="flex flex-col gap-3">
              <span className="text-sm font-medium text-foreground">{column.title}</span>
              {column.links.map((link) =>
                link.external ? (
                  <a
                    key={link.label}
                    href={link.href}
                    target="_blank"
                    rel="noopener noreferrer"
                    className="text-sm text-muted-foreground transition-colors hover:text-foreground"
                  >
                    {link.label}
                  </a>
                ) : (
                  <Link
                    key={link.label}
                    href={link.href}
                    className="text-sm text-muted-foreground transition-colors hover:text-foreground"
                  >
                    {link.label}
                  </Link>
                ),
              )}
            </div>
          ))}
        </div>
      </Container>

      <Container className="mt-10 flex items-center justify-between border-t border-border/60 pt-6">
        <p className="font-mono text-xs text-muted-foreground">
          © {new Date().getFullYear()} {siteConfig.name}
        </p>
      </Container>
    </footer>
  );
}
