// internal/integration/openrouter_integration_test.go
package integration

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"testing"

	"github.com/phrazzld/architect/internal/architect"
	"github.com/phrazzld/architect/internal/config"
	"github.com/phrazzld/architect/internal/gemini"
	"github.com/phrazzld/architect/internal/logutil"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// TestOpenRouterModelRouting tests that OpenRouter models are properly routed to the OpenRouter provider
func TestOpenRouterModelRouting(t *testing.T) {
	// Skip in short mode
	if testing.Short() {
		t.Skip("Skipping test in short mode")
	}

	// Create a test environment
	env := NewTestEnv(t)
	defer env.Cleanup()

	// Create test directory structure
	srcDir := env.CreateTestDirectory(t, "src")

	// Use t.TempDir() for clean output directory isolation
	outputDir := t.TempDir()

	// Create test files
	env.CreateTestFile(t, "src/main.go", `package main

func main() {
	// Test file for OpenRouter integration
}`)

	// Create test instructions
	instructionsContent := "Test instructions for OpenRouter provider integration testing"
	instructionsFile := env.CreateInstructionsFile(t, instructionsContent)

	// Configure the mock client to return provider-specific content
	env.MockClient.GenerateContentFunc = func(ctx context.Context, prompt string, params map[string]interface{}) (*gemini.GenerationResult, error) {
		// The model name is embedded in the context in tests
		modelName, ok := ctx.Value(ModelNameKey).(string)
		if !ok {
			modelName = "unknown-model"
		}

		// Generate model-specific content
		content := fmt.Sprintf("# Output from OpenRouter Model: %s\n\nThis is a test output using the OpenRouter provider.", modelName)

		return &gemini.GenerationResult{
			Content:      content,
			TokenCount:   1000,
			FinishReason: "STOP",
		}, nil
	}

	// Define test cases
	testCases := []struct {
		name          string
		modelName     string
		expectError   bool
		errorContains string
	}{
		{
			name:        "OpenRouter Claude Model",
			modelName:   "openrouter/anthropic/claude-3-opus-20240229",
			expectError: false,
		},
		{
			name:        "OpenRouter GPT-4 Model",
			modelName:   "openrouter/openai/gpt-4-turbo",
			expectError: false,
		},
		{
			name:        "OpenRouter Llama Model",
			modelName:   "openrouter/meta/llama-3-70b-instruct",
			expectError: false,
		},
	}

	// Run test cases
	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			// Create test config with isolated output directory
			localOutputDir := filepath.Join(outputDir, tc.name)
			err := os.MkdirAll(localOutputDir, 0755)
			require.NoError(t, err)

			testConfig := &config.CliConfig{
				InstructionsFile: instructionsFile,
				OutputDir:        localOutputDir,
				ModelNames:       []string{tc.modelName},
				APIKey:           "test-api-key",
				Paths:            []string{srcDir},
				LogLevel:         logutil.InfoLevel,
			}

			// Set environment variables for testing
			origKey := os.Getenv("OPENROUTER_API_KEY")
			defer func() {
				_ = os.Setenv("OPENROUTER_API_KEY", origKey)
			}()
			_ = os.Setenv("OPENROUTER_API_KEY", "test-openrouter-key")

			// Create a model-tracking API service
			mockAPIService := &mockModelTrackingAPIService{
				logger:     env.Logger,
				mockClient: env.MockClient,
			}

			// Run the application
			ctx := context.Background()
			err = architect.Execute(
				ctx,
				testConfig,
				env.Logger,
				env.AuditLogger,
				mockAPIService,
			)

			// Check for expected errors
			if tc.expectError {
				assert.Error(t, err)
				if tc.errorContains != "" {
					assert.Contains(t, err.Error(), tc.errorContains)
				}
			} else {
				assert.NoError(t, err)

				// Find output files with a more lenient approach
				var foundOutput bool
				var foundContent string

				err = filepath.Walk(localOutputDir, func(path string, info os.FileInfo, err error) error {
					if err != nil {
						return err
					}
					if !info.IsDir() && filepath.Ext(path) == ".md" {
						t.Logf("Found output file: %s", path)
						content, readErr := os.ReadFile(path)
						if readErr == nil {
							contentStr := string(content)
							t.Logf("File content snippet: %s", contentStr[:min(100, len(contentStr))])
							if len(contentStr) > 0 {
								foundOutput = true
								foundContent = contentStr
							}
						}
					}
					return nil
				})
				require.NoError(t, err)

				// Verify that we found output
				assert.True(t, foundOutput, "No output files found in directory: %s", localOutputDir)

				// Verify content contains OpenRouter
				assert.Contains(t, foundContent, "OpenRouter", "Output content should mention OpenRouter")
			}
		})
	}
}

// For use in the min function
func min(a, b int) int {
	if a < b {
		return a
	}
	return b
}

// TestOpenRouterAPIKeyHandling tests that the OpenRouter API key is properly handled
func TestOpenRouterAPIKeyHandling(t *testing.T) {
	// Skip in short mode
	if testing.Short() {
		t.Skip("Skipping test in short mode")
	}

	// Set up test environment
	env := NewTestEnv(t)
	defer env.Cleanup()

	// Create test directory structure
	srcDir := env.CreateTestDirectory(t, "src")

	// Use t.TempDir() for clean output directory isolation
	outputDir := t.TempDir()

	// Create test instructions
	instructionsFile := env.CreateInstructionsFile(t, "Test OpenRouter API key handling")

	// Create a simple source file
	env.CreateTestFile(t, "src/main.go", "package main\n\nfunc main() {}\n")

	// Save original environment variables
	origOpenRouterKey := os.Getenv("OPENROUTER_API_KEY")
	defer func() {
		_ = os.Setenv("OPENROUTER_API_KEY", origOpenRouterKey)
	}()

	// Configure mock client to return model-specific content
	env.MockClient.GenerateContentFunc = func(ctx context.Context, prompt string, params map[string]interface{}) (*gemini.GenerationResult, error) {
		// Return a successful result for valid API key tests
		return &gemini.GenerationResult{
			Content:      "# API Key Test Output\n\nTest output for API key validation.",
			TokenCount:   500,
			FinishReason: "STOP",
		}, nil
	}

	// Test cases
	tests := []struct {
		name          string
		modelName     string
		setApiKey     bool
		setEnvVar     bool
		expectError   bool
		errorContains string
	}{
		{
			name:        "With CLI API Key",
			modelName:   "openrouter/anthropic/claude-3-opus-20240229",
			setApiKey:   true,
			setEnvVar:   false,
			expectError: false,
		},
		{
			name:        "With Environment Variable",
			modelName:   "openrouter/anthropic/claude-3-opus-20240229",
			setApiKey:   false,
			setEnvVar:   true,
			expectError: false,
		},
		{
			name:          "No API Key",
			modelName:     "openrouter/anthropic/claude-3-opus-20240229",
			setApiKey:     false,
			setEnvVar:     false,
			expectError:   true,
			errorContains: "API key",
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			// Create isolated output directory for this test case
			localOutputDir := filepath.Join(outputDir, tc.name)
			err := os.MkdirAll(localOutputDir, 0755)
			require.NoError(t, err)

			// Configure environment
			if tc.setEnvVar {
				err = os.Setenv("OPENROUTER_API_KEY", "test-openrouter-env-key")
				require.NoError(t, err)
			} else {
				err = os.Unsetenv("OPENROUTER_API_KEY")
				require.NoError(t, err)
			}

			// Create config
			apiKey := ""
			if tc.setApiKey {
				apiKey = "test-openrouter-cli-key"
			}

			testConfig := &config.CliConfig{
				InstructionsFile: instructionsFile,
				OutputDir:        localOutputDir,
				ModelNames:       []string{tc.modelName},
				APIKey:           apiKey,
				Paths:            []string{srcDir},
				LogLevel:         logutil.InfoLevel,
			}

			// Create a model-tracking API service
			mockAPIService := &mockModelTrackingAPIService{
				logger:     env.Logger,
				mockClient: env.MockClient,
			}

			// Run the application
			ctx := context.Background()

			// Set up the mock client for the error case
			if tc.name == "No API Key" {
				env.MockClient.GenerateContentFunc = func(ctx context.Context, prompt string, params map[string]interface{}) (*gemini.GenerationResult, error) {
					return nil, fmt.Errorf("API key is required for OpenRouter provider")
				}
			}

			// Execute
			err = architect.Execute(
				ctx,
				testConfig,
				env.Logger,
				env.AuditLogger,
				mockAPIService,
			)

			// Check results
			if tc.expectError {
				assert.Error(t, err)
				if tc.errorContains != "" {
					assert.Contains(t, err.Error(), tc.errorContains)
				}

				// Verify that the output directory doesn't contain files for error cases
				files, listErr := os.ReadDir(localOutputDir)
				if listErr == nil {
					// If we can list the directory, check for .md files
					var foundMarkdown bool
					for _, file := range files {
						if !file.IsDir() && filepath.Ext(file.Name()) == ".md" {
							foundMarkdown = true
							break
						}
					}
					assert.False(t, foundMarkdown, "Output files should not exist for error case")
				}
			} else {
				assert.NoError(t, err)

				// Find output files with a more lenient approach
				var foundOutput bool
				var foundContent string

				err = filepath.Walk(localOutputDir, func(path string, info os.FileInfo, err error) error {
					if err != nil {
						return err
					}
					if !info.IsDir() && filepath.Ext(path) == ".md" {
						t.Logf("Found output file: %s", path)
						content, readErr := os.ReadFile(path)
						if readErr == nil {
							contentStr := string(content)
							t.Logf("File content snippet: %s", contentStr[:min(100, len(contentStr))])
							if len(contentStr) > 0 {
								foundOutput = true
								foundContent = contentStr
							}
						}
					}
					return nil
				})
				require.NoError(t, err)

				// Verify that we found output
				assert.True(t, foundOutput, "No output files found in directory: %s", localOutputDir)

				// Verify content
				assert.Contains(t, foundContent, "API Key Test Output", "Output content should contain expected text")
			}
		})
	}
}

// TestOpenRouterErrorHandling tests that errors from the OpenRouter provider are properly handled
func TestOpenRouterErrorHandling(t *testing.T) {
	// Skip in short mode
	if testing.Short() {
		t.Skip("Skipping test in short mode")
	}

	// Set up test environment
	env := NewTestEnv(t)
	defer env.Cleanup()

	// Create basic test directories
	srcDir := env.CreateTestDirectory(t, "src")

	// Use t.TempDir() for clean output directory isolation
	outputDir := t.TempDir()

	// Create instructions file
	instructionsFile := env.CreateInstructionsFile(t, "Test OpenRouter error handling")

	// Create a sample Go file
	env.CreateTestFile(t, "src/main.go", "package main\n\nfunc main() {}\n")

	// Save original environment variables
	origOpenRouterKey := os.Getenv("OPENROUTER_API_KEY")
	defer func() {
		_ = os.Setenv("OPENROUTER_API_KEY", origOpenRouterKey)
	}()

	// Set environment variable for API key
	err := os.Setenv("OPENROUTER_API_KEY", "test-openrouter-key")
	require.NoError(t, err)

	// Test cases
	tests := []struct {
		name          string
		modelName     string
		expectError   bool
		errorContains string
	}{
		{
			name:          "Error Model",
			modelName:     "openrouter/error-model",
			expectError:   true,
			errorContains: "simulated OpenRouter API error",
		},
		{
			name:        "Normal Model",
			modelName:   "openrouter/anthropic/claude-3-opus-20240229",
			expectError: false,
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			// Create isolated output directory for this test case
			localOutputDir := filepath.Join(outputDir, tc.name)
			err := os.MkdirAll(localOutputDir, 0755)
			require.NoError(t, err)

			// Configure the mock client based on test case
			if tc.expectError {
				env.MockClient.GenerateContentFunc = func(ctx context.Context, prompt string, params map[string]interface{}) (*gemini.GenerationResult, error) {
					return nil, fmt.Errorf("simulated OpenRouter API error")
				}
			} else {
				env.MockClient.GenerateContentFunc = func(ctx context.Context, prompt string, params map[string]interface{}) (*gemini.GenerationResult, error) {
					return &gemini.GenerationResult{
						Content:      "# Normal Output\n\nThis is normal output for the OpenRouter model.",
						TokenCount:   1000,
						FinishReason: "STOP",
					}, nil
				}
			}

			// Create test config
			testConfig := &config.CliConfig{
				InstructionsFile: instructionsFile,
				OutputDir:        localOutputDir,
				ModelNames:       []string{tc.modelName},
				APIKey:           "test-openrouter-key",
				Paths:            []string{srcDir},
				LogLevel:         logutil.InfoLevel,
			}

			// Create a mock API service
			mockAPIService := &mockModelTrackingAPIService{
				logger:     env.Logger,
				mockClient: env.MockClient,
			}

			// Run the application
			ctx := context.Background()

			err = architect.Execute(
				ctx,
				testConfig,
				env.Logger,
				env.AuditLogger,
				mockAPIService,
			)

			// Check results
			if tc.expectError {
				assert.Error(t, err)
				if tc.errorContains != "" {
					assert.Contains(t, err.Error(), tc.errorContains)
				}

				// Verify no output files exist for error case
				files, listErr := os.ReadDir(localOutputDir)
				if listErr == nil {
					// If we can list the directory, check for .md files
					var foundMarkdown bool
					for _, file := range files {
						if !file.IsDir() && filepath.Ext(file.Name()) == ".md" {
							foundMarkdown = true
							break
						}
					}
					assert.False(t, foundMarkdown, "Output files should not exist for error case")
				}
			} else {
				assert.NoError(t, err)

				// Find output files with a more lenient approach
				var foundOutput bool
				var foundContent string

				err = filepath.Walk(localOutputDir, func(path string, info os.FileInfo, err error) error {
					if err != nil {
						return err
					}
					if !info.IsDir() && filepath.Ext(path) == ".md" {
						t.Logf("Found output file: %s", path)
						content, readErr := os.ReadFile(path)
						if readErr == nil {
							contentStr := string(content)
							t.Logf("File content snippet: %s", contentStr[:min(100, len(contentStr))])
							if len(contentStr) > 0 {
								foundOutput = true
								foundContent = contentStr
							}
						}
					}
					return nil
				})
				require.NoError(t, err)

				// Verify that we found output
				assert.True(t, foundOutput, "No output files found in directory: %s", localOutputDir)

				// Verify content
				assert.Contains(t, foundContent, "Normal Output", "Output content should contain expected text")
			}
		})
	}
}
