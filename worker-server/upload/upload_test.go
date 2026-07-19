package upload

import (
	"io"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"Zero_Devops/worker_server/domain"

	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/service/s3"
	"go.uber.org/zap"
)

var nopLogger = zap.NewNop()

func TestNewUploadUsecaseReturnsDomainInterface(t *testing.T) {
	var _ domain.UploadUsecase = (*clientUsecase)(nil)

	usecase := NewUploadUsecase(&s3.Client{}, "bucket", "https://cdn.example.com", nopLogger)
	if usecase == nil {
		t.Fatal("NewUploadUsecase returned nil")
	}
}

func TestUploadImageReturnsErrorForMissingFile(t *testing.T) {
	usecase := NewUploadUsecase(&s3.Client{}, "bucket", "", nopLogger)

	_, err := usecase.UploadImage(filepath.Join(t.TempDir(), "missing.tar"))
	if err == nil {
		t.Fatal("UploadImage returned nil error for missing file")
	}
	if !strings.Contains(err.Error(), "failed to open file") {
		t.Fatalf("error = %q, want failed to open file", err.Error())
	}
}

func TestUploadImageUploadsToS3AndReturnsPublicURL(t *testing.T) {
	var gotMethod string
	var gotPath string
	var gotBody string

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		gotMethod = r.Method
		gotPath = r.URL.Path

		body, err := io.ReadAll(r.Body)
		if err != nil {
			t.Fatal(err)
		}
		gotBody = string(body)

		w.WriteHeader(http.StatusOK)
	}))
	defer server.Close()

	client := s3.NewFromConfig(aws.Config{
		Region:      "auto",
		Credentials: aws.AnonymousCredentials{},
		HTTPClient:  server.Client(),
	}, func(o *s3.Options) {
		o.BaseEndpoint = aws.String(server.URL)
		o.UsePathStyle = true
	})

	filePath := filepath.Join(t.TempDir(), "artifact.tar")
	if err := os.WriteFile(filePath, []byte("image-tar-content"), 0o644); err != nil {
		t.Fatal(err)
	}

	usecase := NewUploadUsecase(client, "bucket", "https://cdn.example.com", nopLogger)
	gotURL, err := usecase.UploadImage(filePath)
	if err != nil {
		t.Fatal(err)
	}

	if gotURL != "https://cdn.example.com/images/artifact.tar" {
		t.Fatalf("url = %q, want %q", gotURL, "https://cdn.example.com/images/artifact.tar")
	}
	if gotMethod != http.MethodPut {
		t.Fatalf("method = %q, want %q", gotMethod, http.MethodPut)
	}
	if gotPath != "/bucket/images/artifact.tar" {
		t.Fatalf("path = %q, want %q", gotPath, "/bucket/images/artifact.tar")
	}
	if gotBody != "image-tar-content" {
		t.Fatalf("body = %q, want %q", gotBody, "image-tar-content")
	}
}

func TestUploadImageReturnsS3URLWithoutPublicBaseURL(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusOK)
	}))
	defer server.Close()

	client := s3.NewFromConfig(aws.Config{
		Region:      "auto",
		Credentials: aws.AnonymousCredentials{},
		HTTPClient:  server.Client(),
	}, func(o *s3.Options) {
		o.BaseEndpoint = aws.String(server.URL)
		o.UsePathStyle = true
	})

	filePath := filepath.Join(t.TempDir(), "artifact.tar")
	if err := os.WriteFile(filePath, []byte("image-tar-content"), 0o644); err != nil {
		t.Fatal(err)
	}

	usecase := NewUploadUsecase(client, "bucket", "", nopLogger)
	gotURL, err := usecase.UploadImage(filePath)
	if err != nil {
		t.Fatal(err)
	}

	if gotURL != "s3://bucket/images/artifact.tar" {
		t.Fatalf("url = %q, want %q", gotURL, "s3://bucket/images/artifact.tar")
	}
}
