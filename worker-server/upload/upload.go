package upload

import (
	"context"
	"fmt"
	"os"
	"path/filepath"

	"github.com/aws/aws-sdk-go-v2/service/s3"
)

type Client struct {
	s3Client   *s3.Client
	bucketName string
	publicBaseURL string
}

type Uploader interface {
	UploadImage(filePath string) (string, error)
}

func NewUpload(client *s3.Client, bucketName string, publicBaseURL string) Uploader {
	return &Client{s3Client: client, bucketName: bucketName, publicBaseURL: publicBaseURL}
}

func (c *Client) UploadImage(filePath string) (string, error) {
	file, err := os.Open(filePath)
	if err != nil {
		return "", fmt.Errorf("failed to open file %q: %w", filePath, err)
	}
	defer file.Close()

	filename := filepath.Base(filePath)
	key := "images/" + filename

	_, err = c.s3Client.PutObject(context.TODO(), &s3.PutObjectInput{
		Bucket: &c.bucketName,
		Key:    &key,
		Body:   file,
	})
	if err != nil {
		return "", err
	}

	if c.publicBaseURL == "" {
		return fmt.Sprintf("s3://%s/%s", c.bucketName, key), nil
	}

	return fmt.Sprintf("%s/%s", c.publicBaseURL, key), nil
}
