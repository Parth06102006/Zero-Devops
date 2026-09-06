# Outbox Dispatcher — Implementation Plan

Status: DRAFT for review · Target: Task 5, `docs/plan-server-12-08.md` § "Reliable server-to-worker delivery"

## 1. Goal

Close the dual-write gap: today manual builds do `StoreProjectBuild` (DB commit) then
`synchronous publishBuildRequestV1` (RabbitMQ). If the process dies between the two, the
accepted build is lost. The dispatcher is the single `deploy.jobs` producer: it claims
outbox rows, publishes with broker confirmation, and only then marks them sent.

Everything it needs already exists and is tested:

- Repo: `ClaimOutboxBatch` (atomic claim, `FOR UPDATE SKIP LOCKED`, 30s lease),
  `MarkOutboxSent`, `MarkOutboxPublishFailed` (dead-letters at 5 attempts),
  `ResetStuckPublishing`, `DeleteSentOutboxOlderThan`
- Contract: `contract.DecodeV1` + `contract.Publishing` (already sets persistent
  delivery, message ID = `event_id`, correlation ID, version header, content type)
- Topology: `deploy.jobs` + DLQ already declared by `SetUpQueues`

## 2. Component shape

New package: `server/internal/deployments/dispatcher/outbox_dispatcher.go`
(delete the commented-out foreign skeleton currently in that file).

```go
type Config struct {
    PollInterval     time.Duration // default 2s; 0 → default
    BatchSize        int           // default 50
    Concurrency      int           // default 4 (worker pool size)
    BackoffBase      time.Duration // default 2s, exponential: base * 2^attempt
    BackoffMax       time.Duration // default 5m
    StuckLease       time.Duration // default 30s — MUST match repo outboxClaimLease
    Retention        time.Duration // default 24h for sent-row cleanup
    ReconcileEvery   time.Duration // default 1m
}

type Dispatcher struct {
    repo     domain.DeploymentRepository // NOT *sql.DB — claim/mark via repository
    conn     *amqp.Connection            // dispatcher owns its own channels
    log      *zap.Logger
    cfg      Config
    workerID string // hostname + short random suffix, for logs
}

func New(repo domain.DeploymentRepository, conn *amqp.Connection, log *zap.Logger, cfg Config) *Dispatcher
func (d *Dispatcher) Run(ctx context.Context) error // blocks; call via go + errgroup in main
```

Wiring in `cmd/server/main.go` right after `NewDeploymentUsecase` (~line 166):

```go
disp := dispatcher.New(deploymentRepo, rmqConn, logger, dispatcher.Config{...})
g.Go(func() error { return disp.Run(ctx) }) // g = errgroup.Group with signal ctx
```

## 3. Run loop (single goroutine is enough)

```
Run:
  1. open one channel, ch.Confirm(false)            ← confirm mode, before any publish
  2. ticker(PollInterval); reconcileTicker(ReconcileEvery)
  3. loop select:
     ctx.Done → drain in-flight confirms, close channel, return nil
     ticker  → dispatchBatch(ctx, ch)
     reconcileTicker → reconcile(ctx)
```

Concurrency decision: **start with a single channel, sequential batch dispatch.**
Rationale: `ClaimOutboxBatch` already serializes claiming across dispatcher
instances via `SKIP LOCKED`; a confirm-gated sequential loop publishes
BatchSize messages per tick, which is far above this system's build rate.
Keep `Concurrency` in Config reserved; only add a worker pool if profiling
demands it. One channel also makes confirm bookkeeping trivial.

## 4. dispatchBatch — the core sequence

```
events, err := repo.ClaimOutboxBatch(ctx, BatchSize)
  err → log Error, return (next tick retries; DB blips are self-healing)

for each event (state is now "publishing", owned by us, lease 30s):
  req, err := contract.DecodeV1(event.Payload)
    err → poison payload: repo.MarkOutboxPublishFailed(id, "decode: "+err, now)
          (after 5 attempts the repo dead-letters it — reuse, don't invent)
          log Warn with deployment_id + event_id, continue

  pub, err := contract.Publishing(req)   // envelope incl. persistence + headers
    err (validation) → same poison path as above, continue

  dc, err := ch.PublishWithDeferredConfirmWithContext(ctx, "", "deploy.jobs", false, false, pub)
    err → transient: repo.MarkOutboxPublishFailed(id, err, nextAvailableAt(backoff)),
          log Warn, continue        (row returns to pending at available_at; retry later)

  ack, err := dc.WaitContext(ctx)
    ctx cancelled  → return; row stays "publishing", ResetStuckPublishing recovers it
    !ack (nack)    → repo.MarkOutboxPublishFailed(id, "broker nack", backoff), continue
    ack            → repo.MarkOutboxSent(id)
    MarkOutboxSent err → log Error (row stays publishing → stuck-reset recovers
                        and republishes; duplicate job is safe: worker is idempotent
                        by deployment_id — this is the at-least-once tradeoff)

backoff(attempt) = min(BackoffBase << attempt, BackoffMax)
nextAvailableAt = time.Now().Add(backoff(event.AttemptCount))
```

Key invariant: **`MarkOutboxSent` is only called after broker confirmation.** That is
the plan's "require confirmation before marking sent" bullet — the whole point of
the dispatcher.

Crash-safety summary (all states recoverable):
- crash before publish → row stuck `publishing` → `ResetStuckPublishing` re-arms it
- crash after publish, before MarkOutboxSent → same → republish → duplicate delivery →
  worker-side idempotency handles it (Task 6 already covers duplicate job delivery)
- broker down → publish errors → attempts + backoff → dead-letter after 5

## 5. reconcile

```
n, err := repo.ResetStuckPublishing(ctx, StuckLease)   // recovers crashed-dispatcher rows
n > 0 → log Warn("reset stuck outbox rows", count)     // operational signal
err   → log Error, return

m, err := repo.DeleteSentOutboxOlderThan(ctx, Retention)  // table hygiene
```

Note: StuckLease must be ≥ repo's `outboxClaimLease` (30s, pgsql_deployment.go:529).
Use 30s+ or a slightly larger value; a smaller value would reset rows still in
active publish.

## 6. Cleanup enabled by this work (follow-ups, same plan)

1. `CreateProjectBuild` (usecase): replace `StoreProjectBuild` +
   `publishBuildRequestV1` with `repo.StoreProjectBuildWithOutbox(...)` — the
   job struct moves into `StoreProjectBuildWithOutboxParams` mostly as-is;
   EventID = uuid, RepositoryFullName, InstallationID, CorrelationID required.
   Marked with LEGACY PATH MARKER comments in deployment_ucase.go.
2. Delete `publishBuildRequestV1`, `publishCh`, `pubMutex`, `rmqConn` from
   `deploymentUsecase` once the dispatcher is the only producer.
3. Rewrite `consumeStatusUpdate` → autoAck=false + `ApplyStatusUpdate` +
   ack-after-commit (separate item; same Task 5).
4. Remove `CreateDeployment`/`githubRepoResponse`/`jwtExpiryMinutes` legacy chain.

## 7. Testing strategy

Unit (fake driver / mocks, fast):
- decode-failure → MarkOutboxPublishFailed called with attempt context
- publish error → MarkOutboxPublishFailed with backoff-computed available_at
- confirm nack → MarkOutboxPublishFailed
- confirm ack → MarkOutboxSent; MarkOutboxSent error → logged, not fatal
- ctx cancel mid-WaitContext → returns without marking sent
- reconcile calls ResetStuckPublishing + DeleteSentOutboxOlderThan
- backoff table: base<<attempt capped at max

Mock seams: `domain.DeploymentRepository` (existing mock patterns in
deployment_ucase_test.go) + amqp. The amqp seam is the hard part — wrap the
publish/confirm step in a small interface:

```go
type publisher interface {
    publishConfirmed(ctx context.Context, pub amqp.Publishing) error // wraps PublishWithDeferredConfirm + WaitContext
}
```

so the loop is unit-testable without a broker; the thin AMQP adapter (~20 lines)
gets covered by integration tests.

Integration (docker/dockertest, the plan's real-PostgreSQL ask):
- commit-then-publish-failure: row returns to pending, retry succeeds later
- broker nack / dead-letter at max attempts
- two dispatcher instances → SKIP LOCKED claim, no double publish within lease
- crash simulation: claim without mark → ResetStuckPublishing re-arms

## 8. Open decisions to confirm before coding

- [ ] Sequential single channel vs worker pool (plan recommends sequential; confirm)
- [ ] Config source: hard-coded defaults vs env vars — project currently uses env
      vars in main.go; follow that pattern
- [ ] PollInterval default 2s acceptable? (worst-case publish latency after DB commit)
- [ ] Dead-lettered outbox rows: alerting/observability hook (metric counter or log)
- [ ] Manual + webhook events share the `deploy.jobs` event type — verify intended
      (flagged in handoff; worker treats both identically today)
