package llm

import (
	"errors"
	"fmt"
	"net/http"
	"strings"
	"testing"
)

// Test the String method of ErrorCategory
func TestErrorCategory_String(t *testing.T) {
	tests := []struct {
		category ErrorCategory
		expected string
	}{
		{CategoryUnknown, "Unknown"},
		{CategoryAuth, "Auth"},
		{CategoryRateLimit, "RateLimit"},
		{CategoryInvalidRequest, "InvalidRequest"},
		{CategoryNotFound, "NotFound"},
		{CategoryServer, "Server"},
		{CategoryNetwork, "Network"},
		{CategoryCancelled, "Cancelled"},
		{CategoryInputLimit, "InputLimit"},
		{CategoryContentFiltered, "ContentFiltered"},
		{CategoryInsufficientCredits, "InsufficientCredits"},
		{ErrorCategory(99), "Unknown"}, // Unknown value should return "Unknown"
	}

	for _, tc := range tests {
		t.Run(tc.expected, func(t *testing.T) {
			result := tc.category.String()
			if result != tc.expected {
				t.Errorf("Expected ErrorCategory(%d).String() to be %q, got %q", tc.category, tc.expected, result)
			}
		})
	}
}

// Simple error type implementing CategorizedError for testing
type testCategorizedError struct {
	msg      string
	category ErrorCategory
}

func (e testCategorizedError) Error() string {
	return e.msg
}

func (e testCategorizedError) Category() ErrorCategory {
	return e.category
}

// Test that IsCategorizedError correctly identifies a CategorizedError
func TestIsCategorizedError(t *testing.T) {
	tests := []struct {
		name     string
		err      error
		isCatErr bool
		category ErrorCategory
	}{
		{
			name:     "nil error",
			err:      nil,
			isCatErr: false,
		},
		{
			name:     "standard error",
			err:      errors.New("standard error"),
			isCatErr: false,
		},
		{
			name:     "categorized error",
			err:      testCategorizedError{msg: "test error", category: CategoryRateLimit},
			isCatErr: true,
			category: CategoryRateLimit,
		},
		{
			name:     "wrapped categorized error",
			err:      fmt.Errorf("wrapped: %w", testCategorizedError{msg: "inner error", category: CategoryAuth}),
			isCatErr: true,
			category: CategoryAuth,
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			catErr, ok := IsCategorizedError(tc.err)
			if ok != tc.isCatErr {
				t.Errorf("IsCategorizedError(%v) = %v, want %v", tc.err, ok, tc.isCatErr)
			}

			if ok && catErr.Category() != tc.category {
				t.Errorf("Category() = %v, want %v", catErr.Category(), tc.category)
			}
		})
	}
}

// Test LLMError creation and methods
func TestLLMError(t *testing.T) {
	// Create an LLMError
	origErr := errors.New("something went wrong")
	llmErr := &LLMError{
		Provider:      "test-provider",
		Code:          "test-code",
		StatusCode:    429,
		Message:       "Rate limit exceeded",
		RequestID:     "req-123",
		Original:      origErr,
		ErrorCategory: CategoryRateLimit,
		Suggestion:    "Wait and try again later",
		Details:       "Additional details",
	}

	// Test Error() method
	expectedErrMsg := "Rate limit exceeded: something went wrong"
	if llmErr.Error() != expectedErrMsg {
		t.Errorf("Expected Error() = %q, got %q", expectedErrMsg, llmErr.Error())
	}

	// Test Unwrap() method
	if unwrappedErr := llmErr.Unwrap(); unwrappedErr != origErr {
		t.Errorf("Expected Unwrap() = %v, got %v", origErr, unwrappedErr)
	}

	// Test Category() method
	if cat := llmErr.Category(); cat != CategoryRateLimit {
		t.Errorf("Expected Category() = %v, got %v", CategoryRateLimit, cat)
	}

	// Test UserFacingError() method
	expectedUserMsg := "Rate limit exceeded: something went wrong\n\nSuggestion: Wait and try again later"
	if userMsg := llmErr.UserFacingError(); userMsg != expectedUserMsg {
		t.Errorf("Expected UserFacingError() = %q, got %q", expectedUserMsg, userMsg)
	}

	// Test DebugInfo() method
	debugInfo := llmErr.DebugInfo()
	expectedDebugParts := []string{
		"Provider: test-provider",
		"Error Category: RateLimit",
		"Message: Rate limit exceeded",
		"Error Code: test-code",
		"Status Code: 429",
		"Request ID: req-123",
		"Original Error: something went wrong",
		"Details: Additional details",
		"Suggestion: Wait and try again later",
	}
	for _, part := range expectedDebugParts {
		if !strings.Contains(debugInfo, part) {
			t.Errorf("DebugInfo() missing expected part: %q", part)
		}
	}
}

// Test New function
func TestNew(t *testing.T) {
	origErr := errors.New("original error")
	llmErr := New("test-provider", "err-code", 400, "Bad request", "req-123", origErr, CategoryInvalidRequest)

	if llmErr.Provider != "test-provider" {
		t.Errorf("Expected Provider = %q, got %q", "test-provider", llmErr.Provider)
	}
	if llmErr.Code != "err-code" {
		t.Errorf("Expected Code = %q, got %q", "err-code", llmErr.Code)
	}
	if llmErr.StatusCode != 400 {
		t.Errorf("Expected StatusCode = %d, got %d", 400, llmErr.StatusCode)
	}
	if llmErr.Message != "Bad request" {
		t.Errorf("Expected Message = %q, got %q", "Bad request", llmErr.Message)
	}
	if llmErr.RequestID != "req-123" {
		t.Errorf("Expected RequestID = %q, got %q", "req-123", llmErr.RequestID)
	}
	if llmErr.Original != origErr {
		t.Errorf("Expected Original = %v, got %v", origErr, llmErr.Original)
	}
	if llmErr.ErrorCategory != CategoryInvalidRequest {
		t.Errorf("Expected ErrorCategory = %v, got %v", CategoryInvalidRequest, llmErr.ErrorCategory)
	}
}

// Test Wrap function
func TestWrap(t *testing.T) {
	t.Run("wrap nil error", func(t *testing.T) {
		if err := Wrap(nil, "test-provider", "wrapped message", CategoryAuth); err != nil {
			t.Errorf("Expected Wrap(nil, ...) = nil, got %v", err)
		}
	})

	t.Run("wrap standard error", func(t *testing.T) {
		stdErr := errors.New("standard error")
		wrapped := Wrap(stdErr, "test-provider", "wrapped message", CategoryAuth)

		if wrapped.Provider != "test-provider" {
			t.Errorf("Expected Provider = %q, got %q", "test-provider", wrapped.Provider)
		}
		if wrapped.Message != "wrapped message" {
			t.Errorf("Expected Message = %q, got %q", "wrapped message", wrapped.Message)
		}
		if wrapped.Original != stdErr {
			t.Errorf("Expected Original = %v, got %v", stdErr, wrapped.Original)
		}
		if wrapped.ErrorCategory != CategoryAuth {
			t.Errorf("Expected ErrorCategory = %v, got %v", CategoryAuth, wrapped.ErrorCategory)
		}
	})

	t.Run("wrap existing LLMError", func(t *testing.T) {
		existing := &LLMError{
			Provider:      "original-provider",
			Message:       "original message",
			ErrorCategory: CategoryUnknown,
		}
		wrapped := Wrap(existing, "new-provider", "new message", CategoryAuth)

		// Should modify the existing error
		if wrapped != existing {
			t.Errorf("Expected to update existing error, got new error instance")
		}
		if wrapped.Provider != "new-provider" {
			t.Errorf("Expected Provider = %q, got %q", "new-provider", wrapped.Provider)
		}
		if wrapped.Message != "new message" {
			t.Errorf("Expected Message = %q, got %q", "new message", wrapped.Message)
		}
		if wrapped.ErrorCategory != CategoryAuth {
			t.Errorf("Expected ErrorCategory = %v, got %v", CategoryAuth, wrapped.ErrorCategory)
		}
	})

	t.Run("wrap with empty fields", func(t *testing.T) {
		existing := &LLMError{
			Provider:      "original-provider",
			Message:       "original message",
			ErrorCategory: CategoryAuth,
		}
		wrapped := Wrap(existing, "", "", CategoryUnknown)

		// Should not modify these fields since the provided values are empty/unknown
		if wrapped.Provider != "original-provider" {
			t.Errorf("Expected Provider to remain %q, got %q", "original-provider", wrapped.Provider)
		}
		if wrapped.Message != "original message" {
			t.Errorf("Expected Message to remain %q, got %q", "original message", wrapped.Message)
		}
		if wrapped.ErrorCategory != CategoryAuth {
			t.Errorf("Expected ErrorCategory to remain %v, got %v", CategoryAuth, wrapped.ErrorCategory)
		}
	})
}

// Test error category helper functions (IsAuth, IsRateLimit, etc.)
func TestErrorCategoryHelpers(t *testing.T) {
	testCases := []struct {
		name     string
		err      error
		category ErrorCategory
		checkFn  func(error) bool
	}{
		{"IsAuth", &LLMError{ErrorCategory: CategoryAuth}, CategoryAuth, IsAuth},
		{"IsRateLimit", &LLMError{ErrorCategory: CategoryRateLimit}, CategoryRateLimit, IsRateLimit},
		{"IsInvalidRequest", &LLMError{ErrorCategory: CategoryInvalidRequest}, CategoryInvalidRequest, IsInvalidRequest},
		{"IsNotFound", &LLMError{ErrorCategory: CategoryNotFound}, CategoryNotFound, IsNotFound},
		{"IsServer", &LLMError{ErrorCategory: CategoryServer}, CategoryServer, IsServer},
		{"IsNetwork", &LLMError{ErrorCategory: CategoryNetwork}, CategoryNetwork, IsNetwork},
		{"IsCancelled", &LLMError{ErrorCategory: CategoryCancelled}, CategoryCancelled, IsCancelled},
		{"IsInputLimit", &LLMError{ErrorCategory: CategoryInputLimit}, CategoryInputLimit, IsInputLimit},
		{"IsContentFiltered", &LLMError{ErrorCategory: CategoryContentFiltered}, CategoryContentFiltered, IsContentFiltered},
		{"IsInsufficientCredits", &LLMError{ErrorCategory: CategoryInsufficientCredits}, CategoryInsufficientCredits, IsInsufficientCredits},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			if !tc.checkFn(tc.err) {
				t.Errorf("Expected %s(%v) = true, got false", tc.name, tc.err)
			}

			// Test that the function returns false for other error categories
			for _, otherTC := range testCases {
				if otherTC.category != tc.category {
					if tc.checkFn(&LLMError{ErrorCategory: otherTC.category}) {
						t.Errorf("Expected %s(%v) = false, got true for %v", tc.name, otherTC.category, otherTC.category)
					}
				}
			}

			// Test with nil error
			if tc.checkFn(nil) {
				t.Errorf("Expected %s(nil) = false, got true", tc.name)
			}

			// Test with standard error
			if tc.checkFn(errors.New("standard error")) {
				t.Errorf("Expected %s(standard error) = false, got true", tc.name)
			}
		})
	}
}

// Test GetErrorCategoryFromStatusCode
func TestGetErrorCategoryFromStatusCode(t *testing.T) {
	testCases := []struct {
		statusCode int
		expected   ErrorCategory
	}{
		{http.StatusOK, CategoryUnknown},
		{http.StatusUnauthorized, CategoryAuth},
		{http.StatusForbidden, CategoryAuth},
		{http.StatusPaymentRequired, CategoryInsufficientCredits},
		{http.StatusTooManyRequests, CategoryRateLimit},
		{http.StatusBadRequest, CategoryInvalidRequest},
		{http.StatusNotFound, CategoryNotFound},
		{http.StatusInternalServerError, CategoryServer},
		{http.StatusServiceUnavailable, CategoryServer},
		{0, CategoryUnknown},
	}

	for _, tc := range testCases {
		t.Run(fmt.Sprintf("StatusCode_%d", tc.statusCode), func(t *testing.T) {
			result := GetErrorCategoryFromStatusCode(tc.statusCode)
			if result != tc.expected {
				t.Errorf("Expected GetErrorCategoryFromStatusCode(%d) = %v, got %v", tc.statusCode, tc.expected, result)
			}
		})
	}
}

// Test GetErrorCategoryFromMessage
func TestGetErrorCategoryFromMessage(t *testing.T) {
	testCases := []struct {
		message  string
		expected ErrorCategory
	}{
		{"Authentication failed", CategoryAuth},
		{"Invalid API key", CategoryAuth},
		{"Unauthorized access", CategoryAuth},
		{"Rate limit exceeded", CategoryRateLimit},
		{"Too many requests", CategoryRateLimit},
		{"Quota exceeded", CategoryRateLimit},
		{"Insufficient credits", CategoryInsufficientCredits},
		{"Payment required", CategoryInsufficientCredits},
		{"Billing issue", CategoryInsufficientCredits},
		{"Safety filters blocked content", CategoryContentFiltered},
		{"Content moderation triggered", CategoryContentFiltered},
		{"Token limit exceeded", CategoryInputLimit},
		{"Maximum context length exceeded", CategoryInputLimit},
		{"Network error occurred", CategoryNetwork},
		{"Connection timeout", CategoryNetwork},
		{"Request cancelled", CategoryCancelled},
		{"Operation deadline exceeded", CategoryCancelled},
		{"Unknown error", CategoryUnknown},
	}

	for _, tc := range testCases {
		t.Run(tc.message, func(t *testing.T) {
			result := GetErrorCategoryFromMessage(tc.message)
			if result != tc.expected {
				t.Errorf("Expected GetErrorCategoryFromMessage(%q) = %v, got %v", tc.message, tc.expected, result)
			}
		})
	}

	// Test case insensitivity
	t.Run("case insensitivity", func(t *testing.T) {
		uppercaseMsg := "RATE LIMIT EXCEEDED"
		lowercaseMsg := "rate limit exceeded"

		if GetErrorCategoryFromMessage(uppercaseMsg) != GetErrorCategoryFromMessage(lowercaseMsg) {
			t.Errorf("Expected case-insensitive message matching")
		}
	})
}

// Test DetectErrorCategory
func TestDetectErrorCategory(t *testing.T) {
	t.Run("nil error", func(t *testing.T) {
		if cat := DetectErrorCategory(nil, 0); cat != CategoryUnknown {
			t.Errorf("Expected DetectErrorCategory(nil, 0) = %v, got %v", CategoryUnknown, cat)
		}
	})

	t.Run("already categorized error", func(t *testing.T) {
		catErr := &LLMError{ErrorCategory: CategoryRateLimit}
		if cat := DetectErrorCategory(catErr, 0); cat != CategoryRateLimit {
			t.Errorf("Expected DetectErrorCategory(categorized, 0) = %v, got %v", CategoryRateLimit, cat)
		}
	})

	t.Run("status code based detection", func(t *testing.T) {
		err := errors.New("some error")
		if cat := DetectErrorCategory(err, http.StatusTooManyRequests); cat != CategoryRateLimit {
			t.Errorf("Expected DetectErrorCategory(err, 429) = %v, got %v", CategoryRateLimit, cat)
		}
	})

	t.Run("message based detection", func(t *testing.T) {
		err := errors.New("rate limit exceeded")
		if cat := DetectErrorCategory(err, 0); cat != CategoryRateLimit {
			t.Errorf("Expected DetectErrorCategory(rate limit err, 0) = %v, got %v", CategoryRateLimit, cat)
		}
	})

	t.Run("status code takes precedence", func(t *testing.T) {
		// Message suggests network error, but status code is auth
		err := errors.New("network error")
		if cat := DetectErrorCategory(err, http.StatusUnauthorized); cat != CategoryAuth {
			t.Errorf("Expected status code to take precedence, got %v", cat)
		}
	})
}

// Test CreateStandardErrorWithMessage
func TestCreateStandardErrorWithMessage(t *testing.T) {
	original := errors.New("original error")

	testCases := []struct {
		name         string
		provider     string
		category     ErrorCategory
		details      string
		expectedMsg  string
		expectedSugg string
	}{
		{
			name:         "auth error",
			provider:     "test-provider",
			category:     CategoryAuth,
			details:      "",
			expectedMsg:  "Authentication failed with the test-provider API",
			expectedSugg: "Check that your API key is valid and has not expired",
		},
		{
			name:         "rate limit error with details",
			provider:     "test-provider",
			category:     CategoryRateLimit,
			details:      "limit: 60 requests per minute",
			expectedMsg:  "Request rate limit exceeded on the test-provider API (limit: 60 requests per minute)",
			expectedSugg: "Wait and try again later",
		},
		{
			name:         "unknown error",
			provider:     "test-provider",
			category:     CategoryUnknown,
			details:      "",
			expectedMsg:  "Error calling test-provider API: original error",
			expectedSugg: "Check the logs for more details or try again",
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			llmErr := CreateStandardErrorWithMessage(tc.provider, tc.category, original, tc.details)

			if llmErr.Provider != tc.provider {
				t.Errorf("Expected Provider = %q, got %q", tc.provider, llmErr.Provider)
			}

			if llmErr.ErrorCategory != tc.category {
				t.Errorf("Expected ErrorCategory = %v, got %v", tc.category, llmErr.ErrorCategory)
			}

			if llmErr.Original != original {
				t.Errorf("Expected Original = %v, got %v", original, llmErr.Original)
			}

			if !strings.Contains(llmErr.Message, tc.expectedMsg) {
				t.Errorf("Expected Message to contain %q, got %q", tc.expectedMsg, llmErr.Message)
			}

			if !strings.Contains(llmErr.Suggestion, tc.expectedSugg) {
				t.Errorf("Expected Suggestion to contain %q, got %q", tc.expectedSugg, llmErr.Suggestion)
			}
		})
	}
}

// Test FormatAPIError
func TestFormatAPIError(t *testing.T) {
	t.Run("nil error", func(t *testing.T) {
		if err := FormatAPIError("test-provider", nil, 0, ""); err != nil {
			t.Errorf("Expected FormatAPIError(nil) = nil, got %v", err)
		}
	})

	t.Run("already LLMError", func(t *testing.T) {
		existing := &LLMError{
			Provider:      "test-provider",
			Message:       "test message",
			ErrorCategory: CategoryAuth,
		}

		formatted := FormatAPIError("other-provider", existing, 429, "details")

		// Should return the existing error unchanged
		if formatted != existing {
			t.Errorf("Expected to return existing LLMError, got new instance")
		}
	})

	t.Run("standard error with status code", func(t *testing.T) {
		err := errors.New("some error")
		formatted := FormatAPIError("test-provider", err, http.StatusTooManyRequests, "")

		if formatted.Provider != "test-provider" {
			t.Errorf("Expected Provider = %q, got %q", "test-provider", formatted.Provider)
		}

		if formatted.ErrorCategory != CategoryRateLimit {
			t.Errorf("Expected ErrorCategory = %v, got %v", CategoryRateLimit, formatted.ErrorCategory)
		}

		if formatted.Original != err {
			t.Errorf("Expected Original = %v, got %v", err, formatted.Original)
		}

		if !strings.Contains(formatted.Message, "rate limit") {
			t.Errorf("Expected rate limit message, got %q", formatted.Message)
		}
	})

	t.Run("standard error with message detection", func(t *testing.T) {
		err := errors.New("network connection error")
		formatted := FormatAPIError("test-provider", err, 0, "")

		if formatted.ErrorCategory != CategoryNetwork {
			t.Errorf("Expected ErrorCategory = %v, got %v", CategoryNetwork, formatted.ErrorCategory)
		}
	})

	t.Run("with response body details", func(t *testing.T) {
		err := errors.New("some error")
		details := "error_code=LIMIT_EXCEEDED"
		formatted := FormatAPIError("test-provider", err, 0, details)

		if formatted.Details != details {
			t.Errorf("Expected Details = %q, got %q", details, formatted.Details)
		}

		if !strings.Contains(formatted.Message, details) {
			t.Errorf("Expected Message to contain details %q, got %q", details, formatted.Message)
		}
	})
}
