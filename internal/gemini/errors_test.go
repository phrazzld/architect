// Package gemini provides a client for interacting with Google's Gemini API
package gemini

import (
	"errors"
	"fmt"
	"net/http"
	"strings"
	"testing"

	"github.com/phrazzld/architect/internal/llm"
)

// testError is a simple error for testing purposes
type testError struct {
	message string
}

func (e testError) Error() string {
	return e.message
}

// newTestError creates a new test error with the given message
func newTestError(message string) error {
	return testError{message: message}
}

// TestLLMError_Error tests the Error method of LLMError
func TestLLMError_Error(t *testing.T) {
	tests := []struct {
		name     string
		llmError *llm.LLMError
		want     string
	}{
		{
			name: "with original error",
			llmError: CreateAPIError(
				llm.CategoryInvalidRequest,
				"Test error message",
				newTestError("original error"),
				"Test details",
			),
			want: "Test error message: original error",
		},
		{
			name: "without original error",
			llmError: CreateAPIError(
				llm.CategoryUnknown,
				"Error without original",
				nil,
				"",
			),
			want: "Error without original",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := tt.llmError.Error()
			if !strings.Contains(got, tt.want) {
				t.Errorf("LLMError.Error() = %q, want to contain %q", got, tt.want)
			}
		})
	}
}

// TestLLMError_Unwrap tests the Unwrap method of LLMError
func TestLLMError_Unwrap(t *testing.T) {
	originalErr := newTestError("original error")
	tests := []struct {
		name     string
		llmError *llm.LLMError
		want     error
	}{
		{
			name: "with original error",
			llmError: CreateAPIError(
				llm.CategoryInvalidRequest,
				"Test error message",
				originalErr,
				"",
			),
			want: originalErr,
		},
		{
			name: "without original error",
			llmError: CreateAPIError(
				llm.CategoryInvalidRequest,
				"Test error message",
				nil,
				"",
			),
			want: nil,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := tt.llmError.Unwrap()
			if got != tt.want {
				t.Errorf("LLMError.Unwrap() = %v, want %v", got, tt.want)
			}
		})
	}
}

// TestLLMError_UserFacingError tests the UserFacingError method of LLMError
func TestLLMError_UserFacingError(t *testing.T) {
	tests := []struct {
		name     string
		llmError *llm.LLMError
		want     string
	}{
		{
			name: "with suggestion",
			llmError: CreateAPIError(
				llm.CategoryInvalidRequest,
				"Test error message",
				newTestError("original error"),
				"",
			),
			want: "Test error message",
		},
		{
			name: "without suggestion",
			llmError: CreateAPIError(
				llm.CategoryInvalidRequest,
				"Error without suggestion",
				nil,
				"",
			),
			want: "Error without suggestion",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := tt.llmError.UserFacingError()
			if !strings.Contains(got, tt.want) {
				t.Errorf("LLMError.UserFacingError() = %q, want to contain %q", got, tt.want)
			}
		})
	}
}

// TestLLMError_DebugInfo tests the DebugInfo method of LLMError
func TestLLMError_DebugInfo(t *testing.T) {
	t.Run("full debug info", func(t *testing.T) {
		llmErr := CreateAPIError(
			llm.CategoryInvalidRequest,
			"Test error message",
			newTestError("original error"),
			"Test details",
		)
		info := llmErr.DebugInfo()

		// Check that all fields are included in the debug info
		expectedParts := []string{
			"Error Category",
			"Message: Test error message",
			"Original Error:",
			"Details:",
		}

		for _, part := range expectedParts {
			if !strings.Contains(info, part) {
				t.Errorf("LLMError.DebugInfo() missing %q", part)
			}
		}
	})

	t.Run("with partial fields", func(t *testing.T) {
		llmErr := CreateAPIError(
			llm.CategoryAuth,
			"Partial debug info",
			nil,
			"",
		)
		info := llmErr.DebugInfo()

		// These should be included
		shouldContain := []string{
			"Error Category",
			"Message: Partial debug info",
		}

		for _, part := range shouldContain {
			if !strings.Contains(info, part) {
				t.Errorf("LLMError.DebugInfo() missing %q", part)
			}
		}
	})
}

// TestIsGeminiError tests the IsGeminiError function
func TestIsGeminiError(t *testing.T) {
	tests := []struct {
		name         string
		err          error
		wantErr      *llm.LLMError
		wantIsGemini bool
	}{
		{
			name:         "nil error",
			err:          nil,
			wantErr:      nil,
			wantIsGemini: false,
		},
		{
			name:         "regular error",
			err:          newTestError("regular error"),
			wantErr:      nil,
			wantIsGemini: false,
		},
		{
			name: "Gemini LLMError",
			err: CreateAPIError(
				llm.CategoryInvalidRequest,
				"Test error message",
				newTestError("original error"),
				"",
			),
			wantErr:      nil, // Not checking the exact error as we just want to verify it's detected
			wantIsGemini: true,
		},
		{
			name: "wrapped Gemini LLMError",
			err: fmt.Errorf("wrapped: %w", CreateAPIError(
				llm.CategoryAuth,
				"wrapped LLMError",
				nil,
				"",
			)),
			wantIsGemini: true,
		},
		{
			name: "other provider LLMError",
			err: llm.New(
				"openai",
				"",
				0,
				"not a gemini error",
				"",
				nil,
				llm.CategoryUnknown,
			),
			wantIsGemini: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			gotErr, gotIsGemini := IsGeminiError(tt.err)

			if gotIsGemini != tt.wantIsGemini {
				t.Errorf("IsGeminiError() isGemini = %v, want %v", gotIsGemini, tt.wantIsGemini)
			}

			if tt.wantIsGemini {
				if gotErr == nil {
					t.Errorf("IsGeminiError() returned nil LLMError when expected non-nil")
					return
				}
				if gotErr.Provider != "gemini" {
					t.Errorf("IsGeminiError() returned LLMError with Provider = %q, want 'gemini'", gotErr.Provider)
				}
			} else {
				if gotErr != nil {
					t.Errorf("IsGeminiError() returned non-nil LLMError when expected nil")
				}
			}
		})
	}
}

// TestGetErrorCategory tests the getErrorCategory function
func TestGetErrorCategory(t *testing.T) {
	tests := []struct {
		name       string
		err        error
		statusCode int
		want       llm.ErrorCategory
	}{
		{
			name:       "nil error",
			err:        nil,
			statusCode: 0,
			want:       llm.CategoryUnknown,
		},
		// Status code based classification
		{
			name:       "unauthorized status",
			err:        errors.New("any error"),
			statusCode: http.StatusUnauthorized,
			want:       llm.CategoryAuth,
		},
		{
			name:       "forbidden status",
			err:        errors.New("any error"),
			statusCode: http.StatusForbidden,
			want:       llm.CategoryAuth,
		},
		{
			name:       "too many requests status",
			err:        errors.New("any error"),
			statusCode: http.StatusTooManyRequests,
			want:       llm.CategoryRateLimit,
		},
		{
			name:       "bad request status",
			err:        errors.New("any error"),
			statusCode: http.StatusBadRequest,
			want:       llm.CategoryInvalidRequest,
		},
		{
			name:       "not found status",
			err:        errors.New("any error"),
			statusCode: http.StatusNotFound,
			want:       llm.CategoryNotFound,
		},
		{
			name:       "server error status",
			err:        errors.New("any error"),
			statusCode: http.StatusInternalServerError,
			want:       llm.CategoryServer,
		},

		// Message based classification
		{
			name:       "rate limit in message",
			err:        errors.New("rate limit exceeded"),
			statusCode: 0,
			want:       llm.CategoryRateLimit,
		},
		{
			name:       "quota in message",
			err:        errors.New("quota exceeded"),
			statusCode: 0,
			want:       llm.CategoryRateLimit,
		},
		{
			name:       "safety in message",
			err:        errors.New("safety setting triggered"),
			statusCode: 0,
			want:       llm.CategoryContentFiltered,
		},
		{
			name:       "blocked in message",
			err:        errors.New("content blocked"),
			statusCode: 0,
			want:       llm.CategoryContentFiltered,
		},
		{
			name:       "filtered in message",
			err:        errors.New("content filtered"),
			statusCode: 0,
			want:       llm.CategoryContentFiltered,
		},
		{
			name:       "token limit in message",
			err:        errors.New("token limit exceeded"),
			statusCode: 0,
			want:       llm.CategoryInputLimit,
		},
		{
			name:       "tokens exceeds in message",
			err:        errors.New("tokens exceeds limit"),
			statusCode: 0,
			want:       llm.CategoryInputLimit,
		},
		{
			name:       "network in message",
			err:        errors.New("network error"),
			statusCode: 0,
			want:       llm.CategoryNetwork,
		},
		{
			name:       "connection in message",
			err:        errors.New("connection failed"),
			statusCode: 0,
			want:       llm.CategoryNetwork,
		},
		{
			name:       "timeout in message",
			err:        errors.New("timeout occurred"),
			statusCode: 0,
			want:       llm.CategoryNetwork,
		},
		{
			name:       "canceled in message",
			err:        errors.New("request canceled"),
			statusCode: 0,
			want:       llm.CategoryCancelled,
		},
		{
			name:       "cancelled in message (UK spelling)",
			err:        errors.New("request cancelled"),
			statusCode: 0,
			want:       llm.CategoryCancelled,
		},
		{
			name:       "deadline exceeded in message",
			err:        errors.New("deadline exceeded"),
			statusCode: 0,
			want:       llm.CategoryCancelled,
		},
		{
			name:       "unknown error",
			err:        errors.New("unknown error"),
			statusCode: 0,
			want:       llm.CategoryUnknown,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := getErrorCategory(tt.err, tt.statusCode)
			if got != tt.want {
				t.Errorf("getErrorCategory() = %v, want %v", got, tt.want)
			}
		})
	}
}

// TestLLMError_IsCategorizedError tests that LLMError can be detected as a CategorizedError
func TestLLMError_IsCategorizedError(t *testing.T) {
	tests := []struct {
		name      string
		err       error
		wantCat   llm.ErrorCategory
		wantIsCat bool
	}{
		{
			name:      "nil error",
			err:       nil,
			wantCat:   llm.CategoryUnknown,
			wantIsCat: false,
		},
		{
			name:      "regular error",
			err:       newTestError("regular error"),
			wantCat:   llm.CategoryUnknown,
			wantIsCat: false,
		},
		{
			name: "auth error",
			err: CreateAPIError(
				llm.CategoryAuth,
				"auth error",
				nil,
				"",
			),
			wantCat:   llm.CategoryAuth,
			wantIsCat: true,
		},
		{
			name: "rate limit error",
			err: CreateAPIError(
				llm.CategoryRateLimit,
				"rate limit error",
				nil,
				"",
			),
			wantCat:   llm.CategoryRateLimit,
			wantIsCat: true,
		},
		{
			name: "wrapped LLMError",
			err: fmt.Errorf("wrapped: %w", CreateAPIError(
				llm.CategoryAuth,
				"auth error",
				nil,
				"",
			)),
			wantCat:   llm.CategoryAuth,
			wantIsCat: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			catErr, isCat := llm.IsCategorizedError(tt.err)

			if isCat != tt.wantIsCat {
				t.Errorf("llm.IsCategorizedError() = %v, want %v", isCat, tt.wantIsCat)
			}

			if !isCat {
				if catErr != nil {
					t.Errorf("llm.IsCategorizedError() returned non-nil when isCat is false")
				}
				return
			}

			got := catErr.Category()
			if got != tt.wantCat {
				t.Errorf("catErr.Category() = %v, want %v", got, tt.wantCat)
			}
		})
	}
}

// TestCreateAPIError tests the CreateAPIError function
func TestCreateAPIError(t *testing.T) {
	tests := []struct {
		name                   string
		category               llm.ErrorCategory
		message                string
		originalErr            error
		details                string
		wantProvider           string
		wantSuggestionContains string
	}{
		{
			name:                   "auth error",
			category:               llm.CategoryAuth,
			message:                "Authentication failed",
			originalErr:            nil,
			details:                "",
			wantProvider:           "gemini",
			wantSuggestionContains: "API key",
		},
		{
			name:                   "rate limit error",
			category:               llm.CategoryRateLimit,
			message:                "Rate limit exceeded",
			originalErr:            nil,
			details:                "Test details",
			wantProvider:           "gemini",
			wantSuggestionContains: "rate-limit",
		},
		{
			name:                   "with original error",
			category:               llm.CategoryInvalidRequest,
			message:                "Invalid request",
			originalErr:            newTestError("original error"),
			details:                "Test details",
			wantProvider:           "gemini",
			wantSuggestionContains: "API requirements",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := CreateAPIError(tt.category, tt.message, tt.originalErr, tt.details)

			// Check basic properties
			if result.Provider != tt.wantProvider {
				t.Errorf("CreateAPIError().Provider = %q, want %q", result.Provider, tt.wantProvider)
			}

			if result.Message != tt.message {
				t.Errorf("CreateAPIError().Message = %q, want %q", result.Message, tt.message)
			}

			if result.Original != tt.originalErr {
				t.Errorf("CreateAPIError().Original = %v, want %v", result.Original, tt.originalErr)
			}

			if result.Details != tt.details {
				t.Errorf("CreateAPIError().Details = %q, want %q", result.Details, tt.details)
			}

			if result.ErrorCategory != tt.category {
				t.Errorf("CreateAPIError().ErrorCategory = %v, want %v", result.ErrorCategory, tt.category)
			}

			// Check that suggestion is set correctly
			if !strings.Contains(result.Suggestion, tt.wantSuggestionContains) {
				t.Errorf("CreateAPIError().Suggestion = %q, want to contain %q", result.Suggestion, tt.wantSuggestionContains)
			}
		})
	}
}

// TestFormatAPIError tests the FormatAPIError function
func TestFormatAPIError(t *testing.T) {
	tests := []struct {
		name           string
		err            error
		statusCode     int
		wantCategory   llm.ErrorCategory
		wantMessage    string
		wantSuggestion string
		wantNil        bool
	}{
		{
			name:    "nil error",
			err:     nil,
			wantNil: true,
		},
		{
			name: "already an LLMError",
			err: CreateAPIError(
				llm.CategoryAuth,
				"Original message",
				errors.New("original"),
				"",
			),
			statusCode:   0, // Should be ignored
			wantCategory: llm.CategoryAuth,
			wantMessage:  "Original message",
		},
		// Test each error category to ensure the correct formatting is applied
		{
			name:         "auth error",
			err:          errors.New("some auth error"),
			statusCode:   http.StatusUnauthorized,
			wantCategory: llm.CategoryAuth,
			wantMessage:  "Authentication failed with the Gemini API",
		},
		{
			name:         "rate limit error by status",
			err:          errors.New("some rate limit error"),
			statusCode:   http.StatusTooManyRequests,
			wantCategory: llm.CategoryRateLimit,
			wantMessage:  "Request rate limit or quota exceeded on the Gemini API",
		},
		{
			name:         "rate limit error by message",
			err:          errors.New("rate limit exceeded"),
			statusCode:   http.StatusOK, // Status code doesn't indicate rate limiting
			wantCategory: llm.CategoryRateLimit,
			wantMessage:  "Request rate limit or quota exceeded on the Gemini API",
		},
		{
			name:         "invalid request error",
			err:          errors.New("some invalid request"),
			statusCode:   http.StatusBadRequest,
			wantCategory: llm.CategoryInvalidRequest,
			wantMessage:  "Invalid request sent to the Gemini API",
		},
		{
			name:         "not found error",
			err:          errors.New("model not found"),
			statusCode:   http.StatusNotFound,
			wantCategory: llm.CategoryNotFound,
			wantMessage:  "The requested model or resource was not found",
		},
		{
			name:         "server error",
			err:          errors.New("internal server error"),
			statusCode:   http.StatusInternalServerError,
			wantCategory: llm.CategoryServer,
			wantMessage:  "Gemini API server error occurred",
		},
		{
			name:         "network error",
			err:          errors.New("network error occurred"),
			statusCode:   0,
			wantCategory: llm.CategoryNetwork,
			wantMessage:  "Network error while connecting to the Gemini API",
		},
		{
			name:         "cancelled error",
			err:          errors.New("request cancelled"),
			statusCode:   0,
			wantCategory: llm.CategoryCancelled,
			wantMessage:  "Request to Gemini API was cancelled",
		},
		{
			name:         "input limit error",
			err:          errors.New("token limit exceeded"),
			statusCode:   0,
			wantCategory: llm.CategoryInputLimit,
			wantMessage:  "Input token limit exceeded for the Gemini model",
		},
		{
			name:         "content filtered error",
			err:          errors.New("content blocked by safety settings"),
			statusCode:   0,
			wantCategory: llm.CategoryContentFiltered,
			wantMessage:  "Content was filtered by Gemini API safety settings",
		},
		{
			name:         "unknown error",
			err:          errors.New("some unknown error"),
			statusCode:   0,
			wantCategory: llm.CategoryUnknown,
			wantMessage:  "Error calling Gemini API: some unknown error",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := FormatAPIError(tt.err, tt.statusCode)

			if tt.wantNil {
				if result != nil {
					t.Errorf("FormatAPIError() = %v, want nil", result)
				}
				return
			}

			if result == nil {
				t.Fatal("FormatAPIError() returned nil, want non-nil")
			}

			// For already LLMError cases, check the error is returned as-is
			if _, ok := tt.err.(*llm.LLMError); ok {
				// For existing LLMError, the function should return the same object
				if result != tt.err {
					t.Errorf("FormatAPIError() should return the same LLMError instance for an existing LLMError")
				}
				return
			}

			// For regular errors, check error properties
			// Check error category
			if result.ErrorCategory != tt.wantCategory {
				t.Errorf("FormatAPIError().ErrorCategory = %v, want %v", result.ErrorCategory, tt.wantCategory)
			}

			// Check message
			if !strings.Contains(result.Message, tt.wantMessage) {
				t.Errorf("FormatAPIError().Message = %q, want to contain %q", result.Message, tt.wantMessage)
			}

			// Check provider
			if result.Provider != "gemini" {
				t.Errorf("FormatAPIError().Provider = %q, want \"gemini\"", result.Provider)
			}

			// Check that original error is preserved
			if result.Original == nil {
				t.Errorf("FormatAPIError().Original is nil, expected non-nil")
			}
		})
	}
}

// TestIsAPIError tests backward compatibility with IsAPIError
func TestIsAPIError(t *testing.T) {
	// Create a test error
	testErr := CreateAPIError(
		llm.CategoryInvalidRequest,
		"test error",
		nil,
		"",
	)

	// Test the backward compatibility function
	llmErr, ok := IsAPIError(testErr)

	if !ok {
		t.Errorf("IsAPIError() returned false, want true")
	}

	if llmErr == nil {
		t.Errorf("IsAPIError() returned nil, want non-nil")
		return
	}

	if llmErr.Provider != "gemini" {
		t.Errorf("IsAPIError() returned LLMError with Provider = %q, want \"gemini\"", llmErr.Provider)
	}
}
