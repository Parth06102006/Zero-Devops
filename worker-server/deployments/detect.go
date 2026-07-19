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
	"vite": {
		Name: "vite",
		ConfigFiles: []string{
			"vite.config.ts", "vite.config.js", "vite.config.mjs",
		},
		Deps:     []string{"vite"},
		Template: "Dockerfile.vite.tmpl",
	},
	"nextjs": {
		Name: "nextjs",
		ConfigFiles: []string{
			"next.config.ts", "next.config.js", "next.config.mjs",
			"next.config.tsx", "next.config.jsx",
		},
		Deps:     []string{"next"},
		Template: "Dockerfile.nextjs.tmpl",
	},
	"astro": {
		Name: "astro",
		ConfigFiles: []string{
			"astro.config.ts", "astro.config.js", "astro.config.mjs",
		},
		Deps:     []string{"astro"},
		Template: "Dockerfile.astro.tmpl",
	},
	"react": {
		Name:        "react",
		ConfigFiles: nil,
		Deps:        []string{"react", "react-dom"},
		Template:    "Dockerfile.react.tmpl",
	},
	"node": {
		Name:        "node",
		ConfigFiles: nil,
		Deps:        nil,
		Template:    "Dockerfile.node.tmpl",
	},
	"go": {
		Name:        "go",
		ConfigFiles: []string{"go.mod"},
		Deps:        nil,
		Template:    "Dockerfile.go.tmpl",
	},
	"python": {
		Name: "python",
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
	{"pnpm-lock.yaml", "pnpm"},
	{"yarn.lock", "yarn"},
	{"bun.lockb", "bun"},
	{"package-lock.json", "npm"},
}

type packageJSON struct {
	Dependencies    map[string]string `json:"dependencies"`
	DevDependencies map[string]string `json:"devDependencies"`
}

func readPackageJSON(repoPath string) (*packageJSON, error) {
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

func hasConfigFile(repoPath, configFile string) bool {
	info, err := os.Stat(filepath.Join(repoPath, configFile))
	return err == nil && !info.IsDir()
}

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
			return builders["node"], nil
		}
	}

	if hasConfigFile(repoPath, "go.mod") {
		return builders["go"], nil
	}

	for _, cfg := range builders["python"].ConfigFiles {
		ok, _ := walkForFile(repoPath, cfg)
		if ok {
			return builders["python"], nil
		}
	}

	if hasConfigFile(repoPath, "Dockerfile") {
		return &Builder{Name: "docker", Template: "Dockerfile"}, nil
	}

	return nil, errors.New("no framework detected")
}

func detectPackageManager(repoPath string) string {
	for _, pm := range packageManagers {
		if hasConfigFile(repoPath, pm.lockFile) {
			return pm.name
		}
	}
	return "npm"
}
