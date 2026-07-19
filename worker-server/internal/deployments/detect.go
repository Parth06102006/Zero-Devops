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

var builders = map[string]*Builder{
	frameworkVite: {
		Name: frameworkVite,
		ConfigFiles: []string{
			"vite.config.ts", "vite.config.js", "vite.config.mjs",
		},
		Deps:     []string{frameworkVite},
		Template: "Dockerfile.vite.tmpl",
	},
	frameworkNextJS: {
		Name: frameworkNextJS,
		ConfigFiles: []string{
			"next.config.ts", "next.config.js", "next.config.mjs",
			"next.config.tsx", "next.config.jsx",
		},
		Deps:     []string{"next"},
		Template: "Dockerfile.nextjs.tmpl",
	},
	frameworkAstro: {
		Name: frameworkAstro,
		ConfigFiles: []string{
			"astro.config.ts", "astro.config.js", "astro.config.mjs",
		},
		Deps:     []string{frameworkAstro},
		Template: "Dockerfile.astro.tmpl",
	},
	frameworkReact: {
		Name:        frameworkReact,
		ConfigFiles: nil,
		Deps:        []string{frameworkReact, "react-dom"},
		Template:    "Dockerfile.react.tmpl",
	},
	langNode: {
		Name:        langNode,
		ConfigFiles: nil,
		Deps:        nil,
		Template:    "Dockerfile.node.tmpl",
	},
	langGo: {
		Name:        langGo,
		ConfigFiles: []string{"go.mod"},
		Deps:        nil,
		Template:    "Dockerfile.go.tmpl",
	},
	langPython: {
		Name: langPython,
		ConfigFiles: []string{
			"requirements.txt", "pyproject.toml", "Pipfile",
		},
		Deps:     nil,
		Template: "Dockerfile.python.tmpl",
	},
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
	{"bun.lockb", pkgManagerBun},
	{"package-lock.json", pkgManagerNPM},
}

type packageJSON struct {
	Dependencies    map[string]string `json:"dependencies"`
	DevDependencies map[string]string `json:"devDependencies"`
}

// readPackageJSON reads and parses the package.json file in repoPath.
// It returns the parsed package data or an error if the file cannot be read or parsed.
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

// hasDep reports whether dep is listed in the package's dependencies or development dependencies.
func hasDep(pkg *packageJSON, dep string) bool {
	if pkg.Dependencies != nil {
		if _, ok := pkg.Dependencies[dep]; ok {
			return true
		}
	}
	if pkg.DevDependencies != nil {
		if _, ok := pkg.DevDependencies[dep]; ok {
			return true
		}
	}
	return false
}

// hasConfigFile reports whether configFile exists under repoPath and is a regular file.
func hasConfigFile(repoPath, configFile string) bool {
	info, err := os.Stat(filepath.Join(repoPath, configFile))
	return err == nil && !info.IsDir()
}

// walkForFile searches root and its traversable subdirectories for a file with the specified name.
// Directory traversal skips ignored directories, and filename matching is case-insensitive.
// It returns whether a matching file was found and any error reported by the directory walk.
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

// detectFramework identifies the framework or language associated with a repository.
// It returns the matching builder, or an error if no supported framework is detected.
func detectFramework(repoPath string) (*Builder, error) {
	for _, b := range builders {
		for _, cfg := range b.ConfigFiles {
			if hasConfigFile(repoPath, cfg) {
				return b, nil
			}
		}
	}

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
		if pkg.Dependencies != nil || pkg.DevDependencies != nil {
			return builders[langNode], nil
		}
	}

	if hasConfigFile(repoPath, "go.mod") {
		return builders[langGo], nil
	}

	for _, cfg := range builders[langPython].ConfigFiles {
		ok, _ := walkForFile(repoPath, cfg)
		if ok {
			return builders[langPython], nil
		}
	}

	if hasConfigFile(repoPath, templateDockerfile) {
		return &Builder{Name: builderDocker, Template: templateDockerfile}, nil
	}

	return nil, errors.New("no framework detected")
}

// detectPackageManager determines the package manager from the repository's lockfiles.
// It returns the first matching package manager in the configured detection order, or npm
// when no supported lockfile is present.
func detectPackageManager(repoPath string) string {
	for _, pm := range packageManagers {
		if hasConfigFile(repoPath, pm.lockFile) {
			return pm.name
		}
	}
	return pkgManagerNPM
}
