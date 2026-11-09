package engine

import (
	"context"
	"errors"
	"os"
	"path/filepath"
	"strings"
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
	cfg.WebcamCaptureEnabled = false // Disable real webcam during basic away test

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

type mockCapturer struct {
	imagePath string
	status    string
	err       error
}

func (m *mockCapturer) Capture(ctx context.Context, eventID string) (string, string, error) {
	return m.imagePath, m.status, m.err
}

func TestEngineWebcamAntivirusFallback(t *testing.T) {
	tempDir, err := os.MkdirTemp("", "engine_av_test")
	if err != nil {
		t.Fatalf("failed to create temp dir: %v", err)
	}
	defer os.RemoveAll(tempDir)

	dbPath := filepath.Join(tempDir, "store.json")
	store, _ := storage.NewStorage(dbPath, true) // Away mode ON
	cfg := config.DefaultConfig()

	eng := NewEngine(cfg, store)
	// Inject a mock capturer simulating antivirus blocking
	eng.SetCapturer(&mockCapturer{
		imagePath: "",
		status:    "Blocked by Antivirus, security policy, or OS privacy permissions (Access Denied)",
		err:       errors.New("permission denied"),
	})

	evt := event.SecurityEvent{
		ID:        "fail-av-blocked",
		Timestamp: time.Now(),
		EventType: event.LoginFailure,
		Username:  "intruder",
	}

	alerted, reason := eng.ProcessEvent(evt)
	if !alerted {
		t.Fatalf("expected alert even when webcam capture is blocked by antivirus: %s", reason)
	}

	queuedEvt, err := store.Dequeue()
	if err != nil || queuedEvt == nil {
		t.Fatalf("failed to dequeue event: %v", err)
	}

	if queuedEvt.ImagePath != "" {
		t.Errorf("expected empty ImagePath on AV block, got %s", queuedEvt.ImagePath)
	}

	if !strings.Contains(queuedEvt.WebcamStatus, "Blocked by Antivirus") {
		t.Errorf("expected AV diagnostic status, got %s", queuedEvt.WebcamStatus)
	}
}

func TestEngineWebcamSuccess(t *testing.T) {
	tempDir, err := os.MkdirTemp("", "engine_success_test")
	if err != nil {
		t.Fatalf("failed to create temp dir: %v", err)
	}
	defer os.RemoveAll(tempDir)

	dbPath := filepath.Join(tempDir, "store.json")
	store, _ := storage.NewStorage(dbPath, true) // Away mode ON
	cfg := config.DefaultConfig()

	eng := NewEngine(cfg, store)
	mockPath := filepath.Join(tempDir, "test_capture.jpg")
	eng.SetCapturer(&mockCapturer{
		imagePath: mockPath,
		status:    "Webcam snapshot captured successfully",
		err:       nil,
	})

	evt := event.SecurityEvent{
		ID:        "fail-webcam-ok",
		Timestamp: time.Now(),
		EventType: event.LoginFailure,
		Username:  "intruder",
	}

	alerted, reason := eng.ProcessEvent(evt)
	if !alerted {
		t.Fatalf("expected alert: %s", reason)
	}

	queuedEvt, err := store.Dequeue()
	if err != nil || queuedEvt == nil {
		t.Fatalf("failed to dequeue event: %v", err)
	}

	if queuedEvt.ImagePath != mockPath {
		t.Errorf("expected ImagePath %s, got %s", mockPath, queuedEvt.ImagePath)
	}
	if queuedEvt.WebcamStatus != "Webcam snapshot captured successfully" {
		t.Errorf("expected success status, got %s", queuedEvt.WebcamStatus)
	}
}
