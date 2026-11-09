package capture

import (
	"bytes"
	"context"
	"errors"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strings"
	"time"

	"lap-dog/pkg/osutil"
)

// Capturer defines an interface for capturing webcam photos
type Capturer interface {
	Capture(ctx context.Context, eventID string) (imagePath string, status string, err error)
}

// WebcamCapturer implements Capturer with cross-platform tools and robust error handling
type WebcamCapturer struct {
	CaptureDir string
	Enabled    bool
	Timeout    time.Duration
}

// NewWebcamCapturer creates a new instance with default timeout
func NewWebcamCapturer(captureDir string, enabled bool) *WebcamCapturer {
	return &WebcamCapturer{
		CaptureDir: captureDir,
		Enabled:    enabled,
		Timeout:    4 * time.Second,
	}
}

// Capture attempts to take a webcam snapshot and save it to the capture directory.
// Returns:
//   - imagePath: path to saved JPG file (empty if capture failed)
//   - status: human-readable status or diagnostic reason (e.g. Antivirus blocked, camera busy)
//   - err: raw error if any occurred
func (w *WebcamCapturer) Capture(ctx context.Context, eventID string) (string, string, error) {
	if !w.Enabled {
		return "", "Webcam capture disabled in configuration", nil
	}

	if err := os.MkdirAll(w.CaptureDir, 0700); err != nil {
		status := fmt.Sprintf("Failed to initialize capture directory: %v", err)
		return "", status, err
	}

	captureCtx, cancel := context.WithTimeout(ctx, w.Timeout)
	defer cancel()

	safeID := sanitizeID(eventID)
	outputPath := filepath.Join(w.CaptureDir, fmt.Sprintf("snap_%s_%d.jpg", safeID, time.Now().UnixNano()))

	var captureErr error
	var stderrOutput string

	switch runtime.GOOS {
	case "linux":
		stderrOutput, captureErr = captureLinux(captureCtx, outputPath)
	case "darwin":
		stderrOutput, captureErr = captureDarwin(captureCtx, outputPath)
	case "windows":
		stderrOutput, captureErr = captureWindows(captureCtx, outputPath)
	default:
		return "", fmt.Sprintf("Webcam capture not supported on OS %s", runtime.GOOS), errors.New("unsupported platform")
	}

	if captureErr != nil {
		_ = os.Remove(outputPath) // Ensure partial/corrupt file is removed
		diagStatus := CategorizeCaptureError(captureErr, stderrOutput)
		return "", diagStatus, captureErr
	}

	// Verify file was written and is not empty
	info, err := os.Stat(outputPath)
	if err != nil || info.Size() == 0 {
		_ = os.Remove(outputPath)
		return "", "Webcam produced an empty frame or file write failed", errors.New("empty capture file")
	}

	// Enforce strict file permissions on captured photo
	_ = os.Chmod(outputPath, 0600)

	return outputPath, "Webcam snapshot captured successfully", nil
}

func captureLinux(ctx context.Context, outputPath string) (string, error) {
	// 1. Check if video devices exist
	devs, _ := filepath.Glob("/dev/video*")
	if len(devs) == 0 {
		return "", errors.New("no video device found (/dev/video* missing)")
	}
	videoDev := "/dev/video0"
	if _, err := os.Stat(videoDev); err != nil {
		videoDev = devs[0]
	}

	// 2. Try ffmpeg first (most capable)
	if ffmpegPath, err := exec.LookPath("ffmpeg"); err == nil {
		// Try MJPEG format first (fastest capture on USB webcams)
		cmd := exec.CommandContext(ctx, ffmpegPath,
			"-nostdin", "-y",
			"-f", "v4l2",
			"-input_format", "mjpeg",
			"-i", videoDev,
			"-update", "1",
			"-frames:v", "1",
			outputPath,
		)
		var stderr bytes.Buffer
		cmd.Stderr = &stderr
		if runErr := cmd.Run(); runErr == nil {
			return stderr.String(), nil
		}

		// Fallback without forced mjpeg (handles raw YUYV / fallback drivers)
		cmdFallback := exec.CommandContext(ctx, ffmpegPath,
			"-nostdin", "-y",
			"-f", "v4l2",
			"-i", videoDev,
			"-update", "1",
			"-frames:v", "1",
			outputPath,
		)
		var stderrFallback bytes.Buffer
		cmdFallback.Stderr = &stderrFallback
		if runErr := cmdFallback.Run(); runErr == nil {
			return stderrFallback.String(), nil
		} else {
			return stderrFallback.String(), fmt.Errorf("%w: %s", runErr, stderrFallback.String())
		}
	}

	// 3. Fallback to fswebcam if installed
	if fswebcamPath, err := exec.LookPath("fswebcam"); err == nil {
		cmd := exec.CommandContext(ctx, fswebcamPath,
			"-d", videoDev,
			"-r", "1280x720",
			"--no-banner",
			"-S", "2", // skip 2 frames for auto-exposure
			outputPath,
		)
		var stderr bytes.Buffer
		cmd.Stderr = &stderr
		err := cmd.Run()
		return stderr.String(), err
	}

	return "", errors.New("no capture utility found (ffmpeg or fswebcam required)")
}

func captureDarwin(ctx context.Context, outputPath string) (string, error) {
	// 1. Check imagesnap (standard macOS CLI tool)
	if snapPath, err := exec.LookPath("imagesnap"); err == nil {
		cmd := exec.CommandContext(ctx, snapPath, "-w", "0.5", outputPath)
		var stderr bytes.Buffer
		cmd.Stderr = &stderr
		err := cmd.Run()
		return stderr.String(), err
	}

	// 2. Fallback to ffmpeg on macOS (avfoundation)
	if ffmpegPath, err := exec.LookPath("ffmpeg"); err == nil {
		cmd := exec.CommandContext(ctx, ffmpegPath,
			"-nostdin", "-y",
			"-f", "avfoundation",
			"-framerate", "30",
			"-i", "default",
			"-update", "1",
			"-frames:v", "1",
			outputPath,
		)
		var stderr bytes.Buffer
		cmd.Stderr = &stderr
		err := cmd.Run()
		return stderr.String(), err
	}

	return "", errors.New("no capture utility found (imagesnap or ffmpeg required)")
}

func captureWindows(ctx context.Context, outputPath string) (string, error) {
	ffmpegPath, err := exec.LookPath("ffmpeg")
	if err != nil {
		commonPaths := []string{
			`C:\ffmpeg\bin\ffmpeg.exe`,
			filepath.Join(os.Getenv("ProgramFiles"), "ffmpeg", "bin", "ffmpeg.exe"),
			filepath.Join(os.Getenv("LOCALAPPDATA"), "Microsoft", "WinGet", "Links", "ffmpeg.exe"),
		}
		for _, p := range commonPaths {
			if _, statErr := os.Stat(p); statErr == nil {
				ffmpegPath = p
				err = nil
				break
			}
		}
	}

	if err == nil {
		cmd := exec.CommandContext(ctx, ffmpegPath,
			"-nostdin", "-y",
			"-f", "dshow",
			"-i", "video=Integrated Camera",
			"-update", "1",
			"-frames:v", "1",
			outputPath,
		)
		osutil.PrepareSilentCommand(cmd)
		var stderr bytes.Buffer
		cmd.Stderr = &stderr
		if runErr := cmd.Run(); runErr == nil {
			return stderr.String(), nil
		} else {
			return stderr.String(), runErr
		}
	}

	return "", errors.New("no capture utility found (ffmpeg required on Windows)")
}

// CategorizeCaptureError evaluates errors and stderr output to provide clear diagnostic messages
func CategorizeCaptureError(err error, stderr string) string {
	if err == nil {
		return "Webcam snapshot captured successfully"
	}

	combined := strings.ToLower(err.Error() + " " + stderr)

	// 1. Check for Antivirus / Permission Denied / Privacy restriction
	if strings.Contains(combined, "permission denied") ||
		strings.Contains(combined, "operation not permitted") ||
		strings.Contains(combined, "access is denied") ||
		strings.Contains(combined, "access denied") ||
		strings.Contains(combined, "eacces") ||
		strings.Contains(combined, "eperm") ||
		strings.Contains(combined, "not authorized") ||
		strings.Contains(combined, "unauthorized") {
		return "Blocked by Antivirus, security policy, or OS privacy permissions (Access Denied)"
	}

	// 2. Check for Device / Resource Busy (lock filter driver or in-use)
	if strings.Contains(combined, "device or resource busy") ||
		strings.Contains(combined, "ebusy") ||
		strings.Contains(combined, "device busy") ||
		strings.Contains(combined, "already in use") ||
		strings.Contains(combined, "in use") {
		return "Webcam is busy or locked by another application/antivirus webcam shield"
	}

	// 3. Check for Device Not Found / Disconnected
	if strings.Contains(combined, "no video device found") ||
		strings.Contains(combined, "no such file or directory") ||
		strings.Contains(combined, "cannot find") ||
		strings.Contains(combined, "no such device") ||
		strings.Contains(combined, "device not found") ||
		strings.Contains(combined, "not detected") {
		return "Webcam device not found (hardware disconnected, disabled in BIOS, or privacy shutter closed)"
	}

	// 4. Check for Timeout (Antivirus sandbox delay or hardware hang)
	if errors.Is(err, context.DeadlineExceeded) || strings.Contains(combined, "timed out") || strings.Contains(combined, "deadline exceeded") {
		return "Capture timed out (possible Antivirus sandbox inspection or hardware lag)"
	}

	// 5. Check for Signal Termination (Antivirus kill)
	if strings.Contains(combined, "signal: killed") || strings.Contains(combined, "status 137") {
		return "Capture process killed unexpectedly (possible Antivirus/EDR termination)"
	}

	// 6. Missing Tool
	if strings.Contains(combined, "no capture utility found") || strings.Contains(combined, "executable file not found") {
		return "Webcam capture utility not installed on system (ffmpeg/fswebcam)"
	}

	// Default sanitized error
	return fmt.Sprintf("Webcam capture failed: %v", err)
}

// CleanupOldCaptures removes any temporary photos older than maxAge
func CleanupOldCaptures(captureDir string, maxAge time.Duration) (int, error) {
	if _, err := os.Stat(captureDir); os.IsNotExist(err) {
		return 0, nil
	}

	entries, err := os.ReadDir(captureDir)
	if err != nil {
		return 0, err
	}

	cutoff := time.Now().Add(-maxAge)
	deleted := 0

	for _, entry := range entries {
		if entry.IsDir() || !strings.HasPrefix(entry.Name(), "snap_") {
			continue
		}
		info, err := entry.Info()
		if err != nil {
			continue
		}
		if info.ModTime().Before(cutoff) {
			_ = os.Remove(filepath.Join(captureDir, entry.Name()))
			deleted++
		}
	}

	return deleted, nil
}

func sanitizeID(id string) string {
	r := strings.NewReplacer("/", "_", "\\", "_", ":", "_", " ", "_")
	return r.Replace(id)
}
