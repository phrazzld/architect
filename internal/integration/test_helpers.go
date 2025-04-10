// internal/integration/test_helpers.go
package integration

import (
	"bytes"
	"context"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"testing"

	"github.com/phrazzld/architect/internal/gemini"
	"github.com/phrazzld/architect/internal/logutil"
)

// TestEnv holds the testing environment
type TestEnv struct {
	// Test directory where we'll create test files
	TestDir string

	// Captures stdout/stderr
	StdoutBuffer *bytes.Buffer
	StderrBuffer *bytes.Buffer

	// Original stdout/stderr for restoring after test
	OrigStdout *os.File
	OrigStderr *os.File

	// Mock Gemini client
	MockClient *gemini.MockClient

	// Test logger
	Logger logutil.LoggerInterface

	// Mock standard input for simulating user inputs
	MockStdin *os.File
	OrigStdin *os.File

	// Cleanup function to run after test
	Cleanup func()
}

// NewTestEnv creates a new test environment
func NewTestEnv(t *testing.T) *TestEnv {
	// Create a temporary directory for test files
	testDir, err := os.MkdirTemp("", "architect-integration-test-*")
	if err != nil {
		t.Fatalf("Failed to create test directory: %v", err)
	}

	// Create buffers to capture stdout/stderr
	stdoutBuffer := &bytes.Buffer{}
	stderrBuffer := &bytes.Buffer{}

	// Save original stdout/stderr
	origStdout := os.Stdout
	origStderr := os.Stderr

	// Create pipes for stdin simulation
	mockStdin, err := os.CreateTemp("", "mock-stdin-*")
	if err != nil {
		t.Fatalf("Failed to create mock stdin file: %v", err)
	}
	origStdin := os.Stdin

	// Create a mock client
	mockClient := gemini.NewMockClient()

	// Create a logger that writes to the stderr buffer
	logger := logutil.NewLogger(logutil.DebugLevel, stderrBuffer, "[test] ", true)

	// Create cleanup function
	cleanup := func() {
		// Remove test directory and all contents
		os.RemoveAll(testDir)

		// Restore original stdout/stderr/stdin
		os.Stdout = origStdout
		os.Stderr = origStderr
		os.Stdin = origStdin

		// Close and remove mock stdin file
		mockStdin.Close()
		os.Remove(mockStdin.Name())
	}

	return &TestEnv{
		TestDir:      testDir,
		StdoutBuffer: stdoutBuffer,
		StderrBuffer: stderrBuffer,
		OrigStdout:   origStdout,
		OrigStderr:   origStderr,
		MockClient:   mockClient,
		Logger:       logger,
		MockStdin:    mockStdin,
		OrigStdin:    origStdin,
		Cleanup:      cleanup,
	}
}

// Setup redirects stdout/stderr and prepares the environment
func (env *TestEnv) Setup() {
	// Redirect stdout/stderr to our buffers
	r, w, _ := os.Pipe()
	os.Stdout = w

	go func() {
		if _, err := io.Copy(env.StdoutBuffer, r); err != nil {
			panic(fmt.Sprintf("Failed to copy stdout: %v", err))
		}
	}()

	r2, w2, _ := os.Pipe()
	os.Stderr = w2

	go func() {
		if _, err := io.Copy(env.StderrBuffer, r2); err != nil {
			panic(fmt.Sprintf("Failed to copy stderr: %v", err))
		}
	}()

	// Set stdin to our mock
	os.Stdin = env.MockStdin
}

// SimulateUserInput writes data to mock stdin to simulate user input
func (env *TestEnv) SimulateUserInput(input string) {
	_, err := env.MockStdin.WriteString(input)
	if err != nil {
		panic(fmt.Sprintf("Failed to write to mock stdin: %v", err))
	}
	_, err = env.MockStdin.Seek(0, 0) // Rewind to start
	if err != nil {
		panic(fmt.Sprintf("Failed to seek in mock stdin: %v", err))
	}
}

// CreateTestFile creates a file with the given content in the test directory
func (env *TestEnv) CreateTestFile(t *testing.T, relativePath, content string) string {
	// Ensure parent directories exist
	fullPath := filepath.Join(env.TestDir, relativePath)
	parentDir := filepath.Dir(fullPath)

	err := os.MkdirAll(parentDir, 0755)
	if err != nil {
		t.Fatalf("Failed to create directories for test file: %v", err)
	}

	// Write the file
	err = os.WriteFile(fullPath, []byte(content), 0644)
	if err != nil {
		t.Fatalf("Failed to write test file: %v", err)
	}

	return fullPath
}

// CreateTestDirectory creates a directory in the test environment
func (env *TestEnv) CreateTestDirectory(t *testing.T, relativePath string) string {
	fullPath := filepath.Join(env.TestDir, relativePath)

	err := os.MkdirAll(fullPath, 0755)
	if err != nil {
		t.Fatalf("Failed to create test directory: %v", err)
	}

	return fullPath
}

// SetupMockGeminiClient configures the mock client with standard test responses
func (env *TestEnv) SetupMockGeminiClient() {
	// Mock CountTokens
	env.MockClient.CountTokensFunc = func(ctx context.Context, prompt string) (*gemini.TokenCount, error) {
		return &gemini.TokenCount{Total: int32(len(prompt) / 4)}, nil // Simple estimation
	}

	// Mock GetModelInfo
	env.MockClient.GetModelInfoFunc = func(ctx context.Context) (*gemini.ModelInfo, error) {
		return &gemini.ModelInfo{
			Name:             "test-model",
			InputTokenLimit:  100000, // Large enough for most tests
			OutputTokenLimit: 8192,
		}, nil
	}

	// Mock GenerateContent
	env.MockClient.GenerateContentFunc = func(ctx context.Context, prompt string) (*gemini.GenerationResult, error) {
		return &gemini.GenerationResult{
			Content:      "# Test Generated Plan\n\nThis is a test plan generated by the mock client.\n\n## Details\n\nThe plan would normally contain implementation details based on the prompt.",
			TokenCount:   1000,
			FinishReason: "STOP",
		}, nil
	}
}

// GetOutputFile reads the content of a file in the test directory
func (env *TestEnv) GetOutputFile(t *testing.T, relativePath string) string {
	fullPath := filepath.Join(env.TestDir, relativePath)

	content, err := os.ReadFile(fullPath)
	if err != nil {
		t.Fatalf("Failed to read output file: %v", err)
	}

	return string(content)
}
