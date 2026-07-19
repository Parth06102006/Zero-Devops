package domain

import "errors"

var (
	// ErrProviderNotSupported will throw if any of the supported providers is not found
	ErrProviderNotSupported 		 = errors.New("the requested oauth provider is not supported")
	// ErrInternalServerError will throw if any the Internal Server Error happen
	ErrInternalServerError 			 = errors.New("internal Server Error")
	// ErrNotFound will throw if the requested item is not exists
	ErrNotFound 					 = errors.New("your requested Item is not found")
	// ErrConflict will throw if the current action already exists
	ErrConflict 					 = errors.New("your Item already exist")
	// ErrBadParamInput will throw if the given request-body or params is not valid
	ErrBadParamInput 				 = errors.New("given Param is not valid")
	
	ErrInvalidToken 				 = errors.New("invalid or expired token")

	ErrMissingSecret 				 = errors.New("secret not found")

	ErrLoggingOut 					 = errors.New("error in logging out")

	ErrInvalidCode 					 = errors.New("invalid code")

	ErrInvalidStatus 				 = errors.New("invalid status")

	ErrGithubInstallationFetchFailed = errors.New("github installation failed: error installing github app")

	ErrUserLookupFailed 			 = errors.New("user lookup failed")

	// Github Webhook Errors

	ErrEventNotSpecifiedToParse  	 = errors.New("no Event specified to parse")

	ErrInvalidHTTPMethod         	 = errors.New("invalid HTTP Method")

	ErrMissingGithubEventHeader  	 = errors.New("missing X-GitHub-Event Header")

	ErrMissingHubSignatureHeader 	 = errors.New("missing X-Hub-Signature-256 Header")

	ErrEventNotFound            	 = errors.New("event not defined to be parsed")

	ErrParsingPayload           	 = errors.New("error parsing payload")

	ErrHMACVerificationFailed   	 = errors.New("HMAC verification failed")
)
