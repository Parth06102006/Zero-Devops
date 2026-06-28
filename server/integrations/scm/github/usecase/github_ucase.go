package usecase

import (
	"Zero_Devops/server/domain"
	"context"
	"encoding/json"
	"io"
	"net/http"
	"net/url"
	"strings"
	"time"

	"github.com/spf13/viper"
)

type githubAppUsecase struct{
	// Here gitRepo does not identify github repositories it means the github functions 
	githubRepo domain.GithubRepository
}

func NewGithubAppUsecase(githubRepo domain.GithubRepository) domain.GithubUsecase {
	return &githubAppUsecase{
		githubRepo: githubRepo,
	}
}

type GithubTokenResponse struct {
    AccessToken string `json:"access_token"`
    TokenType   string `json:"token_type"`
    Scope       string `json:"scope"`
}

type Installation struct {
	ID int64 `json:"id"`
	Account struct {
		Login string `json:"login"`
		Type  string `json:"type"`
	}
	AppID int64 `json:"app_id"`
}

type GithubInstallationList struct{
	TotalCount int `json:"total_count"`
	Installations []Installation `json:"installations"`
}

func (g *githubAppUsecase) InstallGithubApp(ctx context.Context,client *http.Client ,code string,user_id int64) error {
	/*
		Here I need the installation id here but for that
		I would first get the code from the string and here it is been used a parameter
		Then I will get the accessToken using that I will use the https://github.com/login/oauth/access_token
		Then there will be the user token used to call
		GET https://api.github.com/user/installations
		It would return the list of the installations of the user and from there I would check the installtion_id and return it
	*/
	GITHUB_APP_CLIENT_ID := viper.GetString("GITHUB_APP_CLIENT_ID")
	GITHUB_APP_CLIENT_SECRET := viper.GetString("GITHUB_APP_CLIENT_SECRET")
	GITHUB_APP_ID := viper.GetInt64("GITHUB_APP_ID")

	data := url.Values{}
	data.Add("client_id",GITHUB_APP_CLIENT_ID)
	data.Add("client_secret",GITHUB_APP_CLIENT_SECRET)
	data.Add("code",code)

	req,err := http.NewRequest("POST","https://github.com/login/oauth/access_token",strings.NewReader(data.Encode()))

	if err != nil{
		return domain.ErrInvalidCode
	}

	req.Header.Set("Content-Type","application/x-www-form-urlencoded")
	req.Header.Set("Accept","application/json")

	response,err := client.Do(req)
	if err != nil{
		return err
	}

	defer response.Body.Close()

	if response.StatusCode < 200 || response.StatusCode >= 300 {
		return domain.ErrInvalidCode
	}

	r := io.Reader(response.Body)
	var githubTokenResponse GithubTokenResponse
	if err := json.NewDecoder(r).Decode(&githubTokenResponse); err != nil {
		return err
	}

	req_installation,err := http.NewRequest("GET","https://api.github.com/user/installations",nil)

	if err != nil {
    	return err
	}

	req_installation.Header.Set("Authorization","Bearer "+githubTokenResponse.AccessToken)

	// the below is the custom github api since most of the apis now support the application/vnd.github+json and application/json types
	req_installation.Header.Set("Accept","application/vnd.github+json")

	response_installation , err := client.Do(req_installation)
	if err != nil {
		return err
	}

	defer response_installation.Body.Close()

	if response_installation.StatusCode < 200 || response_installation.StatusCode >= 300 {
		return domain.ErrGithubInstallationFetchFailed
	}

	var githubAppInstallationList GithubInstallationList;
	if err := json.NewDecoder(response_installation.Body).Decode(&githubAppInstallationList);err != nil{
		return err
	}


	// I am not verifying here the Username since there is the possibility that the User can login with one account but can install github app from another account
	for _,inst := range githubAppInstallationList.Installations {
		if inst.Account.Type == "User" && inst.AppID == GITHUB_APP_ID{
			githubAppInstallation := domain.GithubInstallation{
				UserID:user_id,
				InstallationID: inst.ID,
				Account_Type: inst.Account.Type,
				Account_Login: inst.Account.Login,
				Status: domain.GithubInstallationStatusActive,
				CreatedAt: time.Now(),
				UpdatedAt: time.Now(),
			}
			err := g.githubRepo.StoreInstallation(ctx,&githubAppInstallation)
			if err != nil{
				return err
			}
		}
	}
	return nil
}

func (g *githubAppUsecase) GetGithubAppInstallation(ctx context.Context, userID int64) (*domain.GithubInstallation, error) {
	githubRepo,err := g.githubRepo.GetInstallationByUserID(ctx,userID) 
	
	if err != nil{
		return nil,err
	}

	return githubRepo,nil
}

func (g *githubAppUsecase) DeleteGithubApp(ctx context.Context, userID int64) error {
	return g.githubRepo.DeleteInstallationByUserID(ctx, userID)
}
