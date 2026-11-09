package config

import (
	"os"
	"path/filepath"
	"testing"
	"time"
)

func TestConfigLoadSave(t *testing.T) {
	tempDir, err := os.MkdirTemp("", "lapdog_cfg_test")
	if err != nil {
		t.Fatalf("failed to create temp dir: %v", err)
	}
	defer os.RemoveAll(tempDir)

	cfgPath := filepath.Join(tempDir, "config.yaml")

	cfg, err := LoadConfig(cfgPath)
	if err != nil {
		t.Fatalf("LoadConfig failed: %v", err)
	}

	if cfg.AwayMode != true {
		t.Errorf("expected default away mode true, got %v", cfg.AwayMode)
	}
	if cfg.FailedLoginThreshold != 3 {
		t.Errorf("expected default threshold 3, got %d", cfg.FailedLoginThreshold)
	}

	cfg.TelegramBotToken = "test_token_123"
	cfg.AwayMode = false

	if err := SaveConfig(cfgPath, cfg); err != nil {
		t.Fatalf("SaveConfig failed: %v", err)
	}

	loaded, err := LoadConfig(cfgPath)
	if err != nil {
		t.Fatalf("re-loading config failed: %v", err)
	}

	if loaded.TelegramBotToken != "test_token_123" {
		t.Errorf("expected bot token test_token_123, got %s", loaded.TelegramBotToken)
	}
	if loaded.AwayMode != false {
		t.Errorf("expected away mode false, got %v", loaded.AwayMode)
	}
	if loaded.FailedLoginWindow() != 120*time.Second {
		t.Errorf("expected 120s window, got %v", loaded.FailedLoginWindow())
	}
	if !loaded.WebcamCaptureEnabled {
		t.Errorf("expected default webcam_capture_enabled true")
	}
	if loaded.WebcamCaptureDir == "" {
		t.Errorf("expected non-empty webcam_capture_dir")
	}
}
