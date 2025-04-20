package openrouter_test

import (
	"context"
	"os"
	"testing"

	"github.com/phrazzld/architect/internal/logutil"
	"github.com/phrazzld/architect/internal/providers/openrouter"
)

// TestOpenRouterProviderLoggingNoSecrets verifies provider doesn't log secrets
func TestOpenRouterProviderLoggingNoSecrets(t *testing.T) {
	// Skip if no API key is available for testing
	apiKey := os.Getenv("OPENROUTER_API_KEY")
	if apiKey == "" {
		apiKey = "dummy_api_key_for_testing_only" // Use dummy key just for logging tests
	}

	// Create a context
	ctx := context.Background()

	// Create delegate logger
	mockLogger := logutil.NewLogger(logutil.DebugLevel, nil, "[test] ")

	// Wrap with secret detection
	secretLogger := logutil.WithSecretDetection(mockLogger)
	// Disable auto-failing for this test so we can verify there are no secrets detected
	secretLogger.SetFailOnSecretDetect(false)

	// Test provider creation
	provider := openrouter.NewProvider(secretLogger)

	// Test client creation
	client, err := provider.CreateClient(ctx, apiKey, "openai/gpt-3.5-turbo", "")
	if err != nil {
		t.Logf("Client creation error: %v", err)
		// We still want to check no secrets were logged, so don't fail here
	}

	// If client was created, test some operations
	if client != nil {
		// Get model info
		_, _ = client.GetModelInfo(ctx)

		// Count tokens (doesn't need real API call)
		_, _ = client.CountTokens(ctx, "Test message")
	}

	// If no secrets were detected, the test passes
	if secretLogger.HasDetectedSecrets() {
		detectedSecrets := secretLogger.GetDetectedSecrets()
		t.Errorf("Detected secrets in OpenRouter provider logging: %v", detectedSecrets)
	}
}

// TestOpenRouterClientLoggingNoSecrets directly tests client logging
func TestOpenRouterClientLoggingNoSecrets(t *testing.T) {
	// Create delegate logger
	mockLogger := logutil.NewLogger(logutil.DebugLevel, nil, "[test] ")

	// Wrap with secret detection
	secretLogger := logutil.WithSecretDetection(mockLogger)
	// Disable auto-failing for this test so we can verify there are no secrets detected
	secretLogger.SetFailOnSecretDetect(false)

	// Create a client directly
	client, err := openrouter.NewClient(
		"dummy_api_key_for_testing_only",
		"openai/gpt-3.5-turbo",
		"https://username:password@example.com/api", // URL with credentials that should be sanitized
		secretLogger,
	)

	if err != nil {
		t.Fatalf("Failed to create client: %v", err)
	}

	// Test the SanitizeURL functionality indirectly by doing operations that log URLs
	ctx := context.Background()

	// These operations should use SanitizeURL when logging the API endpoint
	_, _ = client.CountTokens(ctx, "Test message")

	// Check for secrets in logs
	if secretLogger.HasDetectedSecrets() {
		detectedSecrets := secretLogger.GetDetectedSecrets()
		t.Errorf("Detected secrets in OpenRouter client logging: %v", detectedSecrets)
	}
}
