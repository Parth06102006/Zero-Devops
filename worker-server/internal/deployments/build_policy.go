package deployments

import (
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"io/fs"
	"os"
	"path/filepath"
	"regexp"
	"strconv"
	"strings"

	"github.com/pelletier/go-toml/v2"
)

// Build options are repository data, never shell commands. Context is relative
// to the checkout root; package paths are relative to the selected application.
type buildOptions struct {
	BuildContext string `json:"build_context"`
	GoPackage    string `json:"go_package"`
	CargoPackage string `json:"cargo_package"`
	CargoBinary  string `json:"cargo_binary"`
}

func readBuildOptions(appDir string) (buildOptions, error) {
	var options buildOptions
	path := filepath.Join(appDir, "zero-devops.build.json")
	f, err := os.Open(path)
	if errors.Is(err, os.ErrNotExist) {
		return options, nil
	}
	if err != nil {
		return options, err
	}
	defer f.Close()
	if err := requireContainedPath(appDir, path); err != nil {
		return options, err
	}
	dec := json.NewDecoder(io.LimitReader(f, 64*1024))
	dec.DisallowUnknownFields()
	if err := dec.Decode(&options); err != nil {
		return options, fmt.Errorf("zero-devops.build.json: %w", err)
	}
	var extra any
	if err := dec.Decode(&extra); err != io.EOF {
		return options, errors.New("zero-devops.build.json must contain exactly one object")
	}
	return options, nil
}

// Check both lexical and real paths, including symlinked ancestors. Never use
// a caller-supplied directory as its own containment boundary.
func requireContainedPath(root, path string) error {
	root, err := filepath.Abs(root)
	if err != nil {
		return err
	}
	path, err = filepath.Abs(path)
	if err != nil {
		return err
	}
	inside := func(base, candidate string) bool {
		rel, err := filepath.Rel(base, candidate)
		return err == nil && rel != ".." && !strings.HasPrefix(rel, ".."+string(filepath.Separator))
	}
	if !inside(root, path) {
		return fmt.Errorf("path %q escapes repository", path)
	}
	realRoot, err := filepath.EvalSymlinks(root)
	if err != nil {
		return err
	}
	realPath, err := filepath.EvalSymlinks(path)
	if err != nil {
		return err
	}
	if !inside(realRoot, realPath) {
		return fmt.Errorf("symlink path %q escapes repository", path)
	}
	return nil
}

func unlockedNPM(appDir string) bool {
	return !hasConfigFile(appDir, "package-lock.json") && !hasConfigFile(appDir, "npm-shrinkwrap.json")
}

func renderInstallPolicy(appDir, content, pm string) (string, error) {
	if pm != pkgManagerNPM || !strings.Contains(content, "npm ci --ignore-scripts") {
		return content, nil
	}
	if !unlockedNPM(appDir) {
		if hasConfigFile(appDir, "npm-shrinkwrap.json") {
			content = strings.ReplaceAll(content, "COPY package*.json ./", "COPY package.json npm-shrinkwrap.json ./")
		}
		return content, nil
	}
	if os.Getenv("ALLOW_UNLOCKED_NPM_INSTALL") != "true" {
		return "", errors.New("npm lockfile missing: commit package-lock.json or npm-shrinkwrap.json; an operator may opt in with ALLOW_UNLOCKED_NPM_INSTALL=true (non-reproducible dependency resolution; install scripts stay disabled)")
	}
	// Fresh resolution is confined to the image. Never write a lock into the
	// checkout or claim that this reproduces a previous dependency tree.
	content = strings.ReplaceAll(content, "npm ci --ignore-scripts", "npm install --ignore-scripts --package-lock=false")
	return content, nil
}

var releasedGoVersion = regexp.MustCompile(`^1\.(0|[1-9][0-9]{0,2})(\.(0|[1-9][0-9]{0,3}))?$`)
var safeBuildPath = regexp.MustCompile(`^[A-Za-z0-9_./-]+$`)
var cargoName = regexp.MustCompile(`^[A-Za-z0-9_-]+$`)

func newerGoVersion(a, b string) bool {
	aa, bb := strings.Split(a, "."), strings.Split(b, ".")
	for i := 0; i < 3; i++ {
		av, bv := 0, 0
		if i < len(aa) {
			av, _ = strconv.Atoi(aa[i])
		}
		if i < len(bb) {
			bv, _ = strconv.Atoi(bb[i])
		}
		if av != bv {
			return av > bv
		}
	}
	return false
}

func goImageVersion(appDir string) (string, error) {
	path := filepath.Join(appDir, "go.mod")
	if err := requireContainedPath(appDir, path); err != nil {
		return "", err
	}
	data, err := os.ReadFile(path)
	if err != nil {
		return "", err
	}
	version, toolchain := "", ""
	for _, line := range strings.Split(string(data), "\n") {
		fields := strings.Fields(strings.SplitN(line, "//", 2)[0])
		if len(fields) == 0 {
			continue
		}
		switch fields[0] {
		case "go":
			if len(fields) != 2 || version != "" || !releasedGoVersion.MatchString(fields[1]) {
				return "", errors.New("go.mod needs one released go version directive")
			}
			version = fields[1]
		case "toolchain":
			if len(fields) != 2 || toolchain != "" {
				return "", errors.New("invalid go.mod toolchain directive")
			}
			toolchain = fields[1]
			if toolchain != "default" && (!strings.HasPrefix(toolchain, "go") || !releasedGoVersion.MatchString(strings.TrimPrefix(toolchain, "go"))) {
				return "", errors.New("unsupported go.mod toolchain directive; use a released Go version")
			}
		}
	}
	if version == "" {
		return "", errors.New("go.mod needs a valid go version directive")
	}
	if tc := strings.TrimPrefix(toolchain, "go"); toolchain != "" && toolchain != "default" && newerGoVersion(tc, version) {
		version = tc
	}
	return version, nil
}

func renderGoBuild(appDir, content string) (string, error) {
	version, err := goImageVersion(appDir)
	if err != nil {
		return "", err
	}
	options, err := readBuildOptions(appDir)
	if err != nil {
		return "", err
	}
	target := options.GoPackage
	if target == "" {
		target = "."
	}
	if filepath.IsAbs(target) || !safeBuildPath.MatchString(target) || strings.HasPrefix(target, "-") || strings.Contains(target, "...") {
		return "", errors.New("go_package must be one local package path")
	}
	path := filepath.Join(appDir, target)
	if err := requireContainedPath(appDir, path); err != nil {
		return "", err
	}
	if info, err := os.Stat(path); err != nil || !info.IsDir() {
		return "", errors.New("go_package must select a directory")
	}
	if target != "." {
		target = "./" + filepath.ToSlash(filepath.Clean(target))
	}
	command, _ := json.Marshal([]string{"go", "build", "-o", "/app/app", target})
	content = strings.ReplaceAll(content, "__GO_VERSION__", version)
	content = strings.ReplaceAll(content, "__GO_BUILD__", "RUN "+string(command))
	return content, nil
}

type cargoManifest struct {
	Package *struct {
		Name       string `toml:"name"`
		DefaultRun string `toml:"default-run"`
		Autobins   *bool  `toml:"autobins"`
		Workspace  string `toml:"workspace"`
	} `toml:"package"`
	Workspace map[string]any `toml:"workspace"`
	Bin       []struct {
		Name string `toml:"name"`
		Path string `toml:"path"`
	} `toml:"bin"`
}

func readCargo(root, dir string) (cargoManifest, error) {
	var manifest cargoManifest
	path := filepath.Join(dir, "Cargo.toml")
	if err := requireContainedPath(root, path); err != nil {
		return manifest, err
	}
	data, err := os.ReadFile(path)
	if err != nil {
		return manifest, err
	}
	err = toml.Unmarshal(data, &manifest)
	return manifest, err
}

func cargoBinary(appDir string, manifest cargoManifest, options buildOptions) (string, string, error) {
	pkg := options.CargoPackage
	if pkg == "" && manifest.Package != nil {
		pkg = manifest.Package.Name
	}
	if !cargoName.MatchString(pkg) {
		return "", "", errors.New("select one cargo_package for a virtual workspace")
	}
	if options.CargoBinary != "" {
		if !cargoName.MatchString(options.CargoBinary) {
			return "", "", errors.New("invalid cargo_binary")
		}
		return pkg, options.CargoBinary, nil
	}
	if manifest.Package == nil || (options.CargoPackage != "" && options.CargoPackage != manifest.Package.Name) {
		return "", "", errors.New("cargo_binary is required when selecting a different workspace package")
	}
	if manifest.Package.DefaultRun != "" {
		return pkg, manifest.Package.DefaultRun, nil
	}
	bins := map[string]bool{}
	for _, bin := range manifest.Bin {
		if bin.Name != "" {
			bins[bin.Name] = true
		}
	}
	if manifest.Package.Autobins == nil || *manifest.Package.Autobins {
		if hasConfigFile(appDir, "src/main.rs") {
			bins[manifest.Package.Name] = true
		}
		entries, _ := os.ReadDir(filepath.Join(appDir, "src", "bin"))
		for _, entry := range entries {
			if !entry.IsDir() && strings.HasSuffix(entry.Name(), ".rs") {
				bins[strings.TrimSuffix(entry.Name(), ".rs")] = true
			}
			if entry.IsDir() && hasConfigFile(filepath.Join(appDir, "src", "bin", entry.Name()), "main.rs") {
				bins[entry.Name()] = true
			}
		}
	}
	if len(bins) != 1 {
		return "", "", errors.New("ambiguous or missing Cargo binary: set cargo_binary in zero-devops.build.json")
	}
	for bin := range bins {
		if cargoName.MatchString(bin) {
			return pkg, bin, nil
		}
	}
	return "", "", errors.New("invalid Cargo binary name")
}

// prepareBuildContext returns the actual Buildah context independently of the
// app directory. Rust defaults to its enclosing workspace; other recipes keep
// their app context unless a native build explicitly requests a larger one.
func prepareBuildContext(repoDir, appDir string, builder *Builder) (string, error) {
	if err := requireContainedPath(repoDir, appDir); err != nil {
		return "", err
	}
	if builder.Name != langRust && builder.Name != langGo {
		return appDir, nil
	}
	options, err := readBuildOptions(appDir)
	if err != nil {
		return "", err
	}
	contextDir := appDir
	if options.BuildContext != "" {
		if filepath.IsAbs(options.BuildContext) || !safeBuildPath.MatchString(options.BuildContext) {
			return "", errors.New("build_context must be a repository-relative path")
		}
		contextDir = filepath.Join(repoDir, options.BuildContext)
		if err := requireContainedPath(repoDir, contextDir); err != nil {
			return "", err
		}
	}
	if builder.Name == langGo {
		if err := requireContainedPath(contextDir, appDir); err != nil {
			return "", err
		}
		if contextDir != appDir {
			rel, _ := filepath.Rel(contextDir, appDir)
			if !safeBuildPath.MatchString(rel) {
				return "", errors.New("unsupported characters in Go application path")
			}
			path := filepath.Join(appDir, templateDockerfile)
			data, err := os.ReadFile(path)
			if err != nil {
				return "", err
			}
			content := strings.Replace(string(data), "COPY go.mod go.sum* ./\nRUN go mod download\n\nCOPY . .", "COPY . .\nWORKDIR /app/"+filepath.ToSlash(rel)+"\nRUN go mod download", 1)
			if err := os.WriteFile(path, []byte(content), 0o600); err != nil {
				return "", err
			}
		}
		return contextDir, validateContextPaths(repoDir, contextDir)
	}
	manifest, err := readCargo(repoDir, appDir)
	if err != nil {
		return "", err
	}
	lockDir := appDir
	if options.BuildContext == "" && manifest.Workspace == nil {
		if manifest.Package != nil && manifest.Package.Workspace != "" {
			contextDir = filepath.Join(appDir, manifest.Package.Workspace)
			if err := requireContainedPath(repoDir, contextDir); err != nil {
				return "", err
			}
		} else {
			for dir := filepath.Dir(appDir); ; dir = filepath.Dir(dir) {
				if err := requireContainedPath(repoDir, dir); err != nil {
					break
				}
				if hasConfigFile(dir, "Cargo.toml") {
					parent, err := readCargo(repoDir, dir)
					if err != nil {
						return "", err
					}
					if parent.Workspace != nil {
						contextDir = dir
						break
					}
				}
				if dir == repoDir || dir == filepath.Dir(dir) {
					break
				}
			}
		}
	}
	lockDir = contextDir
	// Keep the checkout, including sibling path dependencies, as the default
	// Rust context. Cargo still builds only the selected package and binary.
	if options.BuildContext == "" {
		contextDir = repoDir
	}
	if err := validateContextPaths(repoDir, contextDir); err != nil {
		return "", err
	}
	if err := validateCargoPaths(contextDir); err != nil {
		return "", err
	}
	if err := requireContainedPath(contextDir, appDir); err != nil {
		return "", err
	}
	pkg, bin, err := cargoBinary(appDir, manifest, options)
	if err != nil {
		return "", err
	}
	if !cargoName.MatchString(bin) {
		return "", errors.New("invalid Cargo binary name")
	}
	rel, _ := filepath.Rel(contextDir, appDir)
	args := []string{"cargo", "build", "--release", "--manifest-path", filepath.ToSlash(filepath.Join(rel, "Cargo.toml")), "--package", pkg, "--bin", bin, "--target-dir", "/app/.zerodevops-target"}
	if hasConfigFile(lockDir, "Cargo.lock") {
		args = append(args, "--locked")
	}
	command, _ := json.Marshal(args)
	path := filepath.Join(appDir, templateDockerfile)
	data, err := os.ReadFile(path)
	if err != nil {
		return "", err
	}
	content := strings.ReplaceAll(string(data), "__CARGO_BUILD__", "RUN "+string(command))
	content = strings.ReplaceAll(content, "__CARGO_BINARY__", bin)
	if err := os.WriteFile(path, []byte(content), 0o600); err != nil {
		return "", err
	}
	return contextDir, nil
}

func validateContextPaths(repoDir, contextDir string) error {
	return filepath.WalkDir(contextDir, func(path string, entry fs.DirEntry, err error) error {
		if err != nil {
			return err
		}
		if entry.IsDir() && entry.Name() == ".git" {
			return filepath.SkipDir
		}
		if entry.Type()&os.ModeSymlink != 0 {
			return requireContainedPath(repoDir, path)
		}
		return nil
	})
}

// Cargo can load path dependencies in workspace and target-specific tables.
// Reject dependencies outside the supplied context before invoking the build.
func validateCargoPaths(contextDir string) error {
	return filepath.WalkDir(contextDir, func(path string, entry fs.DirEntry, err error) error {
		if err != nil {
			return err
		}
		if entry.IsDir() && (entry.Name() == ".git" || entry.Name() == "target") {
			return filepath.SkipDir
		}
		if entry.Name() != "Cargo.toml" || entry.IsDir() {
			return nil
		}
		data, err := os.ReadFile(path)
		if err != nil {
			return err
		}
		var doc map[string]any
		if err := toml.Unmarshal(data, &doc); err != nil {
			return fmt.Errorf("%s: %w", path, err)
		}
		var check func(any, bool) error
		check = func(value any, dependencies bool) error {
			switch value := value.(type) {
			case map[string]any:
				for key, child := range value {
					if key == "metadata" {
						continue
					}
					if key == "path" && dependencies {
						if relative, ok := child.(string); ok {
							if filepath.IsAbs(relative) {
								return fmt.Errorf("absolute Cargo dependency path in %s", path)
							}
							if err := requireContainedPath(contextDir, filepath.Join(filepath.Dir(path), relative)); err != nil {
								return err
							}
						}
					}
					next := dependencies || key == "dependencies" || key == "dev-dependencies" || key == "build-dependencies" || key == "patch" || key == "replace"
					if err := check(child, next); err != nil {
						return err
					}
				}
			case []any:
				for _, child := range value {
					if err := check(child, dependencies); err != nil {
						return err
					}
				}
			}
			return nil
		}
		return check(doc, false)
	})
}
