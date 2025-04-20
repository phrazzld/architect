// Package openai provides a client for interacting with the OpenAI API
package openai

import (
	"errors"

	"github.com/phrazzld/architect/internal/llm"
)

// IsOpenAIError checks if an error is an llm.LLMError originating from OpenAI
func IsOpenAIError(err error) (*llm.LLMError, bool) {
	var llmErr *llm.LLMError
	if errors.As(err, &llmErr) && llmErr.Provider == "openai" {
		return llmErr, true
	}
	return nil, false
}

// FormatAPIError creates a standardized LLMError from an OpenAI API error
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
	category := llm.DetectErrorCategory(err, statusCode)

	// Create a formatted error with OpenAI-specific suggestions
	llmError := llm.CreateStandardErrorWithMessage("openai", category, err, "")

	// Add OpenAI-specific suggestions for certain error types
	switch category {
	case llm.CategoryAuth:
		llmError.Suggestion = "Check that your API key is valid and has not expired. Ensure environment variables are set correctly."
	case llm.CategoryRateLimit:
		llmError.Suggestion = "Wait and try again later. Consider adjusting the --max-concurrent and --rate-limit flags to limit request rate. You can also upgrade your API usage tier if this happens frequently."
	case llm.CategoryInvalidRequest:
		llmError.Suggestion = "Check the prompt format and parameters. Ensure they comply with the API requirements."
	case llm.CategoryNotFound:
		llmError.Suggestion = "Verify that the model name is correct and that the model is available in your region."
	case llm.CategoryServer:
		llmError.Suggestion = "This is typically a temporary issue. Wait a few moments and try again."
	case llm.CategoryNetwork:
		llmError.Suggestion = "Check your internet connection and try again. If persistent, there may be connectivity issues to OpenAI's servers."
	case llm.CategoryCancelled:
		llmError.Suggestion = "The operation was interrupted. Try again with a longer timeout if needed."
	case llm.CategoryInputLimit:
		llmError.Suggestion = "Reduce the input size by using --include, --exclude, or --exclude-names flags to filter the context."
	case llm.CategoryContentFiltered:
		llmError.Suggestion = "Your prompt or content may have triggered safety filters. Review and modify your input to comply with content policies."
	}

	return llmError
}

// CreateAPIError creates a new LLMError with OpenAI-specific settings
func CreateAPIError(category llm.ErrorCategory, errMsg string, originalErr error, details string) *llm.LLMError {
	llmError := llm.New(
		"openai",    // Provider
		"",          // Code
		0,           // StatusCode
		errMsg,      // Message
		"",          // RequestID
		originalErr, // Original error
		category,    // Error category
	)

	// Add details if provided
	if details != "" {
		llmError.Details = details
	}

	// Add OpenAI-specific suggestions
	switch category {
	case llm.CategoryAuth:
		llmError.Suggestion = "Check that your API key is valid and has not expired. Ensure environment variables are set correctly."
	case llm.CategoryRateLimit:
		llmError.Suggestion = "Wait and try again later. Consider adjusting the --max-concurrent and --rate-limit flags to limit request rate. You can also upgrade your API usage tier if this happens frequently."
	case llm.CategoryInvalidRequest:
		llmError.Suggestion = "Check the prompt format and parameters. Ensure they comply with the API requirements."
	case llm.CategoryNotFound:
		llmError.Suggestion = "Verify that the model name is correct and that the model is available in your region."
	case llm.CategoryServer:
		llmError.Suggestion = "This is typically a temporary issue. Wait a few moments and try again."
	case llm.CategoryNetwork:
		llmError.Suggestion = "Check your internet connection and try again. If persistent, there may be connectivity issues to OpenAI's servers."
	case llm.CategoryCancelled:
		llmError.Suggestion = "The operation was interrupted. Try again with a longer timeout if needed."
	case llm.CategoryInputLimit:
		llmError.Suggestion = "Reduce the input size by using --include, --exclude, or --exclude-names flags to filter the context."
	case llm.CategoryContentFiltered:
		llmError.Suggestion = "Your prompt or content may have triggered safety filters. Review and modify your input to comply with content policies."
	default:
		llmError.Suggestion = "Check the logs for more details or try again."
	}

	return llmError
}

// MockAPIErrorResponse creates a mock error response for testing
// This function maintains backward compatibility with tests
func MockAPIErrorResponse(errorType int, statusCode int, message string, details string) *llm.LLMError {
	// Map old ErrorType values to new categories
	var category llm.ErrorCategory
	switch errorType {
	case 1: // ErrorTypeAuth
		category = llm.CategoryAuth
	case 2: // ErrorTypeRateLimit
		category = llm.CategoryRateLimit
	case 3: // ErrorTypeInvalidRequest
		category = llm.CategoryInvalidRequest
	case 4: // ErrorTypeNotFound
		category = llm.CategoryNotFound
	case 5: // ErrorTypeServer
		category = llm.CategoryServer
	case 6: // ErrorTypeNetwork
		category = llm.CategoryNetwork
	case 7: // ErrorTypeCancelled
		category = llm.CategoryCancelled
	case 8: // ErrorTypeInputLimit
		category = llm.CategoryInputLimit
	case 9: // ErrorTypeContentFiltered
		category = llm.CategoryContentFiltered
	default:
		category = llm.CategoryUnknown
	}

	// Create and return error with appropriate suggestion based on category
	llmError := CreateAPIError(category, message, nil, details)
	llmError.StatusCode = statusCode
	return llmError
}
