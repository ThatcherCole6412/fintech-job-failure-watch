package paymentjobs

import (
	"fmt"
	"strings"
	"time"
)

type Event struct {
	EventID      string    `json:"event_id"`
	JobName      string    `json:"job_name"`
	PaymentID    string    `json:"payment_id"`
	Status       string    `json:"status"`
	RiskScore    int       `json:"risk_score"`
	FailureClass string    `json:"failure_class,omitempty"`
	OccurredAt   time.Time `json:"occurred_at"`
}

type Decision struct {
	Action    string `json:"action"`
	Notify    bool   `json:"notify"`
	Severity  string `json:"severity"`
	AuditNote string `json:"audit_note"`
}

func Evaluate(event Event) (Decision, error) {
	if strings.TrimSpace(event.EventID) == "" || strings.TrimSpace(event.JobName) == "" || strings.TrimSpace(event.PaymentID) == "" {
		return Decision{}, fmt.Errorf("event_id, job_name, and payment_id are required")
	}
	if event.RiskScore < 0 || event.RiskScore > 100 {
		return Decision{}, fmt.Errorf("risk_score must be between 0 and 100")
	}
	if event.Status == "settled" {
		return Decision{Action: "none", Severity: "info", AuditNote: "payment settled; no operator action"}, nil
	}
	if event.Status != "failed" {
		return Decision{}, fmt.Errorf("status must be settled or failed")
	}
	if event.RiskScore >= 70 || event.FailureClass == "risk_review" {
		return Decision{Action: "hold_for_review", Notify: true, Severity: "critical", AuditNote: "failed payment held for manual risk review"}, nil
	}
	return Decision{Action: "retry_once", Notify: true, Severity: "error", AuditNote: "failed payment scheduled for one controlled retry"}, nil
}
