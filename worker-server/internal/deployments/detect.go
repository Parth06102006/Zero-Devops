package deployments

import (
	"encoding/json"
	"errors"
	"os"
	"path/filepath"
	"strings"
)

// Builder describes a detected framework with its config files, dependencies, and Docker template.
type Builder struct {
	Name        string
	ConfigFiles []string
	Deps        []string
	Template    string
}

// Named builder variables allow detectFramework to reference specific builders
// (nodeBuilder, pythonBuilder) for fallback cases without map lookups.
var (
	// JS/TS — frameworks with unique config files
	angularBuilder = &Builder{
		Name:        frameworkAngular,
		ConfigFiles: []string{"angular.json"},
		Deps:        []string{"@angular/core"},
		Template:    "Dockerfile.angular.tmpl",
	}
	nextjsBuilder = &Builder{
		Name:        frameworkNextJS,
		ConfigFiles: []string{"next.config.ts", "next.config.js", "next.config.mjs", "next.config.tsx", "next.config.jsx"},
		Deps:        []string{"next"},
		Template:    "Dockerfile.nextjs.tmpl",
	}
	nuxtBuilder = &Builder{
		Name:        frameworkNuxt,
		ConfigFiles: []string{"nuxt.config.ts", "nuxt.config.js", "nuxt.config.mjs"},
		Deps:        []string{"nuxt"},
		Template:    "Dockerfile.nuxt.tmpl",
	}
	sveltekitBuilder = &Builder{
		Name:        frameworkSvelteKit,
		ConfigFiles: []string{"svelte.config.ts", "svelte.config.js", "svelte.config.cjs"},
		Deps:        []string{"@sveltejs/kit"},
		Template:    "Dockerfile.sveltekit.tmpl",
	}
	remixBuilder = &Builder{
		Name:        frameworkRemix,
		ConfigFiles: []string{"remix.config.js", "remix.config.ts", "remix.vite.config.ts", "remix.vite.config.js"},
		Deps:        []string{"@remix-run/node", "@remix-run/react"},
		Template:    "Dockerfile.remix.tmpl",
	}
	gatsbyBuilder = &Builder{
		Name:        frameworkGatsby,
		ConfigFiles: []string{"gatsby-config.ts", "gatsby-config.js", "gatsby-config.mjs"},
		Deps:        []string{"gatsby"},
		Template:    "Dockerfile.gatsby.tmpl",
	}
	astroBuilder = &Builder{
		Name:        frameworkAstro,
		ConfigFiles: []string{"astro.config.ts", "astro.config.js", "astro.config.mjs"},
		Deps:        []string{frameworkAstro},
		Template:    "Dockerfile.astro.tmpl",
	}
	viteBuilder = &Builder{
		Name:        frameworkVite,
		ConfigFiles: []string{"vite.config.ts", "vite.config.js", "vite.config.mjs"},
		Deps:        []string{frameworkVite},
		Template:    "Dockerfile.vite.tmpl",
	}

	// JS/TS — dep-only detection (no unique config file)
	svelteBuilder = &Builder{
		Name:     frameworkSvelte,
		Deps:     []string{"svelte"},
		Template: "Dockerfile.svelte.tmpl",
	}
	vueBuilder = &Builder{
		Name:     frameworkVue,
		Deps:     []string{"vue"},
		Template: "Dockerfile.vue.tmpl",
	}
	solidBuilder = &Builder{
		Name:     frameworkSolid,
		Deps:     []string{"solid-js"},
		Template: "Dockerfile.solid.tmpl",
	}
	reactBuilder = &Builder{
		Name:     frameworkReact,
		Deps:     []string{frameworkReact, "react-dom"},
		Template: "Dockerfile.react.tmpl",
	}

	// nodeBuilder is the generic JS fallback — not in the builders slice because it has
	// no config files and no deps; detectFramework returns it explicitly.
	nodeBuilder = &Builder{
		Name:     langNode,
		Template: "Dockerfile.node.tmpl",
	}

	// Compiled / interpreted languages
	goBuilder = &Builder{
		Name:        langGo,
		ConfigFiles: []string{"go.mod"},
		Template:    "Dockerfile.go.tmpl",
	}
	rustBuilder = &Builder{
		Name:        langRust,
		ConfigFiles: []string{"Cargo.toml"},
		Template:    "Dockerfile.rust.tmpl",
	}
	dotnetBuilder = &Builder{
		Name:        langDotNet,
		ConfigFiles: []string{"global.json", "Directory.Build.props"},
		Template:    "Dockerfile.dotnet.tmpl",
	}
	javaMavenBuilder = &Builder{
		Name:        langJavaMaven,
		ConfigFiles: []string{"pom.xml"},
		Template:    "Dockerfile.java.tmpl",
	}
	javaGradleBuilder = &Builder{
		Name:        langJavaGradle,
		ConfigFiles: []string{"build.gradle", "build.gradle.kts"},
		Template:    "Dockerfile.java.tmpl",
	}
	rubyBuilder = &Builder{
		Name:        langRuby,
		ConfigFiles: []string{"Gemfile"},
		Template:    "Dockerfile.ruby.tmpl",
	}
	phpBuilder = &Builder{
		Name:        langPHP,
		ConfigFiles: []string{"composer.json"},
		Template:    "Dockerfile.php.tmpl",
	}
	elixirBuilder = &Builder{
		Name:        langElixir,
		ConfigFiles: []string{"mix.exs"},
		Template:    "Dockerfile.elixir.tmpl",
	}
	// pythonBuilder is referenced by detectFramework for the Phase 3 deep-walk fallback.
	pythonBuilder = &Builder{
		Name:        langPython,
		ConfigFiles: []string{"requirements.txt", "pyproject.toml", "Pipfile"},
		Template:    "Dockerfile.python.tmpl",
	}
)

// builders is an ordered slice — position determines detection priority.
//
// Ordering rules:
//   - If framework A's npm deps are a superset of framework B's deps, A must come before B.
//   - Example: Next.js projects always have "react" → Next.js must come before React.
//   - Example: Nuxt projects always have "vue"   → Nuxt must come before Vue.
//   - Example: SvelteKit projects have "svelte"  → SvelteKit must come before Svelte.
//
// nodeBuilder is intentionally absent — it is the explicit catch-all for any
// package.json repo that does not match a specific framework.
var builders = []*Builder{
	// JS/TS: config-file-based detection (most reliable) -------------------------
	angularBuilder,   // angular.json           — beats React (@angular/core dep)
	nextjsBuilder,    // next.config.*          — beats React
	nuxtBuilder,      // nuxt.config.*          — beats Vue
	sveltekitBuilder, // svelte.config.*        — beats Svelte
	remixBuilder,     // remix.config.*         — beats React
	gatsbyBuilder,    // gatsby-config.*        — beats React
	astroBuilder,     // astro.config.*
	viteBuilder,      // vite.config.*
	// JS/TS: dep-only detection (no unique config file) --------------------------
	svelteBuilder, // dep: "svelte"    — after SvelteKit
	vueBuilder,    // dep: "vue"       — after Nuxt
	solidBuilder,  // dep: "solid-js"
	reactBuilder,  // dep: "react"     — after Next.js, Gatsby, Remix
	// Compiled / interpreted languages --------------------------------------------
	goBuilder,
	rustBuilder,
	dotnetBuilder,
	javaMavenBuilder,
	javaGradleBuilder,
	rubyBuilder,
	phpBuilder,
	elixirBuilder,
	pythonBuilder, // root-level config checked here; nested configs via Phase 3 walk
}

var ignoredDirs = map[string]bool{
	"node_modules": true,
	".git":         true,
	"dist":         true,
	"build":        true,
	".next":        true,
	".cache":       true,
	"__pycache__":  true,
	"vendor":       true,
	".vercel":      true,
	"target":       true,
}

var packageManagers = []struct {
	lockFile string
	name     string
}{
	{"pnpm-lock.yaml", pkgManagerPNPM},
	{"yarn.lock", pkgManagerYarn},
	{"bun.lock", pkgManagerBun},  // text lockfile (Bun >= 1.1)
	{"bun.lockb", pkgManagerBun}, // binary lockfile (Bun < 1.1)
	{"package-lock.json", pkgManagerNPM},
}

type packageJSON struct {
	Dependencies    map[string]string `json:"dependencies"`
	DevDependencies map[string]string `json:"devDependencies"`
}

// buildRootFileSet reads the top-level directory entries of repoPath once and returns
// a set of lowercased filenames. A single os.ReadDir replaces N os.Stat calls — one
// per config file candidate — so detection cost is O(1) per lookup regardless of how
// many builders are in the slice.
func buildRootFileSet(repoPath string) map[string]bool {
	entries, err := os.ReadDir(repoPath)
	if err != nil {
		return map[string]bool{}
	}
	set := make(map[string]bool, len(entries))
	for _, e := range entries {
		if !e.IsDir() {
			set[strings.ToLower(e.Name())] = true
		}
	}
	return set
}

// readPackageJSON reads and parses the package.json file in repoPath.
func readPackageJSON(repoPath string) (*packageJSON, error) {
	//nolint:gosec // path is constructed from a controlled repoPath, not user input
	data, err := os.ReadFile(filepath.Join(repoPath, "package.json"))
	if err != nil {
		return nil, err
	}
	var pkg packageJSON
	if err := json.Unmarshal(data, &pkg); err != nil {
		return nil, err
	}
	return &pkg, nil
}

// hasDep reports whether dep appears in dependencies or devDependencies.
// Nil map lookup in Go is safe and returns false, so no nil guards are needed.
func hasDep(pkg *packageJSON, dep string) bool {
	if _, ok := pkg.Dependencies[dep]; ok {
		return true
	}
	if _, ok := pkg.DevDependencies[dep]; ok {
		return true
	}
	return false
}

// hasConfigFile reports whether configFile exists under repoPath and is a regular file.
// Used by detectPackageManager; detectFramework prefers buildRootFileSet instead.
func hasConfigFile(repoPath, configFile string) bool {
	info, err := os.Stat(filepath.Join(repoPath, configFile))
	return err == nil && !info.IsDir()
}

// walkForFile searches root and its subdirectories for a file named filename.
// Traversal skips ignoredDirs; matching is case-insensitive.
func walkForFile(root, filename string) (bool, error) {
	var found bool
	err := filepath.WalkDir(root, func(_ string, d os.DirEntry, err error) error {
		if err != nil {
			return nil
		}
		if d.IsDir() && ignoredDirs[d.Name()] {
			return filepath.SkipDir
		}
		if !d.IsDir() && strings.EqualFold(d.Name(), filename) {
			found = true
			return filepath.SkipAll
		}
		return nil
	})
	return found, err
}

// detectFramework identifies the framework or language of a repository.
// Detection runs in four ordered phases, returning on the first match:
//
//  1. Config-file phase: looks for well-known config files at the repo root via a
//     single os.ReadDir call. Builders in the slice are checked in priority order,
//     so the result is deterministic regardless of map iteration.
//
//  2. Package.json dep phase: for JS/TS repos without a config-file match, scans
//     declared dependencies. Builder order in the slice determines precedence
//     (e.g. Next.js before React).
//
//  3. Python deep-walk phase: walks the directory tree for Python config files that
//     may live in a subdirectory (common in monorepos).
//
//  4. Dockerfile fallback: returns a bare Builder when a Dockerfile is present at root.
func detectFramework(repoPath string) (*Builder, error) {
	// Phase 1: one ReadDir → O(1) per-file checks, deterministic order.
	rootFiles := buildRootFileSet(repoPath)

	for _, b := range builders {
		for _, cfg := range b.ConfigFiles {
			if rootFiles[strings.ToLower(cfg)] {
				return b, nil
			}
		}
	}

	// Phase 2: package.json dependency scan (JS/TS only).
	if rootFiles["package.json"] {
		pkg, err := readPackageJSON(repoPath)
		if err == nil && pkg != nil {
			for _, b := range builders {
				if b.Deps == nil {
					continue
				}
				for _, dep := range b.Deps {
					if hasDep(pkg, dep) {
						return b, nil
					}
				}
			}
			// package.json exists with declared deps but no specific framework matched.
			if pkg.Dependencies != nil || pkg.DevDependencies != nil {
				return nodeBuilder, nil
			}
		}
	}

	// Phase 3: Python deep-walk for configs not at the repo root.
	for _, cfg := range pythonBuilder.ConfigFiles {
		found, err := walkForFile(repoPath, cfg)
		if err != nil {
			// Non-fatal: permission errors on some directories should not block detection.
			continue
		}
		if found {
			return pythonBuilder, nil
		}
	}

	// Phase 4: bare Dockerfile fallback.
	if rootFiles[strings.ToLower(templateDockerfile)] {
		return &Builder{Name: builderDocker, Template: templateDockerfile}, nil
	}

	return nil, errors.New("no framework detected")
}

// detectPackageManager returns the package manager inferred from lockfile presence.
// Order matters: pnpm > yarn > bun > npm. npm is the default when no lockfile is found.
func detectPackageManager(repoPath string) string {
	for _, pm := range packageManagers {
		if hasConfigFile(repoPath, pm.lockFile) {
			return pm.name
		}
	}
	return pkgManagerNPM
}
