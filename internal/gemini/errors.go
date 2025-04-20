// Package gemini provides a client for interacting with Google's Gemini API
package gemini

import (
	"errors"
	"fmt"
	"net/http"
	"strings"

	"github.com/phrazzld/architect/internal/llm"
)

// IsGeminiError checks if an error is an llm.LLMError originating from Gemini
func IsGeminiError(err error) (*llm.LLMError, bool) {
	var llmErr *llm.LLMError
	if errors.As(err, &llmErr) && llmErr.Provider == "gemini" {
		return llmErr, true
	}
	return nil, false
}

// FormatAPIError creates a standardized LLMError from a Gemini API error
func FormatAPIError(err error, statusCode int) *llm.LLMError {
	if err == nil {
		return nil
	}

	// Check if it's already an LLMError
	var llmErr *llm.LLMError
	if errors.As(err, &llmErr) {
		return llmErr
	}

	// Get error category from the shared library
	category := getErrorCategory(err, statusCode)

	// Create a formatted error with Gemini-specific suggestions
	llmError := CreateAPIError(category, "", err, "")

	return llmError
}

// CreateAPIError creates a new LLMError with Gemini-specific settings
func CreateAPIError(category llm.ErrorCategory, errMsg string, originalErr error, details string) *llm.LLMError {
	// If no message provided, use the original error message or a default
	message := errMsg
	if message == "" && originalErr != nil {
		message = originalErr.Error()
	}

	llmError := llm.New(
		"gemini",    // Provider
		"",          // Code
		0,           // StatusCode
		message,     // Message
		"",          // RequestID
		originalErr, // Original error
		category,    // Error category
	)

	// Add details if provided
	if details != "" {
		llmError.Details = details
	}

	// Only set default messages and suggestions if a custom message wasn't provided
	if errMsg == "" {
		// Set Gemini-specific error messages and suggestions based on category
		switch category {
		case llm.CategoryAuth:
			llmError.Message = "Authentication failed with the Gemini API"
			llmError.Suggestion = "Check that your API key is valid and has not expired. Ensure environment variables are set correctly."

		case llm.CategoryRateLimit:
			llmError.Message = "Request rate limit or quota exceeded on the Gemini API"
			llmError.Suggestion = "Wait and try again later. Consider adjusting the --max-concurrent and --rate-limit flags to limit request rate. You can also upgrade your API usage tier if this happens frequently."

		case llm.CategoryInvalidRequest:
			llmError.Message = "Invalid request sent to the Gemini API"
			llmError.Suggestion = "Check the prompt format and parameters. Ensure they comply with the API requirements."

		case llm.CategoryNotFound:
			llmError.Message = "The requested model or resource was not found"
			llmError.Suggestion = "Verify that the model name is correct and that the model is available in your region."

		case llm.CategoryServer:
			llmError.Message = "Gemini API server error occurred"
			llmError.Suggestion = "This is typically a temporary issue. Wait a few moments and try again."

		case llm.CategoryNetwork:
			llmError.Message = "Network error while connecting to the Gemini API"
			llmError.Suggestion = "Check your internet connection and try again. If persistent, there may be connectivity issues to Google's servers."

		case llm.CategoryCancelled:
			llmError.Message = "Request to Gemini API was cancelled"
			llmError.Suggestion = "The operation was interrupted. Try again with a longer timeout if needed."

		case llm.CategoryInputLimit:
			llmError.Message = "Input token limit exceeded for the Gemini model"
			llmError.Suggestion = "Reduce the input size by using --include, --exclude, or --exclude-names flags to filter the context."

		case llm.CategoryContentFiltered:
			llmError.Message = "Content was filtered by Gemini API safety settings"
			llmError.Suggestion = "Your prompt or content may have triggered safety filters. Review and modify your input to comply with content policies."

		default:
			if originalErr != nil {
				llmError.Message = fmt.Sprintf("Error calling Gemini API: %v", originalErr)
			} else {
				llmError.Message = "Unknown error with Gemini API"
			}
			llmError.Suggestion = "Check the logs for more details or try again."
		}
	}

	// Always set the suggestion if not already provided
	if llmError.Suggestion == "" {
		switch category {
		case llm.CategoryAuth:
			llmError.Suggestion = "Check that your API key is valid and has not expired. Ensure environment variables are set correctly."
		case llm.CategoryRateLimit:
			llmError.Suggestion = "Wait and try again later. Consider adjusting the --max-concurrent and --rate-limit flags to limit request rate."
		case llm.CategoryInvalidRequest:
			llmError.Suggestion = "Check the prompt format and parameters. Ensure they comply with the API requirements."
		default:
			llmError.Suggestion = "Check the logs for more details or try again."
		}
	}

	return llmError
}

// getErrorCategory determines the error category based on the error message and status code
// This is a helper function used internally by FormatAPIError to maintain backward compatibility
func getErrorCategory(err error, statusCode int) llm.ErrorCategory {
	if err == nil {
		return llm.CategoryUnknown
	}

	errMsg := err.Error()

	// Map HTTP status codes to error categories
	switch statusCode {
	case http.StatusUnauthorized, http.StatusForbidden:
		return llm.CategoryAuth
	case http.StatusTooManyRequests:
		return llm.CategoryRateLimit
	case http.StatusBadRequest:
		return llm.CategoryInvalidRequest
	case http.StatusNotFound:
		return llm.CategoryNotFound
	}

	if statusCode >= 500 && statusCode < 600 {
		return llm.CategoryServer
	}

	// Use string matching as fallback for error types
	lowerMsg := strings.ToLower(errMsg)

	// Match error categories based on error message
	if strings.Contains(lowerMsg, "rate limit") || strings.Contains(lowerMsg, "quota") {
		return llm.CategoryRateLimit
	}

	if strings.Contains(lowerMsg, "safety") || strings.Contains(lowerMsg, "blocked") || strings.Contains(lowerMsg, "filtered") {
		return llm.CategoryContentFiltered
	}

	if strings.Contains(lowerMsg, "token limit") || strings.Contains(lowerMsg, "tokens exceeds") {
		return llm.CategoryInputLimit
	}

	if strings.Contains(lowerMsg, "network") || strings.Contains(lowerMsg, "connection") || strings.Contains(lowerMsg, "timeout") {
		return llm.CategoryNetwork
	}

	if strings.Contains(lowerMsg, "canceled") || strings.Contains(lowerMsg, "cancelled") || strings.Contains(lowerMsg, "deadline exceeded") {
		return llm.CategoryCancelled
	}

	return llm.CategoryUnknown
}

// Backward compatibility functions

// IsAPIError is maintained for backward compatibility
// It checks if an error is a Gemini error and returns it as an LLMError
func IsAPIError(err error) (*llm.LLMError, bool) {
	return IsGeminiError(err)
}
