package contract

import (
	"encoding/json"
	"reflect"
	"strings"
	"testing"

	amqp "github.com/rabbitmq/amqp091-go"
)

func validRequest() BuildRequestV1 {
	return BuildRequestV1{
		Version:        VersionV1,
		EventID:        "9c5b94b3-1f1c-4f3f-9c8e-0a1b2c3d4e5f",
		DeploymentID:   "dep-01J5XYZABC123",
		ProjectID:      "proj-01J5XYZABC123",
		InstallationID: 12345678,
		RepositoryID:   987654321,
		CloneURL:       "https://github.com/acme/web.git",
		CommitSHA:      "0123456789abcdef0123456789abcdef01234567",
		RequestedRef:   "refs/heads/main",
		Trigger:        TriggerWebhookPush,
		Generation:     7,
		RetryCount:     0,
		CorrelationID:  "req-9c5b94b3-1f1c",
		Configuration: Configuration{
			Executable:           "npm",
			Args:                 []string{"run", "build"},
			WorkingDir:           ".",
			ScannerPolicyVersion: "v1",
		},
	}
}

func TestValidate_Valid(t *testing.T) {
	if err := validRequest().Validate(); err != nil {
		t.Fatalf("Validate() = %v, want nil", err)
	}
}

func TestValidate_RejectsInvalid(t *testing.T) {
	base := validRequest()
	cases := []struct {
		name   string
		mutate func(*BuildRequestV1)
	}{
		{"unsupported version", func(b *BuildRequestV1) { b.Version = 2 }},
		{"missing event_id", func(b *BuildRequestV1) { b.EventID = "" }},
		{"missing deployment_id", func(b *BuildRequestV1) { b.DeploymentID = "" }},
		{"missing project_id", func(b *BuildRequestV1) { b.ProjectID = "" }},
		{"missing installation_id", func(b *BuildRequestV1) { b.InstallationID = 0 }},
		{"missing repository_id", func(b *BuildRequestV1) { b.RepositoryID = 0 }},
		{"missing clone_url", func(b *BuildRequestV1) { b.CloneURL = "" }},
		{"missing commit_sha", func(b *BuildRequestV1) { b.CommitSHA = "" }},
		{"short commit_sha", func(b *BuildRequestV1) { b.CommitSHA = "abc123" }},
		{"non-hex commit_sha", func(b *BuildRequestV1) { b.CommitSHA = "zzzzzzzzzzzzzzzzzzzzzzzzzzzzzzzzzzzzzzzz" }},
		{"uppercase commit_sha", func(b *BuildRequestV1) { b.CommitSHA = strings.ToUpper(base.CommitSHA) }},
		{"invalid trigger", func(b *BuildRequestV1) { b.Trigger = "scheduled" }},
		{"negative generation", func(b *BuildRequestV1) { b.Generation = -1 }},
		{"negative retry_count", func(b *BuildRequestV1) { b.RetryCount = -1 }},
		{"missing correlation_id", func(b *BuildRequestV1) { b.CorrelationID = "" }},
		{"missing configuration executable", func(b *BuildRequestV1) { b.Configuration.Executable = "" }},
		{"missing configuration args", func(b *BuildRequestV1) { b.Configuration.Args = nil }},
		{"missing configuration working_dir", func(b *BuildRequestV1) { b.Configuration.WorkingDir = "" }},
		{"missing scanner policy version", func(b *BuildRequestV1) { b.Configuration.ScannerPolicyVersion = "" }},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			req := base
			tc.mutate(&req)
			if err := req.Validate(); err == nil {
				t.Fatalf("Validate() = nil, want error for %s", tc.name)
			}
		})
	}
}

func TestDecodeV1_RoundTrip(t *testing.T) {
	req := validRequest()
	body, err := json.Marshal(req)
	if err != nil {
		t.Fatal(err)
	}

	decoded, err := DecodeV1(body)
	if err != nil {
		t.Fatalf("DecodeV1() = %v, want nil", err)
	}
	if !reflect.DeepEqual(decoded, req) {
		t.Fatalf("DecodeV1() = %+v, want %+v", decoded, req)
	}
}

func TestDecodeV1_RejectsMalformedAndUnsupported(t *testing.T) {
	valid := validRequest()
	validBody, err := json.Marshal(valid)
	if err != nil {
		t.Fatal(err)
	}

	unsupported := valid
	unsupported.Version = 2
	unsupportedBody, err := json.Marshal(unsupported)
	if err != nil {
		t.Fatal(err)
	}

	cases := []struct {
		name string
		body []byte
	}{
		{"malformed json", []byte("{not json")},
		{"unsupported version", unsupportedBody},
		{"missing required field", []byte(`{"version":1}`)},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			if _, err := DecodeV1(tc.body); err == nil {
				t.Fatalf("DecodeV1(%s) = nil, want error", tc.name)
			}
		})
	}

	// Sanity: the valid body must still decode.
	if _, err := DecodeV1(validBody); err != nil {
		t.Fatalf("DecodeV1(valid) = %v, want nil", err)
	}
}

func TestPublishing_Envelope(t *testing.T) {
	req := validRequest()
	pub, err := Publishing(req)
	if err != nil {
		t.Fatalf("Publishing() = %v, want nil", err)
	}

	if pub.ContentType != ContentTypeJSON {
		t.Errorf("ContentType = %q, want %s", pub.ContentType, ContentTypeJSON)
	}
	if pub.ContentEncoding != "utf-8" {
		t.Errorf("ContentEncoding = %q, want utf-8", pub.ContentEncoding)
	}
	if pub.DeliveryMode != amqp.Persistent {
		t.Errorf("DeliveryMode = %d, want persistent (%d)", pub.DeliveryMode, amqp.Persistent)
	}
	if pub.MessageId != req.EventID {
		t.Errorf("MessageId = %q, want event_id %q", pub.MessageId, req.EventID)
	}
	if pub.CorrelationId != req.CorrelationID {
		t.Errorf("CorrelationId = %q, want correlation_id %q", pub.CorrelationId, req.CorrelationID)
	}
	if got := pub.Headers[HeaderVersion]; got != int32(VersionV1) {
		t.Errorf("header %s = %v, want int32(%d)", HeaderVersion, got, VersionV1)
	}

	decoded, err := DecodeV1(pub.Body)
	if err != nil {
		t.Fatalf("published body does not decode: %v", err)
	}
	if !reflect.DeepEqual(decoded, req) {
		t.Fatalf("published body = %+v, want %+v", decoded, req)
	}
}

func TestPublishing_RejectsInvalid(t *testing.T) {
	req := validRequest()
	req.CommitSHA = "short"
	if _, err := Publishing(req); err == nil {
		t.Fatal("Publishing(invalid) = nil, want error")
	}
}

func TestValidateMetadata_Valid(t *testing.T) {
	req := validRequest()
	err := ValidateMetadata(req, ContentTypeJSON, req.EventID, req.CorrelationID, amqp.Table{
		HeaderVersion: int32(VersionV1),
	})
	if err != nil {
		t.Fatalf("ValidateMetadata() = %v, want nil", err)
	}
}

func TestValidateMetadata_RejectsMismatch(t *testing.T) {
	req := validRequest()
	cases := []struct {
		name          string
		contentType   string
		messageID     string
		correlationID string
		headers       amqp.Table
	}{
		{"wrong content type", "text/plain", req.EventID, req.CorrelationID, amqp.Table{HeaderVersion: int32(VersionV1)}},
		{"message_id mismatch", ContentTypeJSON, "other-event", req.CorrelationID, amqp.Table{HeaderVersion: int32(VersionV1)}},
		{"correlation_id mismatch", ContentTypeJSON, req.EventID, "other-correlation", amqp.Table{HeaderVersion: int32(VersionV1)}},
		{"header version mismatch", ContentTypeJSON, req.EventID, req.CorrelationID, amqp.Table{HeaderVersion: int32(2)}},
		{"header version wrong type", ContentTypeJSON, req.EventID, req.CorrelationID, amqp.Table{HeaderVersion: "one"}},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			err := ValidateMetadata(req, tc.contentType, tc.messageID, tc.correlationID, tc.headers)
			if err == nil {
				t.Fatalf("ValidateMetadata() = nil, want error for %s", tc.name)
			}
		})
	}
}
