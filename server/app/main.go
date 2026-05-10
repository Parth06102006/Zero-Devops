package main

import (
	"database/sql"
	"fmt"
	"log"
	"time"

	"github.com/labstack/echo"
	_ "github.com/lib/pq"
	"github.com/spf13/viper"

	_articleHttpDelivery "github.com/bxcodec/go-clean-arch/article/delivery/http"
	_articleHttpDeliveryMiddleware "github.com/bxcodec/go-clean-arch/article/delivery/http/middleware"
	_articleRepo "github.com/bxcodec/go-clean-arch/article/repository/mysql"
	_articleUcase "github.com/bxcodec/go-clean-arch/article/usecase"
	_authorRepo "github.com/bxcodec/go-clean-arch/author/repository/mysql"


	_authUcase "server/authorization/auth/usecase"
	_githubRepo "server/authorization/github/repository/pgsql"
	_userRepo "server/authorization/user/repository/pgsql"
	_config "server/config"
)

func init() {
	_config.LoadConfig()

	if viper.GetBool(`debug`) {
		log.Println("Service RUN on DEBUG mode")
	}
}

func main() {
	dbHost := viper.GetString(`database.host`)
	dbPort := viper.GetString(`database.port`)
	dbUser := viper.GetString(`database.user`)
	dbPass := viper.GetString(`database.pass`)
	dbName := viper.GetString(`database.name`)
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
	middL := _articleHttpDeliveryMiddleware.InitMiddleware()
	e.Use(middL.CORS)
	
	// database connection pool provides connection pipeline to the reposioties
	// authorRepo := _authorRepo.NewMysqlAuthorRepository(dbConn)
	// ar := _articleRepo.NewMysqlArticleRepository(dbConn)
	
	// Here are the repositories for the authorization layer
	userRepo := _userRepo.NewPgsqlUserRepository(dbConn)
	githubRepo := _githubRepo.NewPgsqlGithubRepository(dbConn)
	
	githubProvider := AuthProvider.NewGithubProvider(
        viper.GetString("OAUTH_GITHUB_CLIENT_ID"),
        viper.GetString("OAUTH_GITHUB_CLIENT_SECRET"),
        viper.GetString("OAUTH_GITHUB_REDIRECT_URL"),
    )

	providers := map[string]domain.OAuthProvider	{
		"github": githubProvider,
	}


	// 2. Pass it to your usecase
	timeoutContext := time.Duration(viper.GetInt("context.timeout")) * time.Second
	authUsecase := _authUcase.NewAuthUsecase(userRepo, providers, timeoutContext)

	log.Fatal(e.Start(viper.GetString("server.address"))) //nolint
}
