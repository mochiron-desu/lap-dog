package config

import (
	"crypto/rand"
	"encoding/hex"
	"fmt"
	"os"
	"path/filepath"
	"time"

	"gopkg.in/yaml.v3"
)

type Config struct {
	TelegramBotToken     string        `yaml:"telegram_bot_token" json:"telegram_bot_token"`
	TelegramChatID       int64         `yaml:"telegram_chat_id" json:"telegram_chat_id"`
	PollIntervalSeconds  int           `yaml:"poll_interval_seconds" json:"poll_interval_seconds"`
	DeviceID             string        `yaml:"device_id" json:"device_id"`
	Hostname             string        `yaml:"hostname" json:"hostname"`
	AwayMode             bool          `yaml:"away_mode" json:"away_mode"`
	StoragePath          string        `yaml:"storage_path" json:"storage_path"`
	LogLevel             string        `yaml:"log_level" json:"log_level"`
	FailedLoginThreshold int           `yaml:"failed_login_threshold" json:"failed_login_threshold"`
	FailedLoginWindowSec int           `yaml:"failed_login_window_seconds" json:"failed_login_window_seconds"`
	RateLimitPerMin      int           `yaml:"rate_limit_per_min" json:"rate_limit_per_min"`
	MockMode             bool          `yaml:"mock_mode" json:"mock_mode"`
}

func DefaultConfig() *Config {
	hostname, err := os.Hostname()
	if err != nil || hostname == "" {
		hostname = "lapdog-host"
	}

	deviceID := generateDeviceID(hostname)

	homeDir, _ := os.UserHomeDir()
	defaultStorage := filepath.Join(homeDir, ".lapdog", "lapdog_store.json")

	return &Config{
		TelegramBotToken:     "",
		TelegramChatID:       0,
		PollIntervalSeconds:  3,
		DeviceID:             deviceID,
		Hostname:             hostname,
		AwayMode:             true, // Default away mode ON for security
		StoragePath:          defaultStorage,
		LogLevel:             "info",
		FailedLoginThreshold: 3,
		FailedLoginWindowSec: 120, // 2 minutes
		RateLimitPerMin:      5,
		MockMode:             false,
	}
}

func LoadConfig(path string) (*Config, error) {
	cfg := DefaultConfig()

	data, err := os.ReadFile(path)
	if err != nil {
		if os.IsNotExist(err) {
			// Save default config if not existing
			if saveErr := SaveConfig(path, cfg); saveErr != nil {
				return nil, fmt.Errorf("failed to save default config: %w", saveErr)
			}
			return cfg, nil
		}
		return nil, fmt.Errorf("failed to read config file %s: %w", path, err)
	}

	if err := yaml.Unmarshal(data, cfg); err != nil {
		return nil, fmt.Errorf("failed to unmarshal config: %w", err)
	}

	if cfg.DeviceID == "" {
		cfg.DeviceID = generateDeviceID(cfg.Hostname)
	}

	return cfg, nil
}

func SaveConfig(path string, cfg *Config) error {
	dir := filepath.Dir(path)
	if dir != "" && dir != "." {
		if err := os.MkdirAll(dir, 0700); err != nil {
			return fmt.Errorf("failed to create config directory %s: %w", dir, err)
		}
	}

	data, err := yaml.Marshal(cfg)
	if err != nil {
		return fmt.Errorf("failed to marshal config: %w", err)
	}

	if err := os.WriteFile(path, data, 0600); err != nil {
		return fmt.Errorf("failed to write config file %s: %w", path, err)
	}

	return nil
}

func generateDeviceID(hostname string) string {
	b := make([]byte, 4)
	_, _ = rand.Read(b)
	return fmt.Sprintf("%s-%s", hostname, hex.EncodeToString(b))
}

func (c *Config) FailedLoginWindow() time.Duration {
	if c.FailedLoginWindowSec <= 0 {
		return 2 * time.Minute
	}
	return time.Duration(c.FailedLoginWindowSec) * time.Second
}

func (c *Config) PollInterval() time.Duration {
	if c.PollIntervalSeconds <= 0 {
		return 3 * time.Second
	}
	return time.Duration(c.PollIntervalSeconds) * time.Second
}
