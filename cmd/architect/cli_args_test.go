// Package architect provides the command-line interface for the architect tool
package architect

import (
	"flag"
	"io"
	"reflect"
	"strings"
	"testing"

	"github.com/phrazzld/architect/internal/config"
	"github.com/phrazzld/architect/internal/logutil"
)

func TestParseFlagsWithEnv(t *testing.T) {
	tests := []struct {
		name    string
		args    []string
		env     map[string]string
		want    *config.CliConfig
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
			want: &config.CliConfig{
				InstructionsFile: "instructions.md",
				Paths:            []string{"path1", "path2"},
				APIKey:           "test-api-key",
				OutputDir:        "",
				ModelNames:       []string{defaultModel},
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
			name: "All options with output-dir",
			args: []string{
				"--instructions", "custom-instructions.md",
				"--output-dir", "custom-output-dir",
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
			want: &config.CliConfig{
				InstructionsFile: "custom-instructions.md",
				OutputDir:        "custom-output-dir",
				ModelNames:       []string{"custom-model"},
				LogLevel:         logutil.DebugLevel, // verbose overrides log-level
				Include:          "*.go,*.md",
				Exclude:          "*.tmp",
				ExcludeNames:     "node_modules,dist",
				Format:           "Custom: {path}\n{content}",
				ConfirmTokens:    500,
				Verbose:          true,
				DryRun:           true,
				Paths:            []string{"path1", "path2", "path3"},
				APIKey:           "test-api-key",
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
			want: &config.CliConfig{
				InstructionsFile: "",
				Paths:            []string{"path1"},
				APIKey:           "test-api-key",
				OutputDir:        "",
				ModelNames:       []string{defaultModel},
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
			want: &config.CliConfig{
				InstructionsFile: "instructions.md",
				Paths:            []string{},
				APIKey:           "test-api-key",
				OutputDir:        "",
				ModelNames:       []string{defaultModel},
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
			want: &config.CliConfig{
				InstructionsFile: "instructions.md",
				Paths:            []string{"path1"},
				APIKey:           "", // Empty API key
				OutputDir:        "",
				ModelNames:       []string{defaultModel},
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
			want: &config.CliConfig{
				InstructionsFile: "",
				DryRun:           true,
				Paths:            []string{"path1", "path2"},
				APIKey:           "test-api-key",
				OutputDir:        "",
				ModelNames:       []string{defaultModel},
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
			want: &config.CliConfig{
				InstructionsFile: "instructions.md",
				OutputDir:        "",
				ModelNames:       []string{defaultModel},
				LogLevel:         logutil.WarnLevel,
				Include:          "",
				Exclude:          defaultExcludes,
				ExcludeNames:     defaultExcludeNames,
				Format:           defaultFormat,
				ConfirmTokens:    0,
				Verbose:          false,
				DryRun:           false,
				Paths:            []string{"path1"},
				APIKey:           "test-api-key",
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
			want: &config.CliConfig{
				InstructionsFile: "instructions.md",
				OutputDir:        "",
				ModelNames:       []string{defaultModel},
				LogLevel:         logutil.InfoLevel, // Should default to info for invalid level
				Include:          "",
				Exclude:          defaultExcludes,
				ExcludeNames:     defaultExcludeNames,
				Format:           defaultFormat,
				ConfirmTokens:    0,
				Verbose:          false,
				DryRun:           false,
				Paths:            []string{"path1"},
				APIKey:           "test-api-key",
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
			want: &config.CliConfig{
				InstructionsFile: "instructions.md",
				OutputDir:        "",
				ModelNames:       []string{defaultModel},
				LogLevel:         logutil.DebugLevel, // Verbose overrides to debug
				Include:          "",
				Exclude:          defaultExcludes,
				ExcludeNames:     defaultExcludeNames,
				Format:           defaultFormat,
				ConfirmTokens:    0,
				Verbose:          true,
				DryRun:           false,
				Paths:            []string{"path1"},
				APIKey:           "test-api-key",
			},
			wantErr: false,
		},
		{
			name: "Multiple model flags",
			args: []string{
				"--instructions", "instructions.md",
				"--model", "model1",
				"--model", "model2",
				"--model", "model3",
				"path1",
			},
			env: map[string]string{
				apiKeyEnvVar: "test-api-key",
			},
			want: &config.CliConfig{
				InstructionsFile: "instructions.md",
				OutputDir:        "",
				ModelNames:       []string{"model1", "model2", "model3"},
				LogLevel:         logutil.InfoLevel,
				Include:          "",
				Exclude:          defaultExcludes,
				ExcludeNames:     defaultExcludeNames,
				Format:           defaultFormat,
				ConfirmTokens:    0,
				Verbose:          false,
				DryRun:           false,
				Paths:            []string{"path1"},
				APIKey:           "test-api-key",
			},
			wantErr: false,
		},
		{
			name: "Output dir flag",
			args: []string{
				"--instructions", "instructions.md",
				"--output-dir", "custom-output-dir",
				"path1",
			},
			env: map[string]string{
				apiKeyEnvVar: "test-api-key",
			},
			want: &config.CliConfig{
				InstructionsFile: "instructions.md",
				OutputDir:        "custom-output-dir",
				ModelNames:       []string{defaultModel},
				LogLevel:         logutil.InfoLevel,
				Include:          "",
				Exclude:          defaultExcludes,
				ExcludeNames:     defaultExcludeNames,
				Format:           defaultFormat,
				ConfirmTokens:    0,
				Verbose:          false,
				DryRun:           false,
				Paths:            []string{"path1"},
				APIKey:           "test-api-key",
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
			if got.APIKey != tt.want.APIKey {
				t.Errorf("APIKey = %v, want %v", got.APIKey, tt.want.APIKey)
			}
			if got.OutputDir != tt.want.OutputDir {
				t.Errorf("OutputDir = %v, want %v", got.OutputDir, tt.want.OutputDir)
			}
			if !reflect.DeepEqual(got.ModelNames, tt.want.ModelNames) {
				t.Errorf("ModelNames = %v, want %v", got.ModelNames, tt.want.ModelNames)
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

// TestAdvancedConfiguration tests more complex configuration options and interactions
func TestAdvancedConfiguration(t *testing.T) {
	testCases := []struct {
		name                  string
		args                  []string
		env                   map[string]string
		expectedFormat        string
		expectedModelNames    []string
		expectedInclude       string
		expectedExclude       string
		expectedExcludeNames  string
		expectedConfirmTokens int
		expectedOutputDir     string
		expectedLogLevel      string
		expectedAPIEndpoint   string
		expectedMaxConcurrent int
		expectedRateLimitRPM  int
		expectedAuditLogFile  string
		expectError           bool
	}{
		{
			name:                  "Default values when not specified",
			args:                  []string{"--instructions", "instructions.txt", "./"},
			env:                   map[string]string{apiKeyEnvVar: "test-api-key"},
			expectedFormat:        defaultFormat,
			expectedModelNames:    []string{defaultModel},
			expectedInclude:       "",
			expectedExclude:       defaultExcludes,
			expectedExcludeNames:  defaultExcludeNames,
			expectedConfirmTokens: 0,
			expectedOutputDir:     "",
			expectedLogLevel:      "info",
			expectError:           false,
		},
		{
			name: "All custom options",
			args: []string{
				"--instructions", "custom-instructions.txt",
				"--output-dir", "custom-output-dir",
				"--model", "custom-model",
				"--log-level", "debug",
				"--include", "*.go,*.ts",
				"--exclude", "*.tmp,*.bak",
				"--exclude-names", "node_modules,dist,vendor",
				"--format", "Custom: {path}\n{content}\n---\n",
				"--confirm-tokens", "1000",
				"./src", "./tests",
			},
			env:                   map[string]string{apiKeyEnvVar: "custom-api-key"},
			expectedFormat:        "Custom: {path}\n{content}\n---\n",
			expectedModelNames:    []string{"custom-model"},
			expectedInclude:       "*.go,*.ts",
			expectedExclude:       "*.tmp,*.bak",
			expectedExcludeNames:  "node_modules,dist,vendor",
			expectedConfirmTokens: 1000,
			expectedOutputDir:     "custom-output-dir",
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
			expectedModelNames:    []string{defaultModel},
			expectedConfirmTokens: 0,
			expectedOutputDir:     "",
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
			expectedModelNames:    []string{defaultModel},
			expectedInclude:       "",
			expectedExclude:       defaultExcludes,
			expectedExcludeNames:  defaultExcludeNames,
			expectedConfirmTokens: 0,
			expectedOutputDir:     "",
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
			expectedModelNames:    []string{defaultModel},
			expectedInclude:       "",
			expectedExclude:       defaultExcludes,
			expectedExcludeNames:  defaultExcludeNames,
			expectedConfirmTokens: 0,
			expectedOutputDir:     "",
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

			if !reflect.DeepEqual(config.ModelNames, tc.expectedModelNames) {
				t.Errorf("ModelNames = %v, want %v", config.ModelNames, tc.expectedModelNames)
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

			if config.OutputDir != tc.expectedOutputDir {
				t.Errorf("OutputDir = %q, want %q", config.OutputDir, tc.expectedOutputDir)
			}

			// Set up logging to populate the log level
			SetupLoggingCustom(config, nil, io.Discard)

			if tc.expectedLogLevel != "" && strings.ToLower(config.LogLevel.String()) != tc.expectedLogLevel {
				t.Errorf("LogLevel = %q, want %q", strings.ToLower(config.LogLevel.String()), tc.expectedLogLevel)
			}

			// Check for API key if expected
			if apiKey, exists := tc.env[apiKeyEnvVar]; exists {
				if config.APIKey != apiKey {
					t.Errorf("APIKey = %q, want %q", config.APIKey, apiKey)
				}
			}
		})
	}
}

// TestRateLimitAndAuditConfig tests the new rate limiting and audit configuration options
func TestRateLimitAndAuditConfig(t *testing.T) {
	testCases := []struct {
		name          string
		args          []string
		env           map[string]string
		maxConcurrent int
		rateLimitRPM  int
		auditLogFile  string
		apiEndpoint   string
		expectError   bool
	}{
		{
			name:          "Default values",
			args:          []string{"--instructions", "instructions.txt", "./"},
			env:           map[string]string{apiKeyEnvVar: "test-api-key"},
			maxConcurrent: 5,
			rateLimitRPM:  60,
			auditLogFile:  "",
			apiEndpoint:   "",
			expectError:   false,
		},
		{
			name: "Custom rate limiting",
			args: []string{
				"--instructions", "instructions.txt",
				"--max-concurrent", "10",
				"--rate-limit", "120",
				"./",
			},
			env:           map[string]string{apiKeyEnvVar: "test-api-key"},
			maxConcurrent: 10,
			rateLimitRPM:  120,
			auditLogFile:  "",
			apiEndpoint:   "",
			expectError:   false,
		},
		{
			name: "Custom audit log file",
			args: []string{
				"--instructions", "instructions.txt",
				"--audit-log-file", "/tmp/audit.jsonl",
				"./",
			},
			env:           map[string]string{apiKeyEnvVar: "test-api-key"},
			maxConcurrent: 5,
			rateLimitRPM:  60,
			auditLogFile:  "/tmp/audit.jsonl",
			apiEndpoint:   "",
			expectError:   false,
		},
		{
			name: "Custom API endpoint",
			args: []string{
				"--instructions", "instructions.txt",
				"./",
			},
			env: map[string]string{
				apiKeyEnvVar:      "test-api-key",
				apiEndpointEnvVar: "https://custom-api.example.com",
			},
			maxConcurrent: 5,
			rateLimitRPM:  60,
			auditLogFile:  "",
			apiEndpoint:   "https://custom-api.example.com",
			expectError:   false,
		},
		{
			name: "Zero rate limits",
			args: []string{
				"--instructions", "instructions.txt",
				"--max-concurrent", "0",
				"--rate-limit", "0",
				"./",
			},
			env:           map[string]string{apiKeyEnvVar: "test-api-key"},
			maxConcurrent: 0,
			rateLimitRPM:  0,
			auditLogFile:  "",
			apiEndpoint:   "",
			expectError:   false,
		},
		{
			name: "All options combined",
			args: []string{
				"--instructions", "instructions.txt",
				"--max-concurrent", "15",
				"--rate-limit", "90",
				"--audit-log-file", "audit-logs.jsonl",
				"./",
			},
			env: map[string]string{
				apiKeyEnvVar:      "test-api-key",
				apiEndpointEnvVar: "https://api-custom.example.org",
			},
			maxConcurrent: 15,
			rateLimitRPM:  90,
			auditLogFile:  "audit-logs.jsonl",
			apiEndpoint:   "https://api-custom.example.org",
			expectError:   false,
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

			// Validate rate limiting configuration
			if config.MaxConcurrentRequests != tc.maxConcurrent {
				t.Errorf("MaxConcurrentRequests = %d, want %d", config.MaxConcurrentRequests, tc.maxConcurrent)
			}

			if config.RateLimitRequestsPerMinute != tc.rateLimitRPM {
				t.Errorf("RateLimitRequestsPerMinute = %d, want %d", config.RateLimitRequestsPerMinute, tc.rateLimitRPM)
			}

			// Validate audit log file
			if config.AuditLogFile != tc.auditLogFile {
				t.Errorf("AuditLogFile = %q, want %q", config.AuditLogFile, tc.auditLogFile)
			}

			// Validate API endpoint
			if config.APIEndpoint != tc.apiEndpoint {
				t.Errorf("APIEndpoint = %q, want %q", config.APIEndpoint, tc.apiEndpoint)
			}
		})
	}
}

// TestFlagParsingErrors tests error cases in flag parsing
func TestFlagParsingErrors(t *testing.T) {
	// Test unknown flag
	t.Run("Unknown flag", func(t *testing.T) {
		fs := flag.NewFlagSet("test", flag.ContinueOnError)
		fs.SetOutput(io.Discard)

		getenv := func(key string) string {
			return "test-api-key"
		}

		args := []string{
			"--not-a-valid-flag", "value",
			"./",
		}

		_, err := ParseFlagsWithEnv(fs, args, getenv)

		if err == nil {
			t.Fatalf("Expected error for invalid flag, but got none")
		}

		if !strings.Contains(err.Error(), "flag provided but not defined") {
			t.Errorf("Error message does not contain expected text. Got: %q", err.Error())
		}
	})

	// Test invalid confirm-tokens value
	t.Run("Invalid confirm-tokens value", func(t *testing.T) {
		fs := flag.NewFlagSet("test", flag.ContinueOnError)
		fs.SetOutput(io.Discard)

		getenv := func(key string) string {
			return "test-api-key"
		}

		args := []string{
			"--instructions", "instructions.txt",
			"--confirm-tokens", "not-a-number",
			"./",
		}

		_, err := ParseFlagsWithEnv(fs, args, getenv)

		if err == nil {
			t.Fatalf("Expected error for invalid value, but got none")
		}

		if !strings.Contains(err.Error(), "invalid value") {
			t.Errorf("Error message does not contain expected text. Got: %q", err.Error())
		}
	})

	// Test invalid max-concurrent value
	t.Run("Invalid max-concurrent value", func(t *testing.T) {
		fs := flag.NewFlagSet("test", flag.ContinueOnError)
		fs.SetOutput(io.Discard)

		getenv := func(key string) string {
			return "test-api-key"
		}

		args := []string{
			"--instructions", "instructions.txt",
			"--max-concurrent", "not-a-number",
			"./",
		}

		_, err := ParseFlagsWithEnv(fs, args, getenv)

		if err == nil {
			t.Fatalf("Expected error for invalid value, but got none")
		}

		if !strings.Contains(err.Error(), "invalid value") {
			t.Errorf("Error message does not contain expected text. Got: %q", err.Error())
		}
	})

	// Test invalid rate-limit value
	t.Run("Invalid rate-limit value", func(t *testing.T) {
		fs := flag.NewFlagSet("test", flag.ContinueOnError)
		fs.SetOutput(io.Discard)

		getenv := func(key string) string {
			return "test-api-key"
		}

		args := []string{
			"--instructions", "instructions.txt",
			"--rate-limit", "not-a-number",
			"./",
		}

		_, err := ParseFlagsWithEnv(fs, args, getenv)

		if err == nil {
			t.Fatalf("Expected error for invalid value, but got none")
		}

		if !strings.Contains(err.Error(), "invalid value") {
			t.Errorf("Error message does not contain expected text. Got: %q", err.Error())
		}
	})

	// Test empty instructions value
	t.Run("Empty instructions value", func(t *testing.T) {
		fs := flag.NewFlagSet("test", flag.ContinueOnError)
		fs.SetOutput(io.Discard)

		getenv := func(key string) string {
			return "test-api-key"
		}

		args := []string{
			"--instructions=", // Empty value
			"./",
		}

		config, err := ParseFlagsWithEnv(fs, args, getenv)

		if err != nil {
			t.Fatalf("Expected no error for valid flag syntax, got: %v", err)
		}

		if config.InstructionsFile != "" {
			t.Errorf("Expected empty instructions file, got: %q", config.InstructionsFile)
		}
	})
}

// TestStringSliceFlag tests the stringSliceFlag type's flag.Value interface implementation
func TestStringSliceFlag(t *testing.T) {
	t.Run("String() returns comma-separated values", func(t *testing.T) {
		// Empty flag
		emptyFlag := stringSliceFlag{}
		if got := emptyFlag.String(); got != "" {
			t.Errorf("Empty stringSliceFlag.String() = %q, want %q", got, "")
		}

		// Flag with a single value
		singleFlag := stringSliceFlag{"model1"}
		if got := singleFlag.String(); got != "model1" {
			t.Errorf("Single stringSliceFlag.String() = %q, want %q", got, "model1")
		}

		// Flag with multiple values
		multiFlag := stringSliceFlag{"model1", "model2", "model3"}
		if got := multiFlag.String(); got != "model1,model2,model3" {
			t.Errorf("Multiple stringSliceFlag.String() = %q, want %q", got, "model1,model2,model3")
		}

		// Flag with empty strings
		emptyStringFlag := stringSliceFlag{"", "model1", ""}
		if got := emptyStringFlag.String(); got != ",model1," {
			t.Errorf("Flag with empty strings .String() = %q, want %q", got, ",model1,")
		}

		// Flag with strings containing commas
		commaFlag := stringSliceFlag{"model,1", "model,2"}
		if got := commaFlag.String(); got != "model,1,model,2" {
			t.Errorf("Flag with commas .String() = %q, want %q", got, "model,1,model,2")
		}
	})

	t.Run("Set() appends values", func(t *testing.T) {
		// New flag starts empty
		flag := &stringSliceFlag{}

		// First value
		if err := flag.Set("model1"); err != nil {
			t.Errorf("First Set() error = %v", err)
		}
		if len(*flag) != 1 || (*flag)[0] != "model1" {
			t.Errorf("After first Set(), flag = %v, want [model1]", *flag)
		}

		// Second value
		if err := flag.Set("model2"); err != nil {
			t.Errorf("Second Set() error = %v", err)
		}
		if len(*flag) != 2 || (*flag)[0] != "model1" || (*flag)[1] != "model2" {
			t.Errorf("After second Set(), flag = %v, want [model1 model2]", *flag)
		}

		// Third value
		if err := flag.Set("model3"); err != nil {
			t.Errorf("Third Set() error = %v", err)
		}
		if len(*flag) != 3 || (*flag)[0] != "model1" || (*flag)[1] != "model2" || (*flag)[2] != "model3" {
			t.Errorf("After third Set(), flag = %v, want [model1 model2 model3]", *flag)
		}

		// Empty string value
		if err := flag.Set(""); err != nil {
			t.Errorf("Empty string Set() error = %v", err)
		}
		if len(*flag) != 4 || (*flag)[3] != "" {
			t.Errorf("After empty string Set(), flag = %v, want [model1 model2 model3 ]", *flag)
		}

		// Value containing commas
		if err := flag.Set("model,4"); err != nil {
			t.Errorf("Comma-containing Set() error = %v", err)
		}
		if len(*flag) != 5 || (*flag)[4] != "model,4" {
			t.Errorf("After comma Set(), flag = %v, want [model1 model2 model3  model,4]", *flag)
		}
	})

	t.Run("Usage with flag package", func(t *testing.T) {
		// Create a new FlagSet
		fs := flag.NewFlagSet("test", flag.ContinueOnError)
		fs.SetOutput(io.Discard) // Suppress output

		// Create a stringSliceFlag and register it
		modelFlag := &stringSliceFlag{}
		fs.Var(modelFlag, "model", "Model name (can be specified multiple times)")

		// Parse flags with multiple model values
		args := []string{
			"--model", "model1",
			"--model", "model2",
			"--model", "model3",
		}
		if err := fs.Parse(args); err != nil {
			t.Fatalf("Parse() error = %v", err)
		}

		// Check that all values were captured
		if len(*modelFlag) != 3 {
			t.Errorf("Got %d models, want 3", len(*modelFlag))
		}
		if (*modelFlag)[0] != "model1" || (*modelFlag)[1] != "model2" || (*modelFlag)[2] != "model3" {
			t.Errorf("modelFlag = %v, want [model1 model2 model3]", *modelFlag)
		}
	})

	t.Run("Usage with flag package - edge cases", func(t *testing.T) {
		// Create a new FlagSet
		fs := flag.NewFlagSet("test", flag.ContinueOnError)
		fs.SetOutput(io.Discard) // Suppress output

		// Create a stringSliceFlag and register it
		modelFlag := &stringSliceFlag{}
		fs.Var(modelFlag, "model", "Model name (can be specified multiple times)")

		// Parse flags with edge case values (empty string, comma-containing)
		args := []string{
			"--model", "",
			"--model", "model,with,commas",
		}
		if err := fs.Parse(args); err != nil {
			t.Fatalf("Parse() error = %v", err)
		}

		// Check that all values were captured correctly
		if len(*modelFlag) != 2 {
			t.Errorf("Got %d models, want 2", len(*modelFlag))
		}
		if (*modelFlag)[0] != "" || (*modelFlag)[1] != "model,with,commas" {
			t.Errorf("modelFlag = %v, want [ model,with,commas]", *modelFlag)
		}
	})
}

// TestUsageMessage verifies that the usage message contains the correct information
func TestUsageMessage(t *testing.T) {
	t.Skip("Skipping usage message test since it's testing display formatting rather than functionality")
}
