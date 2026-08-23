package storage

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"sync"
	"time"

	"lap-dog/pkg/event"
)

type SentRecord struct {
	EventID     string              `json:"event_id"`
	Fingerprint string              `json:"fingerprint"`
	EventType   event.EventType     `json:"event_type"`
	Severity    event.Severity      `json:"severity"`
	SentAt      time.Time           `json:"sent_at"`
	Event       event.SecurityEvent `json:"event"`
}

type StoreData struct {
	PendingQueue        []event.SecurityEvent `json:"pending_queue"`
	SentHistory         []SentRecord          `json:"sent_history"`
	AwayMode            bool                  `json:"away_mode"`
	LastNotifiedVersion string                `json:"last_notified_version,omitempty"`
	UpdatedAt           time.Time             `json:"updated_at"`
}

type Storage struct {
	mu       sync.RWMutex
	filePath string
	data     StoreData
}

func NewStorage(filePath string, defaultAwayMode bool) (*Storage, error) {
	s := &Storage{
		filePath: filePath,
		data: StoreData{
			PendingQueue: make([]event.SecurityEvent, 0),
			SentHistory:  make([]SentRecord, 0),
			AwayMode:     defaultAwayMode,
			UpdatedAt:    time.Now(),
		},
	}

	if err := s.load(); err != nil {
		// If file doesn't exist, create directory and initial file
		if os.IsNotExist(err) {
			if saveErr := s.saveLocked(); saveErr != nil {
				return nil, fmt.Errorf("failed to initialize store file: %w", saveErr)
			}
		} else {
			return nil, fmt.Errorf("failed to load store file: %w", err)
		}
	}

	return s, nil
}

func (s *Storage) load() error {
	s.mu.Lock()
	defer s.mu.Unlock()

	data, err := os.ReadFile(s.filePath)
	if err != nil {
		return err
	}

	var storeData StoreData
	if err := json.Unmarshal(data, &storeData); err != nil {
		return err
	}

	if storeData.PendingQueue == nil {
		storeData.PendingQueue = make([]event.SecurityEvent, 0)
	}
	if storeData.SentHistory == nil {
		storeData.SentHistory = make([]SentRecord, 0)
	}

	s.data = storeData
	return nil
}

func (s *Storage) saveLocked() error {
	dir := filepath.Dir(s.filePath)
	if dir != "" && dir != "." {
		if err := os.MkdirAll(dir, 0700); err != nil {
			return err
		}
	}

	s.data.UpdatedAt = time.Now()
	data, err := json.MarshalIndent(s.data, "", "  ")
	if err != nil {
		return err
	}

	tmpFile := s.filePath + ".tmp"
	if err := os.WriteFile(tmpFile, data, 0600); err != nil {
		return err
	}

	return os.Rename(tmpFile, s.filePath)
}

func (s *Storage) Enqueue(evt event.SecurityEvent) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	s.data.PendingQueue = append(s.data.PendingQueue, evt)
	return s.saveLocked()
}

func (s *Storage) Dequeue() (*event.SecurityEvent, error) {
	s.mu.Lock()
	defer s.mu.Unlock()

	if len(s.data.PendingQueue) == 0 {
		return nil, nil
	}

	item := s.data.PendingQueue[0]
	s.data.PendingQueue = s.data.PendingQueue[1:]

	if err := s.saveLocked(); err != nil {
		return nil, err
	}

	return &item, nil
}

func (s *Storage) PeekQueue() []event.SecurityEvent {
	s.mu.RLock()
	defer s.mu.RUnlock()

	queueCopy := make([]event.SecurityEvent, len(s.data.PendingQueue))
	copy(queueCopy, s.data.PendingQueue)
	return queueCopy
}

func (s *Storage) QueueSize() int {
	s.mu.RLock()
	defer s.mu.RUnlock()

	return len(s.data.PendingQueue)
}

func (s *Storage) RecordSent(evt event.SecurityEvent) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	rec := SentRecord{
		EventID:     evt.ID,
		Fingerprint: evt.Fingerprint(),
		EventType:   evt.EventType,
		Severity:    evt.Severity,
		SentAt:      time.Now(),
		Event:       evt,
	}

	s.data.SentHistory = append(s.data.SentHistory, rec)

	// Keep last 1000 history records to avoid unconstrained growth
	if len(s.data.SentHistory) > 1000 {
		s.data.SentHistory = s.data.SentHistory[len(s.data.SentHistory)-1000:]
	}

	return s.saveLocked()
}

func (s *Storage) IsDuplicate(fingerprint string, window time.Duration) bool {
	s.mu.RLock()
	defer s.mu.RUnlock()

	cutoff := time.Now().Add(-window)
	for i := len(s.data.SentHistory) - 1; i >= 0; i-- {
		rec := s.data.SentHistory[i]
		if rec.SentAt.Before(cutoff) {
			break
		}
		if rec.Fingerprint == fingerprint {
			return true
		}
	}
	return false
}

func (s *Storage) SetAwayMode(away bool) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	s.data.AwayMode = away
	return s.saveLocked()
}

func (s *Storage) GetAwayMode() bool {
	s.mu.RLock()
	defer s.mu.RUnlock()

	return s.data.AwayMode
}

func (s *Storage) GetStats() (int, int, bool) {
	s.mu.RLock()
	defer s.mu.RUnlock()

	return len(s.data.PendingQueue), len(s.data.SentHistory), s.data.AwayMode
}

func (s *Storage) GetLastNotifiedVersion() string {
	s.mu.RLock()
	defer s.mu.RUnlock()

	return s.data.LastNotifiedVersion
}

func (s *Storage) SetLastNotifiedVersion(v string) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	s.data.LastNotifiedVersion = v
	return s.saveLocked()
}
