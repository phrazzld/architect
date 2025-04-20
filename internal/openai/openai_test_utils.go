// Package openai provides shared test utilities for OpenAI client tests
package openai

import (
	"context"

	"github.com/openai/openai-go"
	"github.com/phrazzld/architect/internal/llm"
)

// mockOpenAIAPI is a test double implementing openaiAPI for capturing or simulating API calls
type mockOpenAIAPI struct {
	createChatCompletionFunc           func(ctx context.Context, messages []openai.ChatCompletionMessageParamUnion, model string) (*openai.ChatCompletion, error)
	createChatCompletionWithParamsFunc func(ctx context.Context, params openai.ChatCompletionNewParams) (*openai.ChatCompletion, error)
}

func (m *mockOpenAIAPI) createChatCompletion(ctx context.Context, messages []openai.ChatCompletionMessageParamUnion, model string) (*openai.ChatCompletion, error) {
	return m.createChatCompletionFunc(ctx, messages, model)
}

func (m *mockOpenAIAPI) createChatCompletionWithParams(ctx context.Context, params openai.ChatCompletionNewParams) (*openai.ChatCompletion, error) {
	if m.createChatCompletionWithParamsFunc != nil {
		return m.createChatCompletionWithParamsFunc(ctx, params)
	}
	return m.createChatCompletionFunc(ctx, params.Messages, params.Model)
}

// mockAPIWithError creates a mockOpenAIAPI that returns the specified error for all API calls
func mockAPIWithError(err error) *mockOpenAIAPI {
	return &mockOpenAIAPI{
		createChatCompletionFunc: func(ctx context.Context, messages []openai.ChatCompletionMessageParamUnion, model string) (*openai.ChatCompletion, error) {
			return nil, err
		},
		createChatCompletionWithParamsFunc: func(ctx context.Context, params openai.ChatCompletionNewParams) (*openai.ChatCompletion, error) {
			return nil, err
		},
	}
}

// Token-related structs and methods removed

// Helper function to convert to a pointer
func toPtr[T any](v T) *T {
	return &v
}

// CreateMockOpenAIClientForTesting creates a mocked OpenAI client for testing
func CreateMockOpenAIClientForTesting(modelName string, responseFunc func(ctx context.Context, messages interface{}, model string) (interface{}, error)) (llm.LLMClient, error) {
	// Create a new client
	client := &openaiClient{
		api: &mockOpenAIAPI{
			createChatCompletionFunc: func(ctx context.Context, messages []openai.ChatCompletionMessageParamUnion, model string) (*openai.ChatCompletion, error) {
				// Call the response function but ignore its result
				_, err := responseFunc(ctx, messages, model)
				if err != nil {
					return nil, err
				}

				// Create a minimal success response
				return &openai.ChatCompletion{
					ID: "test-completion-id",
					Choices: []openai.ChatCompletionChoice{
						{
							Message: openai.ChatCompletionMessage{
								Role:    "assistant",
								Content: "Test response",
							},
						},
					},
					Usage: openai.CompletionUsage{
						CompletionTokens: 10,
					},
				}, nil
			},
		},
		modelName: modelName,
	}

	return client, nil
}

// MockAPIErrorResponse creates a mock error response with specifics about the error
// This is now just a convenience function that forwards to the version in errors.go
func MockAPIErrorResponseOld(errorType int, statusCode int, message string, details string) *llm.LLMError {
	return MockAPIErrorResponse(errorType, statusCode, message, details)
}

// Token-related mock functions removed
