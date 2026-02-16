package logging

import (
	"testing"
	"time"

	"github.com/anthropics/claude-flow-go/internal/shared"
)

func TestLogManager_SetLevelNormalizesWhitespaceAndCase(t *testing.T) {
	manager := NewLogManagerWithDefaults()

	err := manager.SetLevel("  WARNING  ")
	if err != nil {
		t.Fatalf("expected SetLevel success, got %v", err)
	}
	if manager.GetLevel() != shared.MCPLogLevelWarning {
		t.Fatalf("expected normalized level %q, got %q", shared.MCPLogLevelWarning, manager.GetLevel())
	}
}

func TestLogManager_SetLevelNilReceiverReturnsError(t *testing.T) {
	var manager *LogManager
	err := manager.SetLevel("info")
	if err == nil {
		t.Fatal("expected error for nil log manager")
	}
	if err.Error() != "log manager is required" {
		t.Fatalf("expected nil receiver error, got %q", err.Error())
	}
}

func TestLogManager_LogRecoversPanickingHandlers(t *testing.T) {
	manager := NewLogManager(shared.MCPLogLevelDebug, 10)

	received := make(chan LogEntry, 1)
	manager.AddHandler(func(entry LogEntry) {
		panic("intentional handler panic")
	})
	manager.AddHandler(func(entry LogEntry) {
		received <- entry
	})

	manager.Info("safe", map[string]interface{}{"ok": true})

	select {
	case entry := <-received:
		if entry.Message != "safe" {
			t.Fatalf("expected healthy handler message %q, got %q", "safe", entry.Message)
		}
	case <-time.After(2 * time.Second):
		t.Fatal("expected non-panicking handler invocation")
	}

	if manager.Count() != 1 {
		t.Fatalf("expected one stored entry, got %d", manager.Count())
	}
}

func TestLogManager_NilReceiverReadMethodsAreSafe(t *testing.T) {
	var manager *LogManager
	if entries := manager.GetEntries(10); len(entries) != 0 {
		t.Fatalf("expected no entries for nil manager, got %d", len(entries))
	}
	if entries := manager.GetEntriesByLevel(shared.MCPLogLevelInfo, 10); len(entries) != 0 {
		t.Fatalf("expected no level entries for nil manager, got %d", len(entries))
	}
	if count := manager.Count(); count != 0 {
		t.Fatalf("expected count 0 for nil manager, got %d", count)
	}
	manager.Clear()
	manager.Log(shared.MCPLogLevelInfo, "noop", nil)
	manager.LogWithLogger(shared.MCPLogLevelInfo, "logger", "noop", nil)
}
