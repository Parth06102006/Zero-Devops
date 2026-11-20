package deployments

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"Zero_Devops/worker_server/internal/domain"
)

func TestFollowupSelectionErrorsRemainActionable(t *testing.T) {
	for _, tc := range []struct{ name, pkg, options, config, diagnostic string }{
		{"ambiguous frameworks", `{"dependencies":{"astro":"5","next":"15"}}`, "", "", "ambiguous framework"},
		{"unknown override", `{"dependencies":{"next":"15"}}`, `{"framework":"typo"}`, "", "unknown framework override"},
		{"unsupported adapter", `{"devDependencies":{"@sveltejs/kit":"2"}}`, "", `import adapter from '@sveltejs/adapter-auto'`, "adapter"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			app := t.TempDir()
			writeFile(t, app, "package.json", tc.pkg)
			if tc.options != "" {
				writeFile(t, app, "zero-devops.build.json", tc.options)
			}
			if tc.config != "" {
				writeFile(t, app, "svelte.config.js", tc.config)
			}
			_, err := detectFramework(app)
			if err == nil || !strings.Contains(err.Error(), tc.diagnostic) {
				t.Fatalf("got %v; want actionable %q diagnostic", err, tc.diagnostic)
			}
		})
	}
}

func TestFollowupNestCompactCompilerOptions(t *testing.T) {
	app := t.TempDir()
	writeFile(t, app, "tsconfig.json", `{"compilerOptions":{"rootDir":"./","outDir":"./dist"}}`)
	content, err := renderRuntime(app, app, "FROM node AS builder\nRUN npm run build\nFROM node\nCMD [\"npm\",\"start\"]", nestBuilder)
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(content, `CMD ["node","dist/src/main.js"]`) || !strings.Contains(content, "test -f dist/src/main.js") {
		t.Fatalf("compiled entrypoint was not validated and started: %s", content)
	}
}

func TestFollowupRuntimeAssetsUseTheirActualStage(t *testing.T) {
	for _, tc := range []struct {
		name, template string
		b              *Builder
	}{
		{"laravel", "dynamic/Dockerfile.laravel.tmpl", laravelBuilder},
		{"php", "dynamic/Dockerfile.php.tmpl", phpBuilder},
	} {
		t.Run(tc.name, func(t *testing.T) {
			app := t.TempDir()
			writeFile(t, app, "resources/banner.txt", "asset")
			writeFile(t, app, "zero-devops.build.json", `{"runtime_assets":["resources/banner.txt"]}`)
			raw, err := os.ReadFile(filepath.Join("..", "..", "templates", tc.template))
			if err != nil {
				t.Fatal(err)
			}
			content, err := renderRuntime(app, app, string(raw), tc.b)
			if err != nil {
				t.Fatal(err)
			}
			stage := ""
			preparedIn := ""
			copiedFrom := ""
			for _, line := range strings.Split(content, "\n") {
				fields := strings.Fields(line)
				if len(fields) >= 2 && fields[0] == "FROM" {
					stage = ""
					if len(fields) >= 4 && strings.EqualFold(fields[2], "AS") {
						stage = fields[3]
					}
				}
				if strings.Contains(line, "RUN mkdir -p /zerodevops-runtime-assets") {
					preparedIn = stage
				}
				if strings.HasPrefix(line, "COPY --from=") && strings.Contains(line, "/zerodevops-runtime-assets/") {
					copiedFrom = strings.TrimPrefix(fields[1], "--from=")
				}
			}
			if preparedIn == "" || copiedFrom != preparedIn {
				t.Fatalf("assets prepared in %q but copied from %q:\n%s", preparedIn, copiedFrom, content)
			}
		})
	}
}

func TestFollowupCloneVerifiesPinnedSHA(t *testing.T) {
	const sha = "0123456789abcdef0123456789abcdef01234567"
	for _, tc := range []struct {
		name, head string
		wantError  bool
	}{{"matching", sha, false}, {"mismatch", "aaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa", true}} {
		t.Run(tc.name, func(t *testing.T) {
			root := t.TempDir()
			oldRoot := buildRoot
			buildRoot = filepath.Join(root, "builds")
			t.Cleanup(func() { buildRoot = oldRoot })
			bin := filepath.Join(root, "bin")
			if err := os.Mkdir(bin, 0o700); err != nil {
				t.Fatal(err)
			}
			log := filepath.Join(root, "git-args.log")
			script := `#!/bin/sh
printf '%s\n' "$*" >> "$REVIEW_GIT_LOG"
case "$1" in
 init) mkdir -p "$2" ;;
 -C) if [ "$3" = "rev-parse" ]; then printf '%s\n' "$REVIEW_HEAD"; fi ;;
esac
`
			if err := os.WriteFile(filepath.Join(bin, "git"), []byte(script), 0o700); err != nil {
				t.Fatal(err)
			}
			t.Setenv("PATH", bin+string(os.PathListSeparator)+os.Getenv("PATH"))
			t.Setenv("REVIEW_GIT_LOG", log)
			t.Setenv("REVIEW_HEAD", tc.head)
			_, err := cloneRepo("https://github.com/example/repo.git", "review-case", sha, domain.CloneCredential{Username: "x-access-token", Token: "test-only"})
			if (err != nil) != tc.wantError {
				t.Fatalf("got %v, wantError=%t", err, tc.wantError)
			}
			args, readErr := os.ReadFile(log)
			if readErr != nil {
				t.Fatal(readErr)
			}
			for _, expected := range []string{"fetch --depth 1 origin " + sha, "checkout --detach " + sha, "rev-parse HEAD"} {
				if !strings.Contains(string(args), expected) {
					t.Fatalf("missing %q in git calls: %s", expected, args)
				}
			}
			if strings.Contains(string(args), "test-only") {
				t.Fatal("credential leaked into git arguments")
			}
		})
	}
}

func TestFollowupNestRejectsUnsafeCompilerPaths(t *testing.T) {
	for _, tc := range []struct{ name, tsconfig, nestconfig string }{
		{"source shell syntax", `{"compilerOptions":{"rootDir":"."}}`, `{"sourceRoot":"src;echo injected"}`},
		{"root shell syntax", `{"compilerOptions":{"rootDir":".;echo injected"}}`, `{}`},
		{"source traversal", `{"compilerOptions":{"rootDir":"."}}`, `{"sourceRoot":"../outside"}`},
		{"root traversal", `{"compilerOptions":{"rootDir":"../outside"}}`, `{}`},
		{"output traversal", `{"compilerOptions":{"outDir":"../outside"}}`, `{}`},
		{"absolute source", `{"compilerOptions":{"rootDir":"."}}`, `{"sourceRoot":"/outside"}`},
	} {
		t.Run(tc.name, func(t *testing.T) {
			app := t.TempDir()
			writeFile(t, app, "tsconfig.json", tc.tsconfig)
			writeFile(t, app, "nest-cli.json", tc.nestconfig)
			if _, _, err := nestEntrypoint(app); err == nil {
				t.Fatal("unsafe compiler path accepted")
			}
		})
	}
}

func TestFollowupRuntimeOptionsRejectTraversalAndControls(t *testing.T) {
	for _, options := range []string{
		`{"runtime_workdir":"../outside"}`,
		`{"runtime_workdir":"/outside"}`,
		`{"runtime_command":["node","line\nbreak"]}`,
		`{"runtime_command":[""]}`,
	} {
		t.Run(options, func(t *testing.T) {
			app := t.TempDir()
			writeFile(t, app, "zero-devops.build.json", options)
			if _, err := renderRuntime(app, app, "FROM node AS builder\nFROM node\nCMD [\"node\",\"main.js\"]", nodeBuilder); err == nil {
				t.Fatal("unsafe runtime option accepted")
			}
		})
	}
	app := t.TempDir()
	outside := t.TempDir()
	if err := os.Symlink(outside, filepath.Join(app, "linked")); err != nil {
		t.Fatal(err)
	}
	writeFile(t, app, "zero-devops.build.json", `{"runtime_workdir":"linked"}`)
	if _, err := renderRuntime(app, app, "FROM node AS builder\nFROM node\nCMD [\"node\",\"main.js\"]", nodeBuilder); err == nil {
		t.Fatal("escaping runtime workdir symlink accepted")
	}
}
