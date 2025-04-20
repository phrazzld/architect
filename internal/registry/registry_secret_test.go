package registry_test

import (
	"testing"

	"github.com/phrazzld/architect/internal/logutil"
	"github.com/phrazzld/architect/internal/registry"
)

// TestRegistryLoggingNoSecrets verifies that registry code doesn't log secrets
func TestRegistryLoggingNoSecrets(t *testing.T) {
	// Create delegate logger
	mockLogger := logutil.NewLogger(logutil.DebugLevel, nil, "[test] ")

	// Wrap with secret detection
	secretLogger := logutil.WithSecretDetection(mockLogger)
	// Disable auto-failing for this test so we can verify there are no secrets detected
	secretLogger.SetFailOnSecretDetect(false)

	// Create registry manager with secret-detecting logger
	regManager := registry.NewManager(secretLogger)

	// Initialize the registry (should load configuration safely without logging secrets)
	err := regManager.Initialize()
	if err != nil {
		t.Logf("Registry initialization error: %v", err)
		// This is acceptable in this test - we're just checking for secret logging
	}

	// Get the registry
	reg := regManager.GetRegistry()

	// Look up a model (shouldn't log any secrets)
	_, _ = reg.GetModel("gpt-4")
	// We don't care if model exists, just checking no secrets logged

	// If no secrets were detected, the test passes
	if secretLogger.HasDetectedSecrets() {
		detectedSecrets := secretLogger.GetDetectedSecrets()
		t.Errorf("Detected secrets in registry logging: %v", detectedSecrets)
	}
}
