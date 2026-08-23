package detector

import (
	"context"
	"testing"
	"time"

	"lap-dog/pkg/config"
	"lap-dog/pkg/event"
)

func TestMockDetector(t *testing.T) {
	cfg := config.DefaultConfig()
	cfg.MockMode = true

	d, err := NewDetector(cfg)
	if err != nil {
		t.Fatalf("NewDetector failed: %v", err)
	}

	if d.Name() != "Mock/Simulator Detector" {
		t.Errorf("expected Mock name, got %s", d.Name())
	}

	mockDet, ok := d.(*MockDetector)
	if !ok {
		t.Fatalf("expected *MockDetector type")
	}

	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
	defer cancel()

	evtChan := make(chan event.SecurityEvent, 5)

	go func() {
		_ = mockDet.Start(ctx, evtChan)
	}()

	sampleEvt := GenerateSampleEvent(event.LoginFailure, "intruder", "10.0.0.1")
	mockDet.InjectEvent(sampleEvt)

	select {
	case received := <-evtChan:
		if received.Username != "intruder" {
			t.Errorf("expected username intruder, got %s", received.Username)
		}
		if received.EventType != event.LoginFailure {
			t.Errorf("expected LoginFailure, got %s", received.EventType)
		}
	case <-time.After(1 * time.Second):
		t.Errorf("timeout waiting for injected mock event")
	}
}

func TestLinuxParsing(t *testing.T) {
	cfg := config.DefaultConfig()
	linuxDet := NewLinuxDetector(cfg)

	// Test journal entry for SSH failure
	entry := JournalEntry{
		Message: "Failed password for invalid user admin from 192.168.1.50 port 54321 ssh2",
	}

	evt, ok := linuxDet.parseJournalEntry(&entry)
	if !ok {
		t.Fatalf("expected parseJournalEntry to return true")
	}

	if evt.EventType != event.LoginFailure {
		t.Errorf("expected LoginFailure, got %s", evt.EventType)
	}
	if evt.Username != "admin" {
		t.Errorf("expected admin, got %s", evt.Username)
	}
	if evt.SourceIP != "192.168.1.50" {
		t.Errorf("expected 192.168.1.50, got %s", evt.SourceIP)
	}

	// Test cron session ignore
	cronEntry := JournalEntry{
		Message: "pam_unix(cron:session): session opened for user root(uid=0) by root(uid=0)",
	}
	if _, ok := linuxDet.parseJournalEntry(&cronEntry); ok {
		t.Errorf("expected cron session to be ignored, but it was parsed as an event")
	}
}

func TestWindowsParsing(t *testing.T) {
	cfg := config.DefaultConfig()
	winDet := NewWindowsDetector(cfg)

	// Service logon (LogonType 5) should be ignored
	svcEntry := winEventData{
		Id: 4624,
		Message: `An account was successfully logged on.
Account Name: SYSTEM
Logon Type: 5`,
	}
	if _, ok := winDet.parseWinEvent(&svcEntry); ok {
		t.Errorf("expected Windows Service logon to be ignored")
	}

	// Interactive logon (LogonType 2) should be detected
	userEntry := winEventData{
		Id: 4624,
		Message: `An account was successfully logged on.
Account Name: alice
Logon Type: 2
Source Network Address: 127.0.0.1`,
	}
	evt, ok := winDet.parseWinEvent(&userEntry)
	if !ok {
		t.Fatalf("expected interactive Windows logon to be parsed")
	}
	if evt.Username != "alice" {
		t.Errorf("expected alice, got %s", evt.Username)
	}
	if evt.EventType != event.LoginSuccess {
		t.Errorf("expected LoginSuccess, got %s", evt.EventType)
	}
}

func TestDarwinParsing(t *testing.T) {
	cfg := config.DefaultConfig()
	macDet := NewDarwinDetector(cfg)

	// Periodic task should be ignored
	cronEntry := macOSLogEntry{
		EventMessage: "com.apple.periodic sessionDidLogin for root",
	}
	if _, ok := macDet.parseDarwinEntry(&cronEntry); ok {
		t.Errorf("expected macOS periodic task to be ignored")
	}

	// Real login should be parsed
	userEntry := macOSLogEntry{
		EventMessage: "User logged in successfully via loginwindow",
	}
	evt, ok := macDet.parseDarwinEntry(&userEntry)
	if !ok {
		t.Fatalf("expected macOS user login to be parsed")
	}
	if evt.EventType != event.LoginSuccess {
		t.Errorf("expected LoginSuccess, got %s", evt.EventType)
	}
}
