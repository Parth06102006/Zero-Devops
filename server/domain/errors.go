package domain

import "errors"

var (
	// ErrProviderNotSupported will throw if any of the supported providers is not found
	ErrProviderNotSupported = errors.New("the requested oauth provider is not supported")
	// ErrInternalServerError will throw if any the Internal Server Error happen
	ErrInternalServerError = errors.New("internal Server Error")
	// ErrNotFound will throw if the requested item is not exists
	ErrNotFound = errors.New("your requested Item is not found")
	// ErrConflict will throw if the current action already exists
	ErrConflict = errors.New("your Item already exist")
	// ErrBadParamInput will throw if the given request-body or params is not valid
	ErrBadParamInput = errors.New("given Param is not valid")
	
	ErrInvalidToken = errors.New("invalid or expired token")

	ErrMissingSecret = errors.New("Secret Not Found")

	ErrLoggingOut = errors.New("Error In Logging Out")

	ErrInvalidCode = errors.New("Invalid Code")

	ErrGithubInstallationFetchFailed = errors.New("Github Installation Failed.Error Installing Github App")
)
