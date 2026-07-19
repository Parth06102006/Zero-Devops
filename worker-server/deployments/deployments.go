// Package deployments handles deployment processing including cloning, building, and uploading.
package deployments

import (
	"bufio"
	"bytes"
	"context"
	"database/sql"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/url"
	"os"
	"os/exec"
	"path/filepath"
	"strconv"
	"strings"
	"time"

	"Zero_Devops/worker_server/domain"

	"github.com/moby/go-archive"
	"github.com/moby/moby/client"
	"go.uber.org/zap"
)

const (
	buildRoot = "C:\\tmp\\build"

	pkgManagerNPM  = "npm"
	pkgManagerPNPM = "pnpm"
	pkgManagerYarn = "yarn"
	pkgManagerBun  = "bun"

	templateDockerfile = "Dockerfile"
	builderDocker      = "docker"

	frameworkVite   = "vite"
	frameworkNextJS = "nextjs"
	frameworkAstro  = "astro"
	frameworkReact  = "react"
	langNode        = "node"
	langGo          = "go"
	langPython      = "python"
)

var pmInstallCommands = map[string]string{
	pkgManagerNPM:  "npm ci --ignore-scripts",
	pkgManagerPNPM: "pnpm install --frozen-lockfile --ignore-scripts",
	pkgManagerYarn: "yarn install --frozen-lockfile --ignore-scripts",
	pkgManagerBun:  "bun install --frozen-lockfile --ignore-scripts",
}

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

func cloneRepo(cloneURL, deploymentID string) (string, error) {
	if err := validateCloneURL(cloneURL); err != nil {
		return "", fmt.Errorf("clone rejected: %w", err)
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

	cmd := exec.CommandContext(ctx, "git", "clone", "--depth", "1", cloneURL, destPath) //nolint:gosec // cloneURL validated by validateCloneURL above

	var stderr bytes.Buffer
	cmd.Stderr = &stderr

	if err := cmd.Run(); err != nil {
		if ctx.Err() == context.DeadlineExceeded {
			return "", err
		}
		if stderr.Len() > 0 {
			return "", fmt.Errorf("git clone failed: %s", stderr.String())
		}
		return "", errors.New("git clone failed")
	}

	return destPath, nil
}

func publishStatusUpdate(queueUsecase domain.QueueUsecase, deploymentID int64, status string) error {
	return queueUsecase.PublishStatusUpdate(domain.DeployStatusMessage{
		DeploymentID: deploymentID,
		Status:       status,
	})
}

func updateStatus(ctx context.Context, db *sql.DB, job domain.DeployJob, status string) error {
	query := `UPDATE deployments SET status = $1 , retry_count = $2 WHERE id = $3`

	stmt, err := db.PrepareContext(ctx, query)
	if err != nil {
		return err
	}
	defer func() { _ = stmt.Close() }()

	_, err = stmt.ExecContext(ctx, status, job.RetryCount, job.DeploymentID)
	return err
}

func insertDeployment(ctx context.Context, db *sql.DB, job domain.DeployJob) error {
	imageTag := fmt.Sprintf("deploy-%d:latest", job.DeploymentID)
	query := `
		INSERT INTO deployments (id, clone_url, status, retry_count, image_tag)
		VALUES ($1, $2, 'queued', $3, $4)
		ON CONFLICT (id) DO UPDATE SET
			clone_url = EXCLUDED.clone_url,
			status = 'queued',
			retry_count = EXCLUDED.retry_count,
			image_tag = EXCLUDED.image_tag,
			updated_at = now()
	`

	stmt, err := db.PrepareContext(ctx, query)
	if err != nil {
		return err
	}
	defer func() { _ = stmt.Close() }()

	_, err = stmt.ExecContext(ctx, job.DeploymentID, job.CloneURL, job.RetryCount, imageTag)
	return err
}

func updateOutputURL(ctx context.Context, db *sql.DB, job domain.DeployJob, outputURL string) error {
	query := `UPDATE deployments SET output_url = $1 WHERE id = $2`

	stmt, err := db.PrepareContext(ctx, query)
	if err != nil {
		return err
	}
	defer func() { _ = stmt.Close() }()

	_, err = stmt.ExecContext(ctx, outputURL, job.DeploymentID)
	return err
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

func buildImage(ctx context.Context, cli *client.Client, repoPath, imageTag string) error {
	buildCtx, err := archive.TarWithOptions(repoPath, &archive.TarOptions{})
	if err != nil {
		return err
	}
	defer func() { _ = buildCtx.Close() }()

	opts := client.ImageBuildOptions{
		Dockerfile: templateDockerfile,
		Tags:       []string{imageTag},
		Remove:     true,
	}

	result, err := cli.ImageBuild(ctx, buildCtx, opts)
	if err != nil {
		return err
	}
	defer func() { _ = result.Body.Close() }()

	scanner := bufio.NewScanner(result.Body)
	var lastLine string
	for scanner.Scan() {
		lastLine = scanner.Text()
		fmt.Println(lastLine)
	}
	if err := scanner.Err(); err != nil {
		return err
	}

	var errCheck struct {
		Error string `json:"error"`
	}
	if lastLine != "" {
		_ = json.Unmarshal([]byte(lastLine), &errCheck)
	}
	if errCheck.Error != "" {
		return fmt.Errorf("build failed: %s", errCheck.Error)
	}

	return nil
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

func markFailed(ctx context.Context, db *sql.DB, job domain.DeployJob, queueUsecase domain.QueueUsecase) error {
	_ = updateStatus(ctx, db, job, "failed")
	return publishStatusUpdate(queueUsecase, job.DeploymentID, "failed")
}

func prepareAndMarkBuilding(ctx context.Context, db *sql.DB, job domain.DeployJob, queueUsecase domain.QueueUsecase, retryCount int, logger *zap.Logger) error {
	logger.Info("inserting worker deployment row", zap.Int64("deployment_id", job.DeploymentID))
	if retryCount == 0 {
		if err := insertDeployment(ctx, db, job); err != nil {
			return err
		}
	}

	logger.Info("marking as building", zap.Int64("deployment_id", job.DeploymentID))
	if err := updateStatus(ctx, db, job, "building"); err != nil {
		return err
	}

	logger.Info("publishing building status", zap.Int64("deployment_id", job.DeploymentID))
	return publishStatusUpdate(queueUsecase, job.DeploymentID, "building")
}

func cloneAndPrepare(repoPath string, job domain.DeployJob, logger *zap.Logger) (string, error) {
	logger.Info("detecting framework", zap.Int64("deployment_id", job.DeploymentID))
	builder, err := detectFramework(repoPath)
	if err != nil {
		return "", err
	}
	pm := detectPackageManager(repoPath)
	logger.Info("framework detected", zap.Int64("deployment_id", job.DeploymentID), zap.String("framework", builder.Name), zap.String("package_manager", pm))

	if builder.Name != builderDocker {
		logger.Info("writing Dockerfile from template", zap.Int64("deployment_id", job.DeploymentID), zap.String("template", builder.Template))
		if err := writeDockerfile(repoPath, builder, pm); err != nil {
			return "", err
		}
	}
	return pm, nil
}

func buildAndSaveImage(ctx context.Context, job domain.DeployJob, cli *client.Client, repoPath, imageTag string, logger *zap.Logger) (string, error) {
	logger.Info("building Docker image", zap.Int64("deployment_id", job.DeploymentID), zap.String("image_tag", imageTag))
	if err := buildImage(ctx, cli, repoPath, imageTag); err != nil {
		return "", err
	}

	tarPath := filepath.Join(repoPath, fmt.Sprintf("%d.tar", job.DeploymentID))
	logger.Info("saving Docker image tar", zap.Int64("deployment_id", job.DeploymentID), zap.String("tar_path", tarPath))
	if err := saveImageTar(ctx, cli, imageTag, tarPath); err != nil {
		return "", err
	}
	return tarPath, nil
}

func uploadAndFinalize(
	ctx context.Context, db *sql.DB, job domain.DeployJob,
	artifactUploader domain.UploadUsecase, queueUsecase domain.QueueUsecase,
	tarPath string, logger *zap.Logger,
) error {
	logger.Info("uploading image tar", zap.Int64("deployment_id", job.DeploymentID))
	outputURL, err := artifactUploader.UploadImage(tarPath)
	if err != nil {
		return err
	}

	logger.Info("saving output URL", zap.Int64("deployment_id", job.DeploymentID))
	if err := updateOutputURL(ctx, db, job, outputURL); err != nil {
		return err
	}

	logger.Info("marking as done", zap.Int64("deployment_id", job.DeploymentID))
	if err := updateStatus(ctx, db, job, "done"); err != nil {
		return err
	}

	logger.Info("publishing done status", zap.Int64("deployment_id", job.DeploymentID))
	return publishStatusUpdate(queueUsecase, job.DeploymentID, "done")
}

// ProcessDeployment processes a deployment job end-to-end: preparing, cloning, building, and uploading.
func ProcessDeployment(
	ctx context.Context, db *sql.DB, job domain.DeployJob,
	artifactUploader domain.UploadUsecase, queueUsecase domain.QueueUsecase,
	retryCount int, logger *zap.Logger,
) error {
	deploymentID := strconv.FormatInt(job.DeploymentID, 10)

	if err := prepareAndMarkBuilding(ctx, db, job, queueUsecase, retryCount, logger); err != nil {
		return err
	}

	repoPath, err := cloneRepo(job.CloneURL, deploymentID)
	if err != nil {
		return markFailed(ctx, db, job, queueUsecase)
	}

	defer func() { _ = os.RemoveAll(repoPath) }()

	if _, err := cloneAndPrepare(repoPath, job, logger); err != nil {
		return markFailed(ctx, db, job, queueUsecase)
	}

	cli, err := client.New(client.FromEnv)
	if err != nil {
		return markFailed(ctx, db, job, queueUsecase)
	}

	imageTag := fmt.Sprintf("deploy-%d:latest", job.DeploymentID)
	const dockerBuildTimeout = 5 * time.Minute
	buildCtx, cancel := context.WithTimeout(context.Background(), dockerBuildTimeout)
	defer cancel()

	tarPath, err := buildAndSaveImage(buildCtx, job, cli, repoPath, imageTag, logger)
	if err != nil {
		return markFailed(ctx, db, job, queueUsecase)
	}
	defer func() { _ = os.Remove(tarPath) }()

	return uploadAndFinalize(ctx, db, job, artifactUploader, queueUsecase, tarPath, logger)
}
