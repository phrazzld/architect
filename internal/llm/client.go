// internal/llm/client.go
package llm

import (
	"context"
)

// ProviderResult holds the response from a content generation call
type ProviderResult struct {
	Content      string   // The generated content
	FinishReason string   // Why generation stopped, e.g., "stop", "length", "safety"
	Truncated    bool     // Whether the response was truncated
	SafetyInfo   []Safety // Optional safety information
}

// Safety represents content safety evaluation information
type Safety struct {
	Category string  // Safety category name
	Blocked  bool    // Whether content was blocked due to this category
	Score    float32 // Severity score (provider-specific scale)
}

// LLMClient defines the interface for interacting with any LLM provider
type LLMClient interface {
	// GenerateContent sends a text prompt to the LLM and returns the generated content
	// If params is provided, these parameters will override the default model parameters
	GenerateContent(ctx context.Context, prompt string, params map[string]interface{}) (*ProviderResult, error)

	// GetModelName returns the name of the model being used
	GetModelName() string

	// Close releases resources used by the client
	Close() error
}

// MockLLMClient is a testing mock for the LLMClient interface
type MockLLMClient struct {
	GenerateContentFunc func(ctx context.Context, prompt string, params map[string]interface{}) (*ProviderResult, error)
	GetModelNameFunc    func() string
	CloseFunc           func() error
}

// GenerateContent implementation for MockLLMClient
func (m *MockLLMClient) GenerateContent(ctx context.Context, prompt string, params map[string]interface{}) (*ProviderResult, error) {
	if m.GenerateContentFunc != nil {
		return m.GenerateContentFunc(ctx, prompt, params)
	}
	return &ProviderResult{Content: "Mock response"}, nil
}

// GetModelName implementation for MockLLMClient
func (m *MockLLMClient) GetModelName() string {
	if m.GetModelNameFunc != nil {
		return m.GetModelNameFunc()
	}
	return "mock-model"
}

// Close implementation for MockLLMClient
func (m *MockLLMClient) Close() error {
	if m.CloseFunc != nil {
		return m.CloseFunc()
	}
	return nil
}
