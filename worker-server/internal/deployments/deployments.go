// Package deployments handles end-to-end deployment processing: cloning, building, and uploading.
package deployments

import (
	"bytes"
	"context"
	"errors"
	"fmt"
	"io"
	"net/url"
	"os"
	"os/exec"
	"path/filepath"
	"regexp"
	"strings"
	"syscall"
	"time"

	"Zero_Devops/worker_server/internal/domain"

	"go.uber.org/zap"
)

const (
	pkgManagerNPM  = "npm"
	pkgManagerPNPM = "pnpm"
	pkgManagerYarn = "yarn"
	pkgManagerBun  = "bun"

	// Python package managers
	pkgManagerUV     = "uv"
	pkgManagerPoetry = "poetry"
	pkgManagerPipenv = "pipenv"
	pkgManagerPip    = "pip"

	templateDockerfile = "Dockerfile"
	builderDocker      = "docker"

	outputDirDist  = "dist"
	outputDirBuild = "build"

	depVike = "vike"

	frameworkVite      = "vite"
	frameworkNextJS    = "nextjs"
	frameworkAstro     = "astro"
	frameworkReact     = "react"
	frameworkAngular   = "angular"
	frameworkNuxt      = "nuxt"
	frameworkSvelteKit = "sveltekit"
	frameworkSvelte    = "svelte"
	frameworkRemix     = "remix"
	frameworkGatsby    = "gatsby"
	frameworkVue       = "vue"
	frameworkSolid     = "solid"
	frameworkTanStack  = "tanstack-start"

	langNode       = "node"
	langGo         = "go"
	langPython     = "python"
	langRuby       = "ruby"
	langRust       = "rust"
	langJavaMaven  = "java-maven"
	langJavaGradle = "java-gradle"
	langPHP        = "php"
	langElixir     = "elixir"
	langDotNet     = "dotnet"

	gitChangeDirFlag = "-C"

	// outputDirPlaceholder is substituted with the resolved output dir (see resolveOutputDir).
	outputDirPlaceholder = "__OUTPUT_DIR__"

	// ssrMismatchSentinel marks a build that succeeded but produced no index.html,
	// distinguishing it from a genuine build error.
	ssrMismatchSentinel = "ZERODEVOPS_SSR_MISMATCH"

	maxCapturedOutputInError = 500
)

var pmInstallCommands = map[string]string{
	pkgManagerNPM:  "npm ci --ignore-scripts",
	pkgManagerPNPM: "pnpm install --frozen-lockfile --ignore-scripts",
	pkgManagerYarn: "yarn install --frozen-lockfile --ignore-scripts",
	pkgManagerBun:  "bun install --frozen-lockfile --ignore-scripts",
}

// Production dependency installation is not a syntactic substitution for npm's
// --omit=dev: pnpm, yarn, and bun use different flags.
var pmProductionInstallCommands = map[string]string{
	pkgManagerNPM:  "npm ci --ignore-scripts --omit=dev",
	pkgManagerPNPM: "pnpm install --frozen-lockfile --ignore-scripts --prod",
	pkgManagerYarn: "yarn install --frozen-lockfile --ignore-scripts --production=true",
	pkgManagerBun:  "bun install --frozen-lockfile --ignore-scripts --production",
}

var pmManifestCopies = map[string]string{
	pkgManagerNPM:  "COPY package.json package-lock.json ./",
	pkgManagerPNPM: "COPY package.json pnpm-lock.yaml ./",
	pkgManagerYarn: "COPY package.json yarn.lock ./",
	pkgManagerBun:  "COPY package.json bun.lock* ./",
}

var buildRoot = func() string {
	if root := os.Getenv("BUILD_ROOT"); root != "" {
		return root
	}
	return filepath.Join(os.TempDir(), "zerodevops-build")
}()

const gitCloneTimeout = 60 * time.Second

func validateCloneURL(rawURL string) error {
	u, err := url.Parse(rawURL)
	if err != nil {
		return fmt.Errorf("invalid clone URL: %w", err)
	}
	if u.Scheme != "https" {
		return fmt.Errorf("unsupported clone URL scheme: %s", u.Scheme)
	}
	if strings.HasPrefix(rawURL, "-") {
		return errors.New("clone URL must not start with '-'")
	}
	allowedHosts := map[string]bool{"github.com": true}
	if !allowedHosts[u.Hostname()] {
		return fmt.Errorf("clone URL host not allowed: %s", u.Hostname())
	}
	return nil
}

// commitSHAPattern matches a full immutable commit SHA. The contract already
// validates this server-side; this is defense in depth before any git command
// receives the value.
var commitSHAPattern = regexp.MustCompile(`^[0-9a-f]{40}$`)

// cloneRepo materializes the exact commitSHA in destPath. It never clones the
// moving default-branch tip: the worktree is created from the immutable commit
// the job was accepted for. GitHub supports fetching an arbitrary reachable SHA.
func cloneRepo(cloneURL, deploymentID, commitSHA string, credential domain.CloneCredential) (string, error) {
	if err := validateCloneURL(cloneURL); err != nil {
		return "", fmt.Errorf("clone rejected: %w", err)
	}
	if !commitSHAPattern.MatchString(commitSHA) {
		return "", fmt.Errorf("clone rejected: invalid commit SHA %q", commitSHA)
	}

	destPath := filepath.Join(buildRoot, deploymentID)

	if err := os.RemoveAll(destPath); err != nil {
		return "", err
	}

	if err := os.MkdirAll(buildRoot, 0o750); err != nil { //nolint:mnd // directory permission
		return "", err
	}

	ctx, cancel := context.WithTimeout(context.Background(), gitCloneTimeout)
	defer cancel()

	//nolint:gosec // cloneURL validated by validateCloneURL and commitSHA by commitSHAPattern above
	setupCommands := [][]string{
		{"init", destPath},
		{gitChangeDirFlag, destPath, "remote", "add", "origin", cloneURL},
	}
	for _, args := range setupCommands {
		if err := runGit(ctx, args); err != nil {
			return "", fmt.Errorf("git %s failed: %w", args[0], err)
		}
	}

	askpassPath, cleanupAskpass, err := createGitAskPass(credential)
	if err != nil {
		return "", err
	}
	defer cleanupAskpass()
	gitEnv := []string{
		"GIT_TERMINAL_PROMPT=0",
		"GIT_ASKPASS=" + askpassPath,
		"GITHUB_INSTALLATION_TOKEN=" + credential.Token,
	}
	if err := runGitWithEnv(ctx, []string{gitChangeDirFlag, destPath, "fetch", "--depth", "1", "origin", commitSHA}, gitEnv); err != nil {
		return "", fmt.Errorf("git fetch failed: %w", err)
	}
	if err := runGit(ctx, []string{gitChangeDirFlag, destPath, "checkout", "FETCH_HEAD"}); err != nil {
		return "", fmt.Errorf("git checkout failed: %w", err)
	}

	return destPath, nil
}

const (
	gitWaitDelay    = 5 * time.Second
	gitAskPassPerms = 0o700
)

// createGitAskPass keeps the token out of the remote URL and Git config. Git
// receives the token only through the child process environment during fetch.
func createGitAskPass(credential domain.CloneCredential) (path string, cleanup func(), err error) {
	if credential.Username != "x-access-token" || credential.Token == "" {
		return "", nil, errors.New("invalid GitHub clone credential")
	}
	file, err := os.CreateTemp("", "zerodevops-git-askpass-*")
	if err != nil {
		return "", nil, err
	}
	path = file.Name()
	cleanup = func() { _ = os.Remove(path) }
	if err := file.Chmod(gitAskPassPerms); err != nil {
		_ = file.Close()
		cleanup()
		return "", nil, err
	}
	// The token is read from the short-lived environment of git, never written
	// to disk. Git's prompts contain either Username or Password.
	script := "#!/bin/sh\n" +
		"case \"$1\" in\n" +
		"  *Username*) printf '%s\\n' 'x-access-token' ;;\n" +
		"  *Password*) printf '%s\\n' \"$GITHUB_INSTALLATION_TOKEN\" ;;\n" +
		"esac\n"
	if _, err := file.WriteString(script); err != nil {
		_ = file.Close()
		cleanup()
		return "", nil, err
	}
	if err := file.Close(); err != nil {
		cleanup()
		return "", nil, err
	}
	return path, cleanup, nil
}

// runGit executes a git command with the shared clone timeout and captures stderr.
//
// git spawns a git-remote-https child that inherits our stdout/stderr pipes;
// CommandContext's default cancel only kills the direct process, so a child
// stuck on a network read leaves the pipes open and Wait() hangs past ctx's
// deadline. Setpgid+Cancel kill the whole process group; WaitDelay force-closes
// the pipes as a backstop if anything survives that.
func runGit(ctx context.Context, args []string) error {
	return runGitWithEnv(ctx, args, nil)
}

func runGitWithEnv(ctx context.Context, args, extraEnv []string) error {
	cmd := exec.CommandContext(ctx, "git", args...) //nolint:gosec // arguments are constructed internally above
	cmd.Env = append(os.Environ(), extraEnv...)
	cmd.SysProcAttr = &syscall.SysProcAttr{Setpgid: true}
	cmd.Cancel = func() error {
		if cmd.Process == nil {
			return nil
		}
		return syscall.Kill(-cmd.Process.Pid, syscall.SIGKILL) // negative pid = whole group
	}
	cmd.WaitDelay = gitWaitDelay

	var stderr bytes.Buffer
	cmd.Stderr = &stderr

	if err := cmd.Run(); err != nil {
		if ctx.Err() == context.DeadlineExceeded {
			return ctx.Err()
		}
		if stderr.Len() > 0 {
			return errors.New(stderr.String())
		}
		return err
	}
	return nil
}

func publishStatusUpdate(queueUsecase domain.QueueUsecase, deploymentID, status, outputURL, errorMessage string) error {
	return queueUsecase.PublishStatusUpdate(domain.DeployStatusMessage{
		DeploymentID: deploymentID,
		Status:       status,
		OutputURL:    outputURL,
		ErrorMessage: errorMessage,
	})
}

// renderNodePackageManagerTemplate makes a Node template match the repository's
// lockfile. pnpm and yarn are activated with Corepack in every stage that runs
// them; Corepack honors a version pinned in package.json's packageManager field.
func renderNodePackageManagerTemplate(content, pm string) string {
	if pm == pkgManagerNPM {
		return content
	}

	installCmd, knownManager := pmInstallCommands[pm]
	productionInstallCmd := pmProductionInstallCommands[pm]
	manifestCopy := pmManifestCopies[pm]
	if !knownManager || productionInstallCmd == "" || manifestCopy == "" {
		return content
	}

	// package*.json excludes pnpm-lock.yaml, yarn.lock, and Bun lockfiles.
	// Copying the detected lockfile before installation makes frozen installs
	// reproducible and preserves Docker-layer caching for normal single-package
	// projects.
	content = strings.ReplaceAll(content, "COPY package*.json ./", manifestCopy)
	content = strings.ReplaceAll(content, "npm ci --ignore-scripts --omit=dev", productionInstallCmd)
	content = strings.ReplaceAll(content, "npm ci --ignore-scripts", installCmd)
	content = strings.ReplaceAll(content, "npm run", pm+" run")
	content = strings.ReplaceAll(content, `"npm`, `"`+pm)

	switch pm {
	case pkgManagerPNPM, pkgManagerYarn:
		// node:20 includes Corepack but it is disabled by default. Prefix every
		// install command, including the production runtime stage.
		content = strings.ReplaceAll(content, "RUN "+installCmd, "RUN corepack enable\nRUN "+installCmd)
		content = strings.ReplaceAll(content, "RUN "+productionInstallCmd, "RUN corepack enable\nRUN "+productionInstallCmd)
	case pkgManagerBun:
		// Only swap the builder stage and unnamed runtime stages — a stage named
		// "AS runtime" serves static files via npm's bundled `serve` and must
		// remain Node regardless of the package manager that built the project.
		content = strings.ReplaceAll(content, "FROM node:20-alpine AS builder", "FROM docker.io/oven/bun:1-alpine AS builder")
		content = strings.ReplaceAll(content, "FROM node:20-alpine\n", "FROM docker.io/oven/bun:1-alpine\n")
	}
	return content
}

// renderPythonPackageManagerTemplate makes a Python template match the repository's
// lockfile by replacing the dependency installation placeholder.
func renderPythonPackageManagerTemplate(content, pm string) string {
	switch pm {
	case pkgManagerUV:
		// uv: use the official uv image for faster, cached installs
		// Install uv in the base image, then sync dependencies
		content = strings.ReplaceAll(content, "__PYTHON_DEPS_INSTALL__", `COPY --from=ghcr.io/astral-sh/uv:latest /uv /usr/local/bin/uv
COPY pyproject.toml uv.lock ./
RUN uv sync --locked --no-dev`)
	case pkgManagerPoetry:
		// Poetry: install poetry and install dependencies
		content = strings.ReplaceAll(content, "__PYTHON_DEPS_INSTALL__", `RUN pip install --no-cache-dir poetry
COPY pyproject.toml poetry.lock ./
RUN poetry install --no-dev --no-interaction --no-ansi`)
	case pkgManagerPipenv:
		// Pipenv: install pipenv and install dependencies
		content = strings.ReplaceAll(content, "__PYTHON_DEPS_INSTALL__", `RUN pip install --no-cache-dir pipenv
COPY Pipfile Pipfile.lock ./
RUN pipenv install --deploy --ignore-pipfile`)
	case pkgManagerPip:
		// pip: traditional requirements.txt approach
		content = strings.ReplaceAll(content, "__PYTHON_DEPS_INSTALL__", `COPY requirements.txt ./
RUN pip install --no-cache-dir -r requirements.txt`)
	default:
		// Fallback to pip/requirements.txt for backward compatibility
		content = strings.ReplaceAll(content, "__PYTHON_DEPS_INSTALL__", `COPY requirements.txt ./
RUN pip install --no-cache-dir -r requirements.txt`)
	}
	return content
}

func writeDockerfile(repoPath string, builder *Builder, pm string) error {
	templatePath := filepath.Join("templates", builder.Template)
	if builder.Template == templateDockerfile {
		return nil
	}

	//nolint:gosec // path is constructed internally, not user input
	data, err := os.ReadFile(templatePath)
	if err != nil {
		if errors.Is(err, os.ErrNotExist) {
			altPath := filepath.Join(parentDir, parentDir, "templates", builder.Template)
			//nolint:gosec // altPath is constructed internally, not user input
			if altData, altErr := os.ReadFile(altPath); altErr == nil {
				data = altData
				err = nil
			}
		}
		if err != nil {
			return err
		}
	}

	content := string(data)

	// Apply package-manager-specific template rendering
	if builder.Name == langPython {
		content = renderPythonPackageManagerTemplate(content, pm)
	} else {
		content = renderNodePackageManagerTemplate(content, pm)
	}

	if outDir := resolveOutputDir(repoPath, builder); outDir != "" {
		content = strings.ReplaceAll(content, outputDirPlaceholder, outDir)
	}

	//nolint:gosec // repoPath is from cloneRepo which returns a controlled path
	return os.WriteFile(filepath.Join(repoPath, templateDockerfile), []byte(content), 0o600) //nolint:mnd // file permission
}

// buildImage runs buildah, streaming output live while also capturing it so a
// failure can be classified afterward (build error vs SSR mismatch).
func buildImage(ctx context.Context, repoPath, imageTag string) (string, error) {
	//nolint:gosec // repoPath is from cloneRepo which validates the URL
	cmd := exec.CommandContext(ctx, "buildah", "bud",
		"-t", imageTag,
		"-f", filepath.Join(repoPath, templateDockerfile),
		repoPath,
	)
	var captured bytes.Buffer
	cmd.Stdout = io.MultiWriter(os.Stdout, &captured)
	cmd.Stderr = io.MultiWriter(os.Stderr, &captured)
	err := cmd.Run()
	return captured.String(), err
}

func saveImageTar(ctx context.Context, imageTag, tarPath string) error {
	//nolint:gosec // imageTag/tarPath are constructed internally
	cmd := exec.CommandContext(ctx, "buildah", "push", imageTag, "docker-archive:"+tarPath)
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr
	return cmd.Run()
}

// markFailed records the attempt locally and returns an error, but deliberately
// does NOT broadcast "failed" to the server — "failed" is terminal server-side,
// and this fires on attempts the worker loop is about to retry. worker.go is the
// only place that knows whether an attempt was final, and already publishes
// "canceled" once retries are exhausted — that's where terminal status belongs.
func markFailed(ctx context.Context, repo domain.DeploymentRepository, job domain.DeployJob, errMsg string) error {
	_ = repo.MarkFailed(ctx, job.DeploymentID, errMsg)
	return errors.New(errMsg)
}

func prepareAndMarkBuilding(
	ctx context.Context, repo domain.DeploymentRepository, job domain.DeployJob,
	queueUsecase domain.QueueUsecase, retryCount int, logger *zap.Logger,
) error {
	logger.Info("inserting worker deployment row", zap.String("deployment_id", job.DeploymentID))
	if retryCount == 0 {
		if err := repo.Insert(ctx, job); err != nil {
			return err
		}
	}

	logger.Info("marking as building", zap.String("deployment_id", job.DeploymentID))
	if err := repo.MarkBuilding(ctx, job.DeploymentID); err != nil {
		return err
	}

	logger.Info("publishing building status", zap.String("deployment_id", job.DeploymentID))
	return publishStatusUpdate(queueUsecase, job.DeploymentID, "building", "", "")
}

func cloneAndPrepare(appDir string, job domain.DeployJob, logger *zap.Logger, useBuildpacks bool) (*Builder, string, error) {
	logger.Info("detecting framework", zap.String("deployment_id", job.DeploymentID))
	builder, err := detectFramework(appDir)
	if err != nil {
		return nil, "", err
	}
	pm := detectPackageManager(appDir)
	logger.Info("framework detected", zap.String("deployment_id", job.DeploymentID), zap.String("framework", builder.Name), zap.String("package_manager", pm))

	if useBuildpacks {
		logger.Info("using Google Cloud Buildpacks for build", zap.String("deployment_id", job.DeploymentID))
		return builder, pm, nil
	}

	if builder.Name != builderDocker {
		logger.Info("writing Dockerfile from template", zap.String("deployment_id", job.DeploymentID), zap.String("template", builder.Template))
		if err := writeDockerfile(appDir, builder, pm); err != nil {
			return nil, "", err
		}
	}
	return builder, pm, nil
}

// buildAndSaveImage builds the image and saves it to a tar, returning the captured
// build output alongside any error so the caller can classify the failure.
func buildAndSaveImage(ctx context.Context, job domain.DeployJob, appDir, imageTag string, logger *zap.Logger) (tarPath, buildOutput string, err error) {
	logger.Info("building image with buildah", zap.String("deployment_id", job.DeploymentID), zap.String("image_tag", imageTag))
	buildOutput, err = buildImage(ctx, appDir, imageTag)
	if err != nil {
		return "", buildOutput, err
	}

	tarPath = filepath.Join(appDir, fmt.Sprintf("%s.tar", job.DeploymentID))
	logger.Info("saving image tar", zap.String("deployment_id", job.DeploymentID), zap.String("tar_path", tarPath))
	if err := saveImageTar(ctx, imageTag, tarPath); err != nil {
		return "", buildOutput, err
	}
	return tarPath, buildOutput, nil
}

// truncate bounds s to n runes for embedding in log fields and stored error messages.
func truncate(s string, n int) string {
	if len(s) <= n {
		return s
	}
	return s[:n] + "... (truncated)"
}

// retryAsDynamic handles an SSR-mismatch build failure by retrying once through the
// generic dynamic Node path — only when a "start" script proves a server entrypoint
// actually exists, never a blind guess.
func retryAsDynamic(ctx context.Context, appDir string, job domain.DeployJob, imageTag string, logger *zap.Logger) (string, error) {
	pkg, err := readPackageJSON(appDir)
	if err != nil || pkg == nil || !hasStartScript(pkg) {
		return "", errors.New(`no runnable server entrypoint found (no "start" script in package.json) — set deployment_type manually in project settings`)
	}

	logger.Info("SSR mismatch: retrying once via generic dynamic path",
		zap.String("deployment_id", job.DeploymentID))

	pm := detectPackageManager(appDir)
	if err := writeDockerfile(appDir, nodeBuilder, pm); err != nil {
		return "", fmt.Errorf("writing dynamic fallback Dockerfile: %w", err)
	}

	output, err := buildImage(ctx, appDir, imageTag)
	if err != nil {
		return "", fmt.Errorf("dynamic fallback build also failed: %w (output: %s)", err, truncate(output, maxCapturedOutputInError))
	}

	tarPath := filepath.Join(appDir, fmt.Sprintf("%s.tar", job.DeploymentID))
	if err := saveImageTar(ctx, imageTag, tarPath); err != nil {
		return "", fmt.Errorf("saving dynamic fallback image tar: %w", err)
	}
	return tarPath, nil
}

// uploadAndFinalize is the static-build finalize path: uploads the tar to R2 and
// broadcasts success with the real output URL.
func uploadAndFinalize(
	ctx context.Context, repo domain.DeploymentRepository, job domain.DeployJob,
	artifactUploader domain.UploadUsecase, queueUsecase domain.QueueUsecase,
	tarPath string, logger *zap.Logger,
) error {
	logger.Info("uploading image tar", zap.String("deployment_id", job.DeploymentID))
	outputURL, err := artifactUploader.UploadImage(tarPath)
	if err != nil {
		return err
	}

	if err := repo.UpdateOutputURL(ctx, job.DeploymentID, outputURL); err != nil {
		return err
	}

	logger.Info("marking as finished", zap.String("deployment_id", job.DeploymentID))
	if err := repo.MarkFinished(ctx, job.DeploymentID, outputURL); err != nil {
		return err
	}

	return publishStatusUpdate(queueUsecase, job.DeploymentID, "success", outputURL, "")
}

// finalizeDynamicStub is the dynamic-build finalize path — no registry/runner
// exists yet to push a dynamic image to, so this marks the deployment successful
// without uploading anywhere. Placeholder until that upload path is built.
func finalizeDynamicStub(
	ctx context.Context, repo domain.DeploymentRepository, job domain.DeployJob,
	queueUsecase domain.QueueUsecase, logger *zap.Logger,
) error {
	logger.Info("dynamic build finished (stub finalize, no upload yet)", zap.String("deployment_id", job.DeploymentID))
	if err := repo.MarkFinished(ctx, job.DeploymentID, ""); err != nil {
		return err
	}
	return publishStatusUpdate(queueUsecase, job.DeploymentID, "success", "", "")
}

// ProcessDeployment processes a deployment job end-to-end: preparing, cloning, building, and uploading.
func ProcessDeployment(
	ctx context.Context, repo domain.DeploymentRepository, job domain.DeployJob, credential domain.CloneCredential,
	artifactUploader domain.UploadUsecase, queueUsecase domain.QueueUsecase,
	retryCount int, logger *zap.Logger,
) error {
	if err := prepareAndMarkBuilding(ctx, repo, job, queueUsecase, retryCount, logger); err != nil {
		return err
	}

	imageTag, err := repo.ReadImageTag(ctx, job.DeploymentID)
	if err != nil {
		return markFailed(ctx, repo, job, "failed to read image tag: "+err.Error())
	}

	repoPath, err := cloneRepo(job.CloneURL, job.DeploymentID, job.CommitSHA, credential)
	if err != nil {
		return markFailed(ctx, repo, job, "git checkout failed: "+err.Error())
	}

	defer func() { _ = os.RemoveAll(repoPath) }()

	appDir, err := ResolveAppDir(repoPath, job.WorkingDir)
	if err != nil {
		return markFailed(ctx, repo, job, "resolve application directory failed: "+err.Error())
	}

	builder, _, err := cloneAndPrepare(appDir, job, logger, false)
	if err != nil {
		return markFailed(ctx, repo, job, "clone/prepare failed: "+err.Error())
	}
	isStatic := builder.DefaultOutputDir != ""

	const dockerBuildTimeout = 15 * time.Minute
	buildCtx, cancel := context.WithTimeout(context.Background(), dockerBuildTimeout)
	defer cancel()

	tarPath, buildOutput, err := buildAndSaveImage(buildCtx, job, appDir, imageTag, logger)
	if err != nil {
		if !strings.Contains(buildOutput, ssrMismatchSentinel) {
			return markFailed(ctx, repo, job, "build failed: "+err.Error())
		}

		retryTarPath, retryErr := retryAsDynamic(buildCtx, appDir, job, imageTag, logger)
		if retryErr != nil {
			return markFailed(ctx, repo, job,
				"detected server-side rendering (build succeeded but produced no static output): "+retryErr.Error())
		}
		tarPath = retryTarPath
		isStatic = false
	}
	defer func() { _ = os.Remove(tarPath) }()

	if isStatic {
		return uploadAndFinalize(ctx, repo, job, artifactUploader, queueUsecase, tarPath, logger)
	}
	return finalizeDynamicStub(ctx, repo, job, queueUsecase, logger)
}
