// internal/gemini/generate_content_test.go
// Tests for the GenerateContent method
package gemini

import (
	"context"
	"errors"
	"strings"
	"testing"

	"github.com/phrazzld/architect/internal/llm"
)

func TestGenerateContent(t *testing.T) {
	// This test verifies that GenerateContent correctly:
	// - Validates the prompt (empty check)
	// - Handles API errors
	// - Processes responses correctly in various scenarios

	t.Run("Empty prompt validation", func(t *testing.T) {
		// Create a geminiClient directly to test its implementation
		client := &geminiClient{
			apiKey:    "test-key",
			modelName: "test-model",
			logger:    getTestLogger(),
			// No model needed as we're testing the empty prompt check which happens first
		}

		// Call GenerateContent with empty prompt
		result, err := client.GenerateContent(context.Background(), "", nil)

		// Verify error is returned
		if err == nil {
			t.Fatal("Expected error for empty prompt, got nil")
		}

		// Verify it's an APIError with the correct type
		apiErr, ok := IsGeminiError(err)
		if !ok {
			t.Fatalf("Expected LLMError from Gemini, got %T", err)
		}

		if GetErrorType(apiErr) != ErrorTypeInvalidRequest {
			t.Errorf("Expected error type %d, got %d", ErrorTypeInvalidRequest, GetErrorType(apiErr))
		}

		// Verify message mentions empty prompt
		if !strings.Contains(apiErr.Message, "empty prompt") {
			t.Errorf("Expected error message to mention empty prompt, got: %s", apiErr.Message)
		}

		// Result should be nil
		if result != nil {
			t.Errorf("Expected nil result, got %+v", result)
		}
	})

	// For the remaining tests, we'll use a mocked implementation of GenerateContent
	// This lets us test the interface without getting into the genai package details

	t.Run("API error handling", func(t *testing.T) {
		// Setup mock client that returns a specific error
		client := &MockClient{
			GenerateContentFunc: func(ctx context.Context, prompt string, params map[string]interface{}) (*GenerationResult, error) {
				return nil, CreateAPIError(
					llm.CategoryRateLimit,
					"Request rate limit or quota exceeded on the Gemini API",
					errors.New("API error: rate limit exceeded"),
					"Wait and try again later.",
				)
			},
		}

		// Call GenerateContent
		result, err := client.GenerateContent(context.Background(), "Test prompt", nil)

		// Verify error is returned
		if err == nil {
			t.Fatal("Expected error from API, got nil")
		}

		// Verify it's an APIError with the expected type
		apiErr, ok := IsGeminiError(err)
		if !ok {
			t.Fatalf("Expected LLMError from Gemini, got %T", err)
		}

		if GetErrorType(apiErr) != ErrorTypeRateLimit {
			t.Errorf("Expected error type %d, got %d", ErrorTypeRateLimit, GetErrorType(apiErr))
		}

		// Result should be nil
		if result != nil {
			t.Errorf("Expected nil result, got %+v", result)
		}
	})

	t.Run("Successful content generation", func(t *testing.T) {
		// Setup mock client that returns a successful result
		expectedResult := &GenerationResult{
			Content:      "Generated test content",
			FinishReason: "STOP",
			SafetyRatings: []SafetyRating{
				{
					Category: "HARM_CATEGORY_HARASSMENT",
					Score:    0.1,
					Blocked:  false,
				},
			},
			// TokenCount field removed in T036A-1
			Truncated: false,
		}

		client := &MockClient{
			GenerateContentFunc: func(ctx context.Context, prompt string, params map[string]interface{}) (*GenerationResult, error) {
				if prompt != "Test prompt" {
					t.Errorf("Expected prompt 'Test prompt', got '%s'", prompt)
				}
				return expectedResult, nil
			},
		}

		// Call GenerateContent
		result, err := client.GenerateContent(context.Background(), "Test prompt", nil)

		// Should not return an error
		if err != nil {
			t.Fatalf("Expected no error, got %v", err)
		}

		// Verify result matches expected
		if result != expectedResult {
			t.Errorf("Expected result %+v, got %+v", expectedResult, result)
		}
	})

	t.Run("Content with safety blocks", func(t *testing.T) {
		// Setup mock client that returns content with safety ratings
		client := &MockClient{
			GenerateContentFunc: func(ctx context.Context, prompt string, params map[string]interface{}) (*GenerationResult, error) {
				return &GenerationResult{
					Content:      "Safe content",
					FinishReason: "SAFETY",
					SafetyRatings: []SafetyRating{
						{
							Category: "HARM_CATEGORY_DANGEROUS_CONTENT",
							Score:    0.8,
							Blocked:  true,
						},
					},
				}, nil
			},
		}

		// Call GenerateContent
		result, err := client.GenerateContent(context.Background(), "Test prompt", nil)

		// Should not return an error (safety filtering is not an error)
		if err != nil {
			t.Fatalf("Expected no error for safety filtered content, got %v", err)
		}

		// Verify result
		if result == nil {
			t.Fatal("Expected non-nil result, got nil")
		}

		// Content should be available
		expected := "Safe content"
		if result.Content != expected {
			t.Errorf("Expected content %q, got %q", expected, result.Content)
		}

		// FinishReason should indicate safety
		if result.FinishReason != "SAFETY" {
			t.Errorf("Expected finish reason SAFETY, got %q", result.FinishReason)
		}

		// Should have safety ratings with blocked=true
		if len(result.SafetyRatings) != 1 {
			t.Fatalf("Expected 1 safety rating, got %d", len(result.SafetyRatings))
		}

		if !result.SafetyRatings[0].Blocked {
			t.Error("Expected blocked safety rating, got not blocked")
		}
	})

	t.Run("Truncated content", func(t *testing.T) {
		// Setup mock client that returns truncated content
		client := &MockClient{
			GenerateContentFunc: func(ctx context.Context, prompt string, params map[string]interface{}) (*GenerationResult, error) {
				return &GenerationResult{
					Content:      "Truncated content",
					FinishReason: "MAX_TOKENS",
					Truncated:    true,
				}, nil
			},
		}

		// Call GenerateContent
		result, err := client.GenerateContent(context.Background(), "Test prompt", nil)

		// Should not return an error
		if err != nil {
			t.Fatalf("Expected no error for truncated content, got %v", err)
		}

		// Verify result is marked as truncated
		if !result.Truncated {
			t.Error("Expected truncated=true, got truncated=false")
		}

		// FinishReason should indicate max tokens
		if result.FinishReason != "MAX_TOKENS" {
			t.Errorf("Expected finish reason MAX_TOKENS, got %q", result.FinishReason)
		}
	})
}
