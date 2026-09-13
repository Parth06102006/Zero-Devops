package usecase

import (
	"context"
	"errors"
	"testing"
	"time"

	"Zero_Devops/server/internal/domain"
)

// fakeStatusAck implements statusAcknowledger and records every
// acknowledgement decision for assertions.
type fakeStatusAck struct {
	acks     int
	nacks    int
	requeued bool
	ackErr   error
	nackErr  error
}

func (f *fakeStatusAck) Ack(bool) error {
	if f.ackErr != nil {
		return f.ackErr
	}
	f.acks++
	return nil
}

func (f *fakeStatusAck) Nack(_, requeue bool) error {
	if f.nackErr != nil {
		return f.nackErr
	}
	f.nacks++
	f.requeued = requeue
	return nil
}

// statusTestHarness wires a mock repo and ack fake into a usecase value.
type statusTestHarness struct {
	uc   *deploymentUsecase
	repo *deploymentRepoMock
	ack  *fakeStatusAck
}

func newStatusTestHarness(applyFn func(ctx context.Context, params domain.ApplyStatusParams) (*domain.ApplyStatusResult, error)) *statusTestHarness {
	repo := &deploymentRepoMock{applyStatusUpdateFn: applyFn}
	return &statusTestHarness{
		uc:   &deploymentUsecase{deploymentRepo: repo},
		repo: repo,
		ack:  &fakeStatusAck{},
	}
}

// shortStatusBackoff shortens retry sleeps for the duration of a test.
func shortStatusBackoff(t *testing.T) {
	t.Helper()
	original := statusApplyBackoff
	statusApplyBackoff = []time.Duration{time.Millisecond, time.Millisecond}
	t.Cleanup(func() { statusApplyBackoff = original })
}

func statusBody(id, status, output string) []byte {
	return []byte(`{"deployment_id":"` + id + `","status":"` + status + `","output_url":"` + output + `","error_message":""}`)
}

func TestHandleStatusMessage_AcksAfterDurableApply(t *testing.T) {
	var gotParams domain.ApplyStatusParams
	h := newStatusTestHarness(func(_ context.Context, params domain.ApplyStatusParams) (*domain.ApplyStatusResult, error) {
		gotParams = params
		return &domain.ApplyStatusResult{IsCurrentGeneration: true}, nil
	})

	h.uc.handleStatusMessage(context.Background(), h.ack, statusBody("dep-1", "success", "https://out"))

	if h.ack.acks != 1 || h.ack.nacks != 0 {
		t.Fatalf("expected exactly one ack, got acks=%d nacks=%d", h.ack.acks, h.ack.nacks)
	}
	if gotParams.DeploymentID != "dep-1" ||
		gotParams.Status != domain.DeploymentStatusSuccess ||
		gotParams.OutputURL != "https://out" ||
		gotParams.ErrorMessage != "" {
		t.Fatalf("unexpected apply params: %+v", gotParams)
	}
}

func TestHandleStatusMessage_SupersededGenerationStillAcks(t *testing.T) {
	h := newStatusTestHarness(func(_ context.Context, _ domain.ApplyStatusParams) (*domain.ApplyStatusResult, error) {
		// Applied durably but an older generation: not an error.
		return &domain.ApplyStatusResult{IsCurrentGeneration: false}, nil
	})

	h.uc.handleStatusMessage(context.Background(), h.ack, statusBody("dep-1", "success", ""))

	if h.ack.acks != 1 || h.ack.nacks != 0 {
		t.Fatalf("superseded generation must still ack, got acks=%d nacks=%d", h.ack.acks, h.ack.nacks)
	}
}

func TestHandleStatusMessage_PoisonJSONDeadLetters(t *testing.T) {
	calls := 0
	h := newStatusTestHarness(func(context.Context, domain.ApplyStatusParams) (*domain.ApplyStatusResult, error) {
		calls++
		return nil, nil
	})

	h.uc.handleStatusMessage(context.Background(), h.ack, []byte(`{not json`))

	if h.ack.nacks != 1 || h.ack.requeued {
		t.Fatalf("expected one nack without requeue, got nacks=%d requeued=%v", h.ack.nacks, h.ack.requeued)
	}
	if h.ack.acks != 0 {
		t.Fatalf("poison must never be acked, got acks=%d", h.ack.acks)
	}
	if calls != 0 {
		t.Fatalf("poison must not reach the repository, got %d calls", calls)
	}
}

func TestHandleStatusMessage_InvalidStatusDeadLettersWithoutRepo(t *testing.T) {
	cases := []string{"", "deployed", "BUILDING", "unknown"}
	for _, status := range cases {
		calls := 0
		h := newStatusTestHarness(func(context.Context, domain.ApplyStatusParams) (*domain.ApplyStatusResult, error) {
			calls++
			return nil, nil
		})

		h.uc.handleStatusMessage(context.Background(), h.ack, statusBody("dep-1", status, ""))

		if h.ack.nacks != 1 || h.ack.requeued || h.ack.acks != 0 || calls != 0 {
			t.Fatalf("status %q: expected dead-letter without repo call, got acks=%d nacks=%d requeued=%v calls=%d",
				status, h.ack.acks, h.ack.nacks, h.ack.requeued, calls)
		}
	}
}

func TestHandleStatusMessage_EmptyDeploymentIDDeadLetters(t *testing.T) {
	calls := 0
	h := newStatusTestHarness(func(context.Context, domain.ApplyStatusParams) (*domain.ApplyStatusResult, error) {
		calls++
		return nil, nil
	})

	h.uc.handleStatusMessage(context.Background(), h.ack, statusBody("", "building", ""))

	if h.ack.nacks != 1 || h.ack.requeued || calls != 0 {
		t.Fatalf("expected dead-letter without repo call, got nacks=%d requeued=%v calls=%d", h.ack.nacks, h.ack.requeued, calls)
	}
}

func TestHandleStatusMessage_PermanentErrorsDeadLetterImmediately(t *testing.T) {
	cases := map[string]error{
		"not-found":          domain.ErrNotFound,
		"invalid-status":     domain.ErrInvalidStatus,
		"invalid-transition": domain.ErrInvalidStatusTransition,
		"bad-param":          domain.ErrBadParamInput,
	}
	for name, permanentErr := range cases {
		calls := 0
		h := newStatusTestHarness(func(context.Context, domain.ApplyStatusParams) (*domain.ApplyStatusResult, error) {
			calls++
			return nil, permanentErr
		})

		h.uc.handleStatusMessage(context.Background(), h.ack, statusBody("dep-1", "success", ""))

		if calls != 1 {
			t.Fatalf("%s: permanent errors must not be retried, got %d repo calls", name, calls)
		}
		if h.ack.nacks != 1 || h.ack.requeued {
			t.Fatalf("%s: expected immediate dead-letter, got nacks=%d requeued=%v", name, h.ack.nacks, h.ack.requeued)
		}
	}
}

func TestHandleStatusMessage_TransientErrorRetriesThenAcks(t *testing.T) {
	shortStatusBackoff(t)

	calls := 0
	h := newStatusTestHarness(func(context.Context, domain.ApplyStatusParams) (*domain.ApplyStatusResult, error) {
		calls++
		if calls == 1 {
			return nil, errors.New("db connection reset") // transient
		}
		return &domain.ApplyStatusResult{IsCurrentGeneration: true}, nil
	})

	h.uc.handleStatusMessage(context.Background(), h.ack, statusBody("dep-1", "building", ""))

	if calls != 2 {
		t.Fatalf("expected retry after transient failure, got %d repo calls", calls)
	}
	if h.ack.acks != 1 || h.ack.nacks != 0 {
		t.Fatalf("expected ack after successful retry, got acks=%d nacks=%d", h.ack.acks, h.ack.nacks)
	}
}

func TestHandleStatusMessage_TransientExhaustedDeadLetters(t *testing.T) {
	shortStatusBackoff(t)

	calls := 0
	h := newStatusTestHarness(func(context.Context, domain.ApplyStatusParams) (*domain.ApplyStatusResult, error) {
		calls++
		return nil, errors.New("db down")
	})

	h.uc.handleStatusMessage(context.Background(), h.ack, statusBody("dep-1", "success", ""))

	if calls != statusMaxApplyAttempts {
		t.Fatalf("expected exactly %d attempts, got %d", statusMaxApplyAttempts, calls)
	}
	// Never requeued: requeue would hot-loop against the struggling DB.
	if h.ack.nacks != 1 || h.ack.requeued {
		t.Fatalf("expected dead-letter after exhausted retries, got nacks=%d requeued=%v", h.ack.nacks, h.ack.requeued)
	}
	if h.ack.acks != 0 {
		t.Fatalf("must not ack an unapplied update, got acks=%d", h.ack.acks)
	}
}

func TestHandleStatusMessage_CtxCancelledMidRetryNeitherAcksNorNacks(t *testing.T) {
	shortStatusBackoff(t)

	ctx, cancel := context.WithCancel(context.Background())
	calls := 0
	h := newStatusTestHarness(func(context.Context, domain.ApplyStatusParams) (*domain.ApplyStatusResult, error) {
		calls++
		cancel() // shutdown lands after the first transient failure
		return nil, errors.New("db down")
	})

	h.uc.handleStatusMessage(ctx, h.ack, statusBody("dep-1", "building", ""))

	if h.ack.acks != 0 || h.ack.nacks != 0 {
		t.Fatalf("shutdown mid-retry must neither ack nor nack, got acks=%d nacks=%d", h.ack.acks, h.ack.nacks)
	}
	if calls != 1 {
		t.Fatalf("expected the retry wait to be cut short by ctx cancellation, got %d calls", calls)
	}
}

func TestHandleStatusMessage_AckFailureAfterCommitIsLoggedNotFatal(_ *testing.T) {
	h := newStatusTestHarness(func(context.Context, domain.ApplyStatusParams) (*domain.ApplyStatusResult, error) {
		return &domain.ApplyStatusResult{IsCurrentGeneration: true}, nil
	})
	h.ack.ackErr = errors.New("channel closed")

	// Must not panic and must not attempt a compensating nack: the broker
	// redelivers and the idempotent apply converges.
	h.uc.handleStatusMessage(context.Background(), h.ack, statusBody("dep-1", "success", ""))
}

func TestHandleStatusMessage_NackFailureIsLoggedNotFatal(_ *testing.T) {
	h := newStatusTestHarness(func(context.Context, domain.ApplyStatusParams) (*domain.ApplyStatusResult, error) {
		return nil, domain.ErrNotFound
	})
	h.ack.nackErr = errors.New("channel closed")

	h.uc.handleStatusMessage(context.Background(), h.ack, statusBody("dep-1", "success", ""))
}

func TestIsValidWorkerStatus(t *testing.T) {
	valid := []domain.DeploymentStatus{
		domain.DeploymentStatusPending,
		domain.DeploymentStatusBuilding,
		domain.DeploymentStatusSuccess,
		domain.DeploymentStatusFailed,
		domain.DeploymentStatusCanceled,
	}
	for _, status := range valid {
		if !isValidWorkerStatus(status) {
			t.Fatalf("expected status %q to be valid", status)
		}
	}
	for _, status := range []domain.DeploymentStatus{"deployed", "", "QUEUED", "unknown"} {
		if isValidWorkerStatus(status) {
			t.Fatalf("expected status %q to be invalid", status)
		}
	}
}

func TestIsPermanentStatusError(t *testing.T) {
	permanent := []error{
		domain.ErrNotFound,
		domain.ErrInvalidStatus,
		domain.ErrInvalidStatusTransition,
		domain.ErrBadParamInput,
		fmtWrapped(domain.ErrInvalidStatusTransition),
	}
	for _, err := range permanent {
		if !isPermanentStatusError(err) {
			t.Fatalf("expected %v to be permanent", err)
		}
	}
	transient := []error{
		errors.New("driver: bad connection"),
		context.Canceled,
		fmtWrapped(errors.New("deadlock detected")),
	}
	for _, err := range transient {
		if isPermanentStatusError(err) {
			t.Fatalf("expected %v to be transient", err)
		}
	}
}

func fmtWrapped(err error) error {
	return errors.Join(err, errors.New("context"))
}

func TestRunStatusConsumerLoop_StopsOnCtxCancelWithoutConnection(t *testing.T) {
	// rmqConn is nil: the session fails immediately, the loop backs off, and
	// ctx cancellation must end it instead of spinning forever.
	ctx, cancel := context.WithCancel(context.Background())
	done := make(chan struct{})

	uc := &deploymentUsecase{} // no rmqConn
	go func() {
		uc.runStatusConsumerLoop(ctx)
		close(done)
	}()

	time.Sleep(50 * time.Millisecond)
	cancel()

	select {
	case <-done:
	case <-time.After(2 * time.Second):
		t.Fatal("consumer loop did not stop after ctx cancellation")
	}
}
