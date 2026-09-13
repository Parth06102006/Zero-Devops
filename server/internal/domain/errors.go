package domain

import (
	"errors"
	"fmt"
)

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
	// ErrInvalidStatusTransition is returned when a deployment status update
	// would regress an already-terminal deployment status
	ErrInvalidStatusTransition = errors.New("invalid status transition")
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

	// ErrCommandDenied is returned when a build command fails command scan / policy validation
	ErrCommandDenied = errors.New("build command was denied by the security policy")

	// ErrInvalidPayloadSize is returned when the payload size is invalid
	ErrInvalidPayloadSize = errors.New("payload size is invalid")

	// ErrPayloadTooLarge is returned when the payload size becomes large
	ErrPayloadTooLarge = errors.New("payload size is too large then the given limit")

	// ErrMissingGithubDeliveryHeader is returned when the GitHub delivery header is missing or invalid.
	ErrMissingGithubDeliveryHeader = errors.New("github delivery id is invalid")
)

// Outbox dispatcher errors. The dispatcher maps every failure to one of
// these so callers (and tests) can distinguish poison payloads, which can
// never be published, from transient broker failures, which are retried
// with backoff. Internal classification: a poison payload must be recorded
// via MarkOutboxPublishFailed with an immediate retry so it dead-letters;
// transient failures are recorded with a backoff-adjusted available_at.
var (
	// ErrOutboxDispatcherUnavailable is returned when the dispatcher cannot
	// start or continue — e.g. the AMQP channel cannot be opened or confirm
	// mode cannot be enabled. Callers should treat this as fatal for the
	// dispatcher goroutine (it will be restarted or the process should exit).
	ErrOutboxDispatcherUnavailable = errors.New("outbox dispatcher unavailable")

	// ErrOutboxPayloadInvalid is returned when an outbox event's payload
	// fails decode or validation (contract.DecodeV1 / BuildRequestV1.Validate).
	// The event is a poison message: retrying will never succeed, so it is
	// recorded as a publish failure with no backoff and eventually dead-letters
	// for inspection.
	ErrOutboxPayloadInvalid = errors.New("outbox payload is invalid")

	// ErrOutboxPublishFailed is returned when publishing to the broker fails
	// transiently (connection/channel error). The outbox row is retried with
	// exponential backoff, so this error carries the next available_at time.
	ErrOutboxPublishFailed = errors.New("outbox publish failed transiently")

	// ErrOutboxConfirmRejected is returned when the broker explicitly nacked
	// a published message. The message is definitely not in the queue, so the
	// event is retried with backoff.
	ErrOutboxConfirmRejected = errors.New("broker rejected the published message")

	// ErrOutboxMarkSentFailed is returned when an event that was confirmed by
	// the broker could not be marked sent in the database. The row stays in
	// publishing state and is recovered by ResetStuckPublishing, which may
	// republish it — duplicate delivery is handled by worker idempotency.
	ErrOutboxMarkSentFailed = errors.New("failed to mark outbox event as sent")
)

// WrapOutboxError decorates an underlying dispatcher error with the given
// domain sentinel so callers can errors.Is against the sentinel while the
// original cause remains visible in logs. It returns nil if err is nil.
func WrapOutboxError(sentinel, err error) error {
	if err == nil {
		return nil
	}
	return fmt.Errorf("%w: %w", sentinel, err)
}
