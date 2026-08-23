package notifier

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"log"
	"net/http"
	"strings"
	"time"

	"lap-dog/pkg/config"
	"lap-dog/pkg/event"
	"lap-dog/pkg/storage"
)

type TelegramNotifier struct {
	cfg        *config.Config
	store      *storage.Storage
	httpClient *http.Client
	baseURL    string
	startTime  time.Time
}

func NewTelegramNotifier(cfg *config.Config, store *storage.Storage) *TelegramNotifier {
	return &TelegramNotifier{
		cfg:        cfg,
		store:      store,
		httpClient: &http.Client{Timeout: 10 * time.Second},
		baseURL:    "https://api.telegram.org",
		startTime:  time.Now(),
	}
}

// SetBaseURL allows overriding the Telegram API endpoint (useful for testing)
func (t *TelegramNotifier) SetBaseURL(u string) {
	t.baseURL = strings.TrimRight(u, "/")
}

func (t *TelegramNotifier) SendMessage(chatID int64, text string) error {
	if t.cfg.TelegramBotToken == "" {
		return fmt.Errorf("telegram_bot_token is not configured")
	}

	endpoint := fmt.Sprintf("%s/bot%s/sendMessage", t.baseURL, t.cfg.TelegramBotToken)

	payload := map[string]interface{}{
		"chat_id":    chatID,
		"text":       text,
		"parse_mode": "HTML",
	}

	bodyBytes, err := json.Marshal(payload)
	if err != nil {
		return err
	}

	resp, err := t.httpClient.Post(endpoint, "application/json", bytes.NewBuffer(bodyBytes))
	if err != nil {
		return fmt.Errorf("http request failed: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		respBody, _ := io.ReadAll(resp.Body)
		return fmt.Errorf("telegram API error (status %d): %s", resp.StatusCode, string(respBody))
	}

	return nil
}

// StartQueueDispatcher runs the offline-first retry loop to send queued alerts
func (t *TelegramNotifier) StartQueueDispatcher(ctx context.Context) {
	ticker := time.NewTicker(500 * time.Millisecond)
	defer ticker.Stop()

	backoff := 1 * time.Second

	for {
		select {
		case <-ctx.Done():
			return
		case <-ticker.C:
			if t.store.QueueSize() == 0 {
				backoff = 1 * time.Second
				continue
			}

			evt, err := t.store.Dequeue()
			if err != nil || evt == nil {
				continue
			}

			if t.cfg.TelegramBotToken == "" || t.cfg.TelegramChatID == 0 {
				log.Printf("[Notifier] Telegram not configured; skipping alert dispatch for %s", evt.EventType)
				_ = t.store.RecordSent(*evt)
				continue
			}

			msg := evt.FormatTelegramHTML()
			err = t.SendMessage(t.cfg.TelegramChatID, msg)
			if err != nil {
				log.Printf("[Notifier] Failed to dispatch Telegram alert (will re-enqueue): %v", err)
				_ = t.store.Enqueue(*evt) // Re-enqueue for retry

				// Exponential backoff up to 30s
				time.Sleep(backoff)
				backoff *= 2
				if backoff > 30*time.Second {
					backoff = 30 * time.Second
				}
			} else {
				log.Printf("[Notifier] Alert dispatched successfully to Telegram chat %d", t.cfg.TelegramChatID)
				_ = t.store.RecordSent(*evt)
				backoff = 1 * time.Second
			}
		}
	}
}

type TelegramUpdate struct {
	UpdateID int64 `json:"update_id"`
	Message  *struct {
		MessageID int64 `json:"message_id"`
		From      struct {
			ID       int64  `json:"id"`
			Username string `json:"username"`
		} `json:"from"`
		Chat struct {
			ID int64 `json:"id"`
		} `json:"chat"`
		Text string `json:"text"`
	} `json:"message"`
}

type TelegramGetUpdatesResp struct {
	OK     bool             `json:"ok"`
	Result []TelegramUpdate `json:"result"`
}

// StartCommandPoller polls Telegram getUpdates for interactive bot commands
func (t *TelegramNotifier) StartCommandPoller(ctx context.Context) {
	if t.cfg.TelegramBotToken == "" {
		log.Printf("[Notifier] Telegram token missing; command poller disabled")
		return
	}

	var lastUpdateID int64 = 0

	ticker := time.NewTicker(t.cfg.PollInterval())
	defer ticker.Stop()

	for {
		select {
		case <-ctx.Done():
			return
		case <-ticker.C:
			updates, err := t.getUpdates(lastUpdateID + 1)
			if err != nil {
				continue
			}

			for _, upd := range updates {
				if upd.UpdateID > lastUpdateID {
					lastUpdateID = upd.UpdateID
				}
				if upd.Message != nil {
					t.handleCommand(upd.Message.Chat.ID, upd.Message.Text)
				}
			}
		}
	}
}

func (t *TelegramNotifier) getUpdates(offset int64) ([]TelegramUpdate, error) {
	endpoint := fmt.Sprintf("%s/bot%s/getUpdates?offset=%d&timeout=0", t.baseURL, t.cfg.TelegramBotToken, offset)
	resp, err := t.httpClient.Get(endpoint)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("status %d", resp.StatusCode)
	}

	var uResp TelegramGetUpdatesResp
	if err := json.NewDecoder(resp.Body).Decode(&uResp); err != nil {
		return nil, err
	}

	return uResp.Result, nil
}

func (t *TelegramNotifier) handleCommand(chatID int64, text string) {
	cmd := strings.TrimSpace(strings.Split(text, "@")[0])
	cmd = strings.ToLower(cmd)

	switch cmd {
	case "/away":
		_ = t.store.SetAwayMode(true)
		t.cfg.AwayMode = true
		reply := "🔒 <b>Away Mode ARMED</b>\n\nlap-dog is actively watching for authentication & access events. Alerts will fire immediately on suspicious activity."
		_ = t.SendMessage(chatID, reply)

	case "/back":
		_ = t.store.SetAwayMode(false)
		t.cfg.AwayMode = false
		reply := "🟢 <b>Away Mode DISARMED</b>\n\nWelcome back! Non-critical alerts are now suppressed while you are at your laptop."
		_ = t.SendMessage(chatID, reply)

	case "/status":
		pending, sent, away := t.store.GetStats()
		statusStr := "ARMED 🔒"
		if !away {
			statusStr = "DISARMED 🟢"
		}

		uptime := time.Since(t.startTime).Truncate(time.Second)

		reply := fmt.Sprintf("💻 <b>lap-dog Security Agent Status</b>\n\n"+
			"<b>Host:</b> <code>%s</code>\n"+
			"<b>Device ID:</b> <code>%s</code>\n"+
			"<b>Away Mode:</b> %s\n"+
			"<b>Pending Queue:</b> %d\n"+
			"<b>Total Sent History:</b> %d\n"+
			"<b>Uptime:</b> %s",
			t.cfg.Hostname, t.cfg.DeviceID, statusStr, pending, sent, uptime)
		_ = t.SendMessage(chatID, reply)

	case "/test":
		testEvt := event.SecurityEvent{
			ID:        fmt.Sprintf("test-%d", time.Now().Unix()),
			Timestamp: time.Now(),
			DeviceID:  t.cfg.DeviceID,
			Hostname:  t.cfg.Hostname,
			Platform:  t.cfg.Hostname,
			EventType: event.LoginFailure,
			Username:  "test_user",
			Source:    "Telegram /test command",
			Severity:  event.SeverityMedium,
			Details:   "This is a test notification from lap-dog.",
		}
		_ = t.SendMessage(chatID, testEvt.FormatTelegramHTML())

	case "/help", "/start":
		reply := "🐕 <b>lap-dog Security Agent Commands</b>\n\n" +
			"/away - Arm monitoring (away mode ON)\n" +
			"/back - Disarm monitoring (away mode OFF)\n" +
			"/status - Show current agent metrics & device state\n" +
			"/test - Trigger a test alert notification\n" +
			"/help - Display this command menu"
		_ = t.SendMessage(chatID, reply)
	}
}
