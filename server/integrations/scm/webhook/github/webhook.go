// Package github provides GitHub webhook handling
package github

import (
	"Zero_Devops/server/domain"
	"crypto/hmac"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"strings"
)

// Option is a configuration option for the webhook
type Option func(*Webhook) error

// Options is a namespace var for configuration options
var Options = WebhookOptions{}

// WebhookOptions is a namespace for configuration option methods
type WebhookOptions struct{}

// Webhook instance contains all methods needed to process events
type Webhook struct {
	secret string
}

// New creates and returns a WebHook instance denoted by the Provider type
func New(options ...Option) (*Webhook, error) {
	hook := new(Webhook)
	for _, opt := range options {
		if err := opt(hook); err != nil {
			return nil, errors.New("error applying option")
		}
	}
	return hook, nil
}

// Parse verifies and parses the events specified and returns the payload object or an error
func (hook Webhook) Parse(r *http.Request, events ...domain.Event) (interface{}, error) {
	defer func() {
		_, _ = io.Copy(io.Discard, r.Body)
		_ = r.Body.Close()
	}()

	if len(events) == 0 {
		return nil, domain.ErrEventNotSpecifiedToParse
	}
	if r.Method != http.MethodPost {
		return nil, domain.ErrInvalidHTTPMethod
	}

	event := r.Header.Get("X-GitHub-Event")
	if event == "" {
		return nil, domain.ErrMissingGithubEventHeader
	}
	gitHubEvent := domain.Event(event)

	var found bool
	for _, evt := range events {
		if evt == gitHubEvent {
			found = true
			break
		}
	}
	if !found {
		return nil, domain.ErrEventNotFound
	}

	payload, err := io.ReadAll(r.Body)
	if err != nil || len(payload) == 0 {
		return nil, domain.ErrParsingPayload
	}

	if hook.secret != "" {
		signature := r.Header.Get("X-Hub-Signature-256")
		if signature == "" {
			return nil, domain.ErrMissingHubSignatureHeader
		}

		signature = strings.TrimPrefix(signature, "sha256=")

		mac := hmac.New(sha256.New, []byte(hook.secret))
		_, _ = mac.Write(payload)
		expectedMAC := hex.EncodeToString(mac.Sum(nil))

		if !hmac.Equal([]byte(signature), []byte(expectedMAC)) {
			return nil, domain.ErrHMACVerificationFailed
		}
	}

	switch gitHubEvent {
	case domain.InstallationEvent:
		var pl domain.InstallationPayload
		err = json.Unmarshal(payload, &pl)
		return pl, err
	case domain.InstallationRepositoriesEvent:
		var pl domain.InstallationRepositoriesPayload
		err = json.Unmarshal(payload, &pl)
		return pl, err
	case domain.PushEventP:
		var pl domain.PushPayload
		err = json.Unmarshal(payload, &pl)
		return pl, err

	default:
		return nil, fmt.Errorf("unknown event %s", gitHubEvent)
	}
}
