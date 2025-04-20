// internal/gemini/client.go
package gemini

import (
	"context"

	"github.com/phrazzld/architect/internal/llm"
)

// ModelConfig holds Gemini model configuration parameters
type ModelConfig struct {
	MaxOutputTokens int32
	Temperature     float32
	TopP            float32
}

// ModelConfig holds Gemini model configuration parameters
// Parameters like context window size and token limits are now managed by
// individual providers directly and not stored in client structs.

// SafetyRating represents a content safety evaluation
type SafetyRating struct {
	Category string
	Blocked  bool
	Score    float32
}

// GenerationResult holds the response from a generate content call
type GenerationResult struct {
	Content       string
	FinishReason  string
	SafetyRatings []SafetyRating
	// TokenCount field removed in T036A-1
	Truncated bool
}

// TokenCount struct removed in T036A-1

// Client defines the interface for interacting with Gemini API
// This interface is maintained for backward compatibility
// New code should use the provider-agnostic llm.LLMClient interface instead
type Client interface {
	// GenerateContent sends a text prompt to Gemini and returns the generated content
	// If params is provided, these parameters will override the default model parameters
	GenerateContent(ctx context.Context, prompt string, params map[string]interface{}) (*GenerationResult, error)

	// CountTokens and GetModelInfo methods removed in T036A-1
	// Token handling is now managed by individual providers directly

	// GetModelName returns the name of the model being used
	GetModelName() string

	// GetTemperature returns the temperature setting for the model
	GetTemperature() float32

	// GetMaxOutputTokens returns the max output tokens setting for the model
	GetMaxOutputTokens() int32

	// GetTopP returns the topP setting for the model
	GetTopP() float32

	// Close releases resources used by the client
	Close() error
}

// ClientOption defines a function that modifies a client
type ClientOption func(interface{})

// WithHTTPClient is kept for backward compatibility but no longer does anything
func WithHTTPClient(client interface{}) ClientOption {
	return func(c interface{}) {
		// No-op - HTTP client is no longer used for token counting since that functionality has been removed
	}
}

// NewClient creates a new Gemini client implementing the legacy Client interface
// For new code, consider using NewLLMClient which returns the provider-agnostic llm.LLMClient directly
func NewClient(ctx context.Context, apiKey, modelName, apiEndpoint string, opts ...ClientOption) (Client, error) {
	// Create internalOpts that wrap the public options
	var internalOpts []geminiClientOption
	for _, opt := range opts {
		// Create a wrapper that calls the public option with the geminiClient
		internalOpts = append(internalOpts, func(gc *geminiClient) {
			opt(gc)
		})
	}

	// Create the provider-agnostic LLM client
	llmClient, err := newGeminiClient(ctx, apiKey, modelName, apiEndpoint, internalOpts...)
	if err != nil {
		return nil, err
	}

	// Create a mock client adapter for backward compatibility
	return &MockClient{
		GenerateContentFunc: func(ctx context.Context, prompt string, params map[string]interface{}) (*GenerationResult, error) {
			// Call through to the LLM client
			result, err := llmClient.GenerateContent(ctx, prompt, params)
			if err != nil {
				return nil, err
			}
			// Convert to legacy format
			return &GenerationResult{
				Content:       result.Content,
				FinishReason:  result.FinishReason,
				SafetyRatings: nil, // Safety info conversion would go here if needed
				Truncated:     result.Truncated,
			}, nil
		},
		GetModelNameFunc: llmClient.GetModelName,
		CloseFunc:        llmClient.Close,
	}, nil
}

// NewLLMClient creates a new Gemini client implementing the provider-agnostic llm.LLMClient interface
// This is the preferred method for new code
func NewLLMClient(ctx context.Context, apiKey, modelName, apiEndpoint string, opts ...ClientOption) (llm.LLMClient, error) {
	// Create internalOpts that wrap the public options
	var internalOpts []geminiClientOption
	for _, opt := range opts {
		// Create a wrapper that calls the public option with the geminiClient
		internalOpts = append(internalOpts, func(gc *geminiClient) {
			opt(gc)
		})
	}

	return newGeminiClient(ctx, apiKey, modelName, apiEndpoint, internalOpts...)
}

// DefaultModelConfig returns a reasonable default model configuration
func DefaultModelConfig() ModelConfig {
	return ModelConfig{
		MaxOutputTokens: 8192, // High limit for plan generation
		Temperature:     0.3,  // Lower temperature for deterministic output
		TopP:            0.9,  // Allow some creativity
	}
}
