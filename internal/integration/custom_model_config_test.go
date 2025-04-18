// internal/integration/custom_model_config_test.go
package integration

import (
	"bytes"
	"context"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/phrazzld/architect/internal/architect"
	"github.com/phrazzld/architect/internal/auditlog"
	"github.com/phrazzld/architect/internal/llm"
	"github.com/phrazzld/architect/internal/logutil"
	"github.com/phrazzld/architect/internal/registry"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// TestCustomModelConfiguration tests that custom model configurations in models.yaml
// are correctly loaded and their token limits are respected by the system.
func TestCustomModelConfiguration(t *testing.T) {
	// Skip in short mode to reduce CI time
	if testing.Short() {
		t.Skip("Skipping test in short mode")
	}

	// Define test cases for different registry configurations
	testCases := []struct {
		name                 string
		customModelConfig    string
		modelName            string
		promptLength         int    // Length of the test prompt
		expectedContextLimit int32  // Expected context limit from registry
		expectedLogMessage   string // Expected message in logs
	}{
		{
			name: "Custom model with larger context window",
			customModelConfig: `
api_key_sources:
  test: "TEST_API_KEY"

providers:
  - name: test
    base_url: "https://test.example.com"

models:
  - name: custom-large-context
    provider: test
    api_model_id: custom-large-context
    context_window: 50000       # Custom large context window
    max_output_tokens: 4000     # Custom max output
    parameters:
      temperature:
        type: float
        default: 0.7
`,
			modelName:            "custom-large-context",
			promptLength:         15000,
			expectedContextLimit: 50000,
			expectedLogMessage:   "Using token limits from registry for model custom-large-context",
		},
		{
			name: "Custom model with smaller context window",
			customModelConfig: `
api_key_sources:
  test: "TEST_API_KEY"

providers:
  - name: test
    base_url: "https://test.example.com"

models:
  - name: custom-small-context
    provider: test
    api_model_id: custom-small-context
    context_window: 2000        # Smaller context window
    max_output_tokens: 1000     # Custom max output
    parameters:
      temperature:
        type: float
        default: 0.7
`,
			modelName:            "custom-small-context",
			promptLength:         1000,
			expectedContextLimit: 2000,
			expectedLogMessage:   "Using token limits from registry for model custom-small-context",
		},
		{
			name: "Custom model configuration for existing model name",
			customModelConfig: `
api_key_sources:
  test: "TEST_API_KEY"

providers:
  - name: test
    base_url: "https://test.example.com"

models:
  - name: gpt-4-turbo
    provider: test
    api_model_id: gpt-4-turbo
    context_window: 52000       # Higher than the hardcoded 32k limit
    max_output_tokens: 4096
    parameters:
      temperature:
        type: float
        default: 0.7
`,
			modelName:            "gpt-4-turbo",
			promptLength:         3000,
			expectedContextLimit: 52000, // Should use the registry value, not the hardcoded fallback
			expectedLogMessage:   "Using token limits from registry for model gpt-4-turbo",
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			// Create a temporary directory for test files
			testDir, err := os.MkdirTemp("", "architect-test-*")
			require.NoError(t, err, "Failed to create temp dir")
			defer func() {
				_ = os.RemoveAll(testDir)
			}()

			// Create a capture buffer for logs
			logBuffer := &bytes.Buffer{}
			captureLogger := logutil.NewLogger(logutil.DebugLevel, logBuffer, "[test] ")

			// Create a no-op audit logger for tests
			auditLogger := auditlog.NewNoOpAuditLogger()

			// Create a temporary config directory with custom models.yaml
			configDir := filepath.Join(testDir, ".config/architect")
			err = os.MkdirAll(configDir, 0755)
			require.NoError(t, err, "Failed to create config dir")

			configPath := filepath.Join(configDir, "models.yaml")
			err = os.WriteFile(configPath, []byte(tc.customModelConfig), 0644)
			require.NoError(t, err, "Failed to write test config")

			// Create a config loader that points to our test file
			configLoader := &registry.ConfigLoader{}
			configLoader.GetConfigPath = func() (string, error) {
				return configPath, nil
			}

			// Create and initialize a registry
			reg := registry.NewRegistry(captureLogger)
			err = reg.LoadConfig(configLoader)
			require.NoError(t, err, "Failed to load config")

			// Create a mock provider
			mockProvider := &MockCustomModelProvider{t: t}
			err = reg.RegisterProviderImplementation("test", mockProvider)
			require.NoError(t, err, "Failed to register provider")

			// Generate a test prompt of specified length
			testPrompt := strings.Repeat("This is a test prompt. ", tc.promptLength/20)
			if len(testPrompt) < tc.promptLength {
				testPrompt = testPrompt + strings.Repeat("X", tc.promptLength-len(testPrompt))
			}

			// Create a context
			ctx := context.Background()

			// Create a client through the registry
			client, err := reg.CreateLLMClient(ctx, "test-api-key", tc.modelName)
			require.NoError(t, err, "Failed to create client")

			// Create a token manager with the registry and client
			tokenManager, err := architect.NewTokenManager(captureLogger, auditLogger, client, reg)
			require.NoError(t, err, "Failed to create token manager")

			// Test that token limits from registry are used
			tokenInfo, err := tokenManager.GetTokenInfo(ctx, testPrompt)
			require.NoError(t, err, "Failed to get token info")

			// The context window should match what we defined in the registry
			assert.Equal(t, tc.expectedContextLimit, tokenInfo.InputLimit, "Expected registry context window value was not used")

			// Verify the logs to confirm registry was used as the token source
			logOutput := logBuffer.String()
			t.Logf("Log output: %s", logOutput)
			assert.Contains(t, logOutput, tc.expectedLogMessage, "Log should indicate registry values are being used")

			// Check if the token count is what we expect from our mock client
			expectedTokenCount := int32(len(testPrompt) / 4) // Mock client divides by 4
			assert.Equal(t, expectedTokenCount, tokenInfo.TokenCount, "Token count does not match expected mock client behavior")
		})
	}
}

// MockCustomModelProvider is a simplified version of the mock provider for testing
type MockCustomModelProvider struct {
	t *testing.T
}

// MockCustomModelClient implements llm.LLMClient for token limit testing
type MockCustomModelClient struct {
	modelName         string
	tokenCount        int32
	inputTokenLimit   int32
	getModelInfoError error
	countTokensError  error
}

// GenerateContent implements llm.LLMClient
func (m *MockCustomModelClient) GenerateContent(ctx context.Context, prompt string, params map[string]interface{}) (*llm.ProviderResult, error) {
	return &llm.ProviderResult{
		Content:    "mock response",
		TokenCount: m.tokenCount,
	}, nil
}

// CountTokens implements llm.LLMClient
func (m *MockCustomModelClient) CountTokens(ctx context.Context, prompt string) (*llm.ProviderTokenCount, error) {
	if m.countTokensError != nil {
		return nil, m.countTokensError
	}

	// Simple token counting logic - divide string length by 4
	m.tokenCount = int32(len(prompt) / 4)
	return &llm.ProviderTokenCount{Total: m.tokenCount}, nil
}

// GetModelInfo implements llm.LLMClient
func (m *MockCustomModelClient) GetModelInfo(ctx context.Context) (*llm.ProviderModelInfo, error) {
	if m.getModelInfoError != nil {
		return nil, m.getModelInfoError
	}

	return &llm.ProviderModelInfo{
		Name:             m.modelName,
		InputTokenLimit:  m.inputTokenLimit,
		OutputTokenLimit: 1000,
	}, nil
}

// GetModelName implements llm.LLMClient
func (m *MockCustomModelClient) GetModelName() string {
	return m.modelName
}

// Close implements llm.LLMClient
func (m *MockCustomModelClient) Close() error {
	return nil
}

// CreateClient creates a mock LLM client
func (p *MockCustomModelProvider) CreateClient(ctx context.Context, apiKey, modelID, apiEndpoint string) (llm.LLMClient, error) {
	// Important: Return the same model name that was passed in
	// so that the token manager can find it in the registry
	return &MockCustomModelClient{
		modelName:       modelID,
		tokenCount:      0,    // Will be set by CountTokens
		inputTokenLimit: 8192, // Default value, should be overridden by registry
	}, nil
}
