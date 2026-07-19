package domain

import "github.com/aws/aws-sdk-go-v2/service/s3"

// UploadClient holds the S3 client and bucket configuration for artifact uploads.
type UploadClient struct {
	S3Client      *s3.Client
	BucketName    string
	PublicBaseURL string
}

// UploadUsecase defines the interface for uploading artifacts.
type UploadUsecase interface {
	UploadImage(filePath string) (string, error)
}
