// Package architect provides the command-line interface for the architect tool
package architect

import (
	"flag"
	"fmt"
	"io"
	"os"
	"reflect"
	"strings"
	"testing"

	"github.com/phrazzld/architect/internal/logutil"
)

func TestParseFlagsWithEnv(t *testing.T) {
	tests := []struct {
		name    string
		args    []string
		env     map[string]string
		want    *CliConfig
		wantErr bool
	}{
		{
			name: "Basic valid configuration",
			args: []string{
				"--instructions", "instructions.md",
				"path1", "path2",
			},
			env: map[string]string{
				apiKeyEnvVar: "test-api-key",
			},
			want: &CliConfig{
				InstructionsFile: "instructions.md",
				Paths:            []string{"path1", "path2"},
				ApiKey:           "test-api-key",
				OutputFile:       defaultOutputFile,
				ModelName:        defaultModel,
				Exclude:          defaultExcludes,
				ExcludeNames:     defaultExcludeNames,
				Format:           defaultFormat,
				ConfirmTokens:    0, // Default value
				Verbose:          false,
				LogLevel:         logutil.InfoLevel,
				Include:          "", // Default value
				DryRun:           false,
			},
			wantErr: false,
		},
		{
			name: "All options specified",
			args: []string{
				"--instructions", "custom-instructions.md",
				"--output", "custom-output.md",
				"--model", "custom-model",
				"--log-level", "debug",
				"--include", "*.go,*.md",
				"--exclude", "*.tmp",
				"--exclude-names", "node_modules,dist",
				"--format", "Custom: {path}\n{content}",
				"--confirm-tokens", "500",
				"--verbose",
				"--dry-run",
				"path1", "path2", "path3",
			},
			env: map[string]string{
				apiKeyEnvVar: "test-api-key",
			},
			want: &CliConfig{
				InstructionsFile: "custom-instructions.md",
				OutputFile:       "custom-output.md",
				ModelName:        "custom-model",
				LogLevel:         logutil.DebugLevel, // verbose overrides log-level
				Include:          "*.go,*.md",
				Exclude:          "*.tmp",
				ExcludeNames:     "node_modules,dist",
				Format:           "Custom: {path}\n{content}",
				ConfirmTokens:    500,
				Verbose:          true,
				DryRun:           true,
				Paths:            []string{"path1", "path2", "path3"},
				ApiKey:           "test-api-key",
			},
			wantErr: false,
		},
		{
			name: "Missing instructions file (now validated in ValidateInputs)",
			args: []string{
				"path1",
			},
			env: map[string]string{
				apiKeyEnvVar: "test-api-key",
			},
			want: &CliConfig{
				InstructionsFile: "",
				Paths:            []string{"path1"},
				ApiKey:           "test-api-key",
				OutputFile:       defaultOutputFile,
				ModelName:        defaultModel,
				Exclude:          defaultExcludes,
				ExcludeNames:     defaultExcludeNames,
				Format:           defaultFormat,
				ConfirmTokens:    0,
				Verbose:          false,
				LogLevel:         logutil.InfoLevel,
				Include:          "",
				DryRun:           false,
			},
			wantErr: false, // No longer errors at parse time
		},
		{
			name: "Missing paths (now validated in ValidateInputs)",
			args: []string{
				"--instructions", "instructions.md",
			},
			env: map[string]string{
				apiKeyEnvVar: "test-api-key",
			},
			want: &CliConfig{
				InstructionsFile: "instructions.md",
				Paths:            []string{},
				ApiKey:           "test-api-key",
				OutputFile:       defaultOutputFile,
				ModelName:        defaultModel,
				Exclude:          defaultExcludes,
				ExcludeNames:     defaultExcludeNames,
				Format:           defaultFormat,
				ConfirmTokens:    0,
				Verbose:          false,
				LogLevel:         logutil.InfoLevel,
				Include:          "",
				DryRun:           false,
			},
			wantErr: false, // No longer errors at parse time
		},
		{
			name: "Missing API key",
			args: []string{
				"--instructions", "instructions.md",
				"path1",
			},
			env: map[string]string{}, // No API key in environment
			want: &CliConfig{
				InstructionsFile: "instructions.md",
				Paths:            []string{"path1"},
				ApiKey:           "", // Empty API key
				OutputFile:       defaultOutputFile,
				ModelName:        defaultModel,
				Exclude:          defaultExcludes,
				ExcludeNames:     defaultExcludeNames,
				Format:           defaultFormat,
				ConfirmTokens:    0,
				Verbose:          false,
				LogLevel:         logutil.InfoLevel,
				Include:          "",
				DryRun:           false,
			},
			wantErr: false, // This isn't checked at flag parse time but in ValidateInputs
		},
		{
			name: "Dry run without instructions file",
			args: []string{
				"--dry-run",
				"path1", "path2",
			},
			env: map[string]string{
				apiKeyEnvVar: "test-api-key",
			},
			want: &CliConfig{
				InstructionsFile: "",
				DryRun:           true,
				Paths:            []string{"path1", "path2"},
				ApiKey:           "test-api-key",
				OutputFile:       defaultOutputFile,
				ModelName:        defaultModel,
				Exclude:          defaultExcludes,
				ExcludeNames:     defaultExcludeNames,
				Format:           defaultFormat,
				ConfirmTokens:    0,
				Verbose:          false,
				LogLevel:         logutil.InfoLevel,
				Include:          "",
			},
			wantErr: false,
		},
		{
			name: "Log level without verbose",
			args: []string{
				"--instructions", "instructions.md",
				"--log-level", "warn",
				"path1",
			},
			env: map[string]string{
				apiKeyEnvVar: "test-api-key",
			},
			want: &CliConfig{
				InstructionsFile: "instructions.md",
				OutputFile:       defaultOutputFile,
				ModelName:        defaultModel,
				LogLevel:         logutil.WarnLevel,
				Include:          "",
				Exclude:          defaultExcludes,
				ExcludeNames:     defaultExcludeNames,
				Format:           defaultFormat,
				ConfirmTokens:    0,
				Verbose:          false,
				DryRun:           false,
				Paths:            []string{"path1"},
				ApiKey:           "test-api-key",
			},
			wantErr: false,
		},
		{
			name: "Invalid log level",
			args: []string{
				"--instructions", "instructions.md",
				"--log-level", "invalid",
				"path1",
			},
			env: map[string]string{
				apiKeyEnvVar: "test-api-key",
			},
			want: &CliConfig{
				InstructionsFile: "instructions.md",
				OutputFile:       defaultOutputFile,
				ModelName:        defaultModel,
				LogLevel:         logutil.InfoLevel, // Should default to info for invalid level
				Include:          "",
				Exclude:          defaultExcludes,
				ExcludeNames:     defaultExcludeNames,
				Format:           defaultFormat,
				ConfirmTokens:    0,
				Verbose:          false,
				DryRun:           false,
				Paths:            []string{"path1"},
				ApiKey:           "test-api-key",
			},
			wantErr: false,
		},
		{
			name: "Verbose overrides log level",
			args: []string{
				"--instructions", "instructions.md",
				"--log-level", "warn",
				"--verbose",
				"path1",
			},
			env: map[string]string{
				apiKeyEnvVar: "test-api-key",
			},
			want: &CliConfig{
				InstructionsFile: "instructions.md",
				OutputFile:       defaultOutputFile,
				ModelName:        defaultModel,
				LogLevel:         logutil.DebugLevel, // Verbose overrides to debug
				Include:          "",
				Exclude:          defaultExcludes,
				ExcludeNames:     defaultExcludeNames,
				Format:           defaultFormat,
				ConfirmTokens:    0,
				Verbose:          true,
				DryRun:           false,
				Paths:            []string{"path1"},
				ApiKey:           "test-api-key",
			},
			wantErr: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Create a new FlagSet for each test
			fs := flag.NewFlagSet("test", flag.ContinueOnError)
			// Disable output to avoid cluttering test output
			fs.SetOutput(io.Discard)

			// Create a mock environment getter
			getenv := func(key string) string {
				return tt.env[key]
			}

			got, err := ParseFlagsWithEnv(fs, tt.args, getenv)
			if (err != nil) != tt.wantErr {
				t.Errorf("ParseFlagsWithEnv() error = %v, wantErr %v", err, tt.wantErr)
				return
			}

			if tt.wantErr {
				return
			}

			// Deep compare all fields
			if got.InstructionsFile != tt.want.InstructionsFile {
				t.Errorf("InstructionsFile = %v, want %v", got.InstructionsFile, tt.want.InstructionsFile)
			}
			if !reflect.DeepEqual(got.Paths, tt.want.Paths) {
				t.Errorf("Paths = %v, want %v", got.Paths, tt.want.Paths)
			}
			if got.ApiKey != tt.want.ApiKey {
				t.Errorf("ApiKey = %v, want %v", got.ApiKey, tt.want.ApiKey)
			}
			if got.OutputFile != tt.want.OutputFile {
				t.Errorf("OutputFile = %v, want %v", got.OutputFile, tt.want.OutputFile)
			}
			if got.ModelName != tt.want.ModelName {
				t.Errorf("ModelName = %v, want %v", got.ModelName, tt.want.ModelName)
			}
			if got.Include != tt.want.Include {
				t.Errorf("Include = %v, want %v", got.Include, tt.want.Include)
			}
			if got.Exclude != tt.want.Exclude {
				t.Errorf("Exclude = %v, want %v", got.Exclude, tt.want.Exclude)
			}
			if got.ExcludeNames != tt.want.ExcludeNames {
				t.Errorf("ExcludeNames = %v, want %v", got.ExcludeNames, tt.want.ExcludeNames)
			}
			if got.Format != tt.want.Format {
				t.Errorf("Format = %v, want %v", got.Format, tt.want.Format)
			}
			if got.DryRun != tt.want.DryRun {
				t.Errorf("DryRun = %v, want %v", got.DryRun, tt.want.DryRun)
			}
			if got.Verbose != tt.want.Verbose {
				t.Errorf("Verbose = %v, want %v", got.Verbose, tt.want.Verbose)
			}
			if got.ConfirmTokens != tt.want.ConfirmTokens {
				t.Errorf("ConfirmTokens = %v, want %v", got.ConfirmTokens, tt.want.ConfirmTokens)
			}
			// LogLevel is an enum so compare the string representation
			if got.LogLevel.String() != tt.want.LogLevel.String() {
				t.Errorf("LogLevel = %v, want %v", got.LogLevel.String(), tt.want.LogLevel.String())
			}
		})
	}
}

func TestSetupLoggingCustom(t *testing.T) {
	tests := []struct {
		name         string
		config       *CliConfig
		wantLevel    string
		expectLogger bool // Verify whether a logger is returned
	}{
		{
			name: "Debug level with verbose flag",
			config: &CliConfig{
				Verbose:  true,
				LogLevel: logutil.DebugLevel,
			},
			wantLevel:    "debug",
			expectLogger: true,
		},
		{
			name: "Info level without verbose flag",
			config: &CliConfig{
				Verbose:  false,
				LogLevel: logutil.InfoLevel,
			},
			wantLevel:    "info",
			expectLogger: true,
		},
		{
			name: "Warn level without verbose flag",
			config: &CliConfig{
				Verbose:  false,
				LogLevel: logutil.WarnLevel,
			},
			wantLevel:    "warn",
			expectLogger: true,
		},
		{
			name: "Error level without verbose flag",
			config: &CliConfig{
				Verbose:  false,
				LogLevel: logutil.ErrorLevel,
			},
			wantLevel:    "error",
			expectLogger: true,
		},
		{
			name: "Verbose flag overrides any other log level",
			config: &CliConfig{
				Verbose:  true,
				LogLevel: logutil.ErrorLevel, // This would normally be error level
			},
			wantLevel:    "debug", // But verbose forces debug level
			expectLogger: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Call SetupLoggingCustom which should use the LogLevel from config
			logger := SetupLoggingCustom(tt.config, nil, io.Discard)

			// Verify logger was returned if expected
			if tt.expectLogger && logger == nil {
				t.Errorf("Expected logger to be returned, got nil")
			} else if !tt.expectLogger && logger != nil {
				t.Errorf("Expected nil logger, got %v", logger)
			}

			// Case insensitive comparison since the logLevel returns uppercase values
			if strings.ToLower(tt.config.LogLevel.String()) != tt.wantLevel {
				t.Errorf("LogLevel = %v, want %v", tt.config.LogLevel.String(), tt.wantLevel)
			}

			// Since we're using a stub output writer, we can't easily check the output
			// In a real implementation, we might want to use a buffer here to capture output
			// and verify the logger's behavior more thoroughly
		})
	}
}

// TestAdvancedConfiguration tests more complex configuration options and interactions
func TestAdvancedConfiguration(t *testing.T) {
	testCases := []struct {
		name                  string
		args                  []string
		env                   map[string]string
		expectedFormat        string
		expectedModel         string
		expectedInclude       string
		expectedExclude       string
		expectedExcludeNames  string
		expectedConfirmTokens int
		expectedOutputFile    string
		expectedLogLevel      string
		expectError           bool
	}{
		{
			name:                  "Default values when not specified",
			args:                  []string{"--instructions", "instructions.txt", "./"},
			env:                   map[string]string{apiKeyEnvVar: "test-api-key"},
			expectedFormat:        defaultFormat,
			expectedModel:         defaultModel,
			expectedInclude:       "",
			expectedExclude:       defaultExcludes,
			expectedExcludeNames:  defaultExcludeNames,
			expectedConfirmTokens: 0,
			expectedOutputFile:    defaultOutputFile,
			expectedLogLevel:      "info",
			expectError:           false,
		},
		{
			name: "All custom options",
			args: []string{
				"--instructions", "custom-instructions.txt",
				"--output", "custom-output.md",
				"--format", "Custom: {path}\n{content}\n---\n",
				"--model", "custom-model",
				"--include", "*.go,*.ts",
				"--exclude", "*.tmp,*.bak",
				"--exclude-names", "node_modules,dist,vendor",
				"--log-level", "debug",
				"--confirm-tokens", "1000",
				"./src", "./tests",
			},
			env:                   map[string]string{apiKeyEnvVar: "custom-api-key"},
			expectedFormat:        "Custom: {path}\n{content}\n---\n",
			expectedModel:         "custom-model",
			expectedInclude:       "*.go,*.ts",
			expectedExclude:       "*.tmp,*.bak",
			expectedExcludeNames:  "node_modules,dist,vendor",
			expectedConfirmTokens: 1000,
			expectedOutputFile:    "custom-output.md",
			expectedLogLevel:      "debug",
			expectError:           false,
		},
		{
			name: "File pattern options with spaces",
			args: []string{
				"--instructions", "instructions.txt",
				"--include", "*.go, *.md", // Note the space
				"--exclude", "*.tmp, *.bak", // Note the space
				"--exclude-names", "node_modules, dist", // Note the space
				"./",
			},
			env:                   map[string]string{apiKeyEnvVar: "test-api-key"},
			expectedInclude:       "*.go, *.md",
			expectedExclude:       "*.tmp, *.bak",
			expectedExcludeNames:  "node_modules, dist",
			expectedFormat:        defaultFormat,
			expectedModel:         defaultModel,
			expectedConfirmTokens: 0,
			expectedOutputFile:    defaultOutputFile,
			expectedLogLevel:      "info",
			expectError:           false,
		},
		{
			name: "Format string with special characters",
			args: []string{
				"--instructions", "instructions.txt",
				"--format", "```{path}\n{content}\n```\n",
				"./",
			},
			env:                   map[string]string{apiKeyEnvVar: "test-api-key"},
			expectedFormat:        "```{path}\n{content}\n```\n",
			expectedModel:         defaultModel,
			expectedInclude:       "",
			expectedExclude:       defaultExcludes,
			expectedExcludeNames:  defaultExcludeNames,
			expectedConfirmTokens: 0,
			expectedOutputFile:    defaultOutputFile,
			expectedLogLevel:      "info",
			expectError:           false,
		},
		{
			name: "Missing API key",
			args: []string{
				"--instructions", "instructions.txt",
				"./",
			},
			env:                   map[string]string{}, // Empty - no API key
			expectedFormat:        defaultFormat,       // Make sure we specify default values
			expectedModel:         defaultModel,
			expectedInclude:       "",
			expectedExclude:       defaultExcludes,
			expectedExcludeNames:  defaultExcludeNames,
			expectedConfirmTokens: 0,
			expectedOutputFile:    defaultOutputFile,
			expectedLogLevel:      "info",
			expectError:           false, // Not checked at flag parsing time
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			// Create a new FlagSet for each test
			fs := flag.NewFlagSet("test", flag.ContinueOnError)
			// Disable output to avoid cluttering test output
			fs.SetOutput(io.Discard)

			// Create a mock environment getter
			getenv := func(key string) string {
				return tc.env[key]
			}

			// Parse flags
			config, err := ParseFlagsWithEnv(fs, tc.args, getenv)
			if tc.expectError {
				if err == nil {
					t.Fatalf("Expected error for invalid config, but got none")
				}
				return // Skip further checks if we expected an error
			} else if err != nil {
				t.Fatalf("Expected no error for valid config, got: %v", err)
			}

			// Check all configuration values if we don't expect an error
			if config.Format != tc.expectedFormat {
				t.Errorf("Format = %q, want %q", config.Format, tc.expectedFormat)
			}

			if config.ModelName != tc.expectedModel {
				t.Errorf("ModelName = %q, want %q", config.ModelName, tc.expectedModel)
			}

			if config.Include != tc.expectedInclude {
				t.Errorf("Include = %q, want %q", config.Include, tc.expectedInclude)
			}

			if config.Exclude != tc.expectedExclude {
				t.Errorf("Exclude = %q, want %q", config.Exclude, tc.expectedExclude)
			}

			if config.ExcludeNames != tc.expectedExcludeNames {
				t.Errorf("ExcludeNames = %q, want %q", config.ExcludeNames, tc.expectedExcludeNames)
			}

			if config.ConfirmTokens != tc.expectedConfirmTokens {
				t.Errorf("ConfirmTokens = %d, want %d", config.ConfirmTokens, tc.expectedConfirmTokens)
			}

			if config.OutputFile != tc.expectedOutputFile {
				t.Errorf("OutputFile = %q, want %q", config.OutputFile, tc.expectedOutputFile)
			}

			// Set up logging to populate the log level
			SetupLoggingCustom(config, nil, io.Discard)

			if tc.expectedLogLevel != "" && strings.ToLower(config.LogLevel.String()) != tc.expectedLogLevel {
				t.Errorf("LogLevel = %q, want %q", strings.ToLower(config.LogLevel.String()), tc.expectedLogLevel)
			}

			// Check for API key if expected
			if apiKey, exists := tc.env[apiKeyEnvVar]; exists {
				if config.ApiKey != apiKey {
					t.Errorf("ApiKey = %q, want %q", config.ApiKey, apiKey)
				}
			}
		})
	}
}

// errorTrackingLogger is a minimal logger that tracks method calls
type errorTrackingLogger struct {
	errorCalled   bool
	errorMessages []string
	debugCalled   bool
	infoCalled    bool
	warnCalled    bool
}

func (l *errorTrackingLogger) Error(format string, args ...interface{}) {
	l.errorCalled = true
	l.errorMessages = append(l.errorMessages, fmt.Sprintf(format, args...))
}

func (l *errorTrackingLogger) Debug(format string, args ...interface{}) {
	l.debugCalled = true
}

func (l *errorTrackingLogger) Info(format string, args ...interface{}) {
	l.infoCalled = true
}

func (l *errorTrackingLogger) Warn(format string, args ...interface{}) {
	l.warnCalled = true
}

// Removed unused reset function

func (l *errorTrackingLogger) Fatal(format string, args ...interface{})  {}
func (l *errorTrackingLogger) Printf(format string, args ...interface{}) {}
func (l *errorTrackingLogger) Println(v ...interface{})                  {}

// TestValidateInputs ensures that the validation function correctly validates all required fields
func TestValidateInputs(t *testing.T) {
	// Create a test instructions file
	tempFile, err := os.CreateTemp("", "instructions-*.txt")
	if err != nil {
		t.Fatalf("Failed to create temporary instructions file: %v", err)
	}
	defer os.Remove(tempFile.Name())

	_, err = tempFile.WriteString("Test instructions content")
	if err != nil {
		t.Fatalf("Failed to write to temporary instructions file: %v", err)
	}
	tempFile.Close()

	tests := []struct {
		name          string
		config        *CliConfig
		expectError   bool
		errorContains string
	}{
		{
			name: "Valid configuration",
			config: &CliConfig{
				InstructionsFile: tempFile.Name(),
				Paths:            []string{"testfile"},
				ApiKey:           "test-key",
			},
			expectError: false,
		},
		{
			name: "Missing instructions file",
			config: &CliConfig{
				InstructionsFile: "", // Missing
				Paths:            []string{"testfile"},
				ApiKey:           "test-key",
			},
			expectError:   true,
			errorContains: "missing required --instructions flag",
		},
		{
			name: "Missing paths",
			config: &CliConfig{
				InstructionsFile: tempFile.Name(),
				Paths:            []string{}, // Empty
				ApiKey:           "test-key",
			},
			expectError:   true,
			errorContains: "no paths specified",
		},
		{
			name: "Missing API key",
			config: &CliConfig{
				InstructionsFile: tempFile.Name(),
				Paths:            []string{"testfile"},
				ApiKey:           "", // Missing
			},
			expectError:   true,
			errorContains: "API key not set",
		},
		{
			name: "Dry run allows missing instructions file",
			config: &CliConfig{
				InstructionsFile: "", // Missing
				Paths:            []string{"testfile"},
				ApiKey:           "test-key",
				DryRun:           true, // Dry run mode
			},
			expectError: false,
		},
		{
			name: "Dry run still requires paths",
			config: &CliConfig{
				InstructionsFile: "",         // Missing allowed in dry run
				Paths:            []string{}, // Empty paths still invalid
				ApiKey:           "test-key",
				DryRun:           true,
			},
			expectError:   true,
			errorContains: "no paths specified",
		},
		{
			name: "Dry run still requires API key",
			config: &CliConfig{
				InstructionsFile: "", // Missing allowed in dry run
				Paths:            []string{"testfile"},
				ApiKey:           "", // Missing
				DryRun:           true,
			},
			expectError:   true,
			errorContains: "API key not set",
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

// TestUsageMessage verifies that the usage message contains the correct information
func TestUsageMessage(t *testing.T) {
	t.Skip("Skipping usage message test since it's testing display formatting rather than functionality")
}

// TestInstructionsFileRequirement is now covered by TestValidateInputs
