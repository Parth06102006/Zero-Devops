package dispatcher

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"testing"
	"time"

	"Zero_Devops/server/internal/deployments/contract"
	"Zero_Devops/server/internal/domain"

	amqp "github.com/rabbitmq/amqp091-go"
	"go.uber.org/zap"
)

// fakeRepo records every outbox state transition the dispatcher requests.
type fakeRepo struct {
	claimFn func(ctx context.Context, limit int) ([]domain.OutboxEvent, error)

	claimCalls      int
	claimLimits     []int
	markSentIDs     []string
	markSentErr     error
	markFailedCalls []markFailedCall
	markFailedErr   error
	resetCalls      []time.Duration
	resetCount      int64
	resetErr        error
	deleteCalls     []time.Duration
	deleteCount     int64
	deleteErr       error
}

type markFailedCall struct {
	id          string
	errMsg      string
	availableAt time.Time
}

func (f *fakeRepo) ClaimOutboxBatch(ctx context.Context, limit int) ([]domain.OutboxEvent, error) {
	f.claimCalls++
	f.claimLimits = append(f.claimLimits, limit)
	if f.claimFn != nil {
		return f.claimFn(ctx, limit)
	}
	return nil, nil
}

func (f *fakeRepo) MarkOutboxSent(_ context.Context, id string) error {
	f.markSentIDs = append(f.markSentIDs, id)
	return f.markSentErr
}

func (f *fakeRepo) MarkOutboxPublishFailed(_ context.Context, id, errMsg string, availableAt time.Time) error {
	f.markFailedCalls = append(f.markFailedCalls, markFailedCall{id: id, errMsg: errMsg, availableAt: availableAt})
	return f.markFailedErr
}

func (f *fakeRepo) ResetStuckPublishing(_ context.Context, olderThan time.Duration) (int64, error) {
	f.resetCalls = append(f.resetCalls, olderThan)
	return f.resetCount, f.resetErr
}

func (f *fakeRepo) DeleteSentOutboxOlderThan(_ context.Context, olderThan time.Duration) (int64, error) {
	f.deleteCalls = append(f.deleteCalls, olderThan)
	return f.deleteCount, f.deleteErr
}

// Unused DeploymentRepository methods: the dispatcher never calls them.
func (f *fakeRepo) StoreWebhookBuildWithOutbox(_ context.Context, _ domain.StoreWebhookBuildParams) (*domain.Deployment, error) {
	return nil, nil
}
func (f *fakeRepo) GetByUserID(_ context.Context, _ string) ([]domain.Deployment, error) {
	return nil, nil
}
func (f *fakeRepo) GetByID(_ context.Context, _, _ string) (*domain.Deployment, error) {
	return nil, nil
}
func (f *fakeRepo) GetByProjectID(_ context.Context, _, _ string) ([]domain.Deployment, error) {
	return nil, nil
}
func (f *fakeRepo) StoreProjectBuildWithOutbox(_ context.Context, _ domain.StoreProjectBuildWithOutboxParams) (*domain.Deployment, error) {
	return nil, nil
}
func (f *fakeRepo) ApplyStatusUpdate(_ context.Context, _ domain.ApplyStatusParams) (*domain.ApplyStatusResult, error) {
	return nil, nil
}

// fakePublisher implements the confirmedPublisher seam.
type fakePublisher struct {
	published []amqp.Publishing
	ack       bool
	err       error
}

func (p *fakePublisher) PublishConfirmed(_ context.Context, message amqp.Publishing) (bool, error) {
	p.published = append(p.published, message)
	return p.ack, p.err
}

func validV1Payload(t *testing.T) []byte {
	t.Helper()
	job := contract.BuildRequestV1{
		Version:        contract.VersionV1,
		EventID:        "evt-1",
		DeploymentID:   "dep-1",
		ProjectID:      "proj-1",
		InstallationID: 10,
		RepositoryID:   99,
		CloneURL:       "https://github.com/acme/app.git",
		CommitSHA:      "0123456789abcdef0123456789abcdef01234567",
		RequestedRef:   "main",
		Trigger:        contract.TriggerManual,
		Generation:     3,
		CorrelationID:  "corr-1",
		Configuration: contract.Configuration{
			Executable:           "npm",
			Args:                 []string{"run", "build"},
			WorkingDir:           ".",
			ScannerPolicyVersion: "policy-v1",
		},
	}
	payload, err := json.Marshal(job)
	if err != nil {
		t.Fatalf("marshal payload: %v", err)
	}
	return payload
}

func testEvent(t *testing.T) domain.OutboxEvent {
	t.Helper()
	return domain.OutboxEvent{
		ID:             "ob-1",
		DeploymentID:   "dep-1",
		EventType:      domain.WebhookBuildEventType,
		MessageVersion: contract.VersionV1,
		Payload:        validV1Payload(t),
		State:          domain.OutboxStatePublishing,
		AttemptCount:   1,
	}
}

func newTestOutbox(repo *fakeRepo, cfg Config) *Outbox {
	return &Outbox{
		deploymentRepo: repo,
		log:            zap.NewNop(),
		config:         cfg,
		workerID:       "test-worker",
	}
}

func TestDispatchEvent_BrokerAckMarksSent(t *testing.T) {
	repo := &fakeRepo{}
	pub := &fakePublisher{ack: true}
	o := newTestOutbox(repo, Config{BackoffBase: time.Second, BackoffMax: time.Minute})

	err := o.dispatchEvent(context.Background(), pub, testEvent(t))
	if err != nil {
		t.Fatalf("expected success, got %v", err)
	}
	if len(repo.markSentIDs) != 1 || repo.markSentIDs[0] != "ob-1" {
		t.Fatalf("expected MarkOutboxSent(ob-1), got %v", repo.markSentIDs)
	}
	if len(repo.markFailedCalls) != 0 {
		t.Fatalf("expected no failure marks, got %v", repo.markFailedCalls)
	}
}

func TestDispatchEvent_BrokerNackMarksFailedWithBackoff(t *testing.T) {
	repo := &fakeRepo{}
	pub := &fakePublisher{ack: false}
	o := newTestOutbox(repo, Config{BackoffBase: time.Second, BackoffMax: time.Minute})
	event := testEvent(t) // AttemptCount 1 -> backoff 2s

	before := time.Now()
	err := o.dispatchEvent(context.Background(), pub, event)
	if !errors.Is(err, domain.ErrOutboxConfirmRejected) {
		t.Fatalf("expected ErrOutboxConfirmRejected, got %v", err)
	}
	if len(repo.markSentIDs) != 0 {
		t.Fatalf("must not mark sent after NACK, got %v", repo.markSentIDs)
	}
	if len(repo.markFailedCalls) != 1 {
		t.Fatalf("expected one failure mark, got %v", repo.markFailedCalls)
	}
	call := repo.markFailedCalls[0]
	if call.id != "ob-1" {
		t.Fatalf("failure mark id = %q, want ob-1", call.id)
	}
	wantAvailable := before.Add(2 * time.Second)
	if call.availableAt.Before(wantAvailable.Add(-50*time.Millisecond)) || call.availableAt.After(wantAvailable.Add(50*time.Millisecond)) {
		t.Fatalf("failure availableAt = %v, want ~%v (2s backoff)", call.availableAt, wantAvailable)
	}
}

func TestDispatchEvent_PublishErrorMarksFailedWithBackoff(t *testing.T) {
	repo := &fakeRepo{}
	pub := &fakePublisher{err: errors.New("connection reset")}
	o := newTestOutbox(repo, Config{BackoffBase: time.Second, BackoffMax: time.Minute})
	event := testEvent(t)

	before := time.Now()
	err := o.dispatchEvent(context.Background(), pub, event)
	if !errors.Is(err, domain.ErrOutboxPublishFailed) {
		t.Fatalf("expected ErrOutboxPublishFailed, got %v", err)
	}
	if len(repo.markSentIDs) != 0 {
		t.Fatalf("must not mark sent after publish error, got %v", repo.markSentIDs)
	}
	if len(repo.markFailedCalls) != 1 {
		t.Fatalf("expected one failure mark, got %v", repo.markFailedCalls)
	}
	// AttemptCount 1 -> 2s backoff from BackoffBase 1s.
	wantAvailable := before.Add(2 * time.Second)
	if call := repo.markFailedCalls[0]; call.availableAt.Sub(wantAvailable) > 50*time.Millisecond || wantAvailable.Sub(call.availableAt) > 50*time.Millisecond {
		t.Fatalf("failure availableAt = %v, want ~%v", call.availableAt, wantAvailable)
	}
}

func TestDispatchEvent_PublishErrorDuringShutdownLeavesRowUnchanged(t *testing.T) {
	repo := &fakeRepo{}
	pub := &fakePublisher{err: context.Canceled}
	o := newTestOutbox(repo, Config{BackoffBase: time.Second, BackoffMax: time.Minute})

	ctx, cancel := context.WithCancel(context.Background())
	cancel()

	err := o.dispatchEvent(ctx, pub, testEvent(t))
	if err == nil {
		t.Fatal("expected an error, got nil")
	}
	if len(repo.markSentIDs) != 0 || len(repo.markFailedCalls) != 0 {
		t.Fatalf("shutdown must leave the row untouched for reconciliation; sent=%v failed=%v",
			repo.markSentIDs, repo.markFailedCalls)
	}
}

func TestDispatchEvent_InvalidJSONIsPoison(t *testing.T) {
	repo := &fakeRepo{}
	pub := &fakePublisher{ack: true}
	o := newTestOutbox(repo, Config{})
	event := testEvent(t)
	event.Payload = []byte("{not json")

	before := time.Now()
	err := o.dispatchEvent(context.Background(), pub, event)
	if !errors.Is(err, domain.ErrOutboxPayloadInvalid) {
		t.Fatalf("expected ErrOutboxPayloadInvalid, got %v", err)
	}
	if len(pub.published) != 0 {
		t.Fatal("poison payload must not reach the broker")
	}
	if len(repo.markFailedCalls) != 1 {
		t.Fatalf("expected one failure mark, got %v", repo.markFailedCalls)
	}
	// Poison events retry immediately (no transport backoff).
	if call := repo.markFailedCalls[0]; call.availableAt.Sub(before) > 50*time.Millisecond {
		t.Fatalf("poison availableAt = %v, want immediate (~%v)", call.availableAt, before)
	}
}

func TestDispatchEvent_WrongEventTypeIsPoison(t *testing.T) {
	repo := &fakeRepo{}
	pub := &fakePublisher{ack: true}
	o := newTestOutbox(repo, Config{})
	event := testEvent(t)
	event.EventType = "deploy.status"

	err := o.dispatchEvent(context.Background(), pub, event)
	if !errors.Is(err, domain.ErrOutboxPayloadInvalid) {
		t.Fatalf("expected ErrOutboxPayloadInvalid, got %v", err)
	}
	if len(pub.published) != 0 {
		t.Fatal("wrong event type must not be published to deploy.jobs")
	}
}

func TestDispatchEvent_UnsupportedMessageVersionIsPoison(t *testing.T) {
	repo := &fakeRepo{}
	pub := &fakePublisher{ack: true}
	o := newTestOutbox(repo, Config{})
	event := testEvent(t)
	event.MessageVersion = 2

	err := o.dispatchEvent(context.Background(), pub, event)
	if !errors.Is(err, domain.ErrOutboxPayloadInvalid) {
		t.Fatalf("expected ErrOutboxPayloadInvalid, got %v", err)
	}
	if len(pub.published) != 0 {
		t.Fatal("unsupported version must not be published")
	}
}

func TestDispatchEvent_MarkSentFailureAfterAckKeepsRowRecoverable(t *testing.T) {
	repo := &fakeRepo{markSentErr: errors.New("db down")}
	pub := &fakePublisher{ack: true}
	o := newTestOutbox(repo, Config{})

	err := o.dispatchEvent(context.Background(), pub, testEvent(t))
	if !errors.Is(err, domain.ErrOutboxMarkSentFailed) {
		t.Fatalf("expected ErrOutboxMarkSentFailed, got %v", err)
	}
	if len(pub.published) != 1 {
		t.Fatal("broker ACK happened, the message must have been published")
	}
	// No failure mark: the row stays publishing so ResetStuckPublishing can
	// recover it (at-least-once duplicate, never a lost build).
	if len(repo.markFailedCalls) != 0 {
		t.Fatalf("mark-sent failure must not mark the row failed, got %v", repo.markFailedCalls)
	}
}

func TestDispatchEvent_PublishesValidEnvelope(t *testing.T) {
	repo := &fakeRepo{}
	pub := &fakePublisher{ack: true}
	o := newTestOutbox(repo, Config{})

	if err := o.dispatchEvent(context.Background(), pub, testEvent(t)); err != nil {
		t.Fatalf("dispatch failed: %v", err)
	}
	if len(pub.published) != 1 {
		t.Fatalf("expected one publish, got %d", len(pub.published))
	}
	msg := pub.published[0]
	if msg.MessageId != "evt-1" {
		t.Fatalf("message id = %q, want evt-1", msg.MessageId)
	}
	if msg.CorrelationId != "corr-1" {
		t.Fatalf("correlation id = %q, want corr-1", msg.CorrelationId)
	}
	if msg.ContentType != "application/json" {
		t.Fatalf("content type = %q, want application/json", msg.ContentType)
	}
	if msg.DeliveryMode != amqp.Persistent {
		t.Fatalf("delivery mode = %d, want persistent (%d)", msg.DeliveryMode, amqp.Persistent)
	}
	if v, ok := msg.Headers[contract.HeaderVersion]; !ok || v != int32(contract.VersionV1) {
		t.Fatalf("version header = %v (%T), want int32(%d)", v, v, contract.VersionV1)
	}
}

func TestDispatchBatch_EmptyBatchReturnsImmediately(t *testing.T) {
	repo := &fakeRepo{}
	pub := &fakePublisher{ack: true}
	o := newTestOutbox(repo, Config{BatchSize: 10})

	o.dispatchBatch(context.Background(), pub)
	if repo.claimCalls != 1 {
		t.Fatalf("expected one claim, got %d", repo.claimCalls)
	}
	if len(pub.published) != 0 {
		t.Fatalf("nothing should be published, got %d", len(pub.published))
	}
}

func TestDispatchBatch_ClaimErrorReturns(t *testing.T) {
	repo := &fakeRepo{claimFn: func(context.Context, int) ([]domain.OutboxEvent, error) {
		return nil, errors.New("db unavailable")
	}}
	pub := &fakePublisher{ack: true}
	o := newTestOutbox(repo, Config{})

	o.dispatchBatch(context.Background(), pub)
	if repo.claimCalls != 1 {
		t.Fatalf("claim error must stop the batch, got %d claims", repo.claimCalls)
	}
}

func TestDispatchBatch_DrainsFullBatchesWithoutWaiting(t *testing.T) {
	event := testEvent(t)
	batches := 0
	repo := &fakeRepo{claimFn: func(_ context.Context, limit int) ([]domain.OutboxEvent, error) {
		batches++
		if batches == 1 {
			full := make([]domain.OutboxEvent, limit)
			for i := range full {
				full[i] = event
			}
			return full, nil
		}
		return nil, nil // second claim: short (empty) batch
	}}
	pub := &fakePublisher{ack: true}
	o := newTestOutbox(repo, Config{BatchSize: 4})

	o.dispatchBatch(context.Background(), pub)
	if repo.claimCalls != 2 {
		t.Fatalf("full batch must trigger an immediate re-claim, got %d claims", repo.claimCalls)
	}
	if len(pub.published) != 4 {
		t.Fatalf("expected 4 publishes, got %d", len(pub.published))
	}
}

func TestDispatchBatch_StopsOnCanceledContext(t *testing.T) {
	repo := &fakeRepo{claimFn: func(context.Context, int) ([]domain.OutboxEvent, error) {
		return nil, fmt.Errorf("should never be called")
	}}
	pub := &fakePublisher{ack: true}
	o := newTestOutbox(repo, Config{})

	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	o.dispatchBatch(ctx, pub)
	if repo.claimCalls != 0 {
		t.Fatalf("cancelled context must not claim, got %d claims", repo.claimCalls)
	}
}

func TestBackoff_CappedExponential(t *testing.T) {
	o := newTestOutbox(&fakeRepo{}, Config{BackoffBase: time.Second, BackoffMax: 8 * time.Second})

	cases := []struct {
		attempts int
		want     time.Duration
	}{
		{0, time.Second},
		{1, 2 * time.Second},
		{2, 4 * time.Second},
		{3, 8 * time.Second},   // == max
		{4, 8 * time.Second},   // capped
		{100, 8 * time.Second}, // capped
		{-1, time.Second},      // negative clamped
	}
	for _, tc := range cases {
		if got := o.backoff(tc.attempts); got != tc.want {
			t.Fatalf("backoff(%d) = %v, want %v", tc.attempts, got, tc.want)
		}
	}
}

func TestBackoff_ZeroConfigFallsBackToDefaults(t *testing.T) {
	o := newTestOutbox(&fakeRepo{}, Config{})
	if got := o.backoff(0); got != defaultBackoffBase {
		t.Fatalf("backoff(0) with zero config = %v, want %v", got, defaultBackoffBase)
	}
	if got := o.backoff(100); got != defaultBackoffMax {
		t.Fatalf("backoff(100) with zero config = %v, want %v", got, defaultBackoffMax)
	}
}

func TestReconcile_ResetsStuckAndDeletesOld(t *testing.T) {
	repo := &fakeRepo{resetCount: 3, deleteCount: 7}
	o := newTestOutbox(repo, Config{StuckLease: time.Minute, Retention: 24 * time.Hour})

	o.reconcile(context.Background())
	if len(repo.resetCalls) != 1 || repo.resetCalls[0] != time.Minute {
		t.Fatalf("ResetStuckPublishing calls = %v, want [1m]", repo.resetCalls)
	}
	if len(repo.deleteCalls) != 1 || repo.deleteCalls[0] != 24*time.Hour {
		t.Fatalf("DeleteSentOutboxOlderThan calls = %v, want [24h]", repo.deleteCalls)
	}
}

func TestReconcile_ErrorsAreSurvivable(t *testing.T) {
	repo := &fakeRepo{resetErr: errors.New("reset failed"), deleteErr: errors.New("delete failed")}
	o := newTestOutbox(repo, Config{})

	// Must not panic or propagate: reconcile only logs.
	o.reconcile(context.Background())
}

func TestRun_FailsFastOnNilDependencies(t *testing.T) {
	var nilOutbox *Outbox
	if err := nilOutbox.Run(context.Background()); !errors.Is(err, domain.ErrOutboxDispatcherUnavailable) {
		t.Fatalf("nil receiver: expected ErrOutboxDispatcherUnavailable, got %v", err)
	}

	o := newTestOutbox(&fakeRepo{}, Config{})
	if err := o.Run(context.Background()); !errors.Is(err, domain.ErrOutboxDispatcherUnavailable) {
		t.Fatalf("nil connection: expected ErrOutboxDispatcherUnavailable, got %v", err)
	}

	o = &Outbox{queueConn: &amqp.Connection{}, log: zap.NewNop()}
	if err := o.Run(context.Background()); !errors.Is(err, domain.ErrOutboxDispatcherUnavailable) {
		t.Fatalf("nil repository: expected ErrOutboxDispatcherUnavailable, got %v", err)
	}
}

func TestWithDefaults_FillsZeroValues(t *testing.T) {
	o := newTestOutbox(&fakeRepo{}, Config{})
	o.withDefaults()
	cfg := o.config
	if cfg.PollInterval != defaultPollInterval || cfg.BatchSize != defaultBatchSize ||
		cfg.BackoffBase != defaultBackoffBase || cfg.BackoffMax != defaultBackoffMax ||
		cfg.Retention != defaultRetention || cfg.ReconcileEvery != defaultReconcileEvery {
		t.Fatalf("zero config not defaulted: %+v", cfg)
	}
	if cfg.StuckLease < defaultStuckLease {
		t.Fatalf("stuck lease %v must not be shorter than the claim lease %v", cfg.StuckLease, defaultStuckLease)
	}
}

func TestWithDefaults_BackoffMaxNeverBelowBase(t *testing.T) {
	o := newTestOutbox(&fakeRepo{}, Config{BackoffBase: time.Minute, BackoffMax: time.Second})
	o.withDefaults()
	if o.config.BackoffMax < o.config.BackoffBase {
		t.Fatalf("BackoffMax %v below BackoffBase %v", o.config.BackoffMax, o.config.BackoffBase)
	}
}
