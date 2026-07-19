// Package upload provides artifact upload to S3-compatible storage.
package upload

import (
	"context"
	"fmt"
	"os"
	"path/filepath"

	"Zero_Devops/worker_server/domain"

	"github.com/aws/aws-sdk-go-v2/service/s3"
	"go.uber.org/zap"
)

type clientUsecase struct {
	uploadClient *domain.UploadClient
	logger       *zap.Logger
}

// NewUploadUsecase creates a new UploadUsecase for uploading artifacts to S3.
func NewUploadUsecase(client *s3.Client, bucketName, publicBaseURL string, logger *zap.Logger) domain.UploadUsecase {
	return &clientUsecase{
		uploadClient: &domain.UploadClient{
			S3Client:      client,
			BucketName:    bucketName,
			PublicBaseURL: publicBaseURL,
		},
		logger: logger,
	}
}

// UploadImage uploads a file to S3 and returns the public URL.
func (c *clientUsecase) UploadImage(filePath string) (string, error) {
	s3Client := c.uploadClient

	c.logger.Info("opening file for upload", zap.String("file", filePath))
	//nolint:gosec // filePath is constructed internally from the caller (saveImageTar)
	file, err := os.Open(filePath)
	if err != nil {
		return "", fmt.Errorf("failed to open file %q: %w", filePath, err)
	}
	defer func() { _ = file.Close() }()

	filename := filepath.Base(filePath)
	key := "images/" + filename

	c.logger.Info("uploading to S3", zap.String("bucket", s3Client.BucketName), zap.String("key", key))
	_, err = s3Client.S3Client.PutObject(context.TODO(), &s3.PutObjectInput{
		Bucket: &s3Client.BucketName,
		Key:    &key,
		Body:   file,
	})
	if err != nil {
		c.logger.Error("S3 upload failed", zap.Error(err))
		return "", err
	}

	var url string
	if s3Client.PublicBaseURL == "" {
		url = fmt.Sprintf("s3://%s/%s", s3Client.BucketName, key)
	} else {
		url = fmt.Sprintf("%s/%s", s3Client.PublicBaseURL, key)
	}

	c.logger.Info("upload complete", zap.String("url", url))
	return url, nil
}
