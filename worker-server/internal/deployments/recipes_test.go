package deployments

import (
	"context"
	"errors"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

func TestRecipeOwnership(t *testing.T) {
	for _, tc := range []struct{ name, pkg, config, want string }{
		{"plain svelte", `{"devDependencies":{"svelte":"5","vite":"6"}}`, "svelte.config.js", frameworkVite},
		{"astro svelte", `{"dependencies":{"astro":"5","svelte":"5","@astrojs/svelte":"7"}}`, "svelte.config.js", frameworkAstro},
		{"kit node", `{"devDependencies":{"@sveltejs/kit":"2","svelte":"5","vite":"6"}}`, "svelte.config.js", frameworkSvelteKit},
		{"vitepress", `{"devDependencies":{"vitepress":"1","vue":"3"},"scripts":{"build":"vitepress build docs"}}`, "vite.config.js", "vitepress"},
		{"nest", `{"dependencies":{"@nestjs/core":"11"}}`, "", "nestjs"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			app := t.TempDir()
			writeFile(t, app, "package.json", tc.pkg)
			if tc.config != "" {
				writeFile(t, app, tc.config, `import adapter from '@sveltejs/adapter-node'; export default {};`)
			}
			b, err := detectFramework(app)
			if err != nil {
				t.Fatal(err)
			}
			if b.Name != tc.want {
				t.Fatalf("got %s want %s", b.Name, tc.want)
			}
		})
	}
	t.Run("laravel owns vite", func(t *testing.T) {
		app := t.TempDir()
		writeFile(t, app, "composer.json", `{"require":{"laravel/framework":"^12"}}`)
		writeFile(t, app, "vite.config.js", "")
		writeFile(t, app, "package.json", `{"devDependencies":{"vite":"6"}}`)
		b, e := detectFramework(app)
		if e != nil || b.Name != "laravel" {
			t.Fatalf("%v %v", b, e)
		}
	})
	t.Run("ambiguous", func(t *testing.T) {
		app := t.TempDir()
		writeFile(t, app, "package.json", `{"dependencies":{"astro":"5","next":"15"}}`)
		if _, e := detectFramework(app); e == nil {
			t.Fatal("ambiguous frameworks accepted")
		}
		writeFile(t, app, "zero-devops.build.json", `{"framework":"astro"}`)
		b, e := detectFramework(app)
		if e != nil || b.Name != frameworkAstro {
			t.Fatalf("%v %v", b, e)
		}
	})
	t.Run("kit adapter", func(t *testing.T) {
		app := t.TempDir()
		writeFile(t, app, "package.json", `{"devDependencies":{"@sveltejs/kit":"2"}}`)
		writeFile(t, app, "svelte.config.js", `import adapter from '@sveltejs/adapter-auto'`)
		if _, e := detectFramework(app); e == nil {
			t.Fatal("auto guessed as node")
		}
		writeFile(t, app, "svelte.config.js", `import adapter from '@sveltejs/adapter-static'`)
		b, e := detectFramework(app)
		if e != nil || b.DefaultOutputDir != "build" {
			t.Fatalf("%v %v", b, e)
		}
	})
}

func TestRecipeArtifactsAndRuntime(t *testing.T) {
	t.Setenv("ALLOW_UNLOCKED_NPM_INSTALL", "true")
	t.Run("next optional assets", func(t *testing.T) {
		for _, exists := range []bool{false, true} {
			app := t.TempDir()
			writeFile(t, app, "package.json", `{"dependencies":{"next":"15"}}`)
			if exists {
				writeFile(t, app, "public/logo.txt", "retained")
			}
			if e := writeDockerfile(app, nextjsBuilder, pkgManagerNPM); e != nil {
				t.Fatal(e)
			}
			data, _ := os.ReadFile(filepath.Join(app, "Dockerfile"))
			if !strings.Contains(string(data), "mkdir -p /app/public") {
				t.Fatal("public not normalized")
			}
			if exists {
				b, _ := os.ReadFile(filepath.Join(app, "public/logo.txt"))
				if string(b) != "retained" {
					t.Fatal("asset changed")
				}
			}
		}
	})
	t.Run("vitepress output", func(t *testing.T) {
		app := t.TempDir()
		writeFile(t, app, "package.json", `{"scripts":{"build":"vitepress build docs"}}`)
		out, e := recipeOutput(app, vitepressBuilder, buildOptions{})
		if e != nil || out != "docs/.vitepress/dist" {
			t.Fatalf("%s %v", out, e)
		}
		if _, e := recipeOutput(app, vitepressBuilder, buildOptions{OutputDir: "../outside"}); e == nil {
			t.Fatal("escaping output accepted")
		}
	})
	t.Run("nest production", func(t *testing.T) {
		app := t.TempDir()
		writeFile(t, app, "package.json", `{"dependencies":{"@nestjs/core":"11"}}`)
		if e := writeDockerfile(app, nestBuilder, pkgManagerNPM); e != nil {
			t.Fatal(e)
		}
		b, _ := os.ReadFile(filepath.Join(app, "Dockerfile"))
		if !strings.Contains(string(b), `CMD ["node","dist/main.js"]`) || !strings.Contains(string(b), "RUN test -f dist/main.js") {
			t.Fatal(string(b))
		}
	})
	t.Run("native assets workspace relative", func(t *testing.T) {
		root := t.TempDir()
		app := filepath.Join(root, "apps/web")
		writeFile(t, app, "static/root/index.html", "asset")
		content, e := renderRuntime(app, root, "FROM rust AS builder\nWORKDIR /app\nFROM alpine\nWORKDIR /app\nUSER app\nCMD [\"/app/server\"]", rustBuilder)
		if e != nil {
			t.Fatal(e)
		}
		if !strings.Contains(content, `COPY ["apps/web/static","/zerodevops-runtime-assets/static/"]`) || !strings.Contains(content, "COPY --from=builder --chown=app /zerodevops-runtime-assets/ /app/") {
			t.Fatal(content)
		}
	})
	for _, asset := range []string{"../outside", "/etc", "static/../../escape", "static;bad"} {
		t.Run(asset, func(t *testing.T) {
			app := t.TempDir()
			writeFile(t, app, "zero-devops.build.json", `{"runtime_assets":["`+asset+`"]}`)
			if _, e := renderRuntime(app, app, "", rustBuilder); e == nil {
				t.Fatal("unsafe asset accepted")
			}
		})
	}
	t.Run("symlink asset", func(t *testing.T) {
		app := t.TempDir()
		outside := t.TempDir()
		if e := os.Symlink(outside, filepath.Join(app, "static")); e != nil {
			t.Fatal(e)
		}
		if _, e := renderRuntime(app, app, "", rustBuilder); e == nil {
			t.Fatal("escaping symlink accepted")
		}
	})
	t.Run("command arguments", func(t *testing.T) {
		app := t.TempDir()
		writeFile(t, app, "zero-devops.build.json", `{"runtime_command":["node","dist/main.js","literal $(touch /tmp/no)"]}`)
		v, e := renderRuntime(app, app, "FROM node AS builder\nFROM node\nCMD [\"npm\",\"start\"]", nodeBuilder)
		if e != nil || !strings.Contains(v, `CMD ["node","dist/main.js","literal $(touch /tmp/no)"]`) {
			t.Fatalf("%s %v", v, e)
		}
	})
}

func TestCheckoutPolicy(t *testing.T) {
	for _, tc := range []struct {
		message string
		want    bool
	}{{"Authentication failed", false}, {"repository not found", false}, {"not our ref abc", false}, {"requested URL returned error: 503", true}, {"Connection reset by peer", true}, {"syntax error", false}} {
		if got := transientFetch(errors.New(tc.message)); got != tc.want {
			t.Fatalf("%s: %t", tc.message, got)
		}
	}
	p := checkoutPolicy{fetch: time.Second, attempts: 2}
	n := 0
	e := fetchPinned(p, func(ctx context.Context) error {
		n++
		if _, ok := ctx.Deadline(); !ok {
			t.Fatal("missing phase deadline")
		}
		if n == 1 {
			return context.DeadlineExceeded
		}
		return nil
	})
	if e != nil || n != 2 {
		t.Fatalf("%v %d", e, n)
	}
	n = 0
	e = fetchPinned(p, func(context.Context) error { n++; return errors.New("authentication failed") })
	if e == nil || n != 1 {
		t.Fatalf("%v %d", e, n)
	}
	t.Setenv("GIT_FETCH_TIMEOUT_SECONDS", "301")
	if _, e := readCheckoutPolicy(); e == nil {
		t.Fatal("unbounded timeout")
	}
	t.Setenv("GIT_FETCH_TIMEOUT_SECONDS", "90")
	t.Setenv("GIT_CHECKOUT_TIMEOUT_SECONDS", "45")
	v, e := readCheckoutPolicy()
	if e != nil || v.fetch != 90*time.Second || v.checkout != 45*time.Second {
		t.Fatalf("%+v %v", v, e)
	}
}
