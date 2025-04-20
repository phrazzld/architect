package logutil

import (
	"fmt"
	"regexp"
	"sync"
)

// SecretPattern defines a pattern to detect in log messages
type SecretPattern struct {
	Name        string // Name of the pattern (e.g., "API Key", "Auth Token")
	Regex       *regexp.Regexp
	Description string // Description of what this pattern detects
}

// Common secret patterns that should not appear in logs
var DefaultSecretPatterns = []SecretPattern{
	{
		Name:        "API Key Format",
		Regex:       regexp.MustCompile(`(?i)key[-_]?[0-9a-zA-Z]{20,}`), // Generic API key pattern
		Description: "Detected text matching API key format",
	},
	{
		Name:        "Bearer Token",
		Regex:       regexp.MustCompile(`[bB]earer\s+[0-9a-zA-Z._-]{20,}`),
		Description: "Detected text matching Bearer token format",
	},
	{
		Name:        "Basic Auth",
		Regex:       regexp.MustCompile(`[bB]asic\s+[0-9a-zA-Z+/=]{10,}`),
		Description: "Detected text matching Basic auth format",
	},
	{
		Name:        "URL with Credentials",
		Regex:       regexp.MustCompile(`https?://[^/]*:[^/]*@`),
		Description: "Detected URL with embedded credentials",
	},
	{
		Name:        "OpenAI API Key",
		Regex:       regexp.MustCompile(`sk-[0-9a-zA-Z]{48}`),
		Description: "Detected text matching OpenAI API key format",
	},
	{
		Name:        "Google API Key",
		Regex:       regexp.MustCompile(`AIza[0-9A-Za-z-_]{35}`),
		Description: "Detected text matching Google API key format",
	},
	{
		Name:        "Generic Secret",
		Regex:       regexp.MustCompile(`(?i)secret[-_]?[0-9a-zA-Z]{16,}`),
		Description: "Detected text matching general secret format",
	},
}

// SecretDetectingLogger is a logger that detects secrets in log messages
type SecretDetectingLogger struct {
	delegate           LoggerInterface // The actual logger to use
	patterns           []SecretPattern // Patterns to check for
	detectedSecrets    []string        // List of detected secrets (for testing)
	mu                 sync.Mutex      // Mutex for thread safety
	failOnSecretDetect bool            // Whether to panic when a secret is detected
}

// Ensure SecretDetectingLogger implements LoggerInterface
var _ LoggerInterface = (*SecretDetectingLogger)(nil)

// NewSecretDetectingLogger creates a new logger that detects secrets
func NewSecretDetectingLogger(delegate LoggerInterface) *SecretDetectingLogger {
	return &SecretDetectingLogger{
		delegate:           delegate,
		patterns:           DefaultSecretPatterns,
		detectedSecrets:    make([]string, 0),
		failOnSecretDetect: true, // By default, fail tests if secrets are detected
	}
}

// Println implements LoggerInterface.Println
func (s *SecretDetectingLogger) Println(v ...interface{}) {
	msg := fmt.Sprintln(v...)
	s.checkForSecrets(msg)
	s.delegate.Println(v...)
}

// Printf implements LoggerInterface.Printf
func (s *SecretDetectingLogger) Printf(format string, v ...interface{}) {
	msg := fmt.Sprintf(format, v...)
	s.checkForSecrets(msg)
	s.delegate.Printf(format, v...)
}

// Debug implements LoggerInterface.Debug
func (s *SecretDetectingLogger) Debug(format string, v ...interface{}) {
	msg := fmt.Sprintf(format, v...)
	s.checkForSecrets(msg)
	s.delegate.Debug(format, v...)
}

// Info implements LoggerInterface.Info
func (s *SecretDetectingLogger) Info(format string, v ...interface{}) {
	msg := fmt.Sprintf(format, v...)
	s.checkForSecrets(msg)
	s.delegate.Info(format, v...)
}

// Warn implements LoggerInterface.Warn
func (s *SecretDetectingLogger) Warn(format string, v ...interface{}) {
	msg := fmt.Sprintf(format, v...)
	s.checkForSecrets(msg)
	s.delegate.Warn(format, v...)
}

// Error implements LoggerInterface.Error
func (s *SecretDetectingLogger) Error(format string, v ...interface{}) {
	msg := fmt.Sprintf(format, v...)
	s.checkForSecrets(msg)
	s.delegate.Error(format, v...)
}

// Fatal implements LoggerInterface.Fatal
func (s *SecretDetectingLogger) Fatal(format string, v ...interface{}) {
	msg := fmt.Sprintf(format, v...)
	s.checkForSecrets(msg)
	s.delegate.Fatal(format, v...)
}

// AddPattern adds a custom pattern to detect
func (s *SecretDetectingLogger) AddPattern(pattern SecretPattern) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.patterns = append(s.patterns, pattern)
}

// SetFailOnSecretDetect sets whether to panic when a secret is detected
func (s *SecretDetectingLogger) SetFailOnSecretDetect(fail bool) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.failOnSecretDetect = fail
}

// GetDetectedSecrets returns a list of detected secrets
func (s *SecretDetectingLogger) GetDetectedSecrets() []string {
	s.mu.Lock()
	defer s.mu.Unlock()
	result := make([]string, len(s.detectedSecrets))
	copy(result, s.detectedSecrets)
	return result
}

// ClearDetectedSecrets clears the list of detected secrets
func (s *SecretDetectingLogger) ClearDetectedSecrets() {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.detectedSecrets = make([]string, 0)
}

// HasDetectedSecrets returns true if any secrets have been detected
func (s *SecretDetectingLogger) HasDetectedSecrets() bool {
	s.mu.Lock()
	defer s.mu.Unlock()
	return len(s.detectedSecrets) > 0
}

// checkForSecrets checks if a message contains any secrets
// If a secret is detected and failOnSecretDetect is true, it will panic
func (s *SecretDetectingLogger) checkForSecrets(msg string) {
	s.mu.Lock()
	defer s.mu.Unlock()

	for _, pattern := range s.patterns {
		if pattern.Regex.MatchString(msg) {
			detection := fmt.Sprintf("SECURITY VIOLATION: %s in log message: %s", pattern.Description, truncateMessage(msg))
			s.detectedSecrets = append(s.detectedSecrets, detection)

			if s.failOnSecretDetect {
				panic(detection)
			}
		}
	}
}

// truncateMessage truncates a message to a reasonable length for display
func truncateMessage(msg string) string {
	const maxLength = 100
	if len(msg) <= maxLength {
		return msg
	}
	return fmt.Sprintf("%s... [truncated]", msg[:maxLength])
}

// WithSecretDetection wraps a logger with secret detection capabilities
func WithSecretDetection(logger LoggerInterface) *SecretDetectingLogger {
	return NewSecretDetectingLogger(logger)
}
