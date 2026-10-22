#!/bin/sh
set -eu

curl --fail-with-body -X POST "http://localhost:8080/payment-events" \
  -H "Content-Type: application/json" \
  -d '{"event_id":"evt-20260930-001","job_name":"settlement-reconcile","payment_id":"pay-1042","status":"failed","risk_score":88,"failure_class":"risk_review","occurred_at":"2026-09-30T02:15:00Z"}'
