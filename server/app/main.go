package main

import (
	"database/sql"
	"fmt"
	"log"
	"time"

	_authHttp "Zero_Devops/server/authorization/auth/delivery/http"
	_authMiddleware "Zero_Devops/server/authorization/auth/delivery/http/middleware"
	_authUcase "Zero_Devops/server/authorization/auth/usecase"
	_authProvider "Zero_Devops/server/authorization/auth/usecase/auth_provider"
	_userRepo "Zero_Devops/server/authorization/user/repository/pgsql"
	_config "Zero_Devops/server/config"
	_deploymentHttp "Zero_Devops/server/deployments/delivery/http"
	_deploymentRepo "Zero_Devops/server/deployments/repository/pgsql"
	_deploymentUsecase "Zero_Devops/server/deployments/usecase"
	domain "Zero_Devops/server/domain"
	_appHttp "Zero_Devops/server/integrations/scm/delivery/http"
	_githubRepo "Zero_Devops/server/integrations/scm/github/repository/pgsql"
	_githubUsecase "Zero_Devops/server/integrations/scm/github/usecase"
	"Zero_Devops/server/logger"
	middleware "Zero_Devops/server/middleware"
	_queue "Zero_Devops/server/queue"

	"github.com/labstack/echo/v5"
	_ "github.com/lib/pq"
	amqp "github.com/rabbitmq/amqp091-go"
	"github.com/spf13/viper"
	"go.uber.org/zap"
)

func init() {
	_config.LoadConfig()

	if viper.GetBool(`debug`) {
		log.Println("Service RUN on DEBUG mode")
	}
}

func main() {
	baseLogger := logger.New(viper.GetString("APP_ENV"))
	zap.ReplaceGlobals(baseLogger)  
	defer baseLogger.Sync()

	dbHost := viper.GetString("DATABASE_HOST")
	dbPort := viper.GetString("DATABASE_PORT")
	dbUser := viper.GetString("DATABASE_USER")
	dbPass := viper.GetString("DATABASE_PASS")
	dbName := viper.GetString("DATABASE_NAME")
	dsn := fmt.Sprintf("host=%s port=%s user=%s password=%s dbname=%s sslmode=disable",
		dbHost, dbPort, dbUser, dbPass, dbName)
	dbConn, err := sql.Open("postgres", dsn)

	if err != nil {
		log.Fatal(err)
	}
	err = dbConn.Ping()
	if err != nil {
		log.Fatal(err)
	}

	defer func() {
		err := dbConn.Close()
		if err != nil {
			log.Fatal(err)
		}
	}()

	e := echo.New()
	// middleware
	// middL := _articleHttpDeliveryMiddleware.InitMiddleware()
	// e.Use(middL.CORS)

	// request_id middleware
	e.Use(middleware.RequestIDMiddleware)

	// logger middleware
	e.Use(middleware.RequestLoggerMiddleware(baseLogger))

	// Here are the repositories for the authorization layer
	userRepo := _userRepo.NewPgSqlUserRepository(dbConn)
	authMiddleware := _authMiddleware.NewAuthMiddlewareHandler(userRepo)
	e.Use(authMiddleware.ToMiddleware())

	githubRepo := _githubRepo.NewPgSqlGithubRepository(dbConn)

	githubProvider := _authProvider.NewGithubProvider(
		viper.GetString("OAUTH_GITHUB_CLIENT_ID"),
		viper.GetString("OAUTH_GITHUB_CLIENT_SECRET"),
		viper.GetString("OAUTH_GITHUB_REDIRECT_URL"),
	)

	providers := map[string]domain.OAuthProvider{
		"github": githubProvider,
	}

	// 2. Pass it to your usecase
	timeoutContext := time.Duration(viper.GetInt("context.timeout")) * time.Second
	authUsecase := _authUcase.NewAuthUsecase(userRepo, providers, timeoutContext)
	_authHttp.NewAuthHandler(e, authUsecase)

	// 3. Now Intitalize the Github App Installtion for it to integrate
	githubUsecase := _githubUsecase.NewGithubAppUsecase(githubRepo)
	// ** NEED TO ADD THE APP INTEGRATION HTTP FOR IT TO BE ADDED
	_appHttp.NewSCMHandler(e, githubUsecase)

	// 4. Setup RabbitMQ connection
	rmqConn, err := amqp.Dial(viper.GetString("RABBITMQ_CONNECTION_STRING"))
	if err != nil {
		log.Fatal(err)
	}
	defer rmqConn.Close()
	rmqCh, err := rmqConn.Channel()
	if err != nil {
		log.Fatal(err)
	}
	defer rmqCh.Close()

	err = _queue.SetUpQueues(rmqConn, rmqCh)
	if err != nil {
		log.Fatal(err)
	}

	// 5. Initialize the Deployments feature
	deploymentRepo := _deploymentRepo.NewPgSqlDeploymentRepository(dbConn)
	deploymentUsecase := _deploymentUsecase.NewDeploymentUsecase(deploymentRepo, githubRepo, rmqConn)
	_deploymentHttp.NewDeploymentHandler(e, deploymentUsecase)

	log.Fatal(e.Start(viper.GetString("SERVER_ADDRESS"))) //nolint
}
