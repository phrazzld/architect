package logutil

import (
	"fmt"
	"regexp"
	"strings"
	"testing"
)

// Test the SecretDetectingLogger
func TestSecretDetectingLogger(t *testing.T) {
	// Create a mock logger to serve as delegate
	mockLogger := &mockLoggingDelegate{
		messages: make([]string, 0),
	}

	// Create a secret detecting logger
	secretLogger := NewSecretDetectingLogger(mockLogger)
	// Disable auto-failing for this test so we can check detection
	secretLogger.SetFailOnSecretDetect(false)

	// Test cases
	testCases := []struct {
		name         string
		message      string
		level        string
		shouldDetect bool
	}{
		{
			name:         "Safe message",
			message:      "This is a safe message",
			level:        "Info",
			shouldDetect: false,
		},
		{
			name:         "Message with API key",
			message:      "Using API key: api_key_01234567890123456789",
			level:        "Debug",
			shouldDetect: true,
		},
		{
			name:         "Message with bearer token",
			message:      "Authorization: Bearer eyJhbGciOiJIUzI1NiIsInR5cCI6IkpXVCJ9.eyJzdWIiOiIxMjM0NTY3ODkwIiwibmFtZSI6IkpvaG4gRG9lIiwiaWF0IjoxNTE2MjM5MDIyfQ",
			level:        "Debug",
			shouldDetect: true,
		},
		{
			name:         "Message with URL credentials",
			message:      "Connecting to https://username:password@example.com",
			level:        "Debug",
			shouldDetect: true,
		},
		{
			name:         "Message with OpenAI key",
			message:      "Using OpenAI key sk-0123456789abcdef0123456789abcdef0123456789abcdef",
			level:        "Info",
			shouldDetect: true,
		},
		{
			name:         "Message with key presence only",
			message:      "API key is present and valid",
			level:        "Info",
			shouldDetect: false,
		},
		{
			name:         "Message with sanitized URL",
			message:      "Connecting to https://example.com/api (removed credentials)",
			level:        "Debug",
			shouldDetect: false,
		},
	}

	// Run test cases
	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			secretLogger.ClearDetectedSecrets()
			mockLogger.messages = make([]string, 0)

			// Log the message at the appropriate level
			switch tc.level {
			case "Debug":
				secretLogger.Debug(tc.message)
			case "Info":
				secretLogger.Info(tc.message)
			case "Warn":
				secretLogger.Warn(tc.message)
			case "Error":
				secretLogger.Error(tc.message)
			}

			// Check if the message was correctly detected as containing a secret
			if secretLogger.HasDetectedSecrets() != tc.shouldDetect {
				if tc.shouldDetect {
					t.Errorf("Expected secret to be detected in message, but none was found: %s", tc.message)
				} else {
					detections := secretLogger.GetDetectedSecrets()
					t.Errorf("Expected no secrets to be detected, but found: %v", detections)
				}
			}

			// Verify that the message was passed to the delegate logger
			if len(mockLogger.messages) != 1 {
				t.Errorf("Expected 1 message to be logged, but got %d", len(mockLogger.messages))
			}
		})
	}
}

// Test the panic behavior when a secret is detected
func TestSecretDetectingLoggerPanic(t *testing.T) {
	// Create a mock logger to serve as delegate
	mockLogger := &mockLoggingDelegate{
		messages: make([]string, 0),
	}

	// Create a secret detecting logger
	secretLogger := NewSecretDetectingLogger(mockLogger)
	// Enable auto-failing
	secretLogger.SetFailOnSecretDetect(true)

	// Test that logger panics on secret detection
	defer func() {
		if r := recover(); r == nil {
			t.Errorf("Expected logger to panic when a secret is detected, but it didn't")
		}
	}()

	// This should trigger a panic
	secretLogger.Info("Using API key: api_key_01234567890123456789")
}

// Test adding custom patterns
func TestSecretDetectingLoggerCustomPatterns(t *testing.T) {
	// Create a mock logger to serve as delegate
	mockLogger := &mockLoggingDelegate{
		messages: make([]string, 0),
	}

	// Create a secret detecting logger
	secretLogger := NewSecretDetectingLogger(mockLogger)
	secretLogger.SetFailOnSecretDetect(false)

	// Add a custom pattern
	secretLogger.AddPattern(SecretPattern{
		Name:        "Custom Pattern",
		Regex:       regexp.MustCompile(`CUSTOM_SECRET_[A-Z0-9]{10}`),
		Description: "Custom secret pattern",
	})

	// Test with custom pattern
	secretLogger.Debug("Message with custom secret: CUSTOM_SECRET_1234567890")

	// Verify detection
	if !secretLogger.HasDetectedSecrets() {
		t.Errorf("Expected custom secret pattern to be detected, but none was found")
	}
}

// Test using the logger wrapper
func TestWithSecretDetection(t *testing.T) {
	// Create a mock logger
	mockLogger := &mockLoggingDelegate{
		messages: make([]string, 0),
	}

	// Wrap it with secret detection
	secretLogger := WithSecretDetection(mockLogger)
	secretLogger.SetFailOnSecretDetect(false)

	// Log a message with a secret
	secretLogger.Info("Using secret_1234567890abcdef")

	// Verify detection
	if !secretLogger.HasDetectedSecrets() {
		t.Errorf("Expected secret to be detected, but none was found")
	}

	// Verify the message was passed to the delegate
	if len(mockLogger.messages) != 1 {
		t.Errorf("Expected 1 message to be logged, but got %d", len(mockLogger.messages))
	}
}

// Mock logger for testing
type mockLoggingDelegate struct {
	messages []string
}

func (m *mockLoggingDelegate) Println(v ...interface{}) {
	m.messages = append(m.messages, fmt.Sprintln(v...))
}

func (m *mockLoggingDelegate) Printf(format string, v ...interface{}) {
	m.messages = append(m.messages, fmt.Sprintf(format, v...))
}

func (m *mockLoggingDelegate) Debug(format string, v ...interface{}) {
	m.messages = append(m.messages, fmt.Sprintf(format, v...))
}

func (m *mockLoggingDelegate) Info(format string, v ...interface{}) {
	m.messages = append(m.messages, fmt.Sprintf(format, v...))
}

func (m *mockLoggingDelegate) Warn(format string, v ...interface{}) {
	m.messages = append(m.messages, fmt.Sprintf(format, v...))
}

func (m *mockLoggingDelegate) Error(format string, v ...interface{}) {
	m.messages = append(m.messages, fmt.Sprintf(format, v...))
}

func (m *mockLoggingDelegate) Fatal(format string, v ...interface{}) {
	m.messages = append(m.messages, fmt.Sprintf(format, v...))
	// Don't exit in tests
}

// Test the DefaultSecretPatterns
func TestDefaultSecretPatterns(t *testing.T) {
	testCases := []struct {
		name        string
		message     string
		shouldMatch []string // Names of patterns that should match
	}{
		{
			name:        "API Key",
			message:     "Using API key api_key_01234567890123456789",
			shouldMatch: []string{"API Key Format"},
		},
		{
			name:        "Bearer Token",
			message:     "Authorization: Bearer eyJhbGciOiJIUzI1NiIsInR5cCI6IkpXVCJ9.eyJzdWIiOiIxMjM0NTY3ODkwIiwibmFtZSI6IkpvaG4gRG9lIiwiaWF0IjoxNTE2MjM5MDIyfQ",
			shouldMatch: []string{"Bearer Token"},
		},
		{
			name:        "Basic Auth",
			message:     "Authorization: Basic dXNlcm5hbWU6cGFzc3dvcmQ=",
			shouldMatch: []string{"Basic Auth"},
		},
		{
			name:        "URL with Credentials",
			message:     "Connecting to https://username:password@example.com",
			shouldMatch: []string{"URL with Credentials"},
		},
		{
			name:        "OpenAI API Key",
			message:     "OpenAI key: sk-0123456789abcdef0123456789abcdef0123456789abcdef",
			shouldMatch: []string{"OpenAI API Key"},
		},
		{
			name:        "Google API Key",
			message:     "Google key: AIzaSyC6MkjIAB-fJvnuvTqOTvWHP9xxxx_xxxxx",
			shouldMatch: []string{"Google API Key"},
		},
		{
			name:        "Generic Secret",
			message:     "Using secret_1234567890abcdef",
			shouldMatch: []string{"Generic Secret"},
		},
		{
			name:        "Safe Message",
			message:     "This is a safe message without secrets",
			shouldMatch: []string{},
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			// Test each pattern against this message
			for _, pattern := range DefaultSecretPatterns {
				matched := pattern.Regex.MatchString(tc.message)
				shouldMatch := contains(tc.shouldMatch, pattern.Name)

				if matched != shouldMatch {
					if shouldMatch {
						t.Errorf("Pattern '%s' should match message '%s', but it didn't",
							pattern.Name, tc.message)
					} else {
						t.Errorf("Pattern '%s' shouldn't match message '%s', but it did",
							pattern.Name, tc.message)
					}
				}
			}
		})
	}
}

// Helper function to check if a slice contains a string
func contains(slice []string, item string) bool {
	for _, s := range slice {
		if strings.Compare(s, item) == 0 {
			return true
		}
	}
	return false
}
