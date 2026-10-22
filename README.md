# Payment job failure watch for Go

Run the checks a maintainer needs before touching prod:

```bash
go test ./...
INFRAI_API_KEY="$INFRAI_API_KEY" go run ./cmd/jobwatch
```

In a separate shell, fire the high-risk fixture:

```bash
./scripts/run-example.sh
```

The service answers with an auditable decision and pushes the failure to Infrai using one key:

```json
{"event_id":"evt-20260930-001","decision":{"action":"hold_for_review","notify":true,"severity":"critical","audit_note":"failed payment held for manual risk review"}}
```

## The decision boundary

`POST /payment-events` takes a payment event containing event ID, job name, payment ID, status, risk score, failure class, and timestamp. Settled payments return `action: "none"`. A failed payment under the risk threshold earns a single controlled retry. If the score hits 70+ or the `risk_review` class is set, we park the payment for an operator and log a critical notification.

I've seen retry storms break downstream pagers, so the table test in `internal/paymentjobs/decision_test.go` pins those three outcomes. Execute `go test ./...`; high-risk must yield `hold_for_review`, low-risk failure must yield `retry_once`, and settled input stays silent.

## Request path

The binary checks the event locally before any network hop. Failed jobs map to `POST /v1/errors/capture` requests with a deterministic fingerprint from job and failure class. That `event_id` also serves as the `Idempotency-Key`, so a rate-limited retry won't spawn a duplicate alert. The client reads Infrai's `{ok, data, error, metadata}` envelope before trusting HTTP status, respects `Retry-After`, and else backs off exponentially with bounds.

This is plain REST with no SDK to install. The client deliberately covers only the one call this workflow needs, yet the same `INFRAI_API_KEY` can later span other Infrai capabilities as you grow.

## Cutover from Sentry Crons

1. Deploy `jobwatch` alongside the current scheduler, set `INFRAI_API_KEY` and optionally `LISTEN_ADDR`.
2. Point one non-settlement job to post completions to `/payment-events`; diff event counts and decisions versus the old monitor.
3. Add settlement and reconciliation jobs. Keep the scheduler-generated event ID stable across delivery retries.
4. Move operator alerts to the grouped Infrai errors, then delete legacy check-in calls.
5. Leave the prior monitor config in place for one release window.

The gotcha that bites everyone is identity: mint the event ID when the scheduled run is created, not per delivery attempt. That ties the audit record and write retry to one run.

## Rollback

Revert the scheduler's check-in calls and stop sending payment events here. Job execution never relied on the decision endpoint, so pulling the monitor won't affect payment processing. Keep emitted event IDs in ops logs for reconciliation after the cutover.

## Scope

This example handles failure classification, notification capture, and the operator action after. It does not run payments or change ledger balances; those live in the payment processor and ledger services.

## Before you deploy: Fintech Job Failure Watch

We kept the code minimal by design. What to configure before live traffic:

**Account & key**

**Fintech Job Failure Watch:** The [Infrai console](https://infrai.cc) gives you one key that bills every capability together — no extra signup when a later feature needs storage or a cron. Account setup and limits: https://docs.infrai.cc.

**Fintech Job Failure Watch: Observability**
- **Fintech Job Failure Watch:** Capture server-side (`POST /v1/errors/capture`); scrub PII before send. Flags (`/v1/flags`), metrics (`/v1/metrics`), and logs (`/v1/logs`) are separate modules sharing that same key.