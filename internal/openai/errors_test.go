package openai

import (
	"errors"
	"strings"
	"testing"

	"github.com/phrazzld/architect/internal/llm"
)

// Test that LLMError implements the llm.CategorizedError interface correctly
func TestLLMError_CategoryMapping(t *testing.T) {
	tests := []struct {
		name             string
		category         llm.ErrorCategory
		expectedCategory llm.ErrorCategory
	}{
		{"Auth Error", llm.CategoryAuth, llm.CategoryAuth},
		{"Rate Limit Error", llm.CategoryRateLimit, llm.CategoryRateLimit},
		{"Invalid Request Error", llm.CategoryInvalidRequest, llm.CategoryInvalidRequest},
		{"Not Found Error", llm.CategoryNotFound, llm.CategoryNotFound},
		{"Server Error", llm.CategoryServer, llm.CategoryServer},
		{"Network Error", llm.CategoryNetwork, llm.CategoryNetwork},
		{"Cancelled Error", llm.CategoryCancelled, llm.CategoryCancelled},
		{"Input Limit Error", llm.CategoryInputLimit, llm.CategoryInputLimit},
		{"Content Filtered Error", llm.CategoryContentFiltered, llm.CategoryContentFiltered},
		{"Unknown Error", llm.CategoryUnknown, llm.CategoryUnknown},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			// Create an OpenAI error using our helper function
			openaiErr := CreateAPIError(tc.category, "Test error", nil, "")

			// Verify it implements the interface
			var catErr llm.CategorizedError = openaiErr

			// Check that the category is correctly mapped
			if category := catErr.Category(); category != tc.expectedCategory {
				t.Errorf("Expected category %v, got %v", tc.expectedCategory, category)
			}
		})
	}
}

// Test that an OpenAI error can be detected using IsOpenAIError
func TestIsOpenAIError(t *testing.T) {
	// Create an OpenAI error
	openaiErr := CreateAPIError(llm.CategoryRateLimit, "Rate limit exceeded", nil, "")

	// Test with direct error
	llmErr, ok := IsOpenAIError(openaiErr)
	if !ok {
		t.Errorf("Expected OpenAI error to be detected")
		return
	}
	if llmErr.Provider != "openai" {
		t.Errorf("Expected provider 'openai', got '%s'", llmErr.Provider)
	}
	if llmErr.ErrorCategory != llm.CategoryRateLimit {
		t.Errorf("Expected category RateLimit, got %v", llmErr.ErrorCategory)
	}

	// Test with wrapped error that isn't an OpenAI error
	wrappedErr := errors.New("wrapper: " + openaiErr.Error())
	_, ok = IsOpenAIError(wrappedErr)
	if ok {
		t.Errorf("Expected wrapped standard error not to be detected as OpenAIError")
	}

	// Test that it can still be detected as a CategorizedError
	catErr, ok := llm.IsCategorizedError(openaiErr)
	if !ok {
		t.Errorf("Expected OpenAI error to be detected as CategorizedError")
		return
	}
	if catErr.Category() != llm.CategoryRateLimit {
		t.Errorf("Expected category RateLimit, got %v", catErr.Category())
	}
}

// Test that FormatAPIError creates appropriate error objects
func TestFormatAPIError(t *testing.T) {
	testCases := []struct {
		name           string
		inputErr       error
		statusCode     int
		expectCategory llm.ErrorCategory
	}{
		{
			name:           "Nil error",
			inputErr:       nil,
			statusCode:     0,
			expectCategory: llm.CategoryUnknown,
		},
		{
			name:           "Auth error by status code",
			inputErr:       errors.New("invalid API key"),
			statusCode:     401,
			expectCategory: llm.CategoryAuth,
		},
		{
			name:           "Rate limit error by status code",
			inputErr:       errors.New("too many requests"),
			statusCode:     429,
			expectCategory: llm.CategoryRateLimit,
		},
		{
			name:           "Rate limit error by message",
			inputErr:       errors.New("rate limit exceeded"),
			statusCode:     0,
			expectCategory: llm.CategoryRateLimit,
		},
		{
			name:           "Input limit error by message",
			inputErr:       errors.New("token limit exceeded"),
			statusCode:     0,
			expectCategory: llm.CategoryInputLimit,
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			if tc.inputErr == nil {
				// Special case for nil error
				result := FormatAPIError(nil, tc.statusCode)
				if result != nil {
					t.Errorf("Expected nil result for nil error, got %v", result)
				}
				return
			}

			// Test formatting
			result := FormatAPIError(tc.inputErr, tc.statusCode)

			// Check provider
			if result.Provider != "openai" {
				t.Errorf("Expected provider 'openai', got '%s'", result.Provider)
			}

			// Check category
			if result.ErrorCategory != tc.expectCategory {
				t.Errorf("Expected category %v, got %v", tc.expectCategory, result.ErrorCategory)
			}

			// Check that suggestion is set
			if result.Suggestion == "" {
				t.Errorf("Expected suggestion to be set, got empty string")
			}

			// Check that original error is preserved
			if result.Original != tc.inputErr {
				t.Errorf("Expected original error to be preserved")
			}
		})
	}
}

// Test MockAPIErrorResponse for backward compatibility
func TestMockAPIErrorResponse(t *testing.T) {
	const (
		// These are the old ErrorType constants
		ErrorTypeAuth           = 1
		ErrorTypeRateLimit      = 2
		ErrorTypeInvalidRequest = 3
	)

	testCases := []struct {
		name          string
		errorType     int
		statusCode    int
		message       string
		details       string
		expectSuggest string
	}{
		{
			name:          "Auth error",
			errorType:     ErrorTypeAuth,
			statusCode:    401,
			message:       "Invalid API key",
			details:       "Test details",
			expectSuggest: "Check that your API key is valid",
		},
		{
			name:          "Rate limit error",
			errorType:     ErrorTypeRateLimit,
			statusCode:    429,
			message:       "Rate limit exceeded",
			details:       "Test details",
			expectSuggest: "Wait and try again later",
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			// Create mock error using old-style parameters
			result := MockAPIErrorResponse(tc.errorType, tc.statusCode, tc.message, tc.details)

			// Check that fields are set correctly
			if result.StatusCode != tc.statusCode {
				t.Errorf("Expected status code %d, got %d", tc.statusCode, result.StatusCode)
			}
			if result.Message != tc.message {
				t.Errorf("Expected message '%s', got '%s'", tc.message, result.Message)
			}
			if result.Details != tc.details {
				t.Errorf("Expected details '%s', got '%s'", tc.details, result.Details)
			}
			if result.Provider != "openai" {
				t.Errorf("Expected provider 'openai', got '%s'", result.Provider)
			}
			if !strings.Contains(result.Suggestion, tc.expectSuggest) {
				t.Errorf("Expected suggestion to contain '%s', got '%s'", tc.expectSuggest, result.Suggestion)
			}
		})
	}
}
