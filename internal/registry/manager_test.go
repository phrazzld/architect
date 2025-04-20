package registry

import (
	"testing"

	"github.com/phrazzld/architect/internal/logutil"
)

// TestGetGlobalManager tests the singleton behavior of GetGlobalManager
func TestGetGlobalManager(t *testing.T) {
	// Reset the global manager for this test
	managerMu.Lock()
	globalManager = nil
	managerMu.Unlock()

	// Get the global manager
	logger := logutil.NewLogger(logutil.InfoLevel, nil, "[test] ")
	manager1 := GetGlobalManager(logger)
	if manager1 == nil {
		t.Fatal("Expected non-nil manager")
	}

	// Get it again and check if it's the same instance
	manager2 := GetGlobalManager(logger)
	if manager2 != manager1 {
		t.Error("Expected the same manager instance")
	}
}
