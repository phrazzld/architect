// Package llm provides a common interface for interacting with various LLM providers
package llm

import (
	"errors"
	"fmt"
	"net/http"
	"strings"
)

// ErrorCategory represents different categories of errors that can occur when using LLM APIs
type ErrorCategory int

const (
	// CategoryUnknown represents an unknown or uncategorized error
	CategoryUnknown ErrorCategory = iota
	// CategoryAuth represents authentication and authorization errors
	CategoryAuth
	// CategoryRateLimit represents rate limiting or quota errors
	CategoryRateLimit
	// CategoryInvalidRequest represents invalid request errors
	CategoryInvalidRequest
	// CategoryNotFound represents model not found errors
	CategoryNotFound
	// CategoryServer represents server errors
	CategoryServer
	// CategoryNetwork represents network connectivity errors
	CategoryNetwork
	// CategoryCancelled represents cancelled context errors
	CategoryCancelled
	// CategoryInputLimit represents input token limit exceeded errors
	CategoryInputLimit
	// CategoryContentFiltered represents content filtered by safety settings errors
	CategoryContentFiltered
	// CategoryInsufficientCredits represents insufficient credits or payment required errors
	CategoryInsufficientCredits
)

// String returns a string representation of the ErrorCategory
func (c ErrorCategory) String() string {
	switch c {
	case CategoryAuth:
		return "Auth"
	case CategoryRateLimit:
		return "RateLimit"
	case CategoryInvalidRequest:
		return "InvalidRequest"
	case CategoryNotFound:
		return "NotFound"
	case CategoryServer:
		return "Server"
	case CategoryNetwork:
		return "Network"
	case CategoryCancelled:
		return "Cancelled"
	case CategoryInputLimit:
		return "InputLimit"
	case CategoryContentFiltered:
		return "ContentFiltered"
	case CategoryInsufficientCredits:
		return "InsufficientCredits"
	default:
		return "Unknown"
	}
}

// CategorizedError is an interface that extends error with the ability
// to provide error category information for more specific error handling.
type CategorizedError interface {
	error // Embed standard error interface
	Category() ErrorCategory
}

// IsCategorizedError checks if an error implements the CategorizedError interface
// and returns the implementing error and true if it does, or nil and false if not.
func IsCategorizedError(err error) (CategorizedError, bool) {
	var catErr CategorizedError
	if err == nil {
		return nil, false
	}
	if errors.As(err, &catErr) {
		return catErr, true
	}
	return nil, false
}

// LLMError represents a standardized error from any LLM provider
type LLMError struct {
	// Provider is the name of the provider that generated the error
	Provider string

	// Code is the provider-specific error code (if available)
	Code string

	// StatusCode is the HTTP status code (if applicable)
	StatusCode int

	// Message is a user-friendly error message
	Message string

	// RequestID is the ID of the request that failed (if available)
	RequestID string

	// Original is the underlying error that caused this error
	Original error

	// Category is the standardized error category
	ErrorCategory ErrorCategory

	// Suggestion is a helpful suggestion for resolving the error
	Suggestion string

	// Details contains additional error details
	Details string
}

// Error implements the error interface
func (e *LLMError) Error() string {
	if e.Original != nil {
		return fmt.Sprintf("%s: %v", e.Message, e.Original)
	}
	return e.Message
}

// Unwrap returns the original error
func (e *LLMError) Unwrap() error {
	return e.Original
}

// Category implements the CategorizedError interface
func (e *LLMError) Category() ErrorCategory {
	return e.ErrorCategory
}

// UserFacingError returns a user-friendly error message with suggestions
func (e *LLMError) UserFacingError() string {
	var sb strings.Builder

	// Start with the error message
	if e.Original != nil {
		sb.WriteString(fmt.Sprintf("%s: %v", e.Message, e.Original))
	} else {
		sb.WriteString(e.Message)
	}

	// Add suggestions if available
	if e.Suggestion != "" {
		sb.WriteString("\n\nSuggestion: ")
		sb.WriteString(e.Suggestion)
	}

	return sb.String()
}

// DebugInfo returns detailed debugging information about the error
func (e *LLMError) DebugInfo() string {
	var sb strings.Builder

	sb.WriteString(fmt.Sprintf("Provider: %s\n", e.Provider))
	sb.WriteString(fmt.Sprintf("Error Category: %v\n", e.ErrorCategory))
	sb.WriteString(fmt.Sprintf("Message: %s\n", e.Message))

	if e.Code != "" {
		sb.WriteString(fmt.Sprintf("Error Code: %s\n", e.Code))
	}

	if e.StatusCode != 0 {
		sb.WriteString(fmt.Sprintf("Status Code: %d\n", e.StatusCode))
	}

	if e.RequestID != "" {
		sb.WriteString(fmt.Sprintf("Request ID: %s\n", e.RequestID))
	}

	if e.Original != nil {
		sb.WriteString(fmt.Sprintf("Original Error: %v\n", e.Original))
	}

	if e.Details != "" {
		sb.WriteString(fmt.Sprintf("Details: %s\n", e.Details))
	}

	if e.Suggestion != "" {
		sb.WriteString(fmt.Sprintf("Suggestion: %s\n", e.Suggestion))
	}

	return sb.String()
}

// New creates a new LLMError with the specified parameters
func New(provider string, code string, statusCode int, message string, requestID string, original error, category ErrorCategory) *LLMError {
	return &LLMError{
		Provider:      provider,
		Code:          code,
		StatusCode:    statusCode,
		Message:       message,
		RequestID:     requestID,
		Original:      original,
		ErrorCategory: category,
	}
}

// Wrap wraps an existing error with additional LLM-specific context
func Wrap(err error, provider string, message string, category ErrorCategory) *LLMError {
	if err == nil {
		return nil
	}

	// If it's already an LLMError, we can update it
	var llmErr *LLMError
	if errors.As(err, &llmErr) {
		if message != "" {
			llmErr.Message = message
		}
		if provider != "" {
			llmErr.Provider = provider
		}
		if category != CategoryUnknown {
			llmErr.ErrorCategory = category
		}
		return llmErr
	}

	// Otherwise create a new LLMError
	return &LLMError{
		Provider:      provider,
		Message:       message,
		Original:      err,
		ErrorCategory: category,
	}
}

// IsCategory checks if an error belongs to a specific category
func IsCategory(err error, category ErrorCategory) bool {
	if err == nil {
		return false
	}

	// Check if the error implements CategorizedError
	if catErr, ok := IsCategorizedError(err); ok {
		return catErr.Category() == category
	}

	return false
}

// IsAuth returns true if the error is an authentication error
func IsAuth(err error) bool {
	return IsCategory(err, CategoryAuth)
}

// IsRateLimit returns true if the error is a rate limit error
func IsRateLimit(err error) bool {
	return IsCategory(err, CategoryRateLimit)
}

// IsInvalidRequest returns true if the error is an invalid request error
func IsInvalidRequest(err error) bool {
	return IsCategory(err, CategoryInvalidRequest)
}

// IsNotFound returns true if the error is a not found error
func IsNotFound(err error) bool {
	return IsCategory(err, CategoryNotFound)
}

// IsServer returns true if the error is a server error
func IsServer(err error) bool {
	return IsCategory(err, CategoryServer)
}

// IsNetwork returns true if the error is a network error
func IsNetwork(err error) bool {
	return IsCategory(err, CategoryNetwork)
}

// IsCancelled returns true if the error is a cancelled context error
func IsCancelled(err error) bool {
	return IsCategory(err, CategoryCancelled)
}

// IsInputLimit returns true if the error is an input token limit error
func IsInputLimit(err error) bool {
	return IsCategory(err, CategoryInputLimit)
}

// IsContentFiltered returns true if the error is a content filtered error
func IsContentFiltered(err error) bool {
	return IsCategory(err, CategoryContentFiltered)
}

// IsInsufficientCredits returns true if the error is an insufficient credits error
func IsInsufficientCredits(err error) bool {
	return IsCategory(err, CategoryInsufficientCredits)
}

// GetErrorCategoryFromStatusCode determines the error category based on HTTP status code
func GetErrorCategoryFromStatusCode(statusCode int) ErrorCategory {
	switch {
	case statusCode == http.StatusUnauthorized || statusCode == http.StatusForbidden:
		return CategoryAuth
	case statusCode == http.StatusPaymentRequired:
		return CategoryInsufficientCredits
	case statusCode == http.StatusTooManyRequests:
		return CategoryRateLimit
	case statusCode == http.StatusBadRequest:
		return CategoryInvalidRequest
	case statusCode == http.StatusNotFound:
		return CategoryNotFound
	case statusCode >= 500 && statusCode < 600:
		return CategoryServer
	default:
		return CategoryUnknown
	}
}

// GetErrorCategoryFromMessage tries to determine the error category based on error message
func GetErrorCategoryFromMessage(errMsg string) ErrorCategory {
	lowerMsg := strings.ToLower(errMsg)

	// Check for authorization errors
	if strings.Contains(lowerMsg, "auth") ||
		strings.Contains(lowerMsg, "unauthorized") ||
		strings.Contains(lowerMsg, "invalid key") ||
		strings.Contains(lowerMsg, "api key") {
		return CategoryAuth
	}

	// Check for insufficient credits or payment required
	if strings.Contains(lowerMsg, "credit") ||
		strings.Contains(lowerMsg, "payment") ||
		strings.Contains(lowerMsg, "billing") {
		return CategoryInsufficientCredits
	}

	// Check for rate limit or quota errors
	if strings.Contains(lowerMsg, "rate limit") ||
		strings.Contains(lowerMsg, "quota") ||
		strings.Contains(lowerMsg, "too many requests") {
		return CategoryRateLimit
	}

	// Check for content filtering
	if strings.Contains(lowerMsg, "safety") ||
		strings.Contains(lowerMsg, "content_filter") ||
		strings.Contains(lowerMsg, "blocked") ||
		strings.Contains(lowerMsg, "filtered") ||
		strings.Contains(lowerMsg, "moderation") {
		return CategoryContentFiltered
	}

	// Check for token limit errors
	if strings.Contains(lowerMsg, "token limit") ||
		strings.Contains(lowerMsg, "tokens exceeds") ||
		strings.Contains(lowerMsg, "maximum context length") {
		return CategoryInputLimit
	}

	// Check for network errors
	if strings.Contains(lowerMsg, "network") ||
		strings.Contains(lowerMsg, "connection") ||
		strings.Contains(lowerMsg, "timeout") {
		return CategoryNetwork
	}

	// Check for cancellation
	if strings.Contains(lowerMsg, "canceled") ||
		strings.Contains(lowerMsg, "cancelled") ||
		strings.Contains(lowerMsg, "deadline exceeded") {
		return CategoryCancelled
	}

	return CategoryUnknown
}

// DetectErrorCategory determines the most specific error category
// based on a combination of status code and error message
func DetectErrorCategory(err error, statusCode int) ErrorCategory {
	if err == nil {
		return CategoryUnknown
	}

	// First check if the error already implements CategorizedError
	if catErr, ok := IsCategorizedError(err); ok {
		return catErr.Category()
	}

	// Check status code first, as it's more reliable
	category := GetErrorCategoryFromStatusCode(statusCode)
	if category != CategoryUnknown {
		return category
	}

	// Fall back to message-based detection
	return GetErrorCategoryFromMessage(err.Error())
}

// CreateStandardErrorWithMessage creates an appropriate error message and suggestion based on error category
func CreateStandardErrorWithMessage(provider string, category ErrorCategory, originalErr error, details string) *LLMError {
	llmErr := &LLMError{
		Provider:      provider,
		Original:      originalErr,
		ErrorCategory: category,
		Details:       details,
	}

	switch category {
	case CategoryAuth:
		llmErr.Message = fmt.Sprintf("Authentication failed with the %s API", provider)
		llmErr.Suggestion = "Check that your API key is valid and has not expired. Ensure environment variables are set correctly."

	case CategoryRateLimit:
		llmErr.Message = fmt.Sprintf("Request rate limit exceeded on the %s API", provider)
		llmErr.Suggestion = "Wait and try again later. Consider adjusting the --max-concurrent and --rate-limit flags to limit request rate."

	case CategoryInsufficientCredits:
		llmErr.Message = fmt.Sprintf("Insufficient credits or payment required on the %s API", provider)
		llmErr.Suggestion = "Check your account balance and add credits if needed."

	case CategoryInvalidRequest:
		llmErr.Message = fmt.Sprintf("Invalid request sent to the %s API", provider)
		llmErr.Suggestion = "Check the prompt format and parameters. Ensure they comply with the API requirements."

	case CategoryNotFound:
		llmErr.Message = "The requested model or resource was not found"
		llmErr.Suggestion = "Verify that the model name is correct and is available in your region."

	case CategoryServer:
		llmErr.Message = fmt.Sprintf("%s API server error occurred", provider)
		llmErr.Suggestion = "This is typically a temporary issue. Wait a few moments and try again."

	case CategoryNetwork:
		llmErr.Message = fmt.Sprintf("Network error while connecting to the %s API", provider)
		llmErr.Suggestion = "Check your internet connection and try again."

	case CategoryCancelled:
		llmErr.Message = fmt.Sprintf("Request to %s API was cancelled", provider)
		llmErr.Suggestion = "The operation was interrupted. Try again with a longer timeout if needed."

	case CategoryInputLimit:
		llmErr.Message = "Input token limit exceeded for the selected model"
		llmErr.Suggestion = "Reduce the input size by using --include, --exclude, or --exclude-names flags to filter the context."

	case CategoryContentFiltered:
		llmErr.Message = "Content was filtered by safety settings"
		llmErr.Suggestion = "Your prompt or content may have triggered safety filters. Review and modify your input to comply with content policies."

	default:
		llmErr.Message = fmt.Sprintf("Error calling %s API: %v", provider, originalErr)
		llmErr.Suggestion = "Check the logs for more details or try again."
	}

	// If we have detailed error information, include it in the error message
	if details != "" && !strings.Contains(llmErr.Message, details) {
		llmErr.Message = fmt.Sprintf("%s (%s)", llmErr.Message, details)
	}

	return llmErr
}

// FormatAPIError creates a standardized LLMError from a generic error
func FormatAPIError(provider string, err error, statusCode int, responseBody string) *LLMError {
	if err == nil {
		return nil
	}

	// Check if it's already an LLMError
	var llmErr *LLMError
	if errors.As(err, &llmErr) {
		return llmErr
	}

	// Determine error category
	category := DetectErrorCategory(err, statusCode)

	// Create the error with standard message
	return CreateStandardErrorWithMessage(provider, category, err, responseBody)
}

// Ensure LLMError implements the CategorizedError interface (compile-time check)
var _ CategorizedError = (*LLMError)(nil)
