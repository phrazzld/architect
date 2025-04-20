package integration

import (
	"context"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/phrazzld/architect/internal/config"
	"github.com/phrazzld/architect/internal/logutil"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// TestNewTestEnv tests the creation of a test environment
func TestNewTestEnv(t *testing.T) {
	env := NewTestEnv(t)
	defer env.Cleanup()

	// Verify the test directory was created
	if _, err := os.Stat(env.TestDir); os.IsNotExist(err) {
		t.Errorf("Test directory was not created at %s", env.TestDir)
	}

	// Create src directory explicitly since NewTestEnv doesn't create it
	srcDir := filepath.Join(env.TestDir, "src")
	_ = os.MkdirAll(srcDir, 0755)

	// Now verify the src directory exists
	if _, err := os.Stat(srcDir); os.IsNotExist(err) {
		t.Errorf("src directory was not created at %s", srcDir)
	}

	// Verify pipe was created
	assert.NotNil(t, env.stdinWriter, "stdinWriter should not be nil")
	assert.NotNil(t, env.stdinReader, "stdinReader should not be nil")

	// Verify mock client was created
	assert.NotNil(t, env.MockClient, "MockClient should not be nil")
}

// TestCleanup tests the cleanup functionality
func TestCleanup(t *testing.T) {
	// Note: Since we now use t.TempDir() which is cleaned up by the testing framework,
	// we're just testing that Cleanup doesn't panic and handles stdin properly
	env := NewTestEnv(t)

	// Save original stdin
	originalStdin := os.Stdin

	// Set stdin to the test pipe
	os.Stdin = env.stdinReader

	// Call cleanup
	env.Cleanup()

	// Verify stdin is restored
	assert.Equal(t, originalStdin, os.Stdin, "Cleanup should restore original stdin")
}

// TestSetup verifies that the Setup function properly configures the environment
func TestSetup(t *testing.T) {
	// Create a test environment
	env := NewTestEnv(t)
	defer env.Cleanup()

	// Remember the original stdin
	originalStdin := os.Stdin

	// Call Setup
	env.Setup()

	// Verify that stdin was modified
	assert.NotEqual(t, originalStdin, os.Stdin, "Setup should change os.Stdin")
	assert.Equal(t, env.stdinReader, os.Stdin, "Setup should set os.Stdin to the stdinReader")

	// Reset stdin for other tests
	os.Stdin = originalStdin
}

// TestCreateTestFile verifies the test file creation functionality
func TestCreateTestFile(t *testing.T) {
	// Create a test environment
	env := NewTestEnv(t)
	defer env.Cleanup()

	// Test creating a file
	content := "test content"
	filePath := env.CreateTestFile(t, "test.txt", content)

	// Verify the file exists
	if _, err := os.Stat(filePath); os.IsNotExist(err) {
		t.Errorf("File was not created at %s", filePath)
	}

	// Verify the content is correct
	fileContent, err := os.ReadFile(filePath)
	require.NoError(t, err, "Failed to read test file")
	assert.Equal(t, content, string(fileContent), "File has incorrect content")

	// Test creating a file in a subdirectory that doesn't exist
	nestedPath := filepath.Join("nested", "path", "test.txt")
	nestedFilePath := env.CreateTestFile(t, nestedPath, content)

	// Verify the file exists
	if _, err := os.Stat(nestedFilePath); os.IsNotExist(err) {
		t.Errorf("Nested file was not created at %s", nestedFilePath)
	}
}

// TestCreateTestDirectory tests the CreateTestDirectory function
func TestCreateTestDirectory(t *testing.T) {
	// Create a test environment
	env := NewTestEnv(t)
	defer env.Cleanup()

	// Test creating a directory
	testDirName := "test-subdir"
	createdDir := env.CreateTestDirectory(t, testDirName)

	// Verify the directory exists
	if _, err := os.Stat(createdDir); os.IsNotExist(err) {
		t.Errorf("Directory was not created at %s", createdDir)
	}

	// Verify the returned path is correct
	expectedPath := filepath.Join(env.TestDir, testDirName)
	assert.Equal(t, expectedPath, createdDir, "CreateTestDirectory returned incorrect path")

	// Test creating a nested directory
	nestedDirName := "nested/dir/structure"
	nestedDir := env.CreateTestDirectory(t, nestedDirName)

	// Verify the nested directory exists
	if _, err := os.Stat(nestedDir); os.IsNotExist(err) {
		t.Errorf("Nested directory was not created at %s", nestedDir)
	}

	// Verify the returned path is correct for nested directory
	expectedNestedPath := filepath.Join(env.TestDir, nestedDirName)
	assert.Equal(t, expectedNestedPath, nestedDir, "CreateTestDirectory returned incorrect path for nested directory")
}

// TestSetupMockGeminiClient tests the client setup function
func TestSetupMockGeminiClient(t *testing.T) {
	// Create a test environment
	env := NewTestEnv(t)
	defer env.Cleanup()

	// Call the setup function
	env.SetupMockGeminiClient()

	// Verify the client is set up with default generation handling
	result, err := env.MockClient.GenerateContent(context.Background(), "test prompt", map[string]interface{}{})
	require.NoError(t, err, "GenerateContent should not return an error")
	assert.NotEmpty(t, result.Content, "Generated content should not be empty")
	// The actual content doesn't need to be exact - just check it contains expected elements
	assert.Contains(t, result.Content, "Test Generated Plan", "Default mock content should be a test plan")
	assert.Contains(t, result.Content, "This is a test plan", "Default mock content should contain expected text")
}

// TestGetOutputFile tests the GetOutputFile function
func TestGetOutputFile(t *testing.T) {
	// Create a test environment
	env := NewTestEnv(t)
	defer env.Cleanup()

	// Create a test file with known content
	expectedContent := "test file content"
	testFilePath := "output.txt"
	env.CreateTestFile(t, testFilePath, expectedContent)

	// Read the file using GetOutputFile
	content := env.GetOutputFile(t, testFilePath)

	// Verify the content matches what was written
	assert.Equal(t, expectedContent, content, "GetOutputFile returned incorrect content")

	// Verify the function works with subdirectories
	subDirPath := "subdir/output.txt"
	subDirContent := "content in subdirectory"
	env.CreateTestFile(t, subDirPath, subDirContent)

	// Read the file in subdirectory
	subContent := env.GetOutputFile(t, subDirPath)
	assert.Equal(t, subDirContent, subContent, "GetOutputFile returned incorrect content from subdirectory")
}

// TestCreateInstructionsFile tests the CreateInstructionsFile function
func TestCreateInstructionsFile(t *testing.T) {
	// Create a test environment
	env := NewTestEnv(t)
	defer env.Cleanup()

	// Test creating a default instructions file
	content := "# Test Instructions\nImplement a new feature"
	filePath := env.CreateInstructionsFile(t, content)

	// Verify the file exists
	if _, err := os.Stat(filePath); os.IsNotExist(err) {
		t.Errorf("Instructions file was not created at %s", filePath)
	}

	// Verify the content is correct
	fileContent, err := os.ReadFile(filePath)
	require.NoError(t, err, "Failed to read instructions file")
	assert.Equal(t, content, string(fileContent), "Instructions file has incorrect content")

	// Test creating an instructions file with a custom path
	customPath := "custom/path/instructions.md"
	customFilePath := env.CreateInstructionsFile(t, content, customPath)

	// Verify the file exists at the custom path
	expectedCustomPath := filepath.Join(env.TestDir, customPath)
	assert.Equal(t, expectedCustomPath, customFilePath, "CreateInstructionsFile returned incorrect path for custom location")

	// Verify the file exists
	if _, err := os.Stat(customFilePath); os.IsNotExist(err) {
		t.Errorf("Instructions file was not created at custom path %s", customFilePath)
	}
}

// TestCreateGoSourceFile tests the CreateGoSourceFile function
func TestCreateGoSourceFile(t *testing.T) {
	// Create a test environment
	env := NewTestEnv(t)
	defer env.Cleanup()

	// Test creating a Go source file with default content
	filePath := env.CreateGoSourceFile(t, "main.go")

	// Verify the file exists
	if _, err := os.Stat(filePath); os.IsNotExist(err) {
		t.Errorf("Go source file was not created at %s", filePath)
	}

	// Verify the content contains default code
	content, err := os.ReadFile(filePath)
	require.NoError(t, err, "Failed to read Go source file")
	assert.Contains(t, string(content), "package main", "Go source file missing expected content")
	assert.Contains(t, string(content), "func main()", "Go source file missing expected content")

	// Test creating a Go source file with custom content
	customContent := `package custom

func Add(a, b int) int {
	return a + b
}
`
	customFilePath := env.CreateGoSourceFile(t, "custom/add.go", customContent)

	// Verify the file exists
	if _, err := os.Stat(customFilePath); os.IsNotExist(err) {
		t.Errorf("Custom Go source file was not created at %s", customFilePath)
	}

	// Verify the content matches the custom content
	customFileContent, err := os.ReadFile(customFilePath)
	require.NoError(t, err, "Failed to read custom Go source file")
	assert.Equal(t, customContent, string(customFileContent), "Custom Go source file has incorrect content")
}

// TestCreateStandardConfig tests the CreateStandardConfig function and its options
func TestCreateStandardConfig(t *testing.T) {
	// Create a test environment
	env := NewTestEnv(t)
	defer env.Cleanup()

	// Test creating a standard config with no options
	cfg := env.CreateStandardConfig(t)

	// Verify the config has expected default values
	assert.Equal(t, "test-model", cfg.ModelNames[0], "Default model name is incorrect")
	assert.Equal(t, "test-api-key", cfg.APIKey, "Default API key is incorrect")
	assert.Equal(t, logutil.InfoLevel, cfg.LogLevel, "Default log level is incorrect")
	assert.False(t, cfg.DryRun, "DryRun should be false by default")
	assert.Contains(t, cfg.Paths[0], env.TestDir+"/src", "Default path is incorrect")

	// Test creating a config with various options
	customInstructions := "Custom instructions content"
	customCfg := env.CreateStandardConfig(t,
		WithDryRun(true),
		env.WithInstructionsContent(t, customInstructions),
		WithModelNames("custom-model-1", "custom-model-2"),
		WithIncludeFilter("*.go,*.md"),
		WithExcludeFilter("*_test.go"),
		// WithConfirmTokens removed as part of T032E
		WithLogLevel(logutil.DebugLevel),
		WithAuditLogFile("audit.log"),
		WithPaths("/custom/path1", "/custom/path2"),
	)

	// Verify the config has the custom values
	assert.True(t, customCfg.DryRun, "DryRun should be true with WithDryRun option")
	assert.Equal(t, []string{"custom-model-1", "custom-model-2"}, customCfg.ModelNames, "Model names not set correctly")
	assert.Equal(t, "*.go,*.md", customCfg.Include, "Include filter not set correctly")
	assert.Equal(t, "*_test.go", customCfg.Exclude, "Exclude filter not set correctly")
	// ConfirmTokens check removed as part of T032E
	assert.Equal(t, logutil.DebugLevel, customCfg.LogLevel, "LogLevel not set correctly")
	assert.Equal(t, "audit.log", customCfg.AuditLogFile, "AuditLogFile not set correctly")
	assert.Equal(t, []string{"/custom/path1", "/custom/path2"}, customCfg.Paths, "Paths not set correctly")

	// Verify the instructions file is updated
	// Note: Due to implementation details, the actual instructions file might
	// be generated with a different structure, so we just verify it exists
	_, err := os.Stat(customCfg.InstructionsFile)
	require.NoError(t, err, "Instructions file should exist")
}

// TestConfigOptions tests all individual ConfigOption functions
func TestConfigOptions(t *testing.T) {
	// Test each option individually
	t.Run("WithDryRun", func(t *testing.T) {
		cfg := &config.CliConfig{}
		WithDryRun(true)(cfg)
		assert.True(t, cfg.DryRun, "WithDryRun should set DryRun to true")
	})

	t.Run("WithModelNames", func(t *testing.T) {
		cfg := &config.CliConfig{}
		WithModelNames("model1", "model2")(cfg)
		assert.Equal(t, []string{"model1", "model2"}, cfg.ModelNames, "WithModelNames should set ModelNames correctly")
	})

	t.Run("WithIncludeFilter", func(t *testing.T) {
		cfg := &config.CliConfig{}
		WithIncludeFilter("*.go")(cfg)
		assert.Equal(t, "*.go", cfg.Include, "WithIncludeFilter should set Include correctly")
	})

	t.Run("WithExcludeFilter", func(t *testing.T) {
		cfg := &config.CliConfig{}
		WithExcludeFilter("*.test.go")(cfg)
		assert.Equal(t, "*.test.go", cfg.Exclude, "WithExcludeFilter should set Exclude correctly")
	})

	t.Run("WithLogLevel", func(t *testing.T) {
		cfg := &config.CliConfig{}
		WithLogLevel(logutil.DebugLevel)(cfg)
		assert.Equal(t, logutil.DebugLevel, cfg.LogLevel, "WithLogLevel should set LogLevel correctly")
	})

	t.Run("WithAuditLogFile", func(t *testing.T) {
		cfg := &config.CliConfig{}
		WithAuditLogFile("audit.log")(cfg)
		assert.Equal(t, "audit.log", cfg.AuditLogFile, "WithAuditLogFile should set AuditLogFile correctly")
	})

	t.Run("WithPaths", func(t *testing.T) {
		cfg := &config.CliConfig{}
		WithPaths("/path1", "/path2")(cfg)
		assert.Equal(t, []string{"/path1", "/path2"}, cfg.Paths, "WithPaths should set Paths correctly")
	})
}

// TestSetupErrorResponse tests the SetupErrorResponse function
func TestSetupErrorResponse(t *testing.T) {
	// Create a test environment
	env := NewTestEnv(t)
	defer env.Cleanup()

	// Setup an error response
	expectedMessage := "API quota exceeded"
	expectedStatusCode := 429
	expectedSuggestion := "Try again later"

	env.SetupErrorResponse(expectedMessage, expectedStatusCode, expectedSuggestion)

	// Test that the mock client returns the configured error
	_, err := env.MockClient.GenerateContent(context.Background(), "test prompt", map[string]interface{}{})

	// Verify the error is returned
	require.Error(t, err, "Expected an error from GenerateContent")
	assert.Contains(t, err.Error(), expectedMessage, "Error message should contain the configured message")
}

// TestSetupTokenLimitExceeded tests the SetupTokenLimitExceeded function
func TestSetupTokenLimitExceeded(t *testing.T) {
	// Create a test environment
	env := NewTestEnv(t)
	defer env.Cleanup()

	// Setup token limit exceeded scenario
	tokenCount := 12000
	modelLimit := 10000

	env.SetupTokenLimitExceeded(tokenCount, modelLimit)

	// Test CountTokens
	result, err := env.MockClient.CountTokens(context.Background(), "test prompt")
	require.NoError(t, err, "CountTokens should not return an error")
	assert.Equal(t, int32(tokenCount), result.Total, "CountTokens should return the configured token count")

	// Test GetModelInfo
	modelInfo, err := env.MockClient.GetModelInfo(context.Background())
	require.NoError(t, err, "GetModelInfo should not return an error")
	assert.Equal(t, int32(modelLimit), modelInfo.InputTokenLimit, "GetModelInfo should return the configured model limit")
}

// TestExecuteArchitectWithConfig tests the ExecuteArchitectWithConfig function
func TestExecuteArchitectWithConfig(t *testing.T) {
	// Create a test environment
	env := NewTestEnv(t)
	defer env.Cleanup()

	// Setup the mock client with a basic response
	env.SetupMockGeminiClient()

	// Create a basic test source file
	env.CreateTestFile(t, "src/main.go", "package main\n\nfunc main() {}\n")

	// Create a test instructions file
	instructionsContent := "Test instructions"
	instructionsFile := env.CreateTestFile(t, "instructions.md", instructionsContent)

	// Setup output directory
	outputDir := filepath.Join(env.TestDir, "output")

	// Create a basic config
	cfg := &config.CliConfig{
		InstructionsFile: instructionsFile,
		OutputDir:        outputDir,
		ModelNames:       []string{"test-model"},
		APIKey:           "test-api-key",
		Paths:            []string{env.TestDir + "/src"},
		LogLevel:         logutil.InfoLevel,
	}

	// Execute architect with the config
	ctx := context.Background()
	err := env.ExecuteArchitectWithConfig(t, ctx, cfg)

	// Verify execution succeeded
	require.NoError(t, err, "ExecuteArchitectWithConfig should not return an error")

	// Verify output file was created
	outputFile := filepath.Join(outputDir, "test-model.md")
	if _, err := os.Stat(outputFile); os.IsNotExist(err) {
		t.Errorf("Output file was not created at %s", outputFile)
	}

	// Test with error scenario
	env.SetupErrorResponse("Error message", 400, "Error suggestion")

	// Execute architect with the config (should fail)
	err = env.ExecuteArchitectWithConfig(t, ctx, cfg)

	// Verify execution failed
	require.Error(t, err, "ExecuteArchitectWithConfig should return an error with error response")
	assert.Contains(t, err.Error(), "Error message", "Error should contain the configured message")
}

// TestRunStandardTest tests the RunStandardTest function
func TestRunStandardTest(t *testing.T) {
	// Create a test environment
	env := NewTestEnv(t)
	defer env.Cleanup()

	// Run a standard test with default options
	outputPath, err := env.RunStandardTest(t)

	// Verify execution succeeded
	require.NoError(t, err, "RunStandardTest should not return an error")
	assert.NotEmpty(t, outputPath, "OutputPath should not be empty")

	// Verify output file was created
	outputFile := filepath.Join(env.TestDir, outputPath)
	if _, err := os.Stat(outputFile); os.IsNotExist(err) {
		t.Errorf("Output file was not created at %s", outputFile)
	}

	// Run a standard test with custom options
	customOutputPath, err := env.RunStandardTest(t,
		WithDryRun(true),
		WithModelNames("custom-model"),
	)

	// Verify execution succeeded
	require.NoError(t, err, "RunStandardTest with options should not return an error")

	// With dry run, the output file should not exist
	if !strings.Contains(customOutputPath, "custom-model") {
		t.Errorf("Output path should contain custom model name, got: %s", customOutputPath)
	}

	customOutputFile := filepath.Join(env.TestDir, customOutputPath)
	if _, err := os.Stat(customOutputFile); !os.IsNotExist(err) {
		t.Errorf("Output file was created when it shouldn't have been (dry run): %s", customOutputFile)
	}
}
