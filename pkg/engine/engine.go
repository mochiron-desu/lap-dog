package engine

import (
	"context"
	"fmt"
	"log"
	"sync"
	"time"

	"lap-dog/pkg/config"
	"lap-dog/pkg/event"
	"lap-dog/pkg/storage"
)

type Engine struct {
	cfg          *config.Config
	store        *storage.Storage
	recentFails  []time.Time
	rateWindow   []time.Time
	mu           sync.Mutex
}

func NewEngine(cfg *config.Config, store *storage.Storage) *Engine {
	return &Engine{
		cfg:         cfg,
		store:       store,
		recentFails: make([]time.Time, 0),
		rateWindow:  make([]time.Time, 0),
	}
}

func (e *Engine) ProcessEvent(evt event.SecurityEvent) (bool, string) {
	e.mu.Lock()
	defer e.mu.Unlock()

	awayMode := e.store.GetAwayMode()

	// 1. Deduplication check
	fp := evt.Fingerprint()
	if e.store.IsDuplicate(fp, 1*time.Minute) {
		return false, "duplicate event within deduplication window"
	}

	// 2. Rate limiting check
	now := time.Now()
	cutoffRate := now.Add(-1 * time.Minute)
	validRate := make([]time.Time, 0)
	for _, t := range e.rateWindow {
		if t.After(cutoffRate) {
			validRate = append(validRate, t)
		}
	}
	e.rateWindow = validRate

	if len(e.rateWindow) >= e.cfg.RateLimitPerMin {
		return false, fmt.Sprintf("rate limit exceeded (%d alerts in last minute)", e.cfg.RateLimitPerMin)
	}

	// 3. Rule Evaluation
	shouldAlert := false
	alertReason := ""

	if !awayMode {
		// User present: only alert on CRITICAL/HIGH unexpected events
		if evt.Severity == event.SeverityCritical || evt.Severity == event.SeverityHigh {
			shouldAlert = true
			alertReason = "High severity event while active"
		} else {
			return false, "suppressed: away mode is OFF (user present)"
		}
	} else {
		// User AWAY: evaluate rules
		switch evt.EventType {
		case event.LoginSuccess, event.Unlock:
			evt.Severity = event.SeverityCritical
			shouldAlert = true
			alertReason = "Immediate alert: system accessed while away"

		case event.RemoteLogin:
			evt.Severity = event.SeverityHigh
			shouldAlert = true
			alertReason = "Remote login detected while away"

		case event.USBInsert:
			evt.Severity = event.SeverityHigh
			shouldAlert = true
			alertReason = "USB insertion detected while away"

		case event.LoginFailure:
			// Track failure window for escalation
			cutoffFail := now.Add(-e.cfg.FailedLoginWindow())
			validFails := make([]time.Time, 0)
			for _, t := range e.recentFails {
				if t.After(cutoffFail) {
					validFails = append(validFails, t)
				}
			}
			validFails = append(validFails, now)
			e.recentFails = validFails

			failCount := len(e.recentFails)
			if failCount >= e.cfg.FailedLoginThreshold {
				evt.Severity = event.SeverityCritical
				evt.Details = fmt.Sprintf("%d failed login attempts in last %v! (Possible brute force attack)",
					failCount, e.cfg.FailedLoginWindow())
				shouldAlert = true
				alertReason = fmt.Sprintf("Escalated: %d failed login attempts in window", failCount)
			} else {
				evt.Severity = event.SeverityMedium
				shouldAlert = true
				alertReason = fmt.Sprintf("Failed login attempt (%d/%d)", failCount, e.cfg.FailedLoginThreshold)
			}
		default:
			shouldAlert = true
			alertReason = "Security event while away"
		}
	}

	if shouldAlert {
		e.rateWindow = append(e.rateWindow, now)
		if err := e.store.Enqueue(evt); err != nil {
			log.Printf("[Engine] error enqueuing alert: %v", err)
			return false, fmt.Sprintf("failed to enqueue: %v", err)
		}
		return true, alertReason
	}

	return false, "rule conditions not met"
}

func (e *Engine) Run(ctx context.Context, eventChan <-chan event.SecurityEvent) {
	for {
		select {
		case <-ctx.Done():
			return
		case evt, ok := <-eventChan:
			if !ok {
				return
			}
			alerted, reason := e.ProcessEvent(evt)
			if alerted {
				log.Printf("[Engine] ALERT ENQUEUED: %s (%s, User: %s, Reason: %s)",
					evt.EventType, evt.Severity, evt.Username, reason)
			} else {
				log.Printf("[Engine] Event processed (no alert): %s (Reason: %s)", evt.EventType, reason)
			}
		}
	}
}
