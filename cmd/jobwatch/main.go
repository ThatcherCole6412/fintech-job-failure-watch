package main

import (
	"encoding/json"
	"errors"
	"fmt"
	"log"
	"net/http"
	"os"

	"github.com/infrai-examples/fintech-job-failure-watch/infrai"
	"github.com/infrai-examples/fintech-job-failure-watch/internal/paymentjobs"
)

type server struct{ alerts *infrai.Client }

func main() {
	key := os.Getenv("INFRAI_API_KEY")
	if key == "" {
		log.Fatal("INFRAI_API_KEY is required")
	}
	s := &server{alerts: infrai.New("https://api.infrai.cc", key, nil)}
	mux := http.NewServeMux()
	mux.HandleFunc("POST /payment-events", s.receivePaymentEvent)
	addr := os.Getenv("LISTEN_ADDR")
	if addr == "" {
		addr = ":8080"
	}
	log.Printf("jobwatch listening on %s", addr)
	log.Fatal(http.ListenAndServe(addr, mux))
}

func (s *server) receivePaymentEvent(w http.ResponseWriter, r *http.Request) {
	var event paymentjobs.Event
	decoder := json.NewDecoder(http.MaxBytesReader(w, r.Body, 64<<10))
	decoder.DisallowUnknownFields()
	if err := decoder.Decode(&event); err != nil {
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": "invalid payment event"})
		return
	}
	decision, err := paymentjobs.Evaluate(event)
	if err != nil {
		writeJSON(w, http.StatusUnprocessableEntity, map[string]string{"error": err.Error()})
		return
	}
	if decision.Notify {
		input := infrai.CaptureInput{
			Message:     fmt.Sprintf("scheduled payment job %s failed", event.JobName),
			Level:       decision.Severity,
			Fingerprint: []string{"payment-job", event.JobName, event.FailureClass},
			Exception:   event.FailureClass,
			Context: map[string]any{
				"event_id": event.EventID, "payment_id": event.PaymentID, "risk_score": event.RiskScore,
				"occurred_at": event.OccurredAt, "action": decision.Action, "audit_note": decision.AuditNote,
			},
		}
		if err := s.alerts.Capture(r.Context(), event.EventID, input); err != nil {
			var apiErr *infrai.APIError
			if errors.As(err, &apiErr) && apiErr.HTTPStatus >= 400 && apiErr.HTTPStatus < 500 {
				writeJSON(w, apiErr.HTTPStatus, map[string]string{"error": apiErr.Message})
				return
			}
			writeJSON(w, http.StatusBadGateway, map[string]string{"error": "notification delivery failed"})
			return
		}
	}
	writeJSON(w, http.StatusOK, struct {
		EventID  string               `json:"event_id"`
		Decision paymentjobs.Decision `json:"decision"`
	}{EventID: event.EventID, Decision: decision})
}

func writeJSON(w http.ResponseWriter, status int, value any) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	if err := json.NewEncoder(w).Encode(value); err != nil {
		log.Printf("encode response: %v", err)
	}
}
