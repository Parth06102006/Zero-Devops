// Package contract defines the versioned deploy.jobs RabbitMQ message contract
// shared between the server (producer) and worker-server (consumer).
//
// The single source of truth is the JSON Schema at the repo root:
// schemas/deploy-jobs-v1.schema.json. This package is the Go implementation of
// that schema and must stay in sync with it — any breaking change is a new
// version (V2) shipped alongside V1 for a bounded migration window, never an
// edit to V1 in place.
package contract

import (
	"encoding/json"
	"errors"
	"fmt"
	"regexp"

	amqp "github.com/rabbitmq/amqp091-go"
)

// VersionV1 is the deploy.jobs message contract version implemented here.
const VersionV1 = 1

// HeaderVersion is the AMQP header key that carries the contract version. It is
// an advisory routing aid; the body "version" field is authoritative.
const HeaderVersion = "x-contract-version"

// ContentTypeJSON is the AMQP Content-Type for deploy.jobs message bodies.
const ContentTypeJSON = "application/json"

// Trigger values for a build request.
const (
	TriggerManual      = "manual"
	TriggerWebhookPush = "webhook_push"
)

var commitSHAPattern = regexp.MustCompile(`^[0-9a-f]{40}$`)

// BuildRequestV1 is an immutable deploy.jobs job. Every field is supplied by
// the server at publish time; CommitSHA is mandatory and authoritative — the
// worker checks out this exact commit, never a moving branch head.
type BuildRequestV1 struct {
	Version        int           `json:"version"`
	EventID        string        `json:"event_id"`
	DeploymentID   string        `json:"deployment_id"`
	ProjectID      string        `json:"project_id"`
	InstallationID int64         `json:"installation_id"`
	RepositoryID   int64         `json:"repository_id"`
	CloneURL       string        `json:"clone_url"`
	CommitSHA      string        `json:"commit_sha"`
	RequestedRef   string        `json:"requested_ref,omitempty"`
	Trigger        string        `json:"trigger"`
	Generation     int64         `json:"generation"`
	RetryCount     int           `json:"retry_count"`
	CorrelationID  string        `json:"correlation_id"`
	Configuration  Configuration `json:"configuration"`
}

// Configuration is the approved, policy-validated build configuration snapshot.
// It is a structured command (executable + argument array + working directory),
// never an arbitrary shell string.
type Configuration struct {
	Executable           string   `json:"executable"`
	Args                 []string `json:"args"`
	WorkingDir           string   `json:"working_dir"`
	ScannerPolicyVersion string   `json:"scanner_policy_version"`
}

// Validate enforces the V1 schema's required fields and constraints. It is the
// Go mirror of schemas/deploy-jobs-v1.schema.json and must not diverge.
func (b BuildRequestV1) Validate() error {
	switch {
	case b.Version != VersionV1:
		return fmt.Errorf("unsupported deploy.jobs version %d (want %d)", b.Version, VersionV1)
	case b.EventID == "":
		return errors.New("event_id is required")
	case b.DeploymentID == "":
		return errors.New("deployment_id is required")
	case b.ProjectID == "":
		return errors.New("project_id is required")
	case b.InstallationID <= 0:
		return errors.New("installation_id is required")
	case b.RepositoryID <= 0:
		return errors.New("repository_id is required")
	case b.CloneURL == "":
		return errors.New("clone_url is required")
	case !commitSHAPattern.MatchString(b.CommitSHA):
		return errors.New("commit_sha must be a 40-character lowercase hex SHA")
	case b.Trigger != TriggerManual && b.Trigger != TriggerWebhookPush:
		return fmt.Errorf("trigger must be %q or %q, got %q", TriggerManual, TriggerWebhookPush, b.Trigger)
	case b.Generation < 0:
		return errors.New("generation must be >= 0")
	case b.RetryCount < 0:
		return errors.New("retry_count must be >= 0")
	case b.CorrelationID == "":
		return errors.New("correlation_id is required")
	case b.Configuration.Executable == "":
		return errors.New("configuration.executable is required")
	case b.Configuration.Args == nil:
		return errors.New("configuration.args is required")
	case b.Configuration.WorkingDir == "":
		return errors.New("configuration.working_dir is required")
	case b.Configuration.ScannerPolicyVersion == "":
		return errors.New("configuration.scanner_policy_version is required")
	}
	return nil
}

// DecodeV1 decodes and validates a deploy.jobs V1 message body. Malformed or
// unsupported messages return an error so callers can reject them (route to the
// DLQ) instead of falling back to legacy behavior.
func DecodeV1(body []byte) (BuildRequestV1, error) {
	var req BuildRequestV1
	if err := json.Unmarshal(body, &req); err != nil {
		return BuildRequestV1{}, fmt.Errorf("decode deploy.jobs V1 body: %w", err)
	}
	if err := req.Validate(); err != nil {
		return BuildRequestV1{}, err
	}
	return req, nil
}

// ValidateMetadata checks the AMQP envelope against the body. The body is
// authoritative; envelope mismatches are treated as contract violations.
func ValidateMetadata(req BuildRequestV1, contentType, messageID, correlationID string, headers amqp.Table) error {
	if contentType != ContentTypeJSON {
		return fmt.Errorf("unexpected content type %q", contentType)
	}
	if messageID != "" && messageID != req.EventID {
		return fmt.Errorf("message_id %q does not match event_id %q", messageID, req.EventID)
	}
	if correlationID != "" && correlationID != req.CorrelationID {
		return fmt.Errorf("correlation_id %q does not match body correlation_id %q", correlationID, req.CorrelationID)
	}
	if headers != nil {
		if h, ok := headers[HeaderVersion]; ok && h != nil {
			v, ok := h.(int32)
			if !ok {
				return fmt.Errorf("header %s has unsupported type %T", HeaderVersion, h)
			}
			if int(v) != VersionV1 {
				return fmt.Errorf("header %s is %d, want %d", HeaderVersion, v, VersionV1)
			}
		}
	}
	return nil
}

// Publishing builds the AMQP envelope for a validated V1 request.
func Publishing(req BuildRequestV1) (amqp.Publishing, error) {
	if err := req.Validate(); err != nil {
		return amqp.Publishing{}, err
	}
	body, err := json.Marshal(req)
	if err != nil {
		return amqp.Publishing{}, fmt.Errorf("marshal deploy.jobs V1 request: %w", err)
	}
	return amqp.Publishing{
		ContentType:     ContentTypeJSON,
		ContentEncoding: "utf-8",
		DeliveryMode:    amqp.Persistent,
		MessageId:       req.EventID,
		CorrelationId:   req.CorrelationID,
		// AMQP field tables decode signed integer headers as int32.
		Headers: amqp.Table{HeaderVersion: int32(VersionV1)},
		Body:    body,
	}, nil
}
