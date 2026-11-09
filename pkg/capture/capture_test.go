package capture

import (
	"context"
	"errors"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

func TestCategorizeCaptureError(t *testing.T) {
	tests := []struct {
		name           string
		err            error
		stderr         string
		expectedSubstr string
	}{
		{
			name:           "Nil error",
			err:            nil,
			stderr:         "",
			expectedSubstr: "successfully",
		},
		{
			name:           "Permission denied / Antivirus block",
			err:            errors.New("exit status 1"),
			stderr:         "/dev/video0: Permission denied",
			expectedSubstr: "Blocked by Antivirus, security policy, or OS privacy permissions",
		},
		{
			name:           "Operation not permitted",
			err:            errors.New("exit status 126"),
			stderr:         "ioctl VIDIOC_STREAMON: Operation not permitted",
			expectedSubstr: "Blocked by Antivirus, security policy, or OS privacy permissions",
		},
		{
			name:           "Device busy / Webcam shield",
			err:            errors.New("exit status 1"),
			stderr:         "/dev/video0: Device or resource busy",
			expectedSubstr: "Webcam is busy or locked by another application",
		},
		{
			name:           "Device not found / Privacy shutter",
			err:            errors.New("exit status 1"),
			stderr:         "Cannot open /dev/video0: No such file or directory",
			expectedSubstr: "Webcam device not found",
		},
		{
			name:           "Context deadline exceeded / Timeout",
			err:            context.DeadlineExceeded,
			stderr:         "",
			expectedSubstr: "Capture timed out",
		},
		{
			name:           "Process killed by Antivirus / signal: killed",
			err:            errors.New("signal: killed"),
			stderr:         "",
			expectedSubstr: "Capture process killed unexpectedly",
		},
		{
			name:           "Missing tool",
			err:            errors.New("exec: \"ffmpeg\": executable file not found in $PATH"),
			stderr:         "",
			expectedSubstr: "Webcam capture utility not installed",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := CategorizeCaptureError(tt.err, tt.stderr)
			if !strings.Contains(got, tt.expectedSubstr) {
				t.Errorf("CategorizeCaptureError() = %q, expected to contain %q", got, tt.expectedSubstr)
			}
		})
	}
}

func TestWebcamCapturerDisabled(t *testing.T) {
	tempDir, err := os.MkdirTemp("", "capture_test_disabled")
	if err != nil {
		t.Fatalf("failed to create temp dir: %v", err)
	}
	defer os.RemoveAll(tempDir)

	capturer := NewWebcamCapturer(tempDir, false)
	img, status, err := capturer.Capture(context.Background(), "evt-123")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if img != "" {
		t.Errorf("expected empty image path when disabled, got %s", img)
	}
	if !strings.Contains(status, "disabled") {
		t.Errorf("expected disabled status, got %s", status)
	}
}

func TestCleanupOldCaptures(t *testing.T) {
	tempDir, err := os.MkdirTemp("", "capture_test_cleanup")
	if err != nil {
		t.Fatalf("failed to create temp dir: %v", err)
	}
	defer os.RemoveAll(tempDir)

	oldFile := filepath.Join(tempDir, "snap_old_1.jpg")
	newFile := filepath.Join(tempDir, "snap_new_2.jpg")

	if err := os.WriteFile(oldFile, []byte("old image"), 0600); err != nil {
		t.Fatalf("failed to write old file: %v", err)
	}
	if err := os.WriteFile(newFile, []byte("new image"), 0600); err != nil {
		t.Fatalf("failed to write new file: %v", err)
	}

	// Set old file mod time to 10 days ago
	tenDaysAgo := time.Now().Add(-10 * 24 * time.Hour)
	_ = os.Chtimes(oldFile, tenDaysAgo, tenDaysAgo)

	deleted, err := CleanupOldCaptures(tempDir, 7*24*time.Hour)
	if err != nil {
		t.Fatalf("CleanupOldCaptures failed: %v", err)
	}
	if deleted != 1 {
		t.Errorf("expected 1 file deleted, got %d", deleted)
	}

	if _, err := os.Stat(oldFile); !os.IsNotExist(err) {
		t.Errorf("expected old file to be deleted")
	}
	if _, err := os.Stat(newFile); os.IsNotExist(err) {
		t.Errorf("expected new file to still exist")
	}
}
