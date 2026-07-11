package main

import (
	"build_app_test/config"
	"build_app_test/worker/queue"
	"build_app_test/worker/worker"
	"context"
	"database/sql"
	"fmt"
	"log"

	"build_app_test/upload"

	"github.com/aws/aws-sdk-go-v2/aws"
	aws_config "github.com/aws/aws-sdk-go-v2/config"
	aws_credentials "github.com/aws/aws-sdk-go-v2/credentials"
	"github.com/aws/aws-sdk-go-v2/service/s3"
	_ "github.com/lib/pq"
	"github.com/spf13/viper"
)

func main() {
	config.LoadConfig()

	bucketName := viper.GetString("CLOUDFLARE_BUCKET_NAME")
	accountId := viper.GetString("CLOUDFLARE_ACCOUNT_ID")
	accessKeyId := viper.GetString("CLOUDFLARE_ACCESS_KEY_ID")
	accessKeySecret := viper.GetString("CLOUDFLARE_ACCESS_KEY_SECRET")
	publicBaseURL := viper.GetString("CLOUDFLARE_PUBLIC_BASE_URL")

	cfg, err := aws_config.LoadDefaultConfig(context.TODO(),
		aws_config.WithCredentialsProvider(aws_credentials.NewStaticCredentialsProvider(accessKeyId, accessKeySecret, "")),
		aws_config.WithRegion("auto"),
	)
	if err != nil {
		log.Fatal(err)
	}

	client := s3.NewFromConfig(cfg, func(o *s3.Options) {
		o.BaseEndpoint = aws.String(fmt.Sprintf("https://%s.r2.cloudflarestorage.com", accountId))
	})

	artifactUploader := upload.NewUpload(client, bucketName, publicBaseURL)

	rmq, err := queue.NewRabbitMQ(viper.GetString("RABBITMQ_CONNECTION_STRING"))
	if err != nil {
		log.Fatal(err)
	}
	defer rmq.Close()

	if err := rmq.SetUpQueues(); err != nil {
		log.Fatal(err)
	}

	dbHost := viper.GetString("DATABASE_HOST")
	dbPort := viper.GetString("DATABASE_PORT")
	dbUser := viper.GetString("DATABASE_USER")
	dbPass := viper.GetString("DATABASE_PASS")
	dbName := viper.GetString("DATABASE_NAME")

	dsn := fmt.Sprintf("host=%s port=%s user=%s password=%s dbname=%s sslmode=disable",
		dbHost, dbPort, dbUser, dbPass, dbName)

	db, err := sql.Open("postgres", dsn)
	if err != nil {
		log.Fatal(err)
	}
	if err := db.Ping(); err != nil {
		log.Fatal(err)
	}
	defer func() {
		if err := db.Close(); err != nil {
			log.Fatal(err)
		}
	}()

	if err := worker.StartWorker(rmq, db, artifactUploader); err != nil {
		log.Fatalf("worker stopped: %v", err)
	}
}
