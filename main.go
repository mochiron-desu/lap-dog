package main

import (
	"bufio"
	"context"
	"flag"
	"fmt"
	"log"
	"os"
	"os/exec"
	"os/signal"
	"path/filepath"
	"runtime"
	"strconv"
	"strings"
	"syscall"
	"time"

	"lap-dog/pkg/config"
	"lap-dog/pkg/detector"
	"lap-dog/pkg/engine"
	"lap-dog/pkg/event"
	"lap-dog/pkg/notifier"
	"lap-dog/pkg/service"
	"lap-dog/pkg/storage"
	"lap-dog/pkg/updater"
)

var (
	version   = "v0.1.0"
	commit    = "dev"
	buildDate = "unknown"
)

func main() {
	if len(os.Args) < 2 {
		printUsage()
		os.Exit(1)
	}

	command := os.Args[1]

	defaultCfgPath := getDefaultConfigPath()

	switch command {
	case "setup":
		setupCmd := flag.NewFlagSet("setup", flag.ExitOnError)
		cfgPath := setupCmd.String("config", defaultCfgPath, "Path to configuration file")
		_ = setupCmd.Parse(os.Args[2:])

		runInteractiveSetup(*cfgPath)

	case "run":
		runCmd := flag.NewFlagSet("run", flag.ExitOnError)
		cfgPath := runCmd.String("config", defaultCfgPath, "Path to configuration YAML file")
		mockMode := runCmd.Bool("mock", false, "Enable mock simulator detector mode")
		_ = runCmd.Parse(os.Args[2:])

		cfg, err := config.LoadConfig(*cfgPath)
		if err != nil {
			log.Fatalf("[main] Error loading config: %v", err)
		}
		if *mockMode {
			cfg.MockMode = true
		}

		runAgent(cfg)

	case "away":
		awayCmd := flag.NewFlagSet("away", flag.ExitOnError)
		cfgPath := awayCmd.String("config", defaultCfgPath, "Path to configuration file")
		_ = awayCmd.Parse(os.Args[2:])

		toggleAway(*cfgPath, true)

	case "back":
		backCmd := flag.NewFlagSet("back", flag.ExitOnError)
		cfgPath := backCmd.String("config", defaultCfgPath, "Path to configuration file")
		_ = backCmd.Parse(os.Args[2:])

		toggleAway(*cfgPath, false)

	case "status":
		statusCmd := flag.NewFlagSet("status", flag.ExitOnError)
		cfgPath := statusCmd.String("config", defaultCfgPath, "Path to configuration file")
		_ = statusCmd.Parse(os.Args[2:])

		showStatus(*cfgPath)

	case "test":
		testCmd := flag.NewFlagSet("test", flag.ExitOnError)
		cfgPath := testCmd.String("config", defaultCfgPath, "Path to configuration file")
		_ = testCmd.Parse(os.Args[2:])

		sendTestAlert(*cfgPath)

	case "simulate":
		simCmd := flag.NewFlagSet("simulate", flag.ExitOnError)
		cfgPath := simCmd.String("config", defaultCfgPath, "Path to configuration file")
		evtType := simCmd.String("event", "LOGIN_FAILURE", "Event type (LOGIN_FAILURE, LOGIN_SUCCESS, UNLOCK, REMOTE_LOGIN, USB_INSERT)")
		username := simCmd.String("user", "simulated_user", "Username associated with event")
		sourceIP := simCmd.String("ip", "127.0.0.1", "Source IP address")
		_ = simCmd.Parse(os.Args[2:])

		simulateEvent(*cfgPath, *evtType, *username, *sourceIP)

	case "service":
		if len(os.Args) < 3 {
			fmt.Println("Usage: lapdog service [install|uninstall]")
			os.Exit(1)
		}
		subCmd := os.Args[2]
		cfgPath := defaultCfgPath
		execPath, _ := os.Executable()

		if subCmd == "install" {
			if err := service.ServiceInstall(execPath, cfgPath); err != nil {
				log.Fatalf("Service installation failed: %v", err)
			}
		} else if subCmd == "uninstall" {
			if err := service.ServiceUninstall(); err != nil {
				log.Fatalf("Service uninstallation failed: %v", err)
			}
		} else {
			fmt.Printf("Unknown service command: %s\n", subCmd)
		}

	case "uninstall":
		uninstCmd := flag.NewFlagSet("uninstall", flag.ExitOnError)
		purgeFlag := uninstCmd.Bool("purge", false, "Purge configuration and storage files (~/.lapdog)")
		forceFlag := uninstCmd.Bool("force", false, "Non-interactive mode (force without prompting)")
		_ = uninstCmd.Parse(os.Args[2:])

		runUninstall(*purgeFlag, *forceFlag)

	case "update", "check-update":
		runUpdateCheck()

	case "version":
		fmt.Printf("lap-dog %s (commit: %s, built: %s, platform: %s/%s)\n", version, commit, buildDate, runtime.GOOS, runtime.GOARCH)

	default:
		printUsage()
		os.Exit(1)
	}
}

func getDefaultConfigPath() string {
	home, _ := os.UserHomeDir()
	return filepath.Join(home, ".lapdog", "config.yaml")
}

func printUsage() {
	fmt.Printf(`🐕 lap-dog Security Agent %s

Usage:
  lapdog setup [--config config.yaml]        Run interactive configuration & service setup wizard
  lapdog run [--config config.yaml] [--mock] Start background security monitoring daemon
  lapdog away [--config config.yaml]         Arm Away Mode (alerts ON)
  lapdog back [--config config.yaml]         Disarm Away Mode (suppress non-critical alerts)
  lapdog status [--config config.yaml]       Display agent status and queue metrics
  lapdog test [--config config.yaml]         Send a test Telegram alert
  lapdog simulate [--event TYPE] [--user ...] Inject a synthetic security event for testing
  lapdog update                              Check for updates & show platform upgrade instructions
  lapdog service [install|uninstall]         Manage system daemon service
  lapdog uninstall [--purge] [--force]       Uninstall lap-dog and remove service/binary
  lapdog version                             Show version & build info
`, version)
}

func runUpdateCheck() {
	fmt.Printf("🔍 Checking for updates for lap-dog (%s)...\n\n", updater.DefaultRepo)
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	result, err := updater.CheckUpdate(ctx, version, updater.DefaultRepo)
	if err != nil {
		log.Fatalf("Failed to check for updates: %v", err)
	}

	fmt.Print(updater.FormatCLIMessage(result))
}

func runAgent(cfg *config.Config) {
	log.Printf("[main] Initializing lap-dog agent %s (Host: %s, DeviceID: %s)", version, cfg.Hostname, cfg.DeviceID)

	store, err := storage.NewStorage(cfg.StoragePath, cfg.AwayMode)
	if err != nil {
		log.Fatalf("[main] Storage initialization failed: %v", err)
	}

	det, err := detector.NewDetector(cfg)
	if err != nil {
		log.Fatalf("[main] Detector initialization failed: %v", err)
	}

	eng := engine.NewEngine(cfg, store)
	notif := notifier.NewTelegramNotifier(cfg, store)
	notif.SetVersion(version)

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	// 1. Channel for security events
	eventChan := make(chan event.SecurityEvent, 100)

	// 2. Start detector goroutine
	go func() {
		log.Printf("[main] Starting event detector: %s", det.Name())
		if err := det.Start(ctx, eventChan); err != nil && ctx.Err() == nil {
			log.Printf("[main] Detector stopped with error: %v", err)
		}
	}()

	// 3. Start engine processing loop
	go eng.Run(ctx, eventChan)

	// 4. Start Telegram notifier dispatch, command poller & background update checker
	go notif.StartQueueDispatcher(ctx)
	go notif.StartCommandPoller(ctx)
	go notif.StartPeriodicUpdateChecker(ctx)

	log.Printf("[main] lap-dog agent is running. Press Ctrl+C to stop.")

	// Signal handling for graceful shutdown
	sigChan := make(chan os.Signal, 1)
	signal.Notify(sigChan, syscall.SIGINT, syscall.SIGTERM)
	<-sigChan

	log.Println("[main] Shutdown signal received. Stopping lap-dog agent...")
	cancel()
	time.Sleep(500 * time.Millisecond)
	log.Println("[main] lap-dog stopped cleanly.")
}

func toggleAway(cfgPath string, away bool) {
	cfg, err := config.LoadConfig(cfgPath)
	if err != nil {
		log.Fatalf("Error loading config: %v", err)
	}

	store, err := storage.NewStorage(cfg.StoragePath, cfg.AwayMode)
	if err != nil {
		log.Fatalf("Error accessing storage: %v", err)
	}

	_ = store.SetAwayMode(away)
	cfg.AwayMode = away
	_ = config.SaveConfig(cfgPath, cfg)

	if away {
		fmt.Println("🔒 Away Mode ARMED. lap-dog is actively monitoring access.")
	} else {
		fmt.Println("🟢 Away Mode DISARMED. Welcome back!")
	}
}

func showStatus(cfgPath string) {
	cfg, err := config.LoadConfig(cfgPath)
	if err != nil {
		log.Fatalf("Error loading config: %v", err)
	}

	store, err := storage.NewStorage(cfg.StoragePath, cfg.AwayMode)
	if err != nil {
		log.Fatalf("Error accessing storage: %v", err)
	}

	pending, sent, away := store.GetStats()
	statusStr := "ARMED 🔒"
	if !away {
		statusStr = "DISARMED 🟢"
	}

	fmt.Printf("\n--- lap-dog Status Dashboard ---\n")
	fmt.Printf("Version:        %s (commit: %s, built: %s)\n", version, commit, buildDate)
	fmt.Printf("Hostname:       %s\n", cfg.Hostname)
	fmt.Printf("Device ID:      %s\n", cfg.DeviceID)
	fmt.Printf("Platform:       %s/%s\n", runtime.GOOS, runtime.GOARCH)
	fmt.Printf("Away Mode:      %s\n", statusStr)
	fmt.Printf("Pending Queue:  %d events\n", pending)
	fmt.Printf("Sent History:   %d events\n", sent)
	fmt.Printf("Storage Path:   %s\n", cfg.StoragePath)
	if cfg.TelegramBotToken != "" {
		fmt.Printf("Telegram Bot:   Configured (Chat ID: %d)\n", cfg.TelegramChatID)
	} else {
		fmt.Printf("Telegram Bot:   NOT CONFIGURED (Run 'lapdog setup' or edit %s)\n", cfgPath)
	}
	fmt.Println()
}

func sendTestAlert(cfgPath string) {
	cfg, err := config.LoadConfig(cfgPath)
	if err != nil {
		log.Fatalf("Error loading config: %v", err)
	}

	store, err := storage.NewStorage(cfg.StoragePath, cfg.AwayMode)
	if err != nil {
		log.Fatalf("Error accessing storage: %v", err)
	}

	testEvt := event.SecurityEvent{
		ID:        fmt.Sprintf("test-%d", time.Now().Unix()),
		Timestamp: time.Now(),
		DeviceID:  cfg.DeviceID,
		Hostname:  cfg.Hostname,
		Platform:  "cli-test",
		EventType: event.LoginFailure,
		Username:  "test_user",
		Source:    "CLI 'lapdog test'",
		Severity:  event.SeverityMedium,
		Details:   "Manual test alert generated via CLI",
	}

	if err := store.Enqueue(testEvt); err != nil {
		log.Fatalf("Failed to enqueue test alert: %v", err)
	}

	fmt.Printf("✅ Test alert enqueued! (Pending queue size: %d)\nIf lap-dog agent is running, notification will dispatch to Telegram.\n", store.QueueSize())
}

func simulateEvent(cfgPath, evtTypeStr, username, sourceIP string) {
	cfg, err := config.LoadConfig(cfgPath)
	if err != nil {
		log.Fatalf("Error loading config: %v", err)
	}

	store, err := storage.NewStorage(cfg.StoragePath, cfg.AwayMode)
	if err != nil {
		log.Fatalf("Error accessing storage: %v", err)
	}

	eng := engine.NewEngine(cfg, store)

	eType := event.EventType(evtTypeStr)
	simEvt := detector.GenerateSampleEvent(eType, username, sourceIP)

	alerted, reason := eng.ProcessEvent(simEvt)
	if alerted {
		fmt.Printf("🚨 Simulated Event Processed -> ALERT ENQUEUED! (Reason: %s)\n", reason)
	} else {
		fmt.Printf("ℹ️ Simulated Event Processed -> No alert queued. (Reason: %s)\n", reason)
	}
}

func runInteractiveSetup(cfgPath string) {
	reader := bufio.NewReader(os.Stdin)

	fmt.Println("🐕 Welcome to lap-dog Setup Wizard")
	fmt.Println("----------------------------------")
	fmt.Printf("Config file: %s\n\n", cfgPath)

	cfg, err := config.LoadConfig(cfgPath)
	if err != nil {
		log.Fatalf("Failed to load or create config: %v", err)
	}

	// 1. Telegram Bot Token
	if cfg.TelegramBotToken != "" {
		fmt.Printf("Telegram Bot Token [%s...]: ", cfg.TelegramBotToken[:min(len(cfg.TelegramBotToken), 8)])
	} else {
		fmt.Print("Telegram Bot Token (from @BotFather): ")
	}
	tokenInput, _ := reader.ReadString('\n')
	tokenInput = strings.TrimSpace(tokenInput)
	if tokenInput != "" {
		cfg.TelegramBotToken = tokenInput
	}

	// 2. Telegram Chat ID
	if cfg.TelegramChatID != 0 {
		fmt.Printf("Telegram Chat ID [%d]: ", cfg.TelegramChatID)
	} else {
		fmt.Print("Telegram Chat ID: ")
	}
	chatIDInput, _ := reader.ReadString('\n')
	chatIDInput = strings.TrimSpace(chatIDInput)
	if chatIDInput != "" {
		if id, parseErr := strconv.ParseInt(chatIDInput, 10, 64); parseErr == nil {
			cfg.TelegramChatID = id
		} else {
			fmt.Printf("⚠️ Invalid Chat ID format '%s'. Keeping previous value.\n", chatIDInput)
		}
	}

	// 3. Away Mode Preference
	fmt.Print("Enable Away Mode by default? (Y/n): ")
	awayInput, _ := reader.ReadString('\n')
	awayInput = strings.TrimSpace(strings.ToLower(awayInput))
	if awayInput == "n" || awayInput == "no" {
		cfg.AwayMode = false
	} else {
		cfg.AwayMode = true
	}

	// Save configuration
	if err := config.SaveConfig(cfgPath, cfg); err != nil {
		log.Fatalf("Failed to save config: %v", err)
	}
	fmt.Println("\n✓ Configuration saved successfully!")

	// 4. Install Background Service
	fmt.Print("Install lap-dog as a background startup service? (Y/n): ")
	svcInput, _ := reader.ReadString('\n')
	svcInput = strings.TrimSpace(strings.ToLower(svcInput))
	if svcInput != "n" && svcInput != "no" {
		execPath, err := os.Executable()
		if err == nil {
			if err := service.ServiceInstall(execPath, cfgPath); err != nil {
				fmt.Printf("⚠️ Background service installation encountered an issue: %v\n", err)
			} else {
				fmt.Println("✓ Background service installed successfully!")
			}
		}
	}

	// 5. Send Test Alert if configured
	if cfg.TelegramBotToken != "" && cfg.TelegramChatID != 0 {
		fmt.Print("Send a test alert to Telegram right now? (Y/n): ")
		testInput, _ := reader.ReadString('\n')
		testInput = strings.TrimSpace(strings.ToLower(testInput))
		if testInput != "n" && testInput != "no" {
			sendTestAlert(cfgPath)
		}
	}

	fmt.Println("\n🐕 lap-dog setup complete!")
	fmt.Println("Run 'lapdog status' to inspect system status.")
}

func min(a, b int) int {
	if a < b {
		return a
	}
	return b
}

func runUninstall(purge, force bool) {
	fmt.Println("🐕 Uninstalling lap-dog Security Agent...")

	// 1. Teardown background service
	fmt.Println("⏳ Stopping and removing background service...")
	if err := service.ServiceUninstall(); err != nil {
		fmt.Printf("⚠️ Warning during service removal: %v\n", err)
	}

	// 2. Prompt for configuration purge if not specified by flags and interactive
	shouldPurge := purge
	if !purge && !force {
		reader := bufio.NewReader(os.Stdin)
		fmt.Print("Do you also want to remove configuration and data store in ~/.lapdog? (y/N): ")
		answer, _ := reader.ReadString('\n')
		answer = strings.TrimSpace(strings.ToLower(answer))
		if answer == "y" || answer == "yes" {
			shouldPurge = true
		}
	}

	if shouldPurge {
		home, _ := os.UserHomeDir()
		cfgDir := filepath.Join(home, ".lapdog")
		if err := os.RemoveAll(cfgDir); err != nil {
			fmt.Printf("⚠️ Warning: Failed to remove config directory %s: %v\n", cfgDir, err)
		} else {
			fmt.Printf("✓ Removed configuration directory %s\n", cfgDir)
		}
	}

	// 3. Remove binary self executable
	execPath, err := os.Executable()
	if err == nil {
		execPath, _ = filepath.Abs(execPath)
		fmt.Printf("⏳ Removing binary executable: %s...\n", execPath)

		if runtime.GOOS == "windows" {
			cmdStr := fmt.Sprintf("start /b cmd /c \"timeout /t 1 /nobreak >NUL & del /f /q \"%s\"\"", execPath)
			_ = exec.Command("cmd", "/c", cmdStr).Start()
			fmt.Println("✓ Windows binary removal scheduled.")
		} else {
			if removeErr := os.Remove(execPath); removeErr != nil {
				fmt.Printf("⚠️ Warning: Could not remove binary %s: %v\n", execPath, removeErr)
			} else {
				fmt.Println("✓ Removed binary executable.")
			}
		}
	}

	fmt.Println("\n✅ lap-dog has been uninstalled successfully.")
}

