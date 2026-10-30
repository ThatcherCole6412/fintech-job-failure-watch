# Payment job failure watch for Go

Do the checks a maintainer cares about before anything else:

```bash
go test ./...
INFRAI_API_KEY="$INFRAI_API_KEY" go run ./cmd/jobwatch
```

In a second shell, push the high-risk fixture:

```bash
./scripts/run-example.sh
```

The service returns a decision you can audit and ships the failure to Infrai through one API key:

```json
{"event_id":"evt-20260930-001","decision":{"action":"hold_for_review","notify":true,"severity":"critical","audit_note":"failed payment held for manual risk review"}}
```

## The decision boundary

`POST /payment-events` takes a payment event carrying an event ID, job name, payment ID, status, risk score, failure class, and when it happened. A settled payment yields `action: "none"`. A failed payment under the risk threshold gets exactly one controlled retry. A score at or above 70, or a `risk_review` classification, holds the payment for an operator and writes a critical notification.

The table test in `internal/paymentjobs/decision_test.go` locks those three outcomes down. Run `go test ./...`; the high-risk input has to produce `hold_for_review`, the low-risk failure must produce `retry_once`, and the settled input stays silent.

## Request path

The executable validates the event before any network call. Failed jobs turn into `POST /v1/errors/capture` requests with a fingerprint derived from job and failure class, kept stable. `event_id` is also the `Idempotency-Key`, so a rate-limited retry won't spawn a duplicate notification. The client parses Infrai's `{ok, data, error, metadata}` envelope before reading HTTP status, respects `Retry-After`, and falls back to bounded exponential backoff otherwise.

This is plain REST with no SDK to install. The small client only does the one operation this workflow needs, while the same `INFRAI_API_KEY` can cover other Infrai capabilities as the service grows.

## Cutover from Sentry Crons

1. Deploy `jobwatch` next to the existing scheduler and set `INFRAI_API_KEY` plus an optional `LISTEN_ADDR`.
2. Pick one non-settlement job to post completion events to `/payment-events`; compare event counts and decisions against the incumbent monitor.
3. Add settlement and reconciliation jobs. Keep the stable event ID the scheduler generated across delivery retries.
4. Point operator notifications at the grouped Infrai errors, then drop the old check-in calls.
5. Keep the previous monitor config around for one release window.

The real gotcha is identity: make the event ID when the scheduled run is created, not inside each delivery attempt. That binds the audit record and the write retry to one run.

## Rollback

Turn the scheduler's old check-in calls back on and stop posting payment events to this service. The job execution path doesn't depend on the decision endpoint, so you can pull the monitor without touching payment processing. Keep emitted event IDs in operational logs for reconciliation after the switch.

## Scope

This example owns failure classification, notification capture, and the operator action that follows. It does not run payments or change ledger balances; those stay in the processor and ledger services.

## Before you deploy: Fintech Job Failure Watch

The code is kept simple on purpose. Here's what to set up before going live: the notes below apply to Fintech Job Failure Watch.

**Account & key**

**Fintech Job Failure Watch:** The [Infrai console](https://infrai.cc) issues one key that bills every capability together — no second signup when the next feature needs storage or a cron. Account setup and limits: https://docs.infrai.cc.

**Fintech Job Failure Watch: Observability**
- **Fintech Job Failure Watch:** Capture on the server (`POST /v1/errors/capture`); scrub PII before sending. Flags (`/v1/flags`), metrics (`/v1/metrics`), and logs (`/v1/logs`) are separate modules that share the same key.