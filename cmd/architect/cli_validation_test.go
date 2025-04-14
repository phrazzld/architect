// Package architect provides the command-line interface for the architect tool
package architect

import (
	"os"
	"strings"
	"testing"

	"github.com/phrazzld/architect/internal/config"
)

// TestValidateInputs ensures that the validation function correctly validates all required fields
func TestValidateInputs(t *testing.T) {
	// Create a test instructions file
	tempFile, err := os.CreateTemp("", "instructions-*.txt")
	if err != nil {
		t.Fatalf("Failed to create temporary instructions file: %v", err)
	}
	defer func() { _ = os.Remove(tempFile.Name()) }()

	_, err = tempFile.WriteString("Test instructions content")
	if err != nil {
		t.Fatalf("Failed to write to temporary instructions file: %v", err)
	}
	_ = tempFile.Close()

	tests := []struct {
		name          string
		config        *config.CliConfig
		expectError   bool
		errorContains string
	}{
		{
			name: "Valid configuration",
			config: &config.CliConfig{
				InstructionsFile: tempFile.Name(),
				Paths:            []string{"testfile"},
				APIKey:           "test-key",
				ModelNames:       []string{"model1"},
			},
			expectError: false,
		},
		{
			name: "Missing instructions file",
			config: &config.CliConfig{
				InstructionsFile: "", // Missing
				Paths:            []string{"testfile"},
				APIKey:           "test-key",
			},
			expectError:   true,
			errorContains: "missing required --instructions flag",
		},
		{
			name: "Missing paths",
			config: &config.CliConfig{
				InstructionsFile: tempFile.Name(),
				Paths:            []string{}, // Empty
				APIKey:           "test-key",
			},
			expectError:   true,
			errorContains: "no paths specified",
		},
		{
			name: "Missing API key",
			config: &config.CliConfig{
				InstructionsFile: tempFile.Name(),
				Paths:            []string{"testfile"},
				APIKey:           "", // Missing
			},
			expectError:   true,
			errorContains: "API key not set",
		},
		{
			name: "Dry run allows missing instructions file",
			config: &config.CliConfig{
				InstructionsFile: "", // Missing
				Paths:            []string{"testfile"},
				APIKey:           "test-key",
				DryRun:           true, // Dry run mode
			},
			expectError: false,
		},
		{
			name: "Dry run still requires paths",
			config: &config.CliConfig{
				InstructionsFile: "",         // Missing allowed in dry run
				Paths:            []string{}, // Empty paths still invalid
				APIKey:           "test-key",
				DryRun:           true,
			},
			expectError:   true,
			errorContains: "no paths specified",
		},
		{
			name: "Dry run still requires API key",
			config: &config.CliConfig{
				InstructionsFile: "", // Missing allowed in dry run
				Paths:            []string{"testfile"},
				APIKey:           "", // Missing
				DryRun:           true,
			},
			expectError:   true,
			errorContains: "API key not set",
		},
		{
			name: "Missing models",
			config: &config.CliConfig{
				InstructionsFile: tempFile.Name(),
				Paths:            []string{"testfile"},
				APIKey:           "test-key",
				ModelNames:       []string{}, // Empty
			},
			expectError:   true,
			errorContains: "no models specified",
		},
		{
			name: "Dry run allows missing models",
			config: &config.CliConfig{
				InstructionsFile: "", // Missing allowed in dry run
				Paths:            []string{"testfile"},
				APIKey:           "test-key",
				ModelNames:       []string{}, // Empty allowed in dry run
				DryRun:           true,
			},
			expectError: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			logger := &errorTrackingLogger{}
			err := ValidateInputs(tt.config, logger)

			// Check if error matches expectation
			if (err != nil) != tt.expectError {
				t.Errorf("ValidateInputs() error = %v, expectError %v", err, tt.expectError)
			}

			// Verify error contains expected text
			if tt.expectError && err != nil {
				if !strings.Contains(err.Error(), tt.errorContains) {
					t.Errorf("Error message %q doesn't contain expected text %q", err.Error(), tt.errorContains)
				}
			}

			// Verify logger recorded errors for error cases
			if tt.expectError && !logger.errorCalled {
				t.Error("Expected error to be logged, but no error was logged")
			}

			if !tt.expectError && logger.errorCalled {
				t.Error("No error expected, but error was logged")
			}
		})
	}
}
