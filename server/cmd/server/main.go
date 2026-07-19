// Package main is the entry point for the Zero DevOps server
package main

import (
	"context"
	"database/sql"
	"fmt"
	"log"
	"time"

	_authHttp "Zero_Devops/server/internal/auth/delivery/http"
	_authMiddleware "Zero_Devops/server/internal/auth/delivery/http/middleware"
	_userRepo "Zero_Devops/server/internal/auth/repository/pgsql"
	_authUcase "Zero_Devops/server/internal/auth/usecase"
	_authProvider "Zero_Devops/server/internal/auth/usecase/auth_provider"
	_config "Zero_Devops/server/internal/config"
	_deploymentHttp "Zero_Devops/server/internal/deployments/delivery/http"
	_deploymentRepo "Zero_Devops/server/internal/deployments/repository/pgsql"
	_deploymentUsecase "Zero_Devops/server/internal/deployments/usecase"
	domain "Zero_Devops/server/internal/domain"
	_appHttp "Zero_Devops/server/internal/integrations/scm/delivery/http"
	_githubRepo "Zero_Devops/server/internal/integrations/scm/github/repository/pgsql"
	_githubUsecase "Zero_Devops/server/internal/integrations/scm/github/usecase"
	"Zero_Devops/server/internal/logger"
	middleware "Zero_Devops/server/internal/middleware"
	_queue "Zero_Devops/server/internal/queue"

	"github.com/labstack/echo/v5"
	_ "github.com/lib/pq"
	amqp "github.com/rabbitmq/amqp091-go"
	"github.com/spf13/viper"
	"go.uber.org/zap"
)

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

	dbHost := viper.GetString("DATABASE_HOST")
	dbPort := viper.GetString("DATABASE_PORT")
	dbUser := viper.GetString("DATABASE_USER")
	dbPass := viper.GetString("DATABASE_PASS")
	dbName := viper.GetString("DATABASE_NAME")
	dsn := fmt.Sprintf("host=%s port=%s user=%s password=%s dbname=%s sslmode=disable",
		dbHost, dbPort, dbUser, dbPass, dbName)
	dbConn, err := sql.Open("postgres", dsn)

	if err != nil {
		return fmt.Errorf("failed to open database: %w", err)
	}
	defer func() {
		if err := dbConn.Close(); err != nil {
			log.Println("db close failed:", err)
		}
	}()

	ctx := context.Background()
	if err := dbConn.PingContext(ctx); err != nil {
		return fmt.Errorf("failed to ping database: %w", err)
	}

	e := echo.New()

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

	providers := map[string]domain.OAuthProvider{
		"github": githubProvider,
	}

	timeoutContext := time.Duration(viper.GetInt("context.timeout")) * time.Second
	authUsecase := _authUcase.NewAuthUsecase(userRepo, providers, timeoutContext)
	_authHttp.NewAuthHandler(e, authUsecase)

	githubUsecase := _githubUsecase.NewGithubAppUsecase(githubRepo)
	_appHttp.NewSCMHandler(e, githubUsecase)

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
	deploymentUsecase := _deploymentUsecase.NewDeploymentUsecase(deploymentRepo, githubRepo, rmqConn)
	_deploymentHttp.NewDeploymentHandler(e, deploymentUsecase)

	return e.Start(viper.GetString("SERVER_ADDRESS"))
}

func main() {
	if err := run(); err != nil {
		log.Fatal(err)
	}
}
