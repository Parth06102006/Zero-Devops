// Package main is the entry point for the Zero DevOps server
package main

import (
	"context"
	"database/sql"
	"fmt"
	"log"
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"time"

	_config "Zero_Devops/server/config"
	_authHttp "Zero_Devops/server/internal/auth/delivery/http"
	_authMiddleware "Zero_Devops/server/internal/auth/delivery/http/middleware"
	_userRepo "Zero_Devops/server/internal/auth/repository/pgsql"
	_authUcase "Zero_Devops/server/internal/auth/usecase"
	_authProvider "Zero_Devops/server/internal/auth/usecase/auth_provider"
	_deploymentHttp "Zero_Devops/server/internal/deployments/delivery/http"
	_outboxDispatcher "Zero_Devops/server/internal/deployments/dispatcher"
	_deploymentRepo "Zero_Devops/server/internal/deployments/repository/pgsql"
	_deploymentUsecase "Zero_Devops/server/internal/deployments/usecase"
	domain "Zero_Devops/server/internal/domain"
	_appHttp "Zero_Devops/server/internal/integrations/scm/delivery/http"
	"Zero_Devops/server/internal/integrations/scm/github/cache"
	_githubClient "Zero_Devops/server/internal/integrations/scm/github/client"
	_githubRepo "Zero_Devops/server/internal/integrations/scm/github/repository/pgsql"
	_tokenProvider "Zero_Devops/server/internal/integrations/scm/github/token"
	_githubUsecase "Zero_Devops/server/internal/integrations/scm/github/usecase"
	_webhookHttp "Zero_Devops/server/internal/integrations/scm/webhook/delivery/http"
	_webhookUsecase "Zero_Devops/server/internal/integrations/scm/webhook/github"
	_webhookRepo "Zero_Devops/server/internal/integrations/scm/webhook/repository/pgsql"
	_projectHttp "Zero_Devops/server/internal/project/delivery/http"
	_projectRepo "Zero_Devops/server/internal/project/repository/pgsql"
	_projectScanner "Zero_Devops/server/internal/project/scanner"
	_projectUsecase "Zero_Devops/server/internal/project/usecase"

	"Zero_Devops/server/internal/logger"
	middleware "Zero_Devops/server/internal/middleware"
	_queue "Zero_Devops/server/internal/queue"

	"github.com/labstack/echo/v5"
	_ "github.com/lib/pq"
	amqp "github.com/rabbitmq/amqp091-go"
	"github.com/redis/go-redis/v9"
	"github.com/spf13/viper"
	"go.uber.org/zap"
)

const maxWebhookPayloadSize = 10_485_760

//nolint:funlen // main setup wires all HTTP handlers and background workers.
func run() error {
	_config.LoadConfig()

	if viper.GetBool(`debug`) {
		log.Println("Service RUN on DEBUG mode")
	}

	baseLogger := logger.New(viper.GetString("APP_ENV"))
	zap.ReplaceGlobals(baseLogger)
	defer func() {
		if err := baseLogger.Sync(); err != nil {
			log.Println("sync failed:", err)
		}
	}()

	dsn := buildPostgresDSN()
	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer stop()

	dbConn, err := sql.Open("postgres", dsn)

	if err != nil {
		return fmt.Errorf("failed to open database: %w", err)
	}
	defer func() {
		if err := dbConn.Close(); err != nil {
			log.Println("db close failed:", err)
		}
	}()

	if err := dbConn.PingContext(ctx); err != nil {
		return fmt.Errorf("failed to ping database: %w", err)
	}

	redisAddr := viper.GetString("REDIS_ADDR")
	if redisAddr == "" {
		redisAddr = "localhost:6379"
	}
	rdb := redis.NewClient(&redis.Options{
		Addr:     redisAddr,
		Password: viper.GetString("REDIS_PASSWORD"),
		DB:       viper.GetInt("REDIS_DB"),
		Protocol: 2,
	})

	defer func() {
		if err := rdb.Close(); err != nil {
			log.Println("redis-db close failed", err)
		}
	}()

	if _, err = rdb.Ping(ctx).Result(); err != nil {
		log.Println("Redis unavailable; repository listing will bypass cache:", err)
	}

	e := echo.New()

	e.Use(middleware.NewCORS())
	e.Use(middleware.RequestIDMiddleware)
	e.Use(middleware.RequestLoggerMiddleware(baseLogger))

	userRepo := _userRepo.NewPgSQLUserRepository(dbConn)
	authMiddleware := _authMiddleware.NewAuthMiddlewareHandler(userRepo)
	e.Use(authMiddleware.ToMiddleware())

	githubRepo := _githubRepo.NewPgSQLGithubRepository(dbConn)

	githubProvider := _authProvider.NewGithubProvider(
		viper.GetString("OAUTH_GITHUB_CLIENT_ID"),
		viper.GetString("OAUTH_GITHUB_CLIENT_SECRET"),
		viper.GetString("OAUTH_GITHUB_REDIRECT_URL"),
	)

	tokenProvider := _tokenProvider.NewInstallationTokenProvider(viper.GetString("GITHUB_APP_ID"), viper.GetString("GITHUB_APP_PRIVATE_KEY_PATH"))

	providers := map[string]domain.OAuthProvider{
		"github": githubProvider,
	}

	timeoutContext := time.Duration(viper.GetInt("context.timeout")) * time.Second
	authUsecase := _authUcase.NewAuthUsecase(userRepo, providers, timeoutContext)
	_authHttp.NewAuthHandler(e, authUsecase)

	e.GET("/", func(c *echo.Context) error {
		return c.JSON(http.StatusOK, map[string]string{"message": "logged in successfully"})
	})

	ttl := time.Duration(viper.GetInt("REDIS_REPOSITORY_CACHE_TTL_SECONDS")) * time.Second
	cacheUsecase := cache.NewRedisRepositoryListCache(rdb, ttl)

	repositoryClient := _githubClient.NewRepositoryClient(http.DefaultClient)
	githubUsecase := _githubUsecase.NewGithubAppUsecase(githubRepo, tokenProvider, repositoryClient, cacheUsecase)
	_appHttp.NewSCMHandler(e, githubUsecase)

	projectRepo := _projectRepo.NewPgSQLProjectRepository(dbConn)
	projectUsecase := _projectUsecase.NewProjectUsecase(projectRepo, githubUsecase, _projectScanner.DefaultScanner)
	_projectHttp.NewProjectHandler(e, projectUsecase)

	rmqConn, err := amqp.Dial(viper.GetString("RABBITMQ_CONNECTION_STRING"))
	if err != nil {
		return fmt.Errorf("failed to connect to RabbitMQ: %w", err)
	}
	defer func() {
		if err := rmqConn.Close(); err != nil {
			log.Println("rmq conn close failed:", err)
		}
	}()

	rmqCh, err := rmqConn.Channel()
	if err != nil {
		return fmt.Errorf("failed to open RabbitMQ channel: %w", err)
	}
	defer func() {
		if err := rmqCh.Close(); err != nil {
			log.Println("rmq ch close failed:", err)
		}
	}()

	if err := _queue.SetUpQueues(rmqConn, rmqCh); err != nil {
		return fmt.Errorf("failed to set up queues: %w", err)
	}

	deploymentRepo := _deploymentRepo.NewPgSQLDeploymentRepository(dbConn)
	// Durable deploy.status consumer (Task 5): applies worker status updates
	// atomically via ApplyStatusUpdate and acks only after the commit. Same
	// signal context as the outbox dispatcher and HTTP server.
	deploymentUsecase := _deploymentUsecase.NewDeploymentUsecase(ctx, deploymentRepo, githubRepo, tokenProvider, rmqConn, projectRepo, repositoryClient)
	_deploymentHttp.NewDeploymentHandler(e, deploymentUsecase)

	// Outbox dispatcher (Task 5, plan-server-12-08.md): the only deploy.jobs
	// producer. Manual and webhook builds both write their deployment row and
	// outbox event in one transaction; this loop claims pending events and
	// publishes them with broker confirmation. It runs alongside the HTTP
	// server and stops on the same signal context. If it exits with an error
	// (for example the RabbitMQ channel cannot be opened), the server keeps
	// accepting builds — the outbox rows accumulate durably and are dispatched
	// after a restart or once the broker recovers; nothing is lost.
	outbox := _outboxDispatcher.NewOutbox(
		deploymentRepo,
		rmqConn,
		baseLogger,
		_outboxDispatcher.Config{
			PollInterval: time.Duration(viper.GetInt("OUTBOX_POLL_INTERVAL_MS")) * time.Millisecond,
			BatchSize:    viper.GetInt("OUTBOX_BATCH_SIZE"),
		},
	)
	go func() {
		if err := outbox.Run(ctx); err != nil {
			baseLogger.Error("outbox dispatcher stopped with error; builds will accumulate in the outbox until restart",
				zap.Error(err))
		}
	}()

	webhookRepo := _webhookRepo.NewPGSQLWebhookRepository(dbConn)
	webhookUsecase, err := _webhookUsecase.NewWebhookUsecase(webhookRepo,
		githubRepo,
		projectRepo,
		deploymentRepo,
		cacheUsecase,
		_webhookUsecase.Options.Secret(viper.GetString("GITHUB_APP_WEBHOOK_SECRET")),
		_webhookUsecase.Options.MaxPayloadSize(maxWebhookPayloadSize),
	)

	if err != nil {
		return fmt.Errorf("failed to create webhook usecase:%w", err)
	}

	_webhookHttp.NewWebhookHandler(e, webhookUsecase)

	// Start the HTTP server bound to the same signal context as the outbox
	// dispatcher: on SIGINT/SIGTERM echo drains in-flight requests gracefully
	// and the dispatcher stops claiming new batches, all through one ctx.
	return echo.StartConfig{
		Address:         viper.GetString("SERVER_ADDRESS"),
		GracefulTimeout: 10 * time.Second,
	}.Start(ctx, e)
}

func buildPostgresDSN() string {
	dbHost := viper.GetString("DATABASE_HOST")
	dbPort := viper.GetString("DATABASE_PORT")
	dbUser := viper.GetString("DATABASE_USER")
	dbPass := viper.GetString("DATABASE_PASS")
	dbName := viper.GetString("DATABASE_NAME")
	return fmt.Sprintf("host=%s port=%s user=%s password=%s dbname=%s sslmode=disable",
		dbHost, dbPort, dbUser, dbPass, dbName)
}

func main() {
	if err := run(); err != nil {
		log.Fatal(err)
	}
}
