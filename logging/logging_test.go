package logging

import (
	"log"
	"log/slog"
	"os"
	"path/filepath"
	"testing"
)

func TestInitLogger_FileLogging(t *testing.T) {
	logFile := filepath.Join(t.TempDir(), "test.log")
	t.Setenv("LOG_FILE", logFile)

	cleanup := InitLogger("text")
	defer cleanup()

	slog.Info("hello from slog")

	data, err := os.ReadFile(logFile)
	if err != nil {
		t.Fatalf("reading log file: %v", err)
	}
	if len(data) == 0 {
		t.Fatal("expected log file to contain data, but it was empty")
	}
	if got := string(data); !contains(got, "hello from slog") {
		t.Errorf("log file missing slog message, got:\n%s", got)
	}
}

func TestInitLogger_StandardLogRedirect(t *testing.T) {
	logFile := filepath.Join(t.TempDir(), "test.log")
	t.Setenv("LOG_FILE", logFile)

	cleanup := InitLogger("text")
	defer cleanup()

	log.Println("hello from standard log")

	data, err := os.ReadFile(logFile)
	if err != nil {
		t.Fatalf("reading log file: %v", err)
	}
	if got := string(data); !contains(got, "hello from standard log") {
		t.Errorf("log file missing standard log message, got:\n%s", got)
	}
}

func TestInitLogger_NoFile(t *testing.T) {
	t.Setenv("LOG_FILE", "")

	cleanup := InitLogger("text")
	defer cleanup()

	// Verify cleanup is safe to call (no-op) and logging still works.
	slog.Info("stdout only")
}

func TestInitLogger_JSONFormat(t *testing.T) {
	logFile := filepath.Join(t.TempDir(), "test.log")
	t.Setenv("LOG_FILE", logFile)

	cleanup := InitLogger("json")
	defer cleanup()

	slog.Info("json test message")

	data, err := os.ReadFile(logFile)
	if err != nil {
		t.Fatalf("reading log file: %v", err)
	}
	got := string(data)
	if !contains(got, `"msg"`) || !contains(got, "json test message") {
		t.Errorf("expected JSON-formatted log entry, got:\n%s", got)
	}
}

func TestInitLogger_InvalidPath(t *testing.T) {
	t.Setenv("LOG_FILE", "/no/such/directory/test.log")

	cleanup := InitLogger("text")
	defer cleanup()

	// Should gracefully fall back to stdout without panicking.
	slog.Info("fallback test")
}

func TestInitLogger_FileAppendsAcrossCalls(t *testing.T) {
	logFile := filepath.Join(t.TempDir(), "test.log")
	t.Setenv("LOG_FILE", logFile)

	cleanup1 := InitLogger("text")
	slog.Info("first message")
	cleanup1()

	cleanup2 := InitLogger("text")
	slog.Info("second message")
	cleanup2()

	data, err := os.ReadFile(logFile)
	if err != nil {
		t.Fatalf("reading log file: %v", err)
	}
	got := string(data)
	if !contains(got, "first message") {
		t.Errorf("log file missing first message, got:\n%s", got)
	}
	if !contains(got, "second message") {
		t.Errorf("log file missing second message, got:\n%s", got)
	}
}

func contains(s, substr string) bool {
	return len(s) >= len(substr) && searchString(s, substr)
}

func searchString(s, substr string) bool {
	for i := 0; i <= len(s)-len(substr); i++ {
		if s[i:i+len(substr)] == substr {
			return true
		}
	}
	return false
}
