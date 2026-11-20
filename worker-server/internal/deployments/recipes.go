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

var vitepressBuilder = &Builder{Name: "vitepress", Template: "static/Dockerfile.vite.tmpl", DefaultOutputDir: ".vitepress/dist"}
var laravelBuilder = &Builder{Name: "laravel", Template: "dynamic/Dockerfile.laravel.tmpl"}
var nestBuilder = &Builder{Name: frameworkNestJS, Template: "dynamic/Dockerfile.node.tmpl"}

type recipeSelectionError struct{ error }

//nolint:gocyclo // Entry point resolution validates several independent Nest configuration sources.
//nolint:gocritic // Return values are explicit to keep this helper easy to call.
func nestEntrypoint(app string) (string, string, error) {
	out := "dist"
	root := ""
	//nolint:gosec // tsconfig.json is a fixed filename under the selected application path.
	if data, err := os.ReadFile(filepath.Join(app, "tsconfig.json")); err == nil {
		var c struct {
			CompilerOptions struct {
				RootDir string `json:"rootDir"`
				OutDir  string `json:"outDir"`
			} `json:"compilerOptions"`
		}
		if err := json.Unmarshal(data, &c); err != nil {
			return "", "", errors.New("nest tsconfig is not literal JSON; specify runtime_command explicitly")
		}
		root = c.CompilerOptions.RootDir
		if c.CompilerOptions.OutDir != "" {
			out = c.CompilerOptions.OutDir
		}
	}
	if !safeBuildPath.MatchString(out) || sanitizeOutputDir(out) == "" {
		return "", "", errors.New("invalid Nest compiler outDir")
	}
	source := "src"
	entry := "main"
	//nolint:gosec // nest-cli.json is a fixed config filename under app.
	if data, err := os.ReadFile(filepath.Join(app, "nest-cli.json")); err == nil {
		var c struct {
			SourceRoot      string `json:"sourceRoot"`
			EntryFile       string `json:"entryFile"`
			CompilerOptions struct {
				Webpack bool `json:"webpack"`
			} `json:"compilerOptions"`
		}
		if err := json.Unmarshal(data, &c); err != nil {
			return "", "", errors.New("nest config is not literal JSON; specify runtime_command explicitly")
		}
		if c.SourceRoot != "" {
			source = c.SourceRoot
		}
		if c.EntryFile != "" {
			entry = c.EntryFile
		}
		if c.CompilerOptions.Webpack {
			root = ""
		}
	}
	if !safeBuildPath.MatchString(entry) || sanitizeOutputDir(entry) == "" {
		return "", "", errors.New("invalid Nest entryFile")
	}
	if !safeBuildPath.MatchString(source) || (source != "." && sanitizeOutputDir(source) == "") {
		return "", "", errors.New("invalid Nest sourceRoot")
	}
	if root != "" && (!safeBuildPath.MatchString(root) || (filepath.Clean(root) != "." && sanitizeOutputDir(root) == "")) {
		return "", "", errors.New("invalid Nest rootDir")
	}
	folder := ""
	if root != "" {
		rel, err := filepath.Rel(filepath.Clean(root), filepath.Clean(source))
		if err != nil || rel == ".." || strings.HasPrefix(rel, "../") {
			return "", "", errors.New("nest sourceRoot escapes rootDir; specify runtime_command explicitly")
		}
		folder = rel
	}
	return filepath.ToSlash(filepath.Join(out, folder, entry+".js")), filepath.ToSlash(filepath.Clean(out)), nil
}

// Configuration is data. Detection never executes application configuration on the host.
func selectRecipe(app string) (*Builder, error) {
	o, err := readBuildOptions(app)
	if err != nil {
		return nil, err
	}
	if o.Framework != "" {
		for _, b := range append(append([]*Builder{}, builders...), vitepressBuilder, laravelBuilder, nestBuilder, nodeBuilder) {
			if o.Framework == b.Name {
				return b, nil
			}
		}
		return nil, fmt.Errorf("unknown framework override %q", o.Framework)
	}
	//nolint:gosec // composer.json is a fixed config filename under app.
	if data, err := os.ReadFile(filepath.Join(app, "composer.json")); err == nil {
		var c struct {
			Require map[string]string `json:"require"`
		}
		if json.Unmarshal(data, &c) == nil && c.Require["laravel/framework"] != "" {
			return laravelBuilder, nil
		}
	}
	p, err := readPackageJSON(app)
	if err != nil {
		return nil, nil
	}
	// Frameworks precede the libraries/tooling that implement them.
	var found []*Builder
	for _, pair := range []struct {
		dep string
		b   *Builder
	}{
		{"next", nextjsBuilder}, {"nuxt", nuxtBuilder}, {"astro", astroBuilder},
		{"@sveltejs/kit", sveltekitBuilder}, {"vitepress", vitepressBuilder},
		{"@nestjs/core", nestBuilder},
	} {
		if hasDep(p, pair.dep) {
			found = append(found, pair.b)
		}
	}
	if len(found) > 1 {
		return nil, errors.New("ambiguous framework dependencies; specify framework in zero-devops.build.json")
	}
	if len(found) == 1 {
		b := found[0]
		if b == astroBuilder {
			if _, refusal := checkSSRMetaFramework(app, b); refusal != "" {
				return nil, fmt.Errorf("detected %s — no static or dynamic deployment recipe exists", refusal)
			}
		}
		if b == sveltekitBuilder {
			return kitRecipe(app, p)
		}
		return b, nil
	}
	return nil, nil
}

func kitRecipe(app string, _ *packageJSON) (*Builder, error) {
	// Require one literal adapter import; arbitrary/dynamic config needs an explicit override.
	var text string
	for _, name := range []string{"svelte.config.js", "svelte.config.ts", "svelte.config.cjs"} {
		//nolint:gosec // name is selected from the fixed Svelte config filename list.
		b, _ := os.ReadFile(filepath.Join(app, name))
		text += string(b)
	}
	node := strings.Contains(text, "@sveltejs/adapter-node")
	static := strings.Contains(text, "@sveltejs/adapter-static")
	if node && !static {
		return sveltekitBuilder, nil
	}
	if static && !node {
		b := *svelteBuilder
		b.Name = "sveltekit-static"
		b.DefaultOutputDir = "build"
		return &b, nil
	}
	// adapter-auto is intentionally not replaced silently by a different deployment adapter.
	return nil, errors.New("SvelteKit adapter is ambiguous or unsupported; select a supported framework/output_dir explicitly")
}

var vitepressCommand = regexp.MustCompile(`(?:^|[;&|]\s*|\s)vitepress\s+build(?:\s+([A-Za-z0-9_./-]+))?`)

func recipeOutput(app string, b *Builder, o buildOptions) (string, error) {
	out := o.OutputDir
	if out == "" && b.Name == "vitepress" {
		p, err := readPackageJSON(app)
		if err != nil {
			return "", err
		}
		m := vitepressCommand.FindStringSubmatch(p.Scripts["build"])
		if m == nil {
			return "", errors.New("VitePress requires a build script or an explicit output_dir")
		}
		root := "."
		if len(m) > 1 && m[1] != "" {
			root = m[1]
		}
		out = filepath.ToSlash(filepath.Join(root, ".vitepress/dist"))
	}
	if out == "" {
		out = resolveOutputDir(app, b)
	}
	if out != "" && (!safeBuildPath.MatchString(out) || sanitizeOutputDir(out) == "") {
		return "", fmt.Errorf("invalid output_dir %q", out)
	}
	if o.OutputDir != "" && b.DefaultOutputDir == "" {
		return "", errors.New("output_dir applies only to static recipes")
	}
	return out, nil
}

// Produce validated runtime instructions for an app-root context. Native larger
// contexts invoke this after their manifest path has been resolved.
//
//nolint:gocyclo // Runtime rendering applies independent validated command, asset, and workdir options.
//nolint:funlen // Keeps validated runtime command, assets, and workdir rendering together.
func renderRuntime(app, contextDir, content string, b *Builder) (string, error) {
	o, err := readBuildOptions(app)
	if err != nil {
		return "", err
	}
	command := o.RuntimeCommand
	if b.Name == "nestjs" && len(command) == 0 {
		entry, out, err := nestEntrypoint(app)
		if err != nil {
			return "", err
		}
		content = strings.ReplaceAll(content, "/app/dist ./dist", "/app/"+out+" ./"+out)
		command = []string{"node", entry}
		content = strings.Replace(content, "RUN npm run build", "RUN npm run build\nRUN test -f "+entry, 1)
	}
	if len(command) > 0 {
		if strings.TrimSpace(command[0]) == "" {
			return "", errors.New("runtime_command needs an executable")
		}
		for _, arg := range command {
			if strings.ContainsAny(arg, "\x00\r\n") {
				return "", errors.New("invalid runtime_command argument")
			}
		}
		encoded, _ := json.Marshal(command)
		lines := strings.Split(content, "\n")
		for i, line := range lines {
			if strings.HasPrefix(line, "CMD ") {
				lines[i] = "CMD " + string(encoded)
			}
		}
		content = strings.Join(lines, "\n")
	}
	assets := o.RuntimeAssets
	if b.Name == langRust && assets == nil {
		if info, err := os.Stat(filepath.Join(app, "static")); err == nil && info.IsDir() {
			assets = []string{"static"}
		}
	}
	prepare := "RUN mkdir -p /zerodevops-runtime-assets"
	rel, _ := filepath.Rel(contextDir, app)
	for _, asset := range assets {
		if !safeBuildPath.MatchString(asset) || sanitizeOutputDir(asset) == "" {
			return "", fmt.Errorf("invalid runtime asset %q", asset)
		}
		source := filepath.Join(app, asset)
		if err := requireContainedPath(app, source); err != nil {
			return "", err
		}
		if err := validateContextPaths(app, source); err != nil {
			return "", err
		}
		src := filepath.ToSlash(filepath.Join(rel, asset))
		dst := filepath.ToSlash(filepath.Join("/zerodevops-runtime-assets", asset))
		if info, err := os.Stat(source); err == nil && info.IsDir() {
			dst += "/"
		}
		args, _ := json.Marshal([]string{src, dst})
		prepare += "\nCOPY " + string(args)
	}
	if len(assets) > 0 {
		// Insert in builder before the final FROM and copy before the final USER.
		index := strings.LastIndex(content, "\nFROM ")
		if index < 0 {
			return "", errors.New("runtime assets require a multistage recipe")
		}
		stages := regexp.MustCompile(`(?mi)^FROM\s+\S+\s+AS\s+([A-Za-z0-9_-]+)\s*$`).FindAllStringSubmatch(content[:index], -1)
		if len(stages) == 0 {
			return "", errors.New("runtime assets require a named source stage")
		}
		stage := stages[len(stages)-1][1]
		// Preserve source permissions while giving restrictive assets to the
		// configured runtime identity. Avoid globally broadening file modes.
		chown := ""
		users := regexp.MustCompile(`(?m)^USER\s+([A-Za-z0-9_:-]+)\s*$`).FindAllStringSubmatch(content[index:], -1)
		if len(users) > 0 {
			chown = " --chown=" + users[len(users)-1][1]
		}
		content = content[:index] + "\n" + prepare + "\n" + content[index:]
		index = strings.LastIndex(content, "\nUSER ")
		if index < 0 {
			index = strings.LastIndex(content, "\nCMD ")
		}
		if index < 0 {
			return "", errors.New("runtime assets require a final CMD or USER")
		}
		content = content[:index] + "\nCOPY --from=" + stage + chown + " /zerodevops-runtime-assets/ /app/\n" + content[index:]
	}
	if o.RuntimeWorkdir != "" {
		dir := o.RuntimeWorkdir
		if dir != "." && (!safeBuildPath.MatchString(dir) || sanitizeOutputDir(dir) == "") {
			return "", errors.New("invalid runtime_workdir")
		}
		if err := requireContainedPath(app, filepath.Join(app, dir)); err != nil {
			return "", err
		}
		index := strings.LastIndex(content, "\nCMD ")
		encoded, _ := json.Marshal(filepath.ToSlash(filepath.Join("/app", dir)))
		if index < 0 {
			return "", errors.New("runtime_workdir requires CMD")
		}
		content = content[:index] + "\nWORKDIR " + string(encoded) + "\n" + content[index:]
	}
	return content, nil
}

func applyNativeRuntime(app, contextDir string, b *Builder) error {
	path := filepath.Join(app, templateDockerfile)
	//nolint:gosec // Generated Dockerfile path is derived from the selected app directory.
	data, err := os.ReadFile(path)
	if err != nil {
		return err
	}
	content, err := renderRuntime(app, contextDir, string(data), b)
	if err != nil {
		return err
	}
	//nolint:gosec // Generated Dockerfile path is under the selected application directory.
	return os.WriteFile(path, []byte(content), generatedFileMode)
}
