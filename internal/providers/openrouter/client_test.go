// Package openrouter contains tests for the OpenRouter client
package openrouter

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"sync"
	"testing"

	"github.com/phrazzld/architect/internal/logutil"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// Skip client tests that require direct access to the unexported client type.
// These tests would need to be reworked if the client is exported or access is provided through exported functions.

// TestCreateClientThroughProvider tests client creation through the provider
func TestCreateClientThroughProvider(t *testing.T) {
	// Create a provider
	logger := logutil.NewLogger(logutil.InfoLevel, nil, "[test] ")
	provider := NewProvider(logger)

	tests := []struct {
		name        string
		apiKey      string
		modelID     string
		apiEndpoint string
		wantErr     bool
		errContains string
	}{
		{
			name:        "Valid inputs with default endpoint",
			apiKey:      "test-api-key",
			modelID:     "anthropic/claude-3-opus-20240229",
			apiEndpoint: "",
			wantErr:     false,
		},
		{
			name:        "Valid inputs with custom endpoint",
			apiKey:      "test-api-key",
			modelID:     "anthropic/claude-3-opus-20240229",
			apiEndpoint: "https://custom-endpoint.com/api/v1",
			wantErr:     false,
		},
		{
			name:        "Empty API key",
			apiKey:      "",
			modelID:     "anthropic/claude-3-opus-20240229",
			apiEndpoint: "",
			wantErr:     true,
			errContains: "no OpenRouter API key provided",
		},
		{
			name:        "Empty model ID",
			apiKey:      "test-api-key",
			modelID:     "",
			apiEndpoint: "",
			wantErr:     true,
			errContains: "model ID cannot be empty",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Setenv("OPENROUTER_API_KEY", "")

			// Create client via provider
			client, err := provider.CreateClient(context.Background(), tt.apiKey, tt.modelID, tt.apiEndpoint)

			if tt.wantErr {
				assert.Error(t, err)
				if tt.errContains != "" {
					assert.Contains(t, err.Error(), tt.errContains)
				}
				assert.Nil(t, client)
			} else {
				assert.NoError(t, err)
				assert.NotNil(t, client)

				// Check that the client's model name matches what we provided
				assert.Equal(t, tt.modelID, client.GetModelName())
			}
		})
	}
}

// TestClientMethodsThroughProvider tests the client's methods
func TestClientMethodsThroughProvider(t *testing.T) {
	// Create provider
	logger := logutil.NewLogger(logutil.InfoLevel, nil, "[test] ")
	provider := NewProvider(logger)

	// Get client
	client, err := provider.CreateClient(context.Background(), "test-api-key", "anthropic/claude-3-opus-20240229", "")
	require.NoError(t, err)

	// Test GetModelName
	assert.Equal(t, "anthropic/claude-3-opus-20240229", client.GetModelName())

	// Test Close
	err = client.Close()
	assert.NoError(t, err)
}

// MockRoundTripper is a mock implementation of the http.RoundTripper interface
type MockRoundTripper struct {
	// ResponseByParams maps parameter combinations to predefined responses
	// This allows different parameter sets to return different responses
	ResponsesByParams map[string][]byte
	// Lock for thread-safe operations
	mu sync.Mutex
}

// RoundTrip implements the http.RoundTripper interface with thread-safe response selection
func (m *MockRoundTripper) RoundTrip(req *http.Request) (*http.Response, error) {
	m.mu.Lock()
	defer m.mu.Unlock()

	// Read the request body to determine which response to return
	var requestBody bytes.Buffer
	if req.Body != nil {
		bodyBytes, _ := io.ReadAll(req.Body)
		// Restore the request body for later use if needed
		req.Body = io.NopCloser(bytes.NewBuffer(bodyBytes))
		requestBody.Write(bodyBytes)
	}

	// Parse the request to get a key for the response map
	var requestData map[string]interface{}
	if err := json.Unmarshal(requestBody.Bytes(), &requestData); err != nil {
		return nil, err
	}

	// Create a key based on relevant parameters
	var paramsKey string
	if temp, ok := requestData["temperature"]; ok {
		paramsKey += fmt.Sprintf("temp:%.2f", temp.(float64))
	}
	if topP, ok := requestData["top_p"]; ok {
		paramsKey += fmt.Sprintf(",topP:%.2f", topP.(float64))
	}
	if maxTokens, ok := requestData["max_tokens"]; ok {
		paramsKey += fmt.Sprintf(",maxTokens:%d", int(maxTokens.(float64)))
	}

	// Get the predefined response for these parameters or use default
	responseBody, ok := m.ResponsesByParams[paramsKey]
	if !ok {
		// Use a default success response if no specific one is defined
		defaultResponse := ChatCompletionResponse{
			ID:      "test-id",
			Object:  "chat.completion",
			Created: 1677825464,
			Model:   "test-model",
			Choices: []ChatCompletionChoice{
				{
					Index: 0,
					Message: ChatCompletionMessage{
						Role:    "assistant",
						Content: "This is a default mock response",
					},
					FinishReason: "stop",
				},
			},
			Usage: ChatCompletionUsage{
				PromptTokens:     10,
				CompletionTokens: 20,
				TotalTokens:      30,
			},
		}
		responseBody, _ = json.Marshal(defaultResponse)
	}

	// Create and return the HTTP response
	return &http.Response{
		StatusCode: http.StatusOK,
		Body:       io.NopCloser(bytes.NewBuffer(responseBody)),
	}, nil
}

// TestConcurrentGenerateContent tests that GenerateContent can handle concurrent requests
func TestConcurrentGenerateContent(t *testing.T) {
	// Skip in short mode as race detection can be time consuming
	if testing.Short() {
		t.Skip("Skipping race detection test in short mode")
	}

	// Initialize logger
	logger := logutil.NewLogger(logutil.DebugLevel, nil, "[test] ")

	// Create mock responses for different parameter combinations
	responses := make(map[string][]byte)

	// Response 1: Temperature 0.7, TopP 1.0
	response1 := ChatCompletionResponse{
		ID:      "mock-id-1",
		Object:  "chat.completion",
		Created: 1677825464,
		Model:   "test-model",
		Choices: []ChatCompletionChoice{
			{
				Index: 0,
				Message: ChatCompletionMessage{
					Role:    "assistant",
					Content: "Response with temperature 0.7",
				},
				FinishReason: "stop",
			},
		},
		Usage: ChatCompletionUsage{
			PromptTokens:     10,
			CompletionTokens: 20,
			TotalTokens:      30,
		},
	}
	respBytes1, _ := json.Marshal(response1)
	responses["temp:0.70,topP:1.00"] = respBytes1

	// Response 2: Temperature 0.3, Max Tokens 100
	response2 := ChatCompletionResponse{
		ID:      "mock-id-2",
		Object:  "chat.completion",
		Created: 1677825464,
		Model:   "test-model",
		Choices: []ChatCompletionChoice{
			{
				Index: 0,
				Message: ChatCompletionMessage{
					Role:    "assistant",
					Content: "Response with temperature 0.3 and max tokens 100",
				},
				FinishReason: "stop",
			},
		},
		Usage: ChatCompletionUsage{
			PromptTokens:     10,
			CompletionTokens: 20,
			TotalTokens:      30,
		},
	}
	respBytes2, _ := json.Marshal(response2)
	responses["temp:0.30,maxTokens:100"] = respBytes2

	// Response 3: Temperature 0.5, TopP 0.9, Max Tokens 50
	response3 := ChatCompletionResponse{
		ID:      "mock-id-3",
		Object:  "chat.completion",
		Created: 1677825464,
		Model:   "test-model",
		Choices: []ChatCompletionChoice{
			{
				Index: 0,
				Message: ChatCompletionMessage{
					Role:    "assistant",
					Content: "Response with temperature 0.5, topP 0.9, and max tokens 50",
				},
				FinishReason: "stop",
			},
		},
		Usage: ChatCompletionUsage{
			PromptTokens:     10,
			CompletionTokens: 20,
			TotalTokens:      30,
		},
	}
	respBytes3, _ := json.Marshal(response3)
	responses["temp:0.50,topP:0.90,maxTokens:50"] = respBytes3

	// Create a mock round tripper
	mockRoundTripper := &MockRoundTripper{
		ResponsesByParams: responses,
	}

	// Create a custom HTTP client that uses our mock round tripper
	mockHTTPClient := &http.Client{
		Transport: mockRoundTripper,
	}

	// Create a client with the mock HTTP client
	client, err := NewClient("test-api-key", "anthropic/claude-3-opus", "", logger)
	require.NoError(t, err)

	// Replace the client's HTTP client with our mock
	client.httpClient = mockHTTPClient

	// Define test parameters
	testCases := []struct {
		name       string
		prompt     string
		params     map[string]interface{}
		expectResp string
	}{
		{
			name:   "Parameters Set 1 - Temperature 0.7",
			prompt: "Test prompt 1",
			params: map[string]interface{}{
				"temperature": float32(0.7),
				"top_p":       float32(1.0),
			},
			expectResp: "Response with temperature 0.7",
		},
		{
			name:   "Parameters Set 2 - Temperature 0.3 + Max Tokens",
			prompt: "Test prompt 2",
			params: map[string]interface{}{
				"temperature": float32(0.3),
				"max_tokens":  int32(100),
			},
			expectResp: "Response with temperature 0.3 and max tokens 100",
		},
		{
			name:   "Parameters Set 3 - Multiple Parameters",
			prompt: "Test prompt 3",
			params: map[string]interface{}{
				"temperature": float32(0.5),
				"top_p":       float32(0.9),
				"max_tokens":  int32(50),
			},
			expectResp: "Response with temperature 0.5, topP 0.9, and max tokens 50",
		},
	}

	// Wait group to synchronize goroutines
	var wg sync.WaitGroup

	// Run test cases concurrently to check for race conditions
	for i := 0; i < 3; i++ { // Run 3 iterations for each test case
		for _, tc := range testCases {
			wg.Add(1)

			// Create a local copy of test case for goroutine
			testCase := tc

			// Run in a goroutine to test concurrency
			go func() {
				defer wg.Done()

				// Call GenerateContent with the test parameters
				result, err := client.GenerateContent(context.Background(), testCase.prompt, testCase.params)

				// Assert in goroutine is not ideal, but we'll collect errors if any
				assert.NoError(t, err)
				assert.NotNil(t, result)
				assert.Contains(t, result.Content, testCase.expectResp)
			}()
		}
	}

	// Wait for all goroutines to complete
	wg.Wait()
}
