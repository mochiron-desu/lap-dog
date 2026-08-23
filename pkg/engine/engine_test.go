package engine

import (
	"os"
	"path/filepath"
	"testing"
	"time"

	"lap-dog/pkg/config"
	"lap-dog/pkg/event"
	"lap-dog/pkg/storage"
)

func TestEngineAwayRulesAndEscalation(t *testing.T) {
	tempDir, err := os.MkdirTemp("", "engine_test")
	if err != nil {
		t.Fatalf("failed to create temp dir: %v", err)
	}
	defer os.RemoveAll(tempDir)

	dbPath := filepath.Join(tempDir, "store.json")
	store, err := storage.NewStorage(dbPath, true) // Away mode ON
	if err != nil {
		t.Fatalf("NewStorage failed: %v", err)
	}

	cfg := config.DefaultConfig()
	cfg.FailedLoginThreshold = 3
	cfg.RateLimitPerMin = 10

	eng := NewEngine(cfg, store)

	// 1. Single login failure while away -> Enqueued
	evt1 := event.SecurityEvent{
		ID:        "fail-1",
		Timestamp: time.Now(),
		EventType: event.LoginFailure,
		Username:  "attacker",
	}

	alerted, reason := eng.ProcessEvent(evt1)
	if !alerted {
		t.Errorf("expected alert on login failure while away, got reason: %s", reason)
	}
	if store.QueueSize() != 1 {
		t.Errorf("expected 1 queued item, got %d", store.QueueSize())
	}

	// Dequeue item for next test
	_, _ = store.Dequeue()

	// 2. Test escalation on 3rd failure
	evt2 := event.SecurityEvent{ID: "fail-2", Timestamp: time.Now(), EventType: event.LoginFailure, Username: "attacker"}
	evt3 := event.SecurityEvent{ID: "fail-3", Timestamp: time.Now(), EventType: event.LoginFailure, Username: "attacker"}

	_, _ = eng.ProcessEvent(evt2)
	alerted3, reason3 := eng.ProcessEvent(evt3)

	if !alerted3 {
		t.Errorf("expected escalation alert on 3rd failure, got reason: %s", reason3)
	}

	_, _ = store.Dequeue() // Dequeue evt2 first
	queuedEvt, _ := store.Dequeue() // Dequeue escalated evt3
	if queuedEvt != nil && queuedEvt.Severity != event.SeverityCritical {
		t.Errorf("expected severity CRITICAL on 3rd failure, got %s", queuedEvt.Severity)
	}
}

func TestEngineSuppressionWhenNotAway(t *testing.T) {
	tempDir, err := os.MkdirTemp("", "engine_test_home")
	if err != nil {
		t.Fatalf("failed to create temp dir: %v", err)
	}
	defer os.RemoveAll(tempDir)

	dbPath := filepath.Join(tempDir, "store.json")
	store, _ := storage.NewStorage(dbPath, false) // Away mode OFF (User at machine)
	cfg := config.DefaultConfig()

	eng := NewEngine(cfg, store)

	evt := event.SecurityEvent{
		ID:        "fail-home",
		Timestamp: time.Now(),
		EventType: event.LoginFailure,
		Username:  "user",
		Severity:  event.SeverityMedium,
	}

	alerted, reason := eng.ProcessEvent(evt)
	if alerted {
		t.Errorf("expected event to be suppressed when away mode is OFF")
	}
	if store.QueueSize() != 0 {
		t.Errorf("expected queue size 0, got %d", store.QueueSize())
	}
	_ = reason
}
