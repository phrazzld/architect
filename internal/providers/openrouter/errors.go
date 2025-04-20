// Package openrouter provides the implementation of the OpenRouter LLM provider
package openrouter

import (
	"encoding/json"
	"errors"
	"fmt"
	"strings"

	"github.com/phrazzld/architect/internal/llm"
)

// Registry-related errors
var (
	ErrRegistryNotAvailable = errors.New("registry not available")
)

// APIErrorResponse represents the error structure returned by the OpenRouter API
type APIErrorResponse struct {
	Error APIErrorDetail `json:"error"`
}

// APIErrorDetail contains the details of an API error returned by OpenRouter
type APIErrorDetail struct {
	Code    interface{} `json:"code"` // Can be string or int
	Message string      `json:"message"`
	Type    string      `json:"type,omitempty"`
	Param   string      `json:"param,omitempty"`
}

// IsOpenRouterError checks if an error is an llm.LLMError originating from OpenRouter
func IsOpenRouterError(err error) (*llm.LLMError, bool) {
	var llmErr *llm.LLMError
	if errors.As(err, &llmErr) && llmErr.Provider == "openrouter" {
		return llmErr, true
	}
	return nil, false
}

// ParseErrorResponse parses the OpenRouter API error response body
// This function returns the detailed error message extracted from the API response
func ParseErrorResponse(responseBody []byte) (string, string, string) {
	if len(responseBody) == 0 {
		return "", "", ""
	}

	var apiErrorResp APIErrorResponse
	if err := json.Unmarshal(responseBody, &apiErrorResp); err != nil {
		return "", "", ""
	}

	var errorMessage, errorType, errorParam string

	if apiErrorResp.Error.Message != "" {
		errorMessage = apiErrorResp.Error.Message
	}

	if apiErrorResp.Error.Type != "" {
		errorType = apiErrorResp.Error.Type
	}

	if apiErrorResp.Error.Param != "" {
		errorParam = apiErrorResp.Error.Param
	}

	return errorMessage, errorType, errorParam
}

// FormatErrorDetails formats detailed error information from OpenRouter API response
func FormatErrorDetails(errorMessage string, errorType string, errorParam string) string {
	if errorMessage == "" {
		return ""
	}

	details := fmt.Sprintf("API Error: %s", errorMessage)
	if errorType != "" {
		details += fmt.Sprintf(" (Type: %s)", errorType)
	}
	if errorParam != "" {
		details += fmt.Sprintf(" (Param: %s)", errorParam)
	}

	return details
}

// FormatAPIError creates a standardized LLMError from an OpenRouter API error
func FormatAPIError(err error, statusCode int, responseBody []byte) *llm.LLMError {
	if err == nil {
		return nil
	}

	// Check if it's already an LLMError
	var llmErr *llm.LLMError
	if errors.As(err, &llmErr) {
		return llmErr
	}

	// Parse error details from response body if available
	errorMessage, errorType, errorParam := ParseErrorResponse(responseBody)
	errorDetails := FormatErrorDetails(errorMessage, errorType, errorParam)

	// We'll use the shared library error categorization instead of determining a message
	// from the API response - the CreateStandardErrorWithMessage function handles this

	// Try to categorize error from specific OpenRouter error types
	category := llm.CategoryUnknown

	if errorType != "" {
		if strings.Contains(errorType, "auth") ||
			strings.Contains(errorType, "authentication") ||
			strings.Contains(errorType, "unauthorized") {
			category = llm.CategoryAuth
		} else if strings.Contains(errorType, "rate_limit") {
			category = llm.CategoryRateLimit
		} else if strings.Contains(errorType, "invalid_request") {
			category = llm.CategoryInvalidRequest
		} else if strings.Contains(errorType, "not_found") {
			category = llm.CategoryNotFound
		} else if strings.Contains(errorType, "server_error") {
			category = llm.CategoryServer
		}
	}

	// If we couldn't categorize based on OpenRouter specific info,
	// use the shared categorization logic
	if category == llm.CategoryUnknown {
		category = llm.DetectErrorCategory(err, statusCode)
	}

	// Create a formatted error message with OpenRouter specific suggestions
	llmError := llm.CreateStandardErrorWithMessage("openrouter", category, err, errorDetails)

	// Add OpenRouter-specific suggestions for certain error types
	switch category {
	case llm.CategoryAuth:
		llmError.Suggestion = "Check that your OpenRouter API key is valid and has not expired. Ensure OPENROUTER_API_KEY environment variable is set correctly."
	case llm.CategoryInsufficientCredits:
		llmError.Suggestion = "Check your OpenRouter account balance and add credits if needed. Visit https://openrouter.ai/account for account details."
	case llm.CategoryNotFound:
		llmError.Suggestion = "Verify that the model name is correct and uses the format 'provider/model' or 'provider/organization/model'."
	case llm.CategoryServer:
		llmError.Suggestion = "This is typically a temporary issue with OpenRouter or the underlying model provider. Wait a few moments and try again."
	case llm.CategoryNetwork:
		llmError.Suggestion = "Check your internet connection and try again. If persistent, there may be connectivity issues to OpenRouter's servers."
	}

	return llmError
}

// CreateAPIError creates a new LLMError with OpenRouter-specific settings
func CreateAPIError(category llm.ErrorCategory, errMsg string, originalErr error, details string) *llm.LLMError {
	llmError := llm.New(
		"openrouter", // Provider
		"",           // Code
		0,            // StatusCode
		errMsg,       // Message
		"",           // RequestID
		originalErr,  // Original error
		category,     // Error category
	)

	// Add details if provided
	if details != "" {
		llmError.Details = details
	}

	// Add OpenRouter-specific suggestions
	switch category {
	case llm.CategoryAuth:
		llmError.Suggestion = "Check that your OpenRouter API key is valid and has not expired. Ensure OPENROUTER_API_KEY environment variable is set correctly."
	case llm.CategoryRateLimit:
		llmError.Suggestion = "Wait and try again later. Consider adjusting the --max-concurrent and --rate-limit flags to limit request rate."
	case llm.CategoryInsufficientCredits:
		llmError.Suggestion = "Check your OpenRouter account balance and add credits if needed. Visit https://openrouter.ai/account for account details."
	case llm.CategoryInvalidRequest:
		llmError.Suggestion = "Check the prompt format and parameters. Ensure they comply with the API requirements."
	case llm.CategoryNotFound:
		llmError.Suggestion = "Verify that the model name is correct and uses the format 'provider/model' or 'provider/organization/model'."
	case llm.CategoryServer:
		llmError.Suggestion = "This is typically a temporary issue with OpenRouter or the underlying model provider. Wait a few moments and try again."
	case llm.CategoryNetwork:
		llmError.Suggestion = "Check your internet connection and try again. If persistent, there may be connectivity issues to OpenRouter's servers."
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
