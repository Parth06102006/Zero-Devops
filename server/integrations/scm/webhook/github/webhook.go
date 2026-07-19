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
			return nil, errors.New("Error applying Option")
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
	// event not defined to be parsed
	if !found {
		return nil, domain.ErrEventNotFound
	}

	payload, err := io.ReadAll(r.Body)
	if err != nil || len(payload) == 0 {
		return nil, domain.ErrParsingPayload
	}

	// If we have a Secret set, we should check the MAC
	if len(hook.secret) > 0 {
		signature := r.Header.Get("X-Hub-Signature-256")
		if len(signature) == 0 {
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
	case domain.PushEvent_P:
		var pl domain.PushPayload
		err = json.Unmarshal(payload, &pl)
		return pl, err

		
	// Keep the remaining GitHub event parsers here for later expansion.
	// case domain.CheckRunEvent:
	// 	var pl CheckRunPayload
	// 	err = json.Unmarshal([]byte(payload), &pl)
	// 	return pl, err
	// case domain.CheckSuiteEvent:
	// 	var pl CheckSuitePayload
	// 	err = json.Unmarshal([]byte(payload), &pl)
	// 	return pl, err
	// case domain.CommitCommentEvent:
	// 	var pl CommitCommentPayload
	// 	err = json.Unmarshal([]byte(payload), &pl)
	// 	return pl, err
	// case domain.CreateEvent:
	// 	var pl CreatePayload
	// 	err = json.Unmarshal([]byte(payload), &pl)
	// 	return pl, err
	// case domain.DeployKeyEvent:
	// 	var pl DeployKeyPayload
	// 	err = json.Unmarshal([]byte(payload), &pl)
	// 	return pl, err
	// case domain.DeleteEvent:
	// 	var pl DeletePayload
	// 	err = json.Unmarshal([]byte(payload), &pl)
	// 	return pl, err
	// case domain.DependabotAlertEvent:
	// 	var pl DependabotAlertPayload
	// 	err = json.Unmarshal([]byte(payload), &pl)
	// 	return pl, err
	// case domain.DeploymentEvent:
	// 	var pl DeploymentPayload
	// 	err = json.Unmarshal([]byte(payload), &pl)
	// 	return pl, err
	// case domain.DeploymentStatusEvent:
	// 	var pl DeploymentStatusPayload
	// 	err = json.Unmarshal([]byte(payload), &pl)
	// 	return pl, err
	// case domain.ForkEvent:
	// 	var pl ForkPayload
	// 	err = json.Unmarshal([]byte(payload), &pl)
	// 	return pl, err
	// case domain.GollumEvent:
	// 	var pl GollumPayload
	// 	err = json.Unmarshal([]byte(payload), &pl)
	// 	return pl, err
	// case domain.InstallationEvent, domain.IntegrationInstallationEvent:
	// 	var pl InstallationPayload
	// 	err = json.Unmarshal([]byte(payload), &pl)
	// 	return pl, err
	// case domain.InstallationRepositoriesEvent, domain.IntegrationInstallationRepositoriesEvent:
	// 	var pl InstallationRepositoriesPayload
	// 	err = json.Unmarshal([]byte(payload), &pl)
	// 	return pl, err
	// case domain.IssueCommentEvent:
	// 	var pl IssueCommentPayload
	// 	err = json.Unmarshal([]byte(payload), &pl)
	// 	return pl, err
	// case domain.IssuesEvent:
	// 	var pl IssuesPayload
	// 	err = json.Unmarshal([]byte(payload), &pl)
	// 	return pl, err
	// case domain.LabelEvent:
	// 	var pl LabelPayload
	// 	err = json.Unmarshal([]byte(payload), &pl)
	// 	return pl, err
	// case domain.MemberEvent:
	// 	var pl MemberPayload
	// 	err = json.Unmarshal([]byte(payload), &pl)
	// 	return pl, err
	// case domain.MembershipEvent:
	// 	var pl MembershipPayload
	// 	err = json.Unmarshal([]byte(payload), &pl)
	// 	return pl, err
	// case domain.MetaEvent:
	// 	var pl MetaPayload
	// 	err = json.Unmarshal([]byte(payload), &pl)
	// 	return pl, err
	// case domain.MilestoneEvent:
	// 	var pl MilestonePayload
	// 	err = json.Unmarshal([]byte(payload), &pl)
	// 	return pl, err
	// case domain.OrganizationEvent:
	// 	var pl OrganizationPayload
	// 	err = json.Unmarshal([]byte(payload), &pl)
	// 	return pl, err
	// case domain.OrgBlockEvent:
	// 	var pl OrgBlockPayload
	// 	err = json.Unmarshal([]byte(payload), &pl)
	// 	return pl, err
	// case domain.PageBuildEvent:
	// 	var pl PageBuildPayload
	// 	err = json.Unmarshal([]byte(payload), &pl)
	// 	return pl, err
	// case domain.PingEvent:
	// 	var pl PingPayload
	// 	err = json.Unmarshal([]byte(payload), &pl)
	// 	return pl, err
	// case domain.ProjectCardEvent:
	// 	var pl ProjectCardPayload
	// 	err = json.Unmarshal([]byte(payload), &pl)
	// 	return pl, err
	// case domain.ProjectColumnEvent:
	// 	var pl ProjectColumnPayload
	// 	err = json.Unmarshal([]byte(payload), &pl)
	// 	return pl, err
	// case domain.ProjectEvent:
	// 	var pl ProjectPayload
	// 	err = json.Unmarshal([]byte(payload), &pl)
	// 	return pl, err
	// case domain.PublicEvent:
	// 	var pl PublicPayload
	// 	err = json.Unmarshal([]byte(payload), &pl)
	// 	return pl, err
	// case domain.PullRequestEvent:
	// 	var pl PullRequestPayload
	// 	err = json.Unmarshal([]byte(payload), &pl)
	// 	return pl, err
	// case domain.PullRequestReviewEvent:
	// 	var pl PullRequestReviewPayload
	// 	err = json.Unmarshal([]byte(payload), &pl)
	// 	return pl, err
	// case domain.PullRequestReviewCommentEvent:
	// 	var pl PullRequestReviewCommentPayload
	// 	err = json.Unmarshal([]byte(payload), &pl)
	// 	return pl, err
	// case domain.ReleaseEvent:
	// 	var pl ReleasePayload
	// 	err = json.Unmarshal([]byte(payload), &pl)
	// 	return pl, err
	// case domain.RepositoryEvent:
	// 	var pl RepositoryPayload
	// 	err = json.Unmarshal([]byte(payload), &pl)
	// 	return pl, err
	// case domain.RepositoryVulnerabilityAlertEvent:
	// 	var pl RepositoryVulnerabilityAlertPayload
	// 	err = json.Unmarshal([]byte(payload), &pl)
	// 	return pl, err
	// case domain.SecurityAdvisoryEvent:
	// 	var pl SecurityAdvisoryPayload
	// 	err = json.Unmarshal([]byte(payload), &pl)
	// 	return pl, err
	// case domain.StatusEvent:
	// 	var pl StatusPayload
	// 	err = json.Unmarshal([]byte(payload), &pl)
	// 	return pl, err
	// case domain.TeamEvent:
	// 	var pl TeamPayload
	// 	err = json.Unmarshal([]byte(payload), &pl)
	// 	return pl, err
	// case domain.TeamAddEvent:
	// 	var pl TeamAddPayload
	// 	err = json.Unmarshal([]byte(payload), &pl)
	// 	return pl, err
	// case domain.WatchEvent:
	// 	var pl WatchPayload
	// 	err = json.Unmarshal([]byte(payload), &pl)
	// 	return pl, err
	// case domain.WorkflowDispatchEvent:
	// 	var pl WorkflowDispatchPayload
	// 	err = json.Unmarshal([]byte(payload), &pl)
	// 	return pl, err
	// case domain.WorkflowJobEvent:
	// 	var pl WorkflowJobPayload
	// 	err = json.Unmarshal([]byte(payload), &pl)
	// 	return pl, err
	// case domain.WorkflowRunEvent:
	// 	var pl WorkflowRunPayload
	// 	err = json.Unmarshal([]byte(payload), &pl)
	// 	return pl, err
	// case domain.GitHubAppAuthorizationEvent:
	// 	var pl GitHubAppAuthorizationPayload
	// 	err = json.Unmarshal([]byte(payload), &pl)
	// 	return pl, err
	// case domain.CodeScanningAlertEvent:
	// 	var pl CodeScanningAlertPayload
	// 	err = json.Unmarshal([]byte(payload), &pl)
	// 	return pl, err
	default:
		return nil, fmt.Errorf("unknown event %s", gitHubEvent)
	}
}
