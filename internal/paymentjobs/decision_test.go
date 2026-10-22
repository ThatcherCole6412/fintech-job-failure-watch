package paymentjobs

import (
	"testing"
	"time"
)

func TestEvaluatePaymentEvent(t *testing.T) {
	tests := []struct {
		name     string
		event    Event
		action   string
		notify   bool
		severity string
	}{
		{name: "high risk failure is held", event: validEvent("failed", 88), action: "hold_for_review", notify: true, severity: "critical"},
		{name: "low risk failure gets one retry", event: validEvent("failed", 25), action: "retry_once", notify: true, severity: "error"},
		{name: "settled payment is quiet", event: validEvent("settled", 12), action: "none", notify: false, severity: "info"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := Evaluate(tt.event)
			if err != nil {
				t.Fatalf("Evaluate() error = %v", err)
			}
			if got.Action != tt.action || got.Notify != tt.notify || got.Severity != tt.severity {
				t.Fatalf("Evaluate() = %+v, want action=%q notify=%v severity=%q", got, tt.action, tt.notify, tt.severity)
			}
		})
	}
}

func validEvent(status string, risk int) Event {
	return Event{EventID: "evt-20260930-001", JobName: "settlement-reconcile", PaymentID: "pay-1042", Status: status, RiskScore: risk, FailureClass: "processor_rejection", OccurredAt: time.Date(2026, 9, 30, 2, 15, 0, 0, time.UTC)}
}
