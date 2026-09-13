// Package dispatcher publishes committed deployment outbox events to RabbitMQ.
package dispatcher

import (
	"context"
	"errors"
	"fmt"
	"os"
	"time"

	"Zero_Devops/server/internal/deployments/contract"
	"Zero_Devops/server/internal/domain"

	"github.com/google/uuid"
	amqp "github.com/rabbitmq/amqp091-go"
	"go.uber.org/zap"
)

const (
	defaultPollInterval   = 2 * time.Second
	defaultBatchSize      = 50
	defaultBackoffBase    = 2 * time.Second
	defaultBackoffMax     = 5 * time.Minute
	defaultStuckLease     = 30 * time.Second
	defaultRetention      = 24 * time.Hour
	defaultReconcileEvery = time.Minute
	jobsQueue             = "deploy.jobs"
)

// Config controls the dispatcher polling, retry, and cleanup behavior.
type Config struct {
	PollInterval   time.Duration
	BatchSize      int
	BackoffBase    time.Duration
	BackoffMax     time.Duration
	StuckLease     time.Duration
	Retention      time.Duration
	ReconcileEvery time.Duration
}

// Outbox owns the background delivery loop. Database operations are performed
// only through DeploymentRepository; the dispatcher does not contain SQL.
type Outbox struct {
	deploymentRepo domain.DeploymentRepository
	log            *zap.Logger
	queueConn      *amqp.Connection
	config         Config
	workerID       string
}

// NewOutbox constructs an outbox dispatcher. The dispatcher opens and owns its
// RabbitMQ channel when Run starts; callers must not share a publishing channel
// with the deployment usecase.
func NewOutbox(deploymentRepo domain.DeploymentRepository, queueConn *amqp.Connection, log *zap.Logger, cfg Config) *Outbox {
	if log == nil {
		log = zap.NewNop()
	}

	hostname, err := os.Hostname()
	if err != nil || hostname == "" {
		hostname = "unknown-host"
	}

	return &Outbox{
		deploymentRepo: deploymentRepo,
		log:            log,
		queueConn:      queueConn,
		config:         cfg,
		workerID:       fmt.Sprintf("%s-%s", hostname, uuid.NewString()),
	}
}

// withDefaults prevents zero-valued durations from reaching time.NewTicker
// and ensures the stuck-row lease is not shorter than the repository's
// 30-second claim lease.
func (o *Outbox) withDefaults() {
	if o.config.PollInterval <= 0 {
		o.config.PollInterval = defaultPollInterval
	}
	if o.config.BatchSize <= 0 {
		o.config.BatchSize = defaultBatchSize
	}
	if o.config.BackoffBase <= 0 {
		o.config.BackoffBase = defaultBackoffBase
	}
	if o.config.BackoffMax <= 0 {
		o.config.BackoffMax = defaultBackoffMax
	}
	if o.config.BackoffMax < o.config.BackoffBase {
		o.config.BackoffMax = o.config.BackoffBase
	}
	if o.config.StuckLease < defaultStuckLease {
		o.config.StuckLease = defaultStuckLease
	}
	if o.config.Retention <= 0 {
		o.config.Retention = defaultRetention
	}
	if o.config.ReconcileEvery <= 0 {
		o.config.ReconcileEvery = defaultReconcileEvery
	}
}

// confirmedPublisher is deliberately small. The dispatcher depends on the
// behavior "publish and wait for broker confirmation", not on an AMQP channel
// directly. This keeps dispatchEvent unit-testable and allows a pipelined
// publisher implementation to be introduced later without changing the
// outbox state machine.
type confirmedPublisher interface {
	PublishConfirmed(ctx context.Context, message amqp.Publishing) (bool, error)
}

// amqpConfirmedPublisher adapts one RabbitMQ channel to confirmedPublisher.
type amqpConfirmedPublisher struct {
	channel *amqp.Channel
}

func (p amqpConfirmedPublisher) PublishConfirmed(ctx context.Context, message amqp.Publishing) (bool, error) {
	confirmation, err := p.channel.PublishWithDeferredConfirmWithContext(
		ctx,
		"", // default exchange
		jobsQueue,
		false,
		false,
		message,
	)
	if err != nil {
		return false, err
	}
	if confirmation == nil {
		return false, errors.New("RabbitMQ returned no publisher confirmation")
	}
	return confirmation.WaitContext(ctx)
}

// Run starts the dispatcher and blocks until ctx is canceled or the AMQP
// channel cannot be initialized. Individual database and publish failures are
// logged and retried; they do not stop the loop.
func (o *Outbox) Run(ctx context.Context) error {
	if o == nil {
		return fmt.Errorf("%w: dispatcher is nil", domain.ErrOutboxDispatcherUnavailable)
	}
	if o.deploymentRepo == nil {
		return fmt.Errorf("%w: deployment repository is nil", domain.ErrOutboxDispatcherUnavailable)
	}
	if o.queueConn == nil {
		return fmt.Errorf("%w: RabbitMQ connection is nil", domain.ErrOutboxDispatcherUnavailable)
	}
	if ctx == nil {
		ctx = context.Background()
	}
	if o.log == nil {
		o.log = zap.NewNop()
	}

	o.withDefaults()

	channel, err := o.queueConn.Channel()
	if err != nil {
		return fmt.Errorf("%w: open RabbitMQ channel: %w", domain.ErrOutboxDispatcherUnavailable, err)
	}
	defer func() {
		if closeErr := channel.Close(); closeErr != nil {
			o.log.Error("failed to close outbox RabbitMQ channel", zap.Error(closeErr))
		}
	}()

	// Confirm mode must be enabled before the first publish. MarkOutboxSent is
	// never called until the corresponding deferred confirmation is an ACK.
	if err := channel.Confirm(false); err != nil {
		return fmt.Errorf("%w: enable RabbitMQ publisher confirms: %w", domain.ErrOutboxDispatcherUnavailable, err)
	}

	publisher := amqpConfirmedPublisher{channel: channel}
	o.log.Info("outbox dispatcher started",
		zap.String("worker_id", o.workerID),
		zap.Duration("poll_interval", o.config.PollInterval),
		zap.Int("batch_size", o.config.BatchSize),
	)

	// Dispatch immediately on startup instead of waiting for the first tick.
	o.dispatchBatch(ctx, publisher)
	o.reconcile(ctx)

	pollTicker := time.NewTicker(o.config.PollInterval)
	reconcileTicker := time.NewTicker(o.config.ReconcileEvery)
	defer pollTicker.Stop()
	defer reconcileTicker.Stop()

	for {
		select {
		case <-ctx.Done():
			o.log.Info("outbox dispatcher stopped", zap.String("worker_id", o.workerID))
			return nil

		case <-pollTicker.C:
			o.dispatchBatch(ctx, publisher)

		case <-reconcileTicker.C:
			o.reconcile(ctx)
		}
	}
}

// dispatchBatch claims and processes events until the current backlog is
// drained. If a full batch is returned, it immediately claims another batch
// instead of waiting for the next polling tick. This handles bursts efficiently
// without creating one goroutine per job.
func (o *Outbox) dispatchBatch(ctx context.Context, publisher confirmedPublisher) {
	for {
		if ctx.Err() != nil {
			return
		}

		events, err := o.deploymentRepo.ClaimOutboxBatch(ctx, o.config.BatchSize)
		if err != nil {
			o.log.Error("failed to claim outbox batch",
				zap.String("worker_id", o.workerID),
				zap.Int("batch_size", o.config.BatchSize),
				zap.Error(err))
			return
		}
		if len(events) == 0 {
			return
		}

		o.log.Debug("claimed outbox batch",
			zap.String("worker_id", o.workerID),
			zap.Int("count", len(events)))

		for i := range events {
			event := &events[i]
			if ctx.Err() != nil {
				return
			}
			if err := o.dispatchEvent(ctx, publisher, *event); err != nil {
				o.log.Warn("outbox event was not sent",
					zap.String("worker_id", o.workerID),
					zap.String("outbox_id", event.ID),
					zap.String("deployment_id", event.DeploymentID),
					zap.Error(err))
			}
		}

		// A short batch means there are no more immediately available rows.
		// A full batch may indicate a burst, so claim again without sleeping.
		if len(events) < o.config.BatchSize {
			return
		}
	}
}

// dispatchEvent sends one claimed event and advances its state only after the
// correct external result is known:
//
//	publishing -> broker ACK -> sent
//	publishing -> broker error/NACK -> failed, with retry backoff
//	publishing -> unknown result during shutdown -> unchanged; reconciliation
//
// The function processes one event at a time. This preserves simple per-row
// failure semantics while confirmedPublisher leaves room for future batching.
func (o *Outbox) dispatchEvent(ctx context.Context, publisher confirmedPublisher, event domain.OutboxEvent) error {
	o.log.Debug("dispatching outbox event",
		zap.String("worker_id", o.workerID),
		zap.String("outbox_id", event.ID),
		zap.String("deployment_id", event.DeploymentID),
		zap.Int("attempt", event.AttemptCount))

	// The current outbox contains deploy.jobs V1 payloads. Rejecting a wrong
	// type/version here prevents silently sending a payload to the wrong queue.
	if event.EventType != domain.WebhookBuildEventType {
		err := fmt.Errorf("event type %q is not %q", event.EventType, domain.WebhookBuildEventType)
		return o.failPoisonEvent(ctx, event, err)
	}
	if event.MessageVersion != contract.VersionV1 {
		err := fmt.Errorf("message version %d is not supported", event.MessageVersion)
		return o.failPoisonEvent(ctx, event, err)
	}

	request, err := contract.DecodeV1(event.Payload)
	if err != nil {
		return o.failPoisonEvent(ctx, event, err)
	}

	message, err := contract.Publishing(request)
	if err != nil {
		return o.failPoisonEvent(ctx, event, err)
	}

	acked, err := publisher.PublishConfirmed(ctx, message)
	if err != nil {
		// Once shutdown starts, the confirmation result is unknown. Do not
		// mark failed or sent; ResetStuckPublishing will recover the claim.
		if ctx.Err() != nil || errors.Is(err, context.Canceled) || errors.Is(err, context.DeadlineExceeded) {
			return err
		}

		wrapped := domain.WrapOutboxError(domain.ErrOutboxPublishFailed, err)
		if markErr := o.markPublishFailed(ctx, event, wrapped, time.Now().Add(o.backoff(event.AttemptCount))); markErr != nil {
			return fmt.Errorf("%w; record failure: %v", wrapped, markErr)
		}
		return wrapped
	}

	if !acked {
		wrapped := domain.ErrOutboxConfirmRejected
		if markErr := o.markPublishFailed(ctx, event, wrapped, time.Now().Add(o.backoff(event.AttemptCount))); markErr != nil {
			return fmt.Errorf("%w; record failure: %v", wrapped, markErr)
		}
		return wrapped
	}

	// This is the critical invariant: the database is marked sent only after
	// RabbitMQ has confirmed receipt. If this DB operation fails, leave the row
	// publishing so reconciliation can retry it; at-least-once delivery may
	// produce a duplicate, but it cannot lose the accepted build.
	if err := o.deploymentRepo.MarkOutboxSent(ctx, event.ID); err != nil {
		wrapped := domain.WrapOutboxError(domain.ErrOutboxMarkSentFailed, err)
		o.log.Error("broker confirmed outbox event but marking it sent failed; reconciliation will recover it",
			zap.String("worker_id", o.workerID),
			zap.String("outbox_id", event.ID),
			zap.String("deployment_id", event.DeploymentID),
			zap.Error(wrapped))
		return wrapped
	}

	o.log.Info("outbox event sent",
		zap.String("worker_id", o.workerID),
		zap.String("outbox_id", event.ID),
		zap.String("deployment_id", event.DeploymentID),
		zap.Int("attempt", event.AttemptCount))
	return nil
}

// failPoisonEvent records a payload that cannot become a valid V1 job. It is
// retried immediately so the repository's normal attempt limit can move it to
// dead_letter for inspection; retrying with a long transport backoff would not
// make invalid bytes valid.
func (o *Outbox) failPoisonEvent(ctx context.Context, event domain.OutboxEvent, cause error) error {
	wrapped := domain.WrapOutboxError(domain.ErrOutboxPayloadInvalid, cause)
	if markErr := o.markPublishFailed(ctx, event, wrapped, time.Now()); markErr != nil {
		return fmt.Errorf("%w; record poison event failure: %v", wrapped, markErr)
	}
	o.log.Warn("invalid outbox payload; event will be retried and eventually dead-lettered",
		zap.String("worker_id", o.workerID),
		zap.String("outbox_id", event.ID),
		zap.String("deployment_id", event.DeploymentID),
		zap.Error(wrapped))
	return wrapped
}

func (o *Outbox) markPublishFailed(ctx context.Context, event domain.OutboxEvent, cause error, availableAt time.Time) error {
	if err := o.deploymentRepo.MarkOutboxPublishFailed(ctx, event.ID, cause.Error(), availableAt); err != nil {
		o.log.Error("failed to record outbox publish failure",
			zap.String("worker_id", o.workerID),
			zap.String("outbox_id", event.ID),
			zap.String("deployment_id", event.DeploymentID),
			zap.Error(err))
		return err
	}
	return nil
}

// backoff calculates capped exponential retry delay. Attempt 0 waits for the
// base delay, then each subsequent attempt doubles it up to BackoffMax.
func (o *Outbox) backoff(attempts int) time.Duration {
	if attempts < 0 {
		attempts = 0
	}
	if o.config.BackoffBase <= 0 {
		o.config.BackoffBase = defaultBackoffBase
	}
	if o.config.BackoffMax <= 0 {
		o.config.BackoffMax = defaultBackoffMax
	}
	if o.config.BackoffBase >= o.config.BackoffMax {
		return o.config.BackoffMax
	}

	delay := o.config.BackoffBase
	for i := 0; i < attempts; i++ {
		if delay > o.config.BackoffMax/2 {
			return o.config.BackoffMax
		}
		delay *= 2
	}
	if delay > o.config.BackoffMax {
		return o.config.BackoffMax
	}
	return delay
}

// reconcile recovers claims abandoned by a crashed dispatcher and removes old
// sent rows. It is intentionally separate from the hot dispatch path.
func (o *Outbox) reconcile(ctx context.Context) {
	if ctx.Err() != nil {
		return
	}

	resetCount, err := o.deploymentRepo.ResetStuckPublishing(ctx, o.config.StuckLease)
	if err != nil {
		o.log.Error("failed to reset stuck outbox rows",
			zap.String("worker_id", o.workerID),
			zap.Error(err))
	} else if resetCount > 0 {
		o.log.Warn("reset stuck outbox rows",
			zap.String("worker_id", o.workerID),
			zap.Int64("count", resetCount),
			zap.Duration("lease", o.config.StuckLease))
	}

	deletedCount, err := o.deploymentRepo.DeleteSentOutboxOlderThan(ctx, o.config.Retention)
	if err != nil {
		o.log.Error("failed to delete old sent outbox rows",
			zap.String("worker_id", o.workerID),
			zap.Error(err))
	} else if deletedCount > 0 {
		o.log.Info("deleted old sent outbox rows",
			zap.String("worker_id", o.workerID),
			zap.Int64("count", deletedCount),
			zap.Duration("retention", o.config.Retention))
	}
}
