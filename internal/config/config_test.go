package config

import (
	"fmt"
	"strings"
	"testing"

	"github.com/phrazzld/architect/internal/logutil"
)

func TestNewDefaultCliConfig(t *testing.T) {
	cfg := NewDefaultCliConfig()

	// Verify the result is not nil
	if cfg == nil {
		t.Fatal("NewDefaultCliConfig() returned nil")
	}

	// Verify default values
	if cfg.Format != DefaultFormat {
		t.Errorf("Expected Format to be %q, got %q", DefaultFormat, cfg.Format)
	}

	if cfg.Exclude != DefaultExcludes {
		t.Errorf("Expected Exclude to be %q, got %q", DefaultExcludes, cfg.Exclude)
	}

	if cfg.ExcludeNames != DefaultExcludeNames {
		t.Errorf("Expected ExcludeNames to be %q, got %q", DefaultExcludeNames, cfg.ExcludeNames)
	}

	if len(cfg.ModelNames) != 1 || cfg.ModelNames[0] != DefaultModel {
		t.Errorf("Expected ModelNames to be [%q], got %v", DefaultModel, cfg.ModelNames)
	}

	if cfg.LogLevel != logutil.InfoLevel {
		t.Errorf("Expected LogLevel to be %v, got %v", logutil.InfoLevel, cfg.LogLevel)
	}

	if cfg.MaxConcurrentRequests != DefaultMaxConcurrentRequests {
		t.Errorf("Expected MaxConcurrentRequests to be %d, got %d", DefaultMaxConcurrentRequests, cfg.MaxConcurrentRequests)
	}

	if cfg.RateLimitRequestsPerMinute != DefaultRateLimitRequestsPerMinute {
		t.Errorf("Expected RateLimitRequestsPerMinute to be %d, got %d", DefaultRateLimitRequestsPerMinute, cfg.RateLimitRequestsPerMinute)
	}

	// Check that uninitialized fields have zero/empty values
	if cfg.InstructionsFile != "" {
		t.Errorf("Expected InstructionsFile to be empty, got %q", cfg.InstructionsFile)
	}

	if cfg.OutputDir != "" {
		t.Errorf("Expected OutputDir to be empty, got %q", cfg.OutputDir)
	}

	if cfg.AuditLogFile != "" {
		t.Errorf("Expected AuditLogFile to be empty, got %q", cfg.AuditLogFile)
	}

	if len(cfg.Paths) != 0 {
		t.Errorf("Expected Paths to be empty, got %v", cfg.Paths)
	}

	if cfg.Include != "" {
		t.Errorf("Expected Include to be empty, got %q", cfg.Include)
	}

	if cfg.DryRun != false {
		t.Errorf("Expected DryRun to be false, got %v", cfg.DryRun)
	}

	if cfg.Verbose != false {
		t.Errorf("Expected Verbose to be false, got %v", cfg.Verbose)
	}

	if cfg.APIKey != "" {
		t.Errorf("Expected APIKey to be empty, got %q", cfg.APIKey)
	}

	if cfg.APIEndpoint != "" {
		t.Errorf("Expected APIEndpoint to be empty, got %q", cfg.APIEndpoint)
	}

	// ConfirmTokens check removed as part of T032E token management refactoring

	// Test for slice creation (slices are reference types in Go, so modifying a
	// slice does affect the original - we just want to ensure the slice is created)
	if cfg.ModelNames == nil {
		t.Error("ModelNames slice should not be nil")
	}
}

func TestDefaultConfig(t *testing.T) {
	cfg := DefaultConfig()

	// Verify the result is not nil
	if cfg == nil {
		t.Fatal("DefaultConfig() returned nil")
	}

	// Verify default values for all fields
	if cfg.OutputFile != DefaultOutputFile {
		t.Errorf("Expected OutputFile to be %q, got %q", DefaultOutputFile, cfg.OutputFile)
	}

	if cfg.ModelName != DefaultModel {
		t.Errorf("Expected ModelName to be %q, got %q", DefaultModel, cfg.ModelName)
	}

	if cfg.Format != DefaultFormat {
		t.Errorf("Expected Format to be %q, got %q", DefaultFormat, cfg.Format)
	}

	if cfg.LogLevel != logutil.InfoLevel {
		t.Errorf("Expected LogLevel to be %v, got %v", logutil.InfoLevel, cfg.LogLevel)
	}

	// ConfirmTokens check removed as part of T032E token management refactoring

	// Verify nested ExcludeConfig values
	if cfg.Excludes.Extensions != DefaultExcludes {
		t.Errorf("Expected Excludes.Extensions to be %q, got %q", DefaultExcludes, cfg.Excludes.Extensions)
	}

	if cfg.Excludes.Names != DefaultExcludeNames {
		t.Errorf("Expected Excludes.Names to be %q, got %q", DefaultExcludeNames, cfg.Excludes.Names)
	}

	// Verify default values for fields that should be empty or zero
	if cfg.Include != "" {
		t.Errorf("Expected Include to be empty, got %q", cfg.Include)
	}

	if cfg.Verbose != false {
		t.Errorf("Expected Verbose to be false, got %v", cfg.Verbose)
	}

	// Test for nested struct independence
	originalExcludes := cfg.Excludes
	cfg.Excludes.Extensions = "modified-extensions"
	cfg.Excludes.Names = "modified-names"

	// Create a new config to ensure the defaults haven't been modified
	newCfg := DefaultConfig()
	if newCfg.Excludes.Extensions != DefaultExcludes {
		t.Errorf("DefaultExcludes has been modified globally, expected %q, got %q",
			DefaultExcludes, newCfg.Excludes.Extensions)
	}
	if newCfg.Excludes.Names != DefaultExcludeNames {
		t.Errorf("DefaultExcludeNames has been modified globally, expected %q, got %q",
			DefaultExcludeNames, newCfg.Excludes.Names)
	}

	// Reset for further tests
	cfg.Excludes = originalExcludes

	// Test successive calls return independent instances
	config1 := DefaultConfig()
	config2 := DefaultConfig()

	// Modify config1's fields
	config1.OutputFile = "changed-output-file"
	config1.ModelName = "changed-model-name"
	config1.Format = "changed-format"
	config1.LogLevel = logutil.DebugLevel
	config1.Excludes.Extensions = "changed-extensions"

	// Verify config2 remains unaffected
	if config2.OutputFile != DefaultOutputFile {
		t.Errorf("config2.OutputFile changed, expected %q, got %q",
			DefaultOutputFile, config2.OutputFile)
	}
	if config2.ModelName != DefaultModel {
		t.Errorf("config2.ModelName changed, expected %q, got %q",
			DefaultModel, config2.ModelName)
	}
	if config2.Format != DefaultFormat {
		t.Errorf("config2.Format changed, expected %q, got %q",
			DefaultFormat, config2.Format)
	}
	if config2.LogLevel != logutil.InfoLevel {
		t.Errorf("config2.LogLevel changed, expected %v, got %v",
			logutil.InfoLevel, config2.LogLevel)
	}
	if config2.Excludes.Extensions != DefaultExcludes {
		t.Errorf("config2.Excludes.Extensions changed, expected %q, got %q",
			DefaultExcludes, config2.Excludes.Extensions)
	}
}

// MockLogger is a simple logger implementation for testing
type MockLogger struct {
	ErrorCalled   bool
	ErrorMessages []string
}

func (l *MockLogger) Error(format string, args ...interface{}) {
	l.ErrorCalled = true
	l.ErrorMessages = append(l.ErrorMessages, fmt.Sprintf(format, args...))
}

func (l *MockLogger) Debug(format string, args ...interface{})  {}
func (l *MockLogger) Info(format string, args ...interface{})   {}
func (l *MockLogger) Warn(format string, args ...interface{})   {}
func (l *MockLogger) Fatal(format string, args ...interface{})  {}
func (l *MockLogger) Printf(format string, args ...interface{}) {}
func (l *MockLogger) Println(v ...interface{})                  {}

// TestValidateConfig tests the ValidateConfig function
func TestValidateConfig(t *testing.T) {
	tests := []struct {
		name          string
		config        *CliConfig
		logger        logutil.LoggerInterface
		expectError   bool
		errorContains string
	}{
		{
			name: "Valid configuration",
			config: &CliConfig{
				InstructionsFile: "instructions.md",
				Paths:            []string{"testfile"},
				APIKey:           "test-key",
				ModelNames:       []string{"model1"},
			},
			logger:      &MockLogger{},
			expectError: false,
		},
		{
			name: "Missing instructions file",
			config: &CliConfig{
				InstructionsFile: "", // Missing
				Paths:            []string{"testfile"},
				APIKey:           "test-key",
				ModelNames:       []string{"model1"},
			},
			logger:        &MockLogger{},
			expectError:   true,
			errorContains: "missing required --instructions flag",
		},
		{
			name: "Missing paths",
			config: &CliConfig{
				InstructionsFile: "instructions.md",
				Paths:            []string{}, // Empty
				APIKey:           "test-key",
				ModelNames:       []string{"model1"},
			},
			logger:        &MockLogger{},
			expectError:   true,
			errorContains: "no paths specified",
		},
		{
			name: "Missing API key",
			config: &CliConfig{
				InstructionsFile: "instructions.md",
				Paths:            []string{"testfile"},
				APIKey:           "", // Missing
				ModelNames:       []string{"model1"},
			},
			logger:        &MockLogger{},
			expectError:   true,
			errorContains: "API key not set",
		},
		{
			name: "Missing models",
			config: &CliConfig{
				InstructionsFile: "instructions.md",
				Paths:            []string{"testfile"},
				APIKey:           "test-key",
				ModelNames:       []string{}, // Empty
			},
			logger:        &MockLogger{},
			expectError:   true,
			errorContains: "no models specified",
		},
		{
			name: "Dry run allows missing instructions file",
			config: &CliConfig{
				InstructionsFile: "", // Missing but allowed in dry run
				Paths:            []string{"testfile"},
				APIKey:           "test-key",
				ModelNames:       []string{"model1"},
				DryRun:           true,
			},
			logger:      &MockLogger{},
			expectError: false,
		},
		{
			name: "Dry run still requires paths",
			config: &CliConfig{
				InstructionsFile: "",         // Missing but allowed in dry run
				Paths:            []string{}, // Empty paths still invalid
				APIKey:           "test-key",
				ModelNames:       []string{"model1"},
				DryRun:           true,
			},
			logger:        &MockLogger{},
			expectError:   true,
			errorContains: "no paths specified",
		},
		{
			name: "Dry run still requires API key",
			config: &CliConfig{
				InstructionsFile: "", // Missing but allowed in dry run
				Paths:            []string{"testfile"},
				APIKey:           "", // Missing - still required in dry run
				ModelNames:       []string{"model1"},
				DryRun:           true,
			},
			logger:        &MockLogger{},
			expectError:   true,
			errorContains: "API key not set",
		},
		{
			name: "Dry run allows missing models",
			config: &CliConfig{
				InstructionsFile: "", // Missing but allowed in dry run
				Paths:            []string{"testfile"},
				APIKey:           "test-key",
				ModelNames:       []string{}, // Empty - allowed in dry run
				DryRun:           true,
			},
			logger:      &MockLogger{},
			expectError: false,
		},
		{
			name:          "Nil config",
			config:        nil,
			logger:        &MockLogger{},
			expectError:   true,
			errorContains: "nil config",
		},
		{
			name: "Nil logger",
			config: &CliConfig{
				InstructionsFile: "instructions.md",
				Paths:            []string{"testfile"},
				APIKey:           "test-key",
				ModelNames:       []string{"model1"},
			},
			logger:        nil,
			expectError:   false, // Should not panic, just work without logging
			errorContains: "",
		},
		{
			name: "Path with whitespace only",
			config: &CliConfig{
				InstructionsFile: "instructions.md",
				Paths:            []string{"  ", "\t", "\n"},
				APIKey:           "test-key",
				ModelNames:       []string{"model1"},
			},
			logger:        &MockLogger{},
			expectError:   true,
			errorContains: "no paths specified",
		},
		{
			name: "Path with empty string",
			config: &CliConfig{
				InstructionsFile: "instructions.md",
				Paths:            []string{""},
				APIKey:           "test-key",
				ModelNames:       []string{"model1"},
			},
			logger:        &MockLogger{},
			expectError:   true,
			errorContains: "no paths specified",
		},
		{
			name: "Path with mix of valid and invalid paths",
			config: &CliConfig{
				InstructionsFile: "instructions.md",
				Paths:            []string{"validpath", ""},
				APIKey:           "test-key",
				ModelNames:       []string{"model1"},
			},
			logger:      &MockLogger{},
			expectError: false, // Should pass as long as at least one path is valid
		},
		{
			name: "Extreme values for rate limiting - zero",
			config: &CliConfig{
				InstructionsFile:           "instructions.md",
				Paths:                      []string{"testfile"},
				APIKey:                     "test-key",
				ModelNames:                 []string{"model1"},
				MaxConcurrentRequests:      0,
				RateLimitRequestsPerMinute: 0,
			},
			logger:      &MockLogger{},
			expectError: false, // Zero values should be allowed
		},
		{
			name: "Extreme values for rate limiting - negative",
			config: &CliConfig{
				InstructionsFile:           "instructions.md",
				Paths:                      []string{"testfile"},
				APIKey:                     "test-key",
				ModelNames:                 []string{"model1"},
				MaxConcurrentRequests:      -1,
				RateLimitRequestsPerMinute: -10,
			},
			logger:      &MockLogger{},
			expectError: false, // Currently no validation for negative rate limits
		},
		{
			name: "Extreme values for rate limiting - very large",
			config: &CliConfig{
				InstructionsFile:           "instructions.md",
				Paths:                      []string{"testfile"},
				APIKey:                     "test-key",
				ModelNames:                 []string{"model1"},
				MaxConcurrentRequests:      1000000,
				RateLimitRequestsPerMinute: 1000000,
			},
			logger:      &MockLogger{},
			expectError: false, // Currently no validation for extreme rate limits
		},
		{
			name: "All fields missing except path and API key in dry run",
			config: &CliConfig{
				InstructionsFile: "",
				Paths:            []string{"testfile"},
				APIKey:           "test-key",
				ModelNames:       []string{},
				DryRun:           true,
			},
			logger:      &MockLogger{},
			expectError: false, // Should be valid in dry run mode
		},
		{
			name: "All fields missing in non-dry run",
			config: &CliConfig{
				InstructionsFile: "",
				Paths:            []string{},
				APIKey:           "",
				ModelNames:       []string{},
				DryRun:           false,
			},
			logger:        &MockLogger{},
			expectError:   true,
			errorContains: "no paths specified", // We return on first error encountered
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Special case for nil config test
			if tt.name == "Nil config" {
				// This should panic or return an error, but we'll handle it gracefully
				err := ValidateConfig(nil, tt.logger)
				if err == nil {
					t.Error("Expected error for nil config, but got nil")
				}
				return
			}

			// Get logger for error tracking
			var mockLogger *MockLogger
			if ml, ok := tt.logger.(*MockLogger); ok {
				mockLogger = ml
			}

			err := ValidateConfig(tt.config, tt.logger)

			// Check if error matches expectation
			if (err != nil) != tt.expectError {
				t.Errorf("ValidateConfig() error = %v, expectError %v", err, tt.expectError)
			}

			// Verify error contains expected text
			if tt.expectError && err != nil && tt.errorContains != "" {
				if !strings.Contains(err.Error(), tt.errorContains) {
					t.Errorf("Error message %q doesn't contain expected text %q", err.Error(), tt.errorContains)
				}
			}

			// Verify logger recorded errors for error cases (only if we have a mock logger)
			if mockLogger != nil {
				if tt.expectError && !mockLogger.ErrorCalled {
					t.Error("Expected error to be logged, but no error was logged")
				}

				if !tt.expectError && mockLogger.ErrorCalled {
					t.Error("No error expected, but error was logged")
				}
			}
		})
	}
}

// TestValidateConfigWithEnv tests the injectable getenv functionality
func TestValidateConfigWithEnv(t *testing.T) {
	tests := []struct {
		name          string
		config        *CliConfig
		mockGetenv    func(string) string
		expectError   bool
		errorContains string
	}{
		{
			name: "OpenAI model requires OpenAI API key",
			config: &CliConfig{
				InstructionsFile: "instructions.md",
				Paths:            []string{"testfile"},
				APIKey:           "test-key",        // Gemini key set
				ModelNames:       []string{"gpt-4"}, // OpenAI model
			},
			mockGetenv: func(key string) string {
				// Return empty string for OpenAI API key
				if key == OpenAIAPIKeyEnvVar {
					return ""
				}
				return "mock-value"
			},
			expectError:   true,
			errorContains: "openAI API key not set",
		},
		{
			name: "OpenAI model with valid OpenAI API key",
			config: &CliConfig{
				InstructionsFile: "instructions.md",
				Paths:            []string{"testfile"},
				APIKey:           "test-key",        // Gemini key set
				ModelNames:       []string{"gpt-4"}, // OpenAI model
			},
			mockGetenv: func(key string) string {
				// Return valid OpenAI API key
				if key == OpenAIAPIKeyEnvVar {
					return "valid-openai-key"
				}
				return "mock-value"
			},
			expectError: false,
		},
		{
			name: "Multiple models requiring both API keys",
			config: &CliConfig{
				InstructionsFile: "instructions.md",
				Paths:            []string{"testfile"},
				APIKey:           "test-key",                          // Gemini key set
				ModelNames:       []string{"gemini-1.5-pro", "gpt-4"}, // Both models
			},
			mockGetenv: func(key string) string {
				// Return valid OpenAI API key
				if key == OpenAIAPIKeyEnvVar {
					return "valid-openai-key"
				}
				return "mock-value"
			},
			expectError: false,
		},
		{
			name: "Multiple models with missing OpenAI key",
			config: &CliConfig{
				InstructionsFile: "instructions.md",
				Paths:            []string{"testfile"},
				APIKey:           "test-key",                          // Gemini key set
				ModelNames:       []string{"gemini-1.5-pro", "gpt-4"}, // Both models
			},
			mockGetenv: func(key string) string {
				// Return empty OpenAI API key
				if key == OpenAIAPIKeyEnvVar {
					return ""
				}
				return "mock-value"
			},
			expectError:   true,
			errorContains: "openAI API key not set",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			logger := &MockLogger{}

			err := ValidateConfigWithEnv(tt.config, logger, tt.mockGetenv)

			// Check if error matches expectation
			if (err != nil) != tt.expectError {
				t.Errorf("ValidateConfigWithEnv() error = %v, expectError %v", err, tt.expectError)
			}

			// Verify error contains expected text
			if tt.expectError && err != nil && tt.errorContains != "" {
				if !strings.Contains(err.Error(), tt.errorContains) {
					t.Errorf("Error message %q doesn't contain expected text %q", err.Error(), tt.errorContains)
				}
			}

			// Verify logger recorded errors for error cases
			if tt.expectError && !logger.ErrorCalled {
				t.Error("Expected error to be logged, but no error was logged")
			}

			if !tt.expectError && logger.ErrorCalled {
				t.Error("No error expected, but error was logged")
			}
		})
	}
}

// TestValidateConfigWithNilConfig tests the specific case of a nil config
func TestValidateConfigWithNilConfig(t *testing.T) {
	logger := &MockLogger{}

	// Attempt to validate a nil config
	err := ValidateConfig(nil, logger)

	// Verify an error is returned
	if err == nil {
		t.Error("Expected error for nil config, but got nil")
	}

	// Verify error message contains expected text
	if err != nil && !strings.Contains(err.Error(), "nil config") {
		t.Errorf("Error message %q doesn't contain 'nil config'", err.Error())
	}

	// Verify error is logged
	if !logger.ErrorCalled {
		t.Error("Expected error to be logged, but no error was logged")
	}
}
