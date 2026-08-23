package notifier

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"testing"
	"time"

	"lap-dog/pkg/config"
	"lap-dog/pkg/event"
	"lap-dog/pkg/storage"
)

func TestTelegramNotifierSendAndPoll(t *testing.T) {
	tempDir, err := os.MkdirTemp("", "notifier_test")
	if err != nil {
		t.Fatalf("failed to create temp dir: %v", err)
	}
	defer os.RemoveAll(tempDir)

	dbPath := filepath.Join(tempDir, "store.json")
	store, _ := storage.NewStorage(dbPath, false) // Default away mode OFF

	receivedMessages := make([]string, 0)

	// Mock Telegram API HTTP Server
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")

		if r.URL.Path == "/botTEST_TOKEN/sendMessage" {
			var body map[string]interface{}
			_ = json.NewDecoder(r.Body).Decode(&body)
			if txt, ok := body["text"].(string); ok {
				receivedMessages = append(receivedMessages, txt)
			}
			w.WriteHeader(http.StatusOK)
			_, _ = w.Write([]byte(`{"ok": true}`))
			return
		}

		if r.URL.Path == "/botTEST_TOKEN/getUpdates" {
			// Mock command response for /away
			resp := TelegramGetUpdatesResp{
				OK: true,
				Result: []TelegramUpdate{
					{
						UpdateID: 100,
						Message: &struct {
							MessageID int64 `json:"message_id"`
							From      struct {
								ID       int64  `json:"id"`
								Username string `json:"username"`
							} `json:"from"`
							Chat struct {
								ID int64 `json:"id"`
							} `json:"chat"`
							Text string `json:"text"`
						}{
							MessageID: 1,
							Chat:      struct{ ID int64 `json:"id"` }{ID: 999},
							Text:      "/away",
						},
					},
				},
			}
			_ = json.NewEncoder(w).Encode(resp)
			return
		}

		w.WriteHeader(http.StatusNotFound)
	}))
	defer server.Close()

	cfg := config.DefaultConfig()
	cfg.TelegramBotToken = "TEST_TOKEN"
	cfg.TelegramChatID = 999

	notifier := NewTelegramNotifier(cfg, store)
	notifier.SetBaseURL(server.URL)

	// 1. Test SendMessage directly
	err = notifier.SendMessage(999, "Hello World")
	if err != nil {
		t.Fatalf("SendMessage failed: %v", err)
	}

	// 2. Test Queue Dispatcher
	evt := event.SecurityEvent{
		ID:        "evt-test",
		Timestamp: time.Now(),
		EventType: event.LoginFailure,
		Username:  "bob",
	}
	_ = store.Enqueue(evt)

	ctx, cancel := context.WithTimeout(context.Background(), 3*time.Second)
	defer cancel()

	go notifier.StartQueueDispatcher(ctx)

	// Wait for queue dispatch
	time.Sleep(1 * time.Second)

	if store.QueueSize() != 0 {
		t.Errorf("expected queue to be emptied by dispatcher, got size %d", store.QueueSize())
	}

	// 3. Test Command Poller handling /away command
	notifier.handleCommand(999, "/away")
	if !store.GetAwayMode() {
		t.Errorf("expected away mode to be true after /away command")
	}
}
