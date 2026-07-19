package domain

import "errors"

var (
	// ErrProviderNotSupported is returned when the requested OAuth provider is not supported
	ErrProviderNotSupported = errors.New("the requested oauth provider is not supported")
	// ErrInternalServerError is returned when an internal server error occurs
	ErrInternalServerError = errors.New("internal Server Error")
	// ErrNotFound is returned when a requested item is not found
	ErrNotFound = errors.New("your requested Item is not found")
	// ErrConflict is returned when an item already exists
	ErrConflict = errors.New("your Item already exist")
	// ErrBadParamInput is returned when input parameters are invalid
	ErrBadParamInput = errors.New("given Param is not valid")

	// ErrInvalidToken is returned when a token is invalid or expired
	ErrInvalidToken = errors.New("invalid or expired token")
	// ErrMissingSecret is returned when a required secret is missing
	ErrMissingSecret = errors.New("secret not found")
	// ErrLoggingOut is returned when an error occurs during logout
	ErrLoggingOut = errors.New("error in logging out")
	// ErrInvalidCode is returned when an OAuth code is invalid
	ErrInvalidCode = errors.New("invalid code")
	// ErrInvalidStatus is returned when a deployment status is invalid
	ErrInvalidStatus = errors.New("invalid status")
	// ErrGithubInstallationFetchFailed is returned when fetching the GitHub installation fails
	ErrGithubInstallationFetchFailed = errors.New("github installation failed: error installing github app")
	// ErrUserLookupFailed is returned when looking up a user fails
	ErrUserLookupFailed = errors.New("user lookup failed")

	// ErrEventNotSpecifiedToParse is returned when no event is specified to parse
	ErrEventNotSpecifiedToParse = errors.New("no Event specified to parse")
	// ErrInvalidHTTPMethod is returned when an HTTP method is invalid
	ErrInvalidHTTPMethod = errors.New("invalid HTTP Method")
	// ErrMissingGithubEventHeader is returned when the X-GitHub-Event header is missing
	ErrMissingGithubEventHeader = errors.New("missing X-GitHub-Event Header")
	// ErrMissingHubSignatureHeader is returned when the X-Hub-Signature-256 header is missing
	ErrMissingHubSignatureHeader = errors.New("missing X-Hub-Signature-256 Header")
	// ErrEventNotFound is returned when an event is not defined to be parsed
	ErrEventNotFound = errors.New("event not defined to be parsed")
	// ErrParsingPayload is returned when a payload cannot be parsed
	ErrParsingPayload = errors.New("error parsing payload")
	// ErrHMACVerificationFailed is returned when HMAC verification fails
	ErrHMACVerificationFailed = errors.New("HMAC verification failed")
)
