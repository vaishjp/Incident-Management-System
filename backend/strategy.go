package main

import (
	"log"
	"time"
)

//  1. The Strategy Interface
// Every alert channel MUST implement this method
type AlertStrategy interface {
	SendAlert(w WorkItem) error
}

//  2. Concrete Strategy: PagerDuty (For CRITICAL / P0 / P1)
type PagerDutyStrategy struct{}

func (s *PagerDutyStrategy) SendAlert(w WorkItem) error {
	// Simulating an external API call to PagerDuty
	time.Sleep(200 * time.Millisecond) 
	log.Printf("[PAGERDUTY API] 🚨 WAKING UP ON-CALL SRE! %s failure on %s", w.Severity, w.ComponentID)
	return nil
}

//  3. Concrete Strategy: Slack Webhook (For HIGH / P2)
type SlackStrategy struct{}

func (s *SlackStrategy) SendAlert(w WorkItem) error {
	// Simulating a fast webhook post
	time.Sleep(50 * time.Millisecond)
	log.Printf("[SLACK WEBHOOK] ⚠️ Warning mapped to #sre-alerts: %s reported on %s", w.ErrorType, w.ComponentID)
	return nil
}

//  4. Concrete Strategy: Email SMTP (For LOW / P3 / INFO)
type EmailStrategy struct{}

func (s *EmailStrategy) SendAlert(w WorkItem) error {
	// Simulating an SMTP mail queue
	time.Sleep(100 * time.Millisecond)
	log.Printf("[EMAIL SMTP] ✉️ Non-critical incident logged for morning review: %s", w.ComponentID)
	return nil
}

//  5. The Context (AlertManager)
// The manager doesn't care HOW the alert is sent, it just delegates to the active strategy
type AlertManager struct {
	strategy AlertStrategy
}

func (m *AlertManager) SetStrategy(s AlertStrategy) {
	m.strategy = s
}

func (m *AlertManager) ExecuteAlert(w WorkItem) error {
	if m.strategy == nil {
		log.Println("[WARN] No alert strategy set, falling back to default logging.")
		return nil
	}
	return m.strategy.SendAlert(w)
}

//  6. The Dispatcher (Called by main.go)
func HandleAlertStrategy(w WorkItem) {
	manager := &AlertManager{}

	// Dynamically inject the correct strategy based on incident severity
	switch w.Severity {
	case "P0", "P1", "CRITICAL":
		manager.SetStrategy(&PagerDutyStrategy{})
	case "P2", "HIGH":
		manager.SetStrategy(&SlackStrategy{})
	default:
		// P3, LOW, INFO
		manager.SetStrategy(&EmailStrategy{})
	}

	// Execute the chosen strategy
	err := manager.ExecuteAlert(w)
	if err != nil {
		log.Printf("[ERROR] Alert dispatch failed for WorkItem %d: %v", w.ID, err)
	}
}
