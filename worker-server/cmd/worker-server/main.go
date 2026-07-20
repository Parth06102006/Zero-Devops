// Package main is the entry point for the worker server.
package main

import (
	"context"
	"database/sql"
	"fmt"

	"Zero_Devops/worker_server/internal/config"
	deploymentRepo "Zero_Devops/worker_server/internal/deployments/repository/pgsql"
	"Zero_Devops/worker_server/internal/logger"
	"Zero_Devops/worker_server/internal/queue"
	"Zero_Devops/worker_server/internal/upload"
	"Zero_Devops/worker_server/internal/worker"

	"github.com/aws/aws-sdk-go-v2/aws"
	aws_config "github.com/aws/aws-sdk-go-v2/config"
	aws_credentials "github.com/aws/aws-sdk-go-v2/credentials"
	"github.com/aws/aws-sdk-go-v2/service/s3"
	_ "github.com/lib/pq"
	amqp "github.com/rabbitmq/amqp091-go"
	"github.com/spf13/viper"
	"go.uber.org/zap"
)

func main() {
	config.LoadConfig()

	baseLogger, err := logger.New(viper.GetString("APP_ENV"))
	if err != nil {
		baseLogger.Warn("logger initialized with non-fatal error", zap.Error(err))
	}
	zap.ReplaceGlobals(baseLogger)
	defer func() { _ = baseLogger.Sync() }()

	bucketName := viper.GetString("CLOUDFLARE_BUCKET_NAME")
	accountID := viper.GetString("CLOUDFLARE_ACCOUNT_ID")
	accessKeyID := viper.GetString("CLOUDFLARE_ACCESS_KEY_ID")
	accessKeySecret := viper.GetString("CLOUDFLARE_ACCESS_KEY_SECRET")
	publicBaseURL := viper.GetString("CLOUDFLARE_PUBLIC_BASE_URL")

	cfg, err := aws_config.LoadDefaultConfig(context.TODO(),
		aws_config.WithCredentialsProvider(aws_credentials.NewStaticCredentialsProvider(accessKeyID, accessKeySecret, "")),
		aws_config.WithRegion("auto"),
	)
	if err != nil {
		baseLogger.Fatal("failed to load AWS config", zap.Error(err))
	}

	client := s3.NewFromConfig(cfg, func(o *s3.Options) {
		o.BaseEndpoint = aws.String(fmt.Sprintf("https://%s.r2.cloudflarestorage.com", accountID))
	})

	// ArtifactUploader Usecase
	artifactUploader := upload.NewUploadUsecase(client, bucketName, publicBaseURL, baseLogger)

	dbHost := viper.GetString("DATABASE_HOST")
	dbPort := viper.GetString("DATABASE_PORT")
	dbUser := viper.GetString("DATABASE_USER")
	dbPass := viper.GetString("DATABASE_PASS")
	dbName := viper.GetString("DATABASE_NAME")

	dsn := fmt.Sprintf("host=%s port=%s user=%s password=%s dbname=%s sslmode=disable",
		dbHost, dbPort, dbUser, dbPass, dbName)

	db, err := sql.Open("postgres", dsn)
	if err != nil {
		baseLogger.Fatal("failed to open database", zap.Error(err))
	}
	if err := db.PingContext(context.Background()); err != nil {
		baseLogger.Fatal("failed to ping database", zap.Error(err))
	}
	defer func() {
		if err := db.Close(); err != nil {
			baseLogger.Fatal("failed to close database", zap.Error(err))
		}
	}()

	rmqConn, err := amqp.Dial(viper.GetString("RABBITMQ_CONNECTION_STRING"))
	if err != nil {
		baseLogger.Fatal("failed to connect to RabbitMQ", zap.Error(err))
	}
	defer func() {
		if cerr := rmqConn.Close(); cerr != nil {
			baseLogger.Error("failed to close rabbitmq connection", zap.Error(cerr))
		}
	}()

	rmqCh, err := rmqConn.Channel()
	if err != nil {
		baseLogger.Fatal("failed to open RabbitMQ channel", zap.Error(err))
	}
	defer func() {
		if cerr := rmqCh.Close(); cerr != nil {
			baseLogger.Error("failed to close rabbitmq channel", zap.Error(cerr))
		}
	}()

	// Queue Usecase
	queueClient := queue.NewQueueUsecase(baseLogger, rmqConn, rmqCh)

	if err := queueClient.SetUpQueues(); err != nil {
		baseLogger.Fatal("failed to set up queues", zap.Error(err))
	}

	// Deployment Repository
	depRepo := deploymentRepo.NewPgSQLDeploymentRepository(db)

	// Worker Usecase
	workerUsecase := worker.NewWorkerUsecase(queueClient, depRepo, artifactUploader)
	if err := workerUsecase.StartWorker(baseLogger); err != nil {
		baseLogger.Fatal("worker stopped", zap.Error(err))
	}
}
