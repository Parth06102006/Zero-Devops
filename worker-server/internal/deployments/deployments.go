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
	"time"

	"Zero_Devops/worker_server/internal/domain"

	"github.com/moby/moby/client"
	"go.uber.org/zap"
)

const (
	pkgManagerNPM  = "npm"
	pkgManagerPNPM = "pnpm"
	pkgManagerYarn = "yarn"
	pkgManagerBun  = "bun"

	templateDockerfile = "Dockerfile"
	builderDocker      = "docker"

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
)

var pmInstallCommands = map[string]string{
	pkgManagerNPM:  "npm ci --ignore-scripts",
	pkgManagerPNPM: "pnpm install --frozen-lockfile --ignore-scripts",
	pkgManagerYarn: "yarn install --frozen-lockfile --ignore-scripts",
	pkgManagerBun:  "bun install --frozen-lockfile --ignore-scripts",
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
func cloneRepo(cloneURL, deploymentID, commitSHA string) (string, error) {
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
	commands := [][]string{
		{"init", destPath},
		{gitChangeDirFlag, destPath, "remote", "add", "origin", cloneURL},
		{gitChangeDirFlag, destPath, "fetch", "--depth", "1", "origin", commitSHA},
		{gitChangeDirFlag, destPath, "checkout", "FETCH_HEAD"},
	}
	for _, args := range commands {
		if err := runGit(ctx, args); err != nil {
			return "", fmt.Errorf("git %s failed: %w", args[0], err)
		}
	}

	return destPath, nil
}

// runGit executes a git command with the shared clone timeout and captures
// stderr for diagnostics.
func runGit(ctx context.Context, args []string) error {
	cmd := exec.CommandContext(ctx, "git", args...) //nolint:gosec // arguments are constructed internally above

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

func writeDockerfile(repoPath string, builder *Builder, pm string) error {
	templatePath := filepath.Join("templates", builder.Template)
	if builder.Template == templateDockerfile {
		return nil
	}

	//nolint:gosec // path is constructed internally, not user input
	data, err := os.ReadFile(templatePath)
	if err != nil {
		return err
	}

	content := string(data)
	installCmd := pmInstallCommands[pm]
	if installCmd != "" && pm != pkgManagerNPM {
		if pm == pkgManagerBun {
			content = strings.ReplaceAll(content, "FROM node:20-alpine", "FROM oven/bun:1-alpine")
			content = strings.ReplaceAll(content, "--omit=dev", "--production")
		}
		content = strings.ReplaceAll(content, "npm ci --ignore-scripts", installCmd)
		content = strings.ReplaceAll(content, "npm run", pm+" run")
		content = strings.ReplaceAll(content, `"npm`, `"`+pm)
	}

	//nolint:gosec // repoPath is from cloneRepo which returns a controlled path
	return os.WriteFile(filepath.Join(repoPath, templateDockerfile), []byte(content), 0o600) //nolint:mnd // file permission
}

func buildImage(ctx context.Context, repoPath, imageTag string) error {
	//nolint:gosec // repoPath is from cloneRepo which validates the URL
	cmd := exec.CommandContext(ctx, "docker", "build",
		"-t", imageTag,
		"-f", filepath.Join(repoPath, templateDockerfile),
		repoPath,
	)
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr
	return cmd.Run()
}

func saveImageTar(ctx context.Context, cli *client.Client, imageTag, tarPath string) error {
	saveResult, err := cli.ImageSave(ctx, []string{imageTag})
	if err != nil {
		return err
	}
	defer func() { _ = saveResult.Close() }()

	//nolint:gosec // path is constructed internally, not user input
	file, err := os.Create(tarPath)
	if err != nil {
		return err
	}
	defer func() { _ = file.Close() }()

	if _, err := io.Copy(file, saveResult); err != nil {
		return err
	}

	return nil
}

func markFailed(ctx context.Context, repo domain.DeploymentRepository, job domain.DeployJob, queueUsecase domain.QueueUsecase, errMsg string) error {
	_ = repo.MarkFailed(ctx, job.DeploymentID, errMsg)
	return publishStatusUpdate(queueUsecase, job.DeploymentID, "failed", "", errMsg)
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

func cloneAndPrepare(repoPath string, job domain.DeployJob, logger *zap.Logger, useBuildpacks bool) (string, error) {
	logger.Info("detecting framework", zap.String("deployment_id", job.DeploymentID))
	builder, err := detectFramework(repoPath)
	if err != nil {
		return "", err
	}
	pm := detectPackageManager(repoPath)
	logger.Info("framework detected", zap.String("deployment_id", job.DeploymentID), zap.String("framework", builder.Name), zap.String("package_manager", pm))

	if useBuildpacks {
		logger.Info("using Google Cloud Buildpacks for build", zap.String("deployment_id", job.DeploymentID))
		return pm, nil
	}

	if builder.Name != builderDocker {
		logger.Info("writing Dockerfile from template", zap.String("deployment_id", job.DeploymentID), zap.String("template", builder.Template))
		if err := writeDockerfile(repoPath, builder, pm); err != nil {
			return "", err
		}
	}
	return pm, nil
}

func buildAndSaveImage(ctx context.Context, job domain.DeployJob, cli *client.Client, repoPath, imageTag string, logger *zap.Logger) (string, error) {
	logger.Info("building Docker image", zap.String("deployment_id", job.DeploymentID), zap.String("image_tag", imageTag))
	if err := buildImage(ctx, repoPath, imageTag); err != nil {
		return "", err
	}

	tarPath := filepath.Join(repoPath, fmt.Sprintf("%s.tar", job.DeploymentID))
	logger.Info("saving Docker image tar", zap.String("deployment_id", job.DeploymentID), zap.String("tar_path", tarPath))
	if err := saveImageTar(ctx, cli, imageTag, tarPath); err != nil {
		return "", err
	}
	return tarPath, nil
}

func uploadAndFinalize(
	ctx context.Context, repo domain.DeploymentRepository, job domain.DeployJob,
	artifactUploader domain.UploadUsecase, _ domain.QueueUsecase,
	tarPath string, logger *zap.Logger,
) error {
	logger.Info("uploading image tar", zap.String("deployment_id", job.DeploymentID))
	outputURL, err := artifactUploader.UploadImage(tarPath)
	if err != nil {
		return err
	}

	logger.Info("saving output URL", zap.String("deployment_id", job.DeploymentID))
	if err := repo.UpdateOutputURL(ctx, job.DeploymentID, outputURL); err != nil {
		return err
	}

	logger.Info("marking as finished", zap.String("deployment_id", job.DeploymentID))
	return repo.MarkFinished(ctx, job.DeploymentID, outputURL)
}

// ProcessDeployment processes a deployment job end-to-end: preparing, cloning, building, and uploading.
func ProcessDeployment(
	ctx context.Context, repo domain.DeploymentRepository, job domain.DeployJob,
	artifactUploader domain.UploadUsecase, queueUsecase domain.QueueUsecase,
	retryCount int, logger *zap.Logger,
) error {
	if err := prepareAndMarkBuilding(ctx, repo, job, queueUsecase, retryCount, logger); err != nil {
		return err
	}

	imageTag, err := repo.ReadImageTag(ctx, job.DeploymentID)
	if err != nil {
		return markFailed(ctx, repo, job, queueUsecase, "failed to read image tag: "+err.Error())
	}

	repoPath, err := cloneRepo(job.CloneURL, job.DeploymentID, job.CommitSHA)
	if err != nil {
		return markFailed(ctx, repo, job, queueUsecase, "git checkout failed: "+err.Error())
	}

	defer func() { _ = os.RemoveAll(repoPath) }()

	if _, err := cloneAndPrepare(repoPath, job, logger, false); err != nil {
		return markFailed(ctx, repo, job, queueUsecase, "clone/prepare failed: "+err.Error())
	}

	cli, err := client.New(client.FromEnv)
	if err != nil {
		return markFailed(ctx, repo, job, queueUsecase, "docker client error: "+err.Error())
	}

	const dockerBuildTimeout = 5 * time.Minute
	buildCtx, cancel := context.WithTimeout(context.Background(), dockerBuildTimeout)
	defer cancel()

	tarPath, err := buildAndSaveImage(buildCtx, job, cli, repoPath, imageTag, logger)
	if err != nil {
		return markFailed(ctx, repo, job, queueUsecase, "build failed: "+err.Error())
	}
	defer func() { _ = os.Remove(tarPath) }()

	return uploadAndFinalize(ctx, repo, job, artifactUploader, queueUsecase, tarPath, logger)
}
