// Package openai provides an implementation of the LLM client for the OpenAI API
package openai

import (
	"context"
	"errors"
	"net/http"
	"testing"

	"github.com/openai/openai-go"
	"github.com/phrazzld/architect/internal/llm"
	"github.com/stretchr/testify/assert"
)

// TestMockAPIWithError verifies that mockAPIWithError creates a mock API
// that returns the specified error for all API calls
func TestMockAPIWithError(t *testing.T) {
	// Create a test error
	testErr := errors.New("test API error")

	// Use the mockAPIWithError function that was previously unused
	mockAPI := mockAPIWithError(testErr)

	// Test context
	ctx := context.Background()

	// Test that createChatCompletion returns the error
	result, err := mockAPI.createChatCompletion(ctx, nil, "gpt-4")
	assert.Error(t, err, "Should return an error")
	assert.Equal(t, testErr, err, "Should return the specified error")
	assert.Nil(t, result, "Result should be nil when error occurs")

	// Test that createChatCompletionWithParams returns the error
	params := openai.ChatCompletionNewParams{
		Model: "gpt-4",
	}
	paramsResult, paramsErr := mockAPI.createChatCompletionWithParams(ctx, params)
	assert.Error(t, paramsErr, "Should return an error with params")
	assert.Equal(t, testErr, paramsErr, "Should return the specified error with params")
	assert.Nil(t, paramsResult, "Result should be nil when error occurs with params")
}

// TestToPtrFunction verifies that the toPtr function correctly
// converts values to pointers
func TestToPtrFunction(t *testing.T) {
	// Test with integer
	intValue := 42
	intPtr := toPtr(intValue)
	assert.NotNil(t, intPtr, "Pointer should not be nil")
	assert.Equal(t, intValue, *intPtr, "Pointed value should equal original value")

	// Test with string
	strValue := "test string"
	strPtr := toPtr(strValue)
	assert.NotNil(t, strPtr, "Pointer should not be nil")
	assert.Equal(t, strValue, *strPtr, "Pointed value should equal original string")

	// Test with boolean
	boolValue := true
	boolPtr := toPtr(boolValue)
	assert.NotNil(t, boolPtr, "Pointer should not be nil")
	assert.Equal(t, boolValue, *boolPtr, "Pointed value should equal original boolean")
}

// TestAPIErrorCreation verifies that MockAPIErrorResponse creates
// properly formatted API errors with appropriate suggestions
func TestAPIErrorCreation(t *testing.T) {
	// Define the constants for old error types
	const (
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
		expectCat     llm.ErrorCategory
	}{
		{
			name:          "Authentication Error",
			errorType:     ErrorTypeAuth,
			statusCode:    http.StatusUnauthorized,
			message:       "Invalid API key",
			details:       "API key is invalid or expired",
			expectSuggest: "Check that your API key is valid",
			expectCat:     llm.CategoryAuth,
		},
		{
			name:          "Rate Limit Error",
			errorType:     ErrorTypeRateLimit,
			statusCode:    http.StatusTooManyRequests,
			message:       "Rate limit exceeded",
			details:       "Too many requests per minute",
			expectSuggest: "Wait and try again later",
			expectCat:     llm.CategoryRateLimit,
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			// Create error using the MockAPIErrorResponse function
			apiError := MockAPIErrorResponse(tc.errorType, tc.statusCode, tc.message, tc.details)

			// Verify basic fields
			assert.Equal(t, tc.message, apiError.Message)
			assert.Equal(t, tc.details, apiError.Details)
			assert.Equal(t, tc.statusCode, apiError.StatusCode)
			assert.Equal(t, tc.expectCat, apiError.ErrorCategory)

			// Check that the suggestion is correct
			assert.Contains(t, apiError.Suggestion, tc.expectSuggest)

			// Test that it implements the error interface properly
			assert.Contains(t, apiError.Error(), tc.message)
		})
	}
}
