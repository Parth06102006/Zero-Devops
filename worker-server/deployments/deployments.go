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
	"log"
	"os"
	"os/exec"
	"path/filepath"
	"strconv"
	"strings"
	"time"

	"Zero_Devops/worker_server/domain"

	"github.com/moby/go-archive"
	"github.com/moby/moby/client"
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
	query := `UPDATE deployments SET status = $1 WHERE id = $2`

	stmt, err := db.PrepareContext(ctx, query)
	if err != nil {
		return err
	}
	defer stmt.Close()

	_, err = stmt.ExecContext(ctx, status, job.DeploymentID)
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

	_, err = stmt.ExecContext(ctx, job.DeploymentID, job.Clone_URL, job.RetryCount, imageTag)
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

func ProcessDeployment(ctx context.Context, db *sql.DB, job domain.DeployJob, artifactUploader domain.UploadUsecase, queueUsecase domain.QueueUsecase) error {
	deploymentID := strconv.FormatInt(job.DeploymentID, 10)

	log.Printf("Deployment %d: inserting worker deployment row", job.DeploymentID)
	if err := insertDeployment(ctx, db, job); err != nil {
		return err
	}

	log.Printf("Deployment %d: marking as building", job.DeploymentID)
	if err := updateStatus(ctx, db, job, "building"); err != nil {
		return err
	}

	log.Printf("Deployment %d: publishing building status", job.DeploymentID)
	if err := publishStatusUpdate(queueUsecase, job.DeploymentID, "building"); err != nil {
		return err
	}

	log.Printf("Deployment %d: cloning repository", job.DeploymentID)
	repoPath, err := cloneRepo(job.Clone_URL, deploymentID)
	if err != nil {
		_ = updateStatus(ctx, db, job, "failed")
		if err := publishStatusUpdate(queueUsecase, job.DeploymentID, "failed"); err != nil {
			return err
		}
		return err
	}

	defer os.RemoveAll(repoPath)

	log.Printf("Deployment %d: detecting framework", job.DeploymentID)
	builder, err := detectFramework(repoPath)
	if err != nil {
		_ = updateStatus(ctx, db, job, "failed")
		if err := publishStatusUpdate(queueUsecase, job.DeploymentID, "failed"); err != nil {
			return err
		}
		return err
	}
	pm := detectPackageManager(repoPath)
	log.Printf("Deployment %d: framework=%s package_manager=%s", job.DeploymentID, builder.Name, pm)

	if builder.Name != "docker" {
		log.Printf("Deployment %d: writing Dockerfile from template %s", job.DeploymentID, builder.Template)
		if err := writeDockerfile(repoPath, builder, pm); err != nil {
			_ = updateStatus(ctx, db, job, "failed")
			if err := publishStatusUpdate(queueUsecase, job.DeploymentID, "failed"); err != nil {
				return err
			}
			return err
		}
	}

	log.Printf("Deployment %d: creating Docker client", job.DeploymentID)
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

	log.Printf("Deployment %d: building Docker image %s", job.DeploymentID, imageTag)
	if err := buildImage(buildCtx, cli, repoPath, imageTag); err != nil {
		_ = updateStatus(ctx, db, job, "failed")
		if err := publishStatusUpdate(queueUsecase, job.DeploymentID, "failed"); err != nil {
			return err
		}
		return err
	}

	tarPath := filepath.Join(repoPath, fmt.Sprintf("%d.tar", job.DeploymentID))
	log.Printf("Deployment %d: saving Docker image tar to %s", job.DeploymentID, tarPath)
	if err := saveImageTar(buildCtx, cli, imageTag, tarPath); err != nil {
		_ = updateStatus(ctx, db, job, "failed")
		if err := publishStatusUpdate(queueUsecase, job.DeploymentID, "failed"); err != nil {
			return err
		}
		return err
	}
	defer os.Remove(tarPath)

	log.Printf("Deployment %d: uploading image tar", job.DeploymentID)
	outputURL, err := artifactUploader.UploadImage(tarPath)
	if err != nil {
		_ = updateStatus(ctx, db, job, "failed")
		if err := publishStatusUpdate(queueUsecase, job.DeploymentID, "failed"); err != nil {
			return err
		}
		return err
	}

	log.Printf("Deployment %d: saving output URL", job.DeploymentID)
	if err := updateOutputURL(ctx, db, job, outputURL); err != nil {
		_ = updateStatus(ctx, db, job, "failed")
		if err := publishStatusUpdate(queueUsecase, job.DeploymentID, "failed"); err != nil {
			return err
		}
		return err
	}

	log.Printf("Deployment %d: marking as done", job.DeploymentID)
	if err := updateStatus(ctx, db, job, "done"); err != nil {
		return err
	}

	log.Printf("Deployment %d: publishing done status", job.DeploymentID)
	return publishStatusUpdate(queueUsecase, job.DeploymentID, "done")
}
