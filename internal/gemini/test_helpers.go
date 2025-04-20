// Package gemini provides helpers for testing the Gemini client
package gemini

import (
	"github.com/phrazzld/architect/internal/llm"
)

// For backward compatibility in tests
const (
	// ErrorTypeUnknown represents an unknown or uncategorized error
	ErrorTypeUnknown = 0

	// ErrorTypeAuth represents authentication and authorization errors
	ErrorTypeAuth = 1

	// ErrorTypeRateLimit represents rate limiting or quota errors
	ErrorTypeRateLimit = 2

	// ErrorTypeInvalidRequest represents invalid request errors
	ErrorTypeInvalidRequest = 3

	// ErrorTypeNotFound represents model not found errors
	ErrorTypeNotFound = 4

	// ErrorTypeServer represents server errors
	ErrorTypeServer = 5

	// ErrorTypeNetwork represents network connectivity errors
	ErrorTypeNetwork = 6

	// ErrorTypeCancelled represents cancelled context errors
	ErrorTypeCancelled = 7

	// ErrorTypeInputLimit represents input token limit exceeded errors
	ErrorTypeInputLimit = 8

	// ErrorTypeContentFiltered represents content filtered by safety settings errors
	ErrorTypeContentFiltered = 9
)

// APIError is kept for backward compatibility in tests
type APIError = llm.LLMError

// Map old ErrorType values to new ErrorCategory values
var errorTypeToCategory = map[int]llm.ErrorCategory{
	ErrorTypeUnknown:         llm.CategoryUnknown,
	ErrorTypeAuth:            llm.CategoryAuth,
	ErrorTypeRateLimit:       llm.CategoryRateLimit,
	ErrorTypeInvalidRequest:  llm.CategoryInvalidRequest,
	ErrorTypeNotFound:        llm.CategoryNotFound,
	ErrorTypeServer:          llm.CategoryServer,
	ErrorTypeNetwork:         llm.CategoryNetwork,
	ErrorTypeCancelled:       llm.CategoryCancelled,
	ErrorTypeInputLimit:      llm.CategoryInputLimit,
	ErrorTypeContentFiltered: llm.CategoryContentFiltered,
}

// GetErrorType returns the legacy error type constant for a given LLMError
func GetErrorType(e *llm.LLMError) int {
	// Map the Category to the old ErrorType
	category := e.Category()
	for oldType, newCategory := range errorTypeToCategory {
		if category == newCategory {
			return oldType
		}
	}
	return ErrorTypeUnknown
}
