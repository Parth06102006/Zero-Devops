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

const buildRoot = "C:\\tmp\\build"

var pmInstallCommands = map[string]string{
	"npm":  "npm ci --ignore-scripts",
	"pnpm": "pnpm install --frozen-lockfile --ignore-scripts",
	"yarn": "yarn install --frozen-lockfile --ignore-scripts",
	"bun":  "bun install --frozen-lockfile --ignore-scripts",
}

func cloneRepo(cloneURL string, deploymentID string) (string, error) {
	destPath := filepath.Join(buildRoot, deploymentID)

	if err := os.RemoveAll(destPath); err != nil {
		return "", err
	}

	if err := os.MkdirAll(buildRoot, 0o755); err != nil {
		return "", err
	}

	ctx, cancel := context.WithTimeout(context.Background(), 60*time.Second)
	defer cancel()

	cmd := exec.CommandContext(ctx, "git", "clone", "--depth", "1", cloneURL, destPath)

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
	defer stmt.Close()

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
	defer stmt.Close()

	_, err = stmt.ExecContext(ctx, job.DeploymentID, job.CloneURL, job.RetryCount, imageTag)
	return err
}

func updateOutputURL(ctx context.Context, db *sql.DB, job domain.DeployJob, outputURL string) error {
	query := `UPDATE deployments SET output_url = $1 WHERE id = $2`

	stmt, err := db.PrepareContext(ctx, query)
	if err != nil {
		return err
	}
	defer stmt.Close()

	_, err = stmt.ExecContext(ctx, outputURL, job.DeploymentID)
	return err
}

func writeDockerfile(repoPath string, builder *Builder, pm string) error {
	templatePath := filepath.Join("templates", builder.Template)
	if builder.Template == "Dockerfile" {
		return nil
	}

	data, err := os.ReadFile(templatePath)
	if err != nil {
		return err
	}

	content := string(data)
	installCmd := pmInstallCommands[pm]
	if installCmd != "" && pm != "npm" {
		if pm == "bun" {
			content = strings.ReplaceAll(content, "FROM node:20-alpine", "FROM oven/bun:1-alpine")
			content = strings.ReplaceAll(content, "--omit=dev", "--production")
		}
		content = strings.ReplaceAll(content, "npm ci --ignore-scripts", installCmd)
		content = strings.ReplaceAll(content, "npm run", pm+" run")
		content = strings.ReplaceAll(content, `"npm`, `"`+pm)
	}

	return os.WriteFile(filepath.Join(repoPath, "Dockerfile"), []byte(content), 0o644)
}

func buildImage(ctx context.Context, cli *client.Client, repoPath string, imageTag string) error {
	buildCtx, err := archive.TarWithOptions(repoPath, &archive.TarOptions{})
	if err != nil {
		return err
	}
	defer buildCtx.Close()

	opts := client.ImageBuildOptions{
		Dockerfile: "Dockerfile",
		Tags:       []string{imageTag},
		Remove:     true,
	}

	result, err := cli.ImageBuild(ctx, buildCtx, opts)
	if err != nil {
		return err
	}
	defer result.Body.Close()

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
	defer saveResult.Close()

	file, err := os.Create(tarPath)
	if err != nil {
		return err
	}
	defer file.Close()

	if _, err := io.Copy(file, saveResult); err != nil {
		return err
	}

	return nil
}

func ProcessDeployment(ctx context.Context, db *sql.DB, job domain.DeployJob, artifactUploader domain.UploadUsecase, queueUsecase domain.QueueUsecase, retryCount int, logger *zap.Logger) error {
	deploymentID := strconv.FormatInt(job.DeploymentID, 10)

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
	if err := publishStatusUpdate(queueUsecase, job.DeploymentID, "building"); err != nil {
		return err
	}

	logger.Info("cloning repository", zap.Int64("deployment_id", job.DeploymentID))
	repoPath, err := cloneRepo(job.CloneURL, deploymentID)
	if err != nil {
		_ = updateStatus(ctx, db, job, "failed")
		if err := publishStatusUpdate(queueUsecase, job.DeploymentID, "failed"); err != nil {
			return err
		}
		return err
	}

	defer os.RemoveAll(repoPath)

	logger.Info("detecting framework", zap.Int64("deployment_id", job.DeploymentID))
	builder, err := detectFramework(repoPath)
	if err != nil {
		_ = updateStatus(ctx, db, job, "failed")
		if err := publishStatusUpdate(queueUsecase, job.DeploymentID, "failed"); err != nil {
			return err
		}
		return err
	}
	pm := detectPackageManager(repoPath)
	logger.Info("framework detected", zap.Int64("deployment_id", job.DeploymentID), zap.String("framework", builder.Name), zap.String("package_manager", pm))

	if builder.Name != "docker" {
		logger.Info("writing Dockerfile from template", zap.Int64("deployment_id", job.DeploymentID), zap.String("template", builder.Template))
		if err := writeDockerfile(repoPath, builder, pm); err != nil {
			_ = updateStatus(ctx, db, job, "failed")
			if err := publishStatusUpdate(queueUsecase, job.DeploymentID, "failed"); err != nil {
				return err
			}
			return err
		}
	}

	logger.Info("creating Docker client", zap.Int64("deployment_id", job.DeploymentID))
	cli, err := client.New(client.FromEnv)
	if err != nil {
		_ = updateStatus(ctx, db, job, "failed")
		if err := publishStatusUpdate(queueUsecase, job.DeploymentID, "failed"); err != nil {
			return err
		}
		return err
	}

	imageTag := fmt.Sprintf("deploy-%d:latest", job.DeploymentID)
	buildCtx, cancel := context.WithTimeout(context.Background(), 5*time.Minute)
	defer cancel()

	logger.Info("building Docker image", zap.Int64("deployment_id", job.DeploymentID), zap.String("image_tag", imageTag))
	if err := buildImage(buildCtx, cli, repoPath, imageTag); err != nil {
		_ = updateStatus(ctx, db, job, "failed")
		if err := publishStatusUpdate(queueUsecase, job.DeploymentID, "failed"); err != nil {
			return err
		}
		return err
	}

	tarPath := filepath.Join(repoPath, fmt.Sprintf("%d.tar", job.DeploymentID))
	logger.Info("saving Docker image tar", zap.Int64("deployment_id", job.DeploymentID), zap.String("tar_path", tarPath))
	if err := saveImageTar(buildCtx, cli, imageTag, tarPath); err != nil {
		_ = updateStatus(ctx, db, job, "failed")
		if err := publishStatusUpdate(queueUsecase, job.DeploymentID, "failed"); err != nil {
			return err
		}
		return err
	}
	defer os.Remove(tarPath)

	logger.Info("uploading image tar", zap.Int64("deployment_id", job.DeploymentID))
	outputURL, err := artifactUploader.UploadImage(tarPath)
	if err != nil {
		_ = updateStatus(ctx, db, job, "failed")
		if err := publishStatusUpdate(queueUsecase, job.DeploymentID, "failed"); err != nil {
			return err
		}
		return err
	}

	logger.Info("saving output URL", zap.Int64("deployment_id", job.DeploymentID))
	if err := updateOutputURL(ctx, db, job, outputURL); err != nil {
		_ = updateStatus(ctx, db, job, "failed")
		if err := publishStatusUpdate(queueUsecase, job.DeploymentID, "failed"); err != nil {
			return err
		}
		return err
	}

	logger.Info("marking as done", zap.Int64("deployment_id", job.DeploymentID))
	if err := updateStatus(ctx, db, job, "done"); err != nil {
		return err
	}

	logger.Info("publishing done status", zap.Int64("deployment_id", job.DeploymentID))
	return publishStatusUpdate(queueUsecase, job.DeploymentID, "done")
}
