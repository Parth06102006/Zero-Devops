package main

import (
	"database/sql"
	"fmt"
	"log"
	"time"

	"github.com/labstack/echo"
	_ "github.com/lib/pq"
	"github.com/spf13/viper"

	// _articleHttpDelivery "github.com/bxcodec/go-clean-arch/article/delivery/http"
	// _articleHttpDeliveryMiddleware "github.com/bxcodec/go-clean-arch/article/delivery/http/middleware"
	// _articleRepo "github.com/bxcodec/go-clean-arch/article/repository/mysql"
	// _articleUcase "github.com/bxcodec/go-clean-arch/article/usecase"
	// _authorRepo "github.com/bxcodec/go-clean-arch/author/repository/mysql"

	_authHttp "Zero_Devops/server/authorization/auth/delivery/http"
	_authMiddleware "Zero_Devops/server/authorization/auth/delivery/http/middleware"
	_authUcase "Zero_Devops/server/authorization/auth/usecase"
	_authProvider "Zero_Devops/server/authorization/auth/usecase/auth_provider"
	_githubRepo "Zero_Devops/server/authorization/github/repository/pgsql"
	_userRepo "Zero_Devops/server/authorization/user/repository/pgsql"
	_config "Zero_Devops/server/config"
	domain "Zero_Devops/server/domain"
)

func init() {
	_config.LoadConfig()

	if viper.GetBool(`debug`) {
		log.Println("Service RUN on DEBUG mode")
	}
}

func main() {
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

	// database connection pool provides connection pipeline to the reposioties
	// authorRepo := _authorRepo.NewMysqlAuthorRepository(dbConn)
	// ar := _articleRepo.NewMysqlArticleRepository(dbConn)

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
	_ = githubRepo
	log.Fatal(e.Start(viper.GetString("SERVER_ADDRESS"))) //nolint
}
