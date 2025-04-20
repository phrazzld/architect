package openrouter

import (
	"context"
	"os"
	"testing"

	"github.com/phrazzld/architect/internal/logutil"
	"github.com/stretchr/testify/assert"
)

func TestNewProvider(t *testing.T) {
	// Test with logger
	logger := logutil.NewLogger(logutil.InfoLevel, nil, "[test] ")
	provider := NewProvider(logger)

	assert.NotNil(t, provider, "Provider should not be nil")
}

func TestCreateClient(t *testing.T) {
	// Create provider with test logger
	logger := logutil.NewLogger(logutil.InfoLevel, nil, "[test] ")
	provider := NewProvider(logger)

	// Test cases
	tests := []struct {
		name          string
		apiKey        string
		modelID       string
		apiEndpoint   string
		setupEnvVar   bool
		expectError   bool
		errorContains string
	}{
		{
			name:        "Valid inputs with explicit API key",
			apiKey:      "test-api-key",
			modelID:     "anthropic/claude-3-opus-20240229",
			apiEndpoint: "",
			setupEnvVar: false,
			expectError: false,
		},
		{
			name:        "Valid inputs with custom endpoint",
			apiKey:      "test-api-key",
			modelID:     "anthropic/claude-3-opus-20240229",
			apiEndpoint: "https://custom-endpoint.com/api/v1",
			setupEnvVar: false,
			expectError: false,
		},
		{
			name:        "Valid inputs with env var API key",
			apiKey:      "",
			modelID:     "anthropic/claude-3-opus-20240229",
			apiEndpoint: "",
			setupEnvVar: true,
			expectError: false,
		},
		{
			name:          "Empty API key with no env var",
			apiKey:        "",
			modelID:       "anthropic/claude-3-opus-20240229",
			apiEndpoint:   "",
			setupEnvVar:   false,
			expectError:   true,
			errorContains: "no OpenRouter API key provided",
		},
		{
			name:          "Empty model ID",
			apiKey:        "test-api-key",
			modelID:       "",
			apiEndpoint:   "",
			setupEnvVar:   false,
			expectError:   true,
			errorContains: "model ID cannot be empty",
		},
	}

	// Save original env var to restore later
	originalAPIKey := os.Getenv("OPENROUTER_API_KEY")
	defer func() {
		_ = os.Setenv("OPENROUTER_API_KEY", originalAPIKey)
	}()

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Setup environment for this test
			if tt.setupEnvVar {
				_ = os.Setenv("OPENROUTER_API_KEY", "env-var-test-key")
			} else {
				_ = os.Unsetenv("OPENROUTER_API_KEY")
			}

			// Create client
			client, err := provider.CreateClient(context.Background(), tt.apiKey, tt.modelID, tt.apiEndpoint)

			// Check error expectations
			if tt.expectError {
				assert.Error(t, err)
				if tt.errorContains != "" {
					assert.Contains(t, err.Error(), tt.errorContains)
				}
				assert.Nil(t, client)
			} else {
				assert.NoError(t, err)
				assert.NotNil(t, client)

				// We've already verified client is not nil and it's returned from a function with LLMClient return type
				// Go's type system guarantees the interface implementation

				// Check the client's model name matches what we provided
				assert.Equal(t, tt.modelID, client.GetModelName())
			}
		})
	}
}

// TestProviderImplementsProviderInterface verifies the provider implements the Provider interface
func TestProviderImplementsProviderInterface(t *testing.T) {
	// No need for any assertions - this test just needs to compile
	_ = NewProvider(nil)
}
