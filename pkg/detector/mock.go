package detector

import (
	"context"
	"fmt"
	"sync"
	"time"

	"lap-dog/pkg/config"
	"lap-dog/pkg/event"
)

type MockDetector struct {
	cfg       *config.Config
	injectCh  chan event.SecurityEvent
	once      sync.Once
}

func NewMockDetector(cfg *config.Config) *MockDetector {
	return &MockDetector{
		cfg:      cfg,
		injectCh: make(chan event.SecurityEvent, 10),
	}
}

func (m *MockDetector) Name() string {
	return "Mock/Simulator Detector"
}

func (m *MockDetector) InjectEvent(evt event.SecurityEvent) {
	if evt.DeviceID == "" {
		evt.DeviceID = m.cfg.DeviceID
	}
	if evt.Hostname == "" {
		evt.Hostname = m.cfg.Hostname
	}
	if evt.Timestamp.IsZero() {
		evt.Timestamp = time.Now()
	}
	m.injectCh <- evt
}

func (m *MockDetector) Start(ctx context.Context, eventChan chan<- event.SecurityEvent) error {
	for {
		select {
		case <-ctx.Done():
			return ctx.Err()
		case evt := <-m.injectCh:
			eventChan <- evt
		}
	}
}

func GenerateSampleEvent(eventType event.EventType, username, sourceIP string) event.SecurityEvent {
	sev := event.SeverityMedium
	switch eventType {
	case event.LoginSuccess, event.Unlock:
		sev = event.SeverityCritical
	case event.RemoteLogin, event.USBInsert:
		sev = event.SeverityHigh
	}

	return event.SecurityEvent{
		ID:        fmt.Sprintf("mock-%d", time.Now().UnixNano()),
		Timestamp: time.Now(),
		Platform:  "mock",
		EventType: eventType,
		Username:  username,
		Source:    "MockDetector",
		SourceIP:  sourceIP,
		Severity:  sev,
		Details:   "Simulated security event for testing",
	}
}
