package deployments

import (
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"regexp"
	"strings"
)

// Builder describes a detected framework with its config files, dependencies, and Docker template.
// DefaultOutputDir is the conventional build output directory for the framework; it is only
// set for builders whose Dockerfile template serves a static build output directly (see
// resolveOutputDir and the __OUTPUT_DIR__ placeholder in templates/Dockerfile.*.tmpl).
type Builder struct {
	Name             string
	ConfigFiles      []string
	Deps             []string
	Template         string
	DefaultOutputDir string
}

// Named builder variables allow detectFramework to reference specific builders
// (nodeBuilder, pythonBuilder) for fallback cases without map lookups.
var (
	// JS/TS — frameworks with unique config files
	angularBuilder = &Builder{
		Name:             frameworkAngular,
		ConfigFiles:      []string{"angular.json"},
		Deps:             []string{"@angular/core"},
		Template:         "static/Dockerfile.angular.tmpl",
		DefaultOutputDir: outputDirDist,
	}
	nextjsBuilder = &Builder{
		Name:        frameworkNextJS,
		ConfigFiles: []string{"next.config.ts", "next.config.js", "next.config.mjs", "next.config.tsx", "next.config.jsx"},
		Deps:        []string{"next"},
		Template:    "dynamic/Dockerfile.nextjs.tmpl",
	}
	nuxtBuilder = &Builder{
		Name:        frameworkNuxt,
		ConfigFiles: []string{"nuxt.config.ts", "nuxt.config.js", "nuxt.config.mjs"},
		Deps:        []string{"nuxt"},
		Template:    "dynamic/Dockerfile.nuxt.tmpl",
	}
	sveltekitBuilder = &Builder{
		Name:        frameworkSvelteKit,
		ConfigFiles: []string{"svelte.config.ts", "svelte.config.js", "svelte.config.cjs"},
		Deps:        []string{"@sveltejs/kit"},
		Template:    "dynamic/Dockerfile.sveltekit.tmpl",
	}
	remixBuilder = &Builder{
		Name:        frameworkRemix,
		ConfigFiles: []string{"remix.config.js", "remix.config.ts", "remix.vite.config.ts", "remix.vite.config.js"},
		Deps:        []string{"@remix-run/node", "@remix-run/react"},
		Template:    "dynamic/Dockerfile.remix.tmpl",
	}
	gatsbyBuilder = &Builder{
		Name:             frameworkGatsby,
		ConfigFiles:      []string{"gatsby-config.ts", "gatsby-config.js", "gatsby-config.mjs"},
		Deps:             []string{"gatsby"},
		Template:         "static/Dockerfile.gatsby.tmpl",
		DefaultOutputDir: "public",
	}
	astroBuilder = &Builder{
		Name:             frameworkAstro,
		ConfigFiles:      []string{"astro.config.ts", "astro.config.js", "astro.config.mjs"},
		Deps:             []string{frameworkAstro},
		Template:         "static/Dockerfile.astro.tmpl",
		DefaultOutputDir: outputDirDist,
	}
	viteBuilder = &Builder{
		Name:             frameworkVite,
		ConfigFiles:      []string{"vite.config.ts", "vite.config.js", "vite.config.mjs"},
		Deps:             []string{frameworkVite},
		Template:         "static/Dockerfile.vite.tmpl",
		DefaultOutputDir: outputDirDist,
	}
	// tanstackStartBuilder handles TanStack Start — a vite.config.ts-based SSR
	// meta-framework that would otherwise be misdetected as plain static Vite
	// (see viteSSRRecipes / checkSSRMetaFramework). Unlike most Vite SSR
	// frameworks on the refusal list, it builds on Nitro, which respects a
	// NITRO_PRESET env var override at build time — the template forces
	// node-server output regardless of the repo's own committed default
	// (often Cloudflare Workers, which isn't a runnable container at all).
	tanstackStartBuilder = &Builder{
		Name:     frameworkTanStack,
		Template: "dynamic/Dockerfile.tanstack-start.tmpl",
	}

	// JS/TS — dep-only detection (no unique config file)
	svelteBuilder = &Builder{
		Name:             frameworkSvelte,
		Deps:             []string{"svelte"},
		Template:         "static/Dockerfile.svelte.tmpl",
		DefaultOutputDir: outputDirDist,
	}
	vueBuilder = &Builder{
		Name:             frameworkVue,
		Deps:             []string{"vue"},
		Template:         "static/Dockerfile.vue.tmpl",
		DefaultOutputDir: outputDirDist,
	}
	solidBuilder = &Builder{
		Name:             frameworkSolid,
		Deps:             []string{"solid-js"},
		Template:         "static/Dockerfile.solid.tmpl",
		DefaultOutputDir: outputDirDist,
	}
	reactBuilder = &Builder{
		Name:             frameworkReact,
		Deps:             []string{frameworkReact, "react-dom"},
		Template:         "static/Dockerfile.react.tmpl",
		DefaultOutputDir: outputDirBuild,
	}

	// nodeBuilder is the generic JS fallback — not in the builders slice because it has
	// no config files and no deps; detectFramework returns it explicitly.
	nodeBuilder = &Builder{
		Name:     langNode,
		Template: "dynamic/Dockerfile.node.tmpl",
	}

	// Compiled / interpreted languages
	goBuilder = &Builder{
		Name:        langGo,
		ConfigFiles: []string{"go.mod"},
		Template:    "dynamic/Dockerfile.go.tmpl",
	}
	rustBuilder = &Builder{
		Name:        langRust,
		ConfigFiles: []string{"Cargo.toml"},
		Template:    "dynamic/Dockerfile.rust.tmpl",
	}
	dotnetBuilder = &Builder{
		Name:        langDotNet,
		ConfigFiles: []string{"global.json", "Directory.Build.props"},
		Template:    "dynamic/Dockerfile.dotnet.tmpl",
	}
	javaMavenBuilder = &Builder{
		Name:        langJavaMaven,
		ConfigFiles: []string{"pom.xml"},
		Template:    "dynamic/Dockerfile.java.tmpl",
	}
	javaGradleBuilder = &Builder{
		Name:        langJavaGradle,
		ConfigFiles: []string{"build.gradle", "build.gradle.kts"},
		Template:    "dynamic/Dockerfile.java.tmpl",
	}
	rubyBuilder = &Builder{
		Name:        langRuby,
		ConfigFiles: []string{"Gemfile"},
		Template:    "dynamic/Dockerfile.ruby.tmpl",
	}
	phpBuilder = &Builder{
		Name:        langPHP,
		ConfigFiles: []string{"composer.json"},
		Template:    "dynamic/Dockerfile.php.tmpl",
	}
	elixirBuilder = &Builder{
		Name:        langElixir,
		ConfigFiles: []string{"mix.exs"},
		Template:    "dynamic/Dockerfile.elixir.tmpl",
	}
	// pythonBuilder is referenced by detectFramework for the Phase 3 deep-walk fallback.
	pythonBuilder = &Builder{
		Name:        langPython,
		ConfigFiles: []string{"requirements.txt", "pyproject.toml", "Pipfile"},
		Template:    "dynamic/Dockerfile.python.tmpl",
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
	outputDirDist:  true,
	outputDirBuild: true,
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
	Scripts         map[string]string `json:"scripts"`
}

// viteSSRRecipes maps a dependency name to a builder with an actual working
// deployment recipe, checked before viteSSRMetaFrameworkDeps so these bypass the
// refusal below. TanStack Start builds on Nitro (same as Nuxt), which respects a
// NITRO_PRESET env var override at build time — see tanstackStartBuilder.
var viteSSRRecipes = map[string]*Builder{
	"@tanstack/react-start": tanstackStartBuilder,
	"@tanstack/start":       tanstackStartBuilder,
	"@tanstack/solid-start": tanstackStartBuilder,
}

// viteSSRMetaFrameworkDeps maps a dependency name to the framework it indicates —
// present alongside vite.config.* means this isn't plain static Vite, but unlike
// viteSSRRecipes there's no recipe yet, so detection refuses rather than guesses.
// Inherently incomplete (private forks, hand-rolled SSR setups won't appear here);
// resolveOutputDir's index.html check is the real ground truth for everything else.
var viteSSRMetaFrameworkDeps = map[string]string{
	depVike:                 depVike,
	"vite-plugin-ssr":       "vike (vite-plugin-ssr)",
	"@builder.io/qwik-city": "Qwik City",
	"waku":                  "Waku",
}

// astroSSRAdapterDeps indicates Astro is configured for output: 'server' or 'hybrid'.
// An SSR adapter package is only ever installed when Astro needs to run a server —
// never for static output — so its presence is a reliable signal regardless of what
// astro.config.* actually says (which would need a real JS evaluator to parse safely).
var astroSSRAdapterDeps = map[string]string{
	"@astrojs/node":       "Astro (Node adapter)",
	"@astrojs/vercel":     "Astro (Vercel adapter)",
	"@astrojs/cloudflare": "Astro (Cloudflare adapter)",
	"@astrojs/netlify":    "Astro (Netlify adapter)",
	"@astrojs/deno":       "Astro (Deno adapter)",
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

// matchSSRMetaFrameworkDep returns the display name of the first marker dep present in
// pkg, or "" if none match.
func matchSSRMetaFrameworkDep(pkg *packageJSON, markers map[string]string) string {
	for dep, name := range markers {
		if hasDep(pkg, dep) {
			return name
		}
	}
	return ""
}

// checkSSRMetaFramework runs the Layer 1 dependency check for a vite/astro config
// match that could be a disguised SSR meta-framework. Returns (recipeBuilder, "")
// when a known framework WITH a recipe is detected (use recipeBuilder instead of
// b); (nil, name) when a known framework WITHOUT a recipe is detected (refuse);
// (nil, "") otherwise — b is a plain static build.
func checkSSRMetaFramework(repoPath string, b *Builder) (recipeBuilder *Builder, refusalName string) {
	var refusalMarkers map[string]string
	switch b.Name {
	case frameworkVite:
		refusalMarkers = viteSSRMetaFrameworkDeps
	case frameworkAstro:
		refusalMarkers = astroSSRAdapterDeps
	default:
		return nil, ""
	}

	pkg, err := readPackageJSON(repoPath)
	if err != nil || pkg == nil {
		return nil, ""
	}

	if b.Name == frameworkVite {
		for dep, recipeBuilder := range viteSSRRecipes {
			if hasDep(pkg, dep) {
				return recipeBuilder, ""
			}
		}
	}

	return nil, matchSSRMetaFrameworkDep(pkg, refusalMarkers)
}

// hasStartScript reports whether package.json declares a non-empty "start" script —
// real proof a server entrypoint exists, not a guess. Used to decide whether an
// SSR-mismatch build failure is worth retrying through the generic dynamic path.
func hasStartScript(pkg *packageJSON) bool {
	return strings.TrimSpace(pkg.Scripts["start"]) != ""
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
			if !rootFiles[strings.ToLower(cfg)] {
				continue
			}
			if recipeBuilder, refusalName := checkSSRMetaFramework(repoPath, b); recipeBuilder != nil {
				return recipeBuilder, nil
			} else if refusalName != "" {
				return nil, fmt.Errorf(
					"detected %s via %s dependencies — no static or dynamic deployment recipe exists yet for this framework; set deployment_type manually in project settings",
					refusalName, b.Name,
				)
			}
			return b, nil
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

// outDirConfigPattern matches a JS/TS config object's outDir field, e.g. `outDir: "output"`.
// This is a best-effort string scan, not a JS parser — it catches the common static-literal
// case (vite.config.ts / astro.config.ts) without needing to execute the config file.
var outDirConfigPattern = regexp.MustCompile(`outDir\s*:\s*['"]([^'"]+)['"]`)

// buildPathEnvPattern matches Create React App's BUILD_PATH override, e.g. `BUILD_PATH=out`.
var buildPathEnvPattern = regexp.MustCompile(`(?m)^\s*BUILD_PATH\s*=\s*(.+?)\s*$`)

// readConfigOutDir scans builder.ConfigFiles in order and returns the first outDir override
// found. Only the config file that actually exists on disk is read.
func readConfigOutDir(repoPath string, builder *Builder) string {
	for _, cfgName := range builder.ConfigFiles {
		//nolint:gosec // cfgName comes from the builder's own fixed ConfigFiles list, not user input
		data, err := os.ReadFile(filepath.Join(repoPath, cfgName))
		if err != nil {
			continue
		}
		if m := outDirConfigPattern.FindSubmatch(data); m != nil {
			return string(m[1])
		}
	}
	return ""
}

// readBuildPathEnv scans CRA's supported env files, in the precedence order CRA itself uses,
// for a BUILD_PATH override.
func readBuildPathEnv(repoPath string) string {
	for _, envFile := range []string{".env.production.local", ".env.local", ".env.production", ".env"} {
		//nolint:gosec // envFile is a fixed candidate name, not user input
		data, err := os.ReadFile(filepath.Join(repoPath, envFile))
		if err != nil {
			continue
		}
		if m := buildPathEnvPattern.FindSubmatch(data); m != nil {
			return string(m[1])
		}
	}
	return ""
}

// sanitizeOutputDir rejects overrides that would escape the repo root (absolute paths, `..`
// traversal) or resolve to the repo root itself, returning "" for anything unsafe.
func sanitizeOutputDir(dir string) string {
	dir = strings.TrimSpace(dir)
	dir = strings.TrimPrefix(dir, "./")
	dir = filepath.Clean(dir)
	if dir == "." || dir == "" || filepath.IsAbs(dir) || strings.HasPrefix(dir, "..") {
		return ""
	}
	return dir
}

// resolveOutputDir determines the build output directory for a static builder: it checks for
// a framework-specific override (custom outDir in vite/astro config, CRA's BUILD_PATH env var)
// before falling back to the framework's conventional default. Returns "" for builders that
// don't have a DefaultOutputDir (i.e. non-static builders, where this doesn't apply).
func resolveOutputDir(repoPath string, builder *Builder) string {
	if builder.DefaultOutputDir == "" {
		return ""
	}

	var override string
	switch builder.Name {
	case frameworkReact:
		override = readBuildPathEnv(repoPath)
	case frameworkVite, frameworkAstro:
		override = readConfigOutDir(repoPath, builder)
	}

	if clean := sanitizeOutputDir(override); clean != "" {
		return clean
	}
	return builder.DefaultOutputDir
}
