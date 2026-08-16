# deploy.jobs Message Contract — Versioning & Compatibility Policy

Status: **V1 defined.** Applies to the RabbitMQ `deploy.jobs` queue, the only
build-job bus between `server` (producer) and `worker-server` (consumer).

The single source of truth is the JSON Schema at
[`schemas/deploy-jobs-v1.schema.json`](../schemas/deploy-jobs-v1.schema.json)
(repo root). Both services compile against the **same file**, so "both services
accept the same version" is enforced by construction, not by convention. The Go
implementations live at `server/internal/deployments/contract/` and
`worker-server/internal/deployments/contract/`; they are kept identical and are
verified against the schema by conformance tests in each module.

`deploy.status` is a **separate, looser contract** and is out of scope here; its
durable/idempotent consumption is covered by the reliability work in section 5
of `plan-server-12-08.md`.

## 1. Wire format

| AMQP property | Value | Notes |
| --- | --- | --- |
| `content_type` | `application/json` | Required; validated by the consumer. |
| `content_encoding` | `utf-8` | Set by the producer. |
| `delivery_mode` | `2` (persistent) | Jobs must survive broker restart. |
| `message_id` | `event_id` from the body | Deduplication aid. |
| `correlation_id` | `correlation_id` from the body | Tracing aid. |
| `x-contract-version` header | `1` (int32) | Advisory routing aid. RabbitMQ decodes signed integer headers as `int32`. |

The JSON **body is authoritative**: `version` in the body wins over any header
value, and the consumer validates the envelope against the body. The header
exists so dead-letter routing and dashboards can classify messages without
parsing JSON.

## 2. Versioning rules

- The contract is versioned by an integer `version` field, starting at `1`.
- **V1 is immutable.** Once V1 ships to a shared environment, the V1 schema
  file, the Go struct, and its validation must never be edited in place.
- **Backward-compatible (no version bump):** adding a *new optional* field,
  widening an enum, or relaxing a constraint. The schema and both Go packages
  are updated in the same change set.
- **Breaking (requires V2):** removing/renaming a field, making an optional
  field required, changing the meaning of a value, or changing constraints that
  existing producers would violate.
- A new version (V2) ships **alongside** V1 for a bounded migration window:
  both services understand both versions, the producer switches to V2 only
  after all consumers support it, and V1 publishing is removed after the window
  closes. This is the "rolling deployment" rule in reverse — consumers first,
  producers second.

## 3. Failure policy (fail closed, never fall back)

- A message that is malformed JSON, has a missing/unsupported `version`, fails
  body validation, or fails envelope validation is **rejected**:
  `basic.nack(requeue=false)` routes it to the `deploy.jobs.dlq` via the
  established dead-letter exchange.
- Rejection is logged with an observable error. There is **no silent fallback**
  to the legacy `{deployment_id, clone_url, retry_count, request_id}`
  shallow-clone shape — that shape is not a supported message version.
- The worker retries a *valid* V1 message by re-publishing the **same** V1 body
  with an incremented `retry_count`; retries never re-shape the message.
- If `retry_count` exceeds the configured `MAX_RETRIES_COUNT`, the worker marks
  the deployment canceled, publishes the terminal status, and nacks to the DLQ.

## 4. Consumer rules (worker)

1. Decode the body as `BuildRequestV1`; reject on any validation error (→ DLQ).
2. Validate the AMQP envelope against the body (`content_type`,
   `message_id`/`event_id`, `correlation_id`, `x-contract-version`); reject on
   mismatch (→ DLQ).
3. Check out the exact `commit_sha` — never the moving branch head
   (`git clone --depth 1` of the default branch is forbidden as the source of a
   build).
4. Acknowledge only after the outcome/status has been durably reported to the
   queue according to the queue reliability policy.

## 5. Producer rules (server)

1. `publishBuildRequestV1` is the **only** `deploy.jobs` producer. It validates
   the complete `BuildRequestV1` and refuses to publish incomplete jobs.
2. The server never converts a legacy repo-only deployment request into a V1
   job. `POST /deploy` fails closed until a project/manual-build API supplies
   all V1 fields.
3. `commit_sha` is mandatory and authoritative; `requested_ref` is retained
   context only.

## 6. Conformance (how this is enforced)

- `schemas/deploy-jobs-v1.schema.json` is compiled with the JSON Schema
  draft 2020-12 validator in each service's `internal/deployments/schema/`
  conformance test — compiling also validates the schema file itself against the
  meta-schema.
- Golden fixtures in `schemas/fixtures/` (`valid-*` must pass, `invalid-*` must
  fail) are validated in **both** services, proving both accept the same
  version.
- The `contract` package tests in both services assert `Validate`,
  `DecodeV1`, `Publishing`, and `ValidateMetadata` behavior, keeping the Go
  implementation aligned with the schema.

## 7. Worked example

```json
{
  "version": 1,
  "event_id": "9c5b94b3-1f1c-4f3f-9c8e-0a1b2c3d4e5f",
  "deployment_id": "dep-01J5XYZABC123",
  "project_id": "proj-01J5XYZABC123",
  "installation_id": 12345678,
  "repository_id": 987654321,
  "clone_url": "https://github.com/acme/web.git",
  "commit_sha": "0123456789abcdef0123456789abcdef01234567",
  "requested_ref": "refs/heads/main",
  "trigger": "webhook_push",
  "generation": 7,
  "retry_count": 0,
  "correlation_id": "req-9c5b94b3-1f1c",
  "configuration": {
    "executable": "npm",
    "args": ["run", "build"],
    "working_dir": ".",
    "scanner_policy_version": "v1"
  }
}
```

Envelope: `content_type: application/json`, `delivery_mode: 2`,
`message_id: 9c5b94b3-1f1c-4f3f-9c8e-0a1b2c3d4e5f`,
`correlation_id: req-9c5b94b3-1f1c`, header `x-contract-version: 1`.
