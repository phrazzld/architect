// internal/gemini/client_test.go
package gemini

import (
	"context"
	"errors"
	"io"
	"net/http"
	"strings"
	"testing"
)

// mockHTTPClient is a simple mock implementation of HTTPClient interface
type mockHTTPClient struct{}

func (m *mockHTTPClient) Do(req *http.Request) (*http.Response, error) {
	return &http.Response{
		StatusCode: http.StatusOK,
		Body:       io.NopCloser(strings.NewReader(`{"test":"data"}`)),
	}, nil
}

func TestNewClient(t *testing.T) {
	tests := []struct {
		name        string
		apiKey      string
		modelName   string
		apiEndpoint string
		wantErr     bool
		errMessage  string
	}{
		{
			name:        "Empty API key",
			apiKey:      "",
			modelName:   "gemini-pro",
			apiEndpoint: "",
			wantErr:     true,
			errMessage:  "API key cannot be empty",
		},
		{
			name:        "Empty model name",
			apiKey:      "test-api-key",
			modelName:   "",
			apiEndpoint: "",
			wantErr:     true,
			errMessage:  "model name cannot be empty",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			ctx := context.Background()

			// Test validation at the NewClient wrapper level
			client, err := NewClient(ctx, tt.apiKey, tt.modelName, tt.apiEndpoint)

			// Error cases
			if tt.wantErr {
				if err == nil {
					t.Errorf("NewClient() error = nil, wantErr %v", tt.wantErr)
				}
				if tt.errMessage != "" && err.Error() != tt.errMessage {
					t.Errorf("NewClient() error message = %v, want %v", err.Error(), tt.errMessage)
				}
				if client != nil {
					t.Error("NewClient() returned non-nil client when error was expected")
				}
			} else if err != nil {
				t.Errorf("NewClient() error = %v, wantErr %v", err, tt.wantErr)
			}
		})
	}
}

func TestNewClientWithOptions(t *testing.T) {
	ctx := context.Background()

	// Create a mock HTTP client
	mockHTTPClient := &mockHTTPClient{}

	// Test with custom HTTP client
	client, err := NewClient(
		ctx,
		"test-api-key",
		"gemini-pro",
		"",
		WithHTTPClient(mockHTTPClient),
	)

	if err != nil {
		t.Fatalf("NewClient() with custom HTTP client error = %v", err)
	}

	if client == nil {
		t.Fatal("NewClient() with custom HTTP client returned nil")
	}

	// We can't directly test that the HTTP client was set since geminiClient is not exported
	// The proper way to test this would be with integration tests that verify the HTTP client behavior
}

func TestDefaultModelConfig(t *testing.T) {
	config := DefaultModelConfig()

	// Check default values
	if config.MaxOutputTokens != 8192 {
		t.Errorf("Expected MaxOutputTokens to be 8192, got %d", config.MaxOutputTokens)
	}

	if config.Temperature != 0.3 {
		t.Errorf("Expected Temperature to be 0.3, got %f", config.Temperature)
	}

	if config.TopP != 0.9 {
		t.Errorf("Expected TopP to be 0.9, got %f", config.TopP)
	}

	// Verify default config matches documentation
	expectedConfig := ModelConfig{
		MaxOutputTokens: 8192, // High limit for plan generation
		Temperature:     0.3,  // Lower temperature for deterministic output
		TopP:            0.9,  // Allow some creativity
	}

	if config.MaxOutputTokens != expectedConfig.MaxOutputTokens {
		t.Errorf("DefaultModelConfig().MaxOutputTokens = %v, want %v",
			config.MaxOutputTokens, expectedConfig.MaxOutputTokens)
	}

	if config.Temperature != expectedConfig.Temperature {
		t.Errorf("DefaultModelConfig().Temperature = %v, want %v",
			config.Temperature, expectedConfig.Temperature)
	}

	if config.TopP != expectedConfig.TopP {
		t.Errorf("DefaultModelConfig().TopP = %v, want %v",
			config.TopP, expectedConfig.TopP)
	}
}

func TestMockClient(t *testing.T) {
	client := NewMockClient()
	ctx := context.Background()

	// GetModelInfo test removed in T036A-1

	t.Run("GenerateContent", func(t *testing.T) {
		// Test default implementation
		result, err := client.GenerateContent(ctx, "test prompt", nil)
		if err != nil {
			t.Fatalf("GenerateContent returned unexpected error: %v", err)
		}
		if result.Content != "Mock response" {
			t.Errorf("Expected content %q, got %q", "Mock response", result.Content)
		}

		// Test custom implementation
		expectedErr := errors.New("custom error")
		client.GenerateContentFunc = func(ctx context.Context, prompt string, params map[string]interface{}) (*GenerationResult, error) {
			if prompt == "error test" {
				return nil, expectedErr
			}
			return &GenerationResult{Content: "Custom " + prompt}, nil
		}

		// Test error case
		_, err = client.GenerateContent(ctx, "error test", nil)
		if err != expectedErr {
			t.Errorf("Expected error %v, got %v", expectedErr, err)
		}

		// Test success case
		result, err = client.GenerateContent(ctx, "success", nil)
		if err != nil {
			t.Fatalf("GenerateContent returned unexpected error: %v", err)
		}
		if result.Content != "Custom success" {
			t.Errorf("Expected content %q, got %q", "Custom success", result.Content)
		}
	})

	// CountTokens test removed in T036A-1

	t.Run("GetterMethods", func(t *testing.T) {
		// Test default implementations
		if name := client.GetModelName(); name != "mock-model" {
			t.Errorf("GetModelName() = %v, want %v", name, "mock-model")
		}

		if temp := client.GetTemperature(); temp != DefaultModelConfig().Temperature {
			t.Errorf("GetTemperature() = %v, want %v", temp, DefaultModelConfig().Temperature)
		}

		if tokens := client.GetMaxOutputTokens(); tokens != DefaultModelConfig().MaxOutputTokens {
			t.Errorf("GetMaxOutputTokens() = %v, want %v", tokens, DefaultModelConfig().MaxOutputTokens)
		}

		if topP := client.GetTopP(); topP != DefaultModelConfig().TopP {
			t.Errorf("GetTopP() = %v, want %v", topP, DefaultModelConfig().TopP)
		}

		// Test custom implementations
		client.GetModelNameFunc = func() string { return "custom-model" }
		client.GetTemperatureFunc = func() float32 { return 0.7 }
		client.GetMaxOutputTokensFunc = func() int32 { return 100 }
		client.GetTopPFunc = func() float32 { return 0.5 }

		if name := client.GetModelName(); name != "custom-model" {
			t.Errorf("GetModelName() = %v, want %v", name, "custom-model")
		}

		if temp := client.GetTemperature(); temp != 0.7 {
			t.Errorf("GetTemperature() = %v, want %v", temp, 0.7)
		}

		if tokens := client.GetMaxOutputTokens(); tokens != 100 {
			t.Errorf("GetMaxOutputTokens() = %v, want %v", tokens, 100)
		}

		if topP := client.GetTopP(); topP != 0.5 {
			t.Errorf("GetTopP() = %v, want %v", topP, 0.5)
		}
	})

	t.Run("Close", func(t *testing.T) {
		// Test default implementation
		if err := client.Close(); err != nil {
			t.Errorf("Close() returned unexpected error: %v", err)
		}

		// Test custom implementation
		expectedErr := errors.New("close error")
		client.CloseFunc = func() error {
			return expectedErr
		}

		if err := client.Close(); err != expectedErr {
			t.Errorf("Close() = %v, want %v", err, expectedErr)
		}
	})
}

// Note: More extensive tests for the actual geminiClient would require mocking the HTTP client,
// which we'll implement in the integration tests (T3) task later
