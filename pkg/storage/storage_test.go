package storage

import (
	"os"
	"path/filepath"
	"testing"
	"time"

	"lap-dog/pkg/event"
)

func TestStorageQueueAndHistory(t *testing.T) {
	tempDir, err := os.MkdirTemp("", "lapdog_storage_test")
	if err != nil {
		t.Fatalf("failed to create temp dir: %v", err)
	}
	defer os.RemoveAll(tempDir)

	dbFile := filepath.Join(tempDir, "store.json")
	store, err := NewStorage(dbFile, true)
	if err != nil {
		t.Fatalf("NewStorage failed: %v", err)
	}

	if !store.GetAwayMode() {
		t.Errorf("expected default away mode true")
	}

	evt1 := event.SecurityEvent{
		ID:        "evt-001",
		Timestamp: time.Now(),
		EventType: event.LoginFailure,
		Username:  "attacker",
		Severity:  event.SeverityMedium,
	}

	if err := store.Enqueue(evt1); err != nil {
		t.Fatalf("Enqueue failed: %v", err)
	}

	if store.QueueSize() != 1 {
		t.Errorf("expected queue size 1, got %d", store.QueueSize())
	}

	// Test persistence across re-open
	store2, err := NewStorage(dbFile, false)
	if err != nil {
		t.Fatalf("re-opening storage failed: %v", err)
	}
	if store2.QueueSize() != 1 {
		t.Errorf("expected queue size 1 on reload, got %d", store2.QueueSize())
	}

	dequeued, err := store2.Dequeue()
	if err != nil {
		t.Fatalf("Dequeue failed: %v", err)
	}
	if dequeued.ID != "evt-001" {
		t.Errorf("expected dequeued ID evt-001, got %s", dequeued.ID)
	}
	if store2.QueueSize() != 0 {
		t.Errorf("expected empty queue after dequeue, got %d", store2.QueueSize())
	}

	// Test sent history & duplicate detection
	if err := store2.RecordSent(evt1); err != nil {
		t.Fatalf("RecordSent failed: %v", err)
	}

	if !store2.IsDuplicate(evt1.Fingerprint(), 5*time.Minute) {
		t.Errorf("expected event to be detected as duplicate within 5 min window")
	}

	pending, sent, away := store2.GetStats()
	if pending != 0 || sent != 1 || !away {
		t.Errorf("unexpected stats: pending=%d, sent=%d, away=%v", pending, sent, away)
	}

	// Test LastNotifiedVersion
	if err := store2.SetLastNotifiedVersion("v0.2.0"); err != nil {
		t.Fatalf("SetLastNotifiedVersion failed: %v", err)
	}
	if store2.GetLastNotifiedVersion() != "v0.2.0" {
		t.Errorf("expected LastNotifiedVersion v0.2.0, got %s", store2.GetLastNotifiedVersion())
	}
}
