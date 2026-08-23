package detector

import (
	"context"
	"fmt"
	"runtime"

	"lap-dog/pkg/config"
	"lap-dog/pkg/event"
)

type Detector interface {
	Name() string
	Start(ctx context.Context, eventChan chan<- event.SecurityEvent) error
}

func NewDetector(cfg *config.Config) (Detector, error) {
	if cfg.MockMode {
		return NewMockDetector(cfg), nil
	}

	switch runtime.GOOS {
	case "linux":
		return NewLinuxDetector(cfg), nil
	case "windows":
		return NewWindowsDetector(cfg), nil
	case "darwin":
		return NewDarwinDetector(cfg), nil
	default:
		return nil, fmt.Errorf("unsupported platform for auto detector: %s (use mock mode)", runtime.GOOS)
	}
}
