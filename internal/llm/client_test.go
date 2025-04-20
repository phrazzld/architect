package llm

import (
	"context"
	"testing"
)

// TestMockLLMClient ensures the mock client correctly implements the LLMClient interface
func TestMockLLMClient(t *testing.T) {
	ctx := context.Background()
	promptText := "Test prompt"

	// Create a mock client with default implementations
	mockClient := &MockLLMClient{}

	// Test GenerateContent
	result, err := mockClient.GenerateContent(ctx, promptText, nil)
	if err != nil {
		t.Errorf("GenerateContent returned unexpected error: %v", err)
	}
	if result.Content != "Mock response" {
		t.Errorf("Expected content to be 'Mock response', got '%s'", result.Content)
	}

	// Test GetModelName
	modelName := mockClient.GetModelName()
	if modelName != "mock-model" {
		t.Errorf("Expected model name to be 'mock-model', got '%s'", modelName)
	}

	// Test Close
	err = mockClient.Close()
	if err != nil {
		t.Errorf("Close returned unexpected error: %v", err)
	}
}

// TestMockLLMClientCustom ensures the mock client correctly uses custom implementations
func TestMockLLMClientCustom(t *testing.T) {
	ctx := context.Background()
	promptText := "Test prompt"

	// Create a mock client with custom implementations
	mockClient := &MockLLMClient{
		GenerateContentFunc: func(ctx context.Context, prompt string, params map[string]interface{}) (*ProviderResult, error) {
			return &ProviderResult{Content: "Custom response"}, nil
		},
		GetModelNameFunc: func() string {
			return "custom-model"
		},
		CloseFunc: func() error {
			return nil
		},
	}

	// Test custom GenerateContent
	result, err := mockClient.GenerateContent(ctx, promptText, nil)
	if err != nil {
		t.Errorf("GenerateContent returned unexpected error: %v", err)
	}
	if result.Content != "Custom response" {
		t.Errorf("Expected content to be 'Custom response', got '%s'", result.Content)
	}

	// Test custom GetModelName
	modelName := mockClient.GetModelName()
	if modelName != "custom-model" {
		t.Errorf("Expected model name to be 'custom-model', got '%s'", modelName)
	}
}
