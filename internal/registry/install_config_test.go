package registry

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/phrazzld/architect/internal/logutil"
)

// TestInstallDefaultConfigNoAbsolutePath verifies that installDefaultConfig doesn't use
// hardcoded absolute paths when looking for the default config file
func TestInstallDefaultConfigNoAbsolutePath(t *testing.T) {
	// Skip in CI environments where file operations might be restricted
	if os.Getenv("CI") == "true" {
		t.Skip("Skipping test in CI environment")
	}

	// Create a temp directory for testing
	tempDir, err := os.MkdirTemp("", "registry-test-*")
	if err != nil {
		t.Fatalf("Failed to create temp directory: %v", err)
	}
	defer func() {
		if err := os.RemoveAll(tempDir); err != nil {
			t.Logf("Failed to remove temp directory: %v", err)
		}
	}()

	// Create a test project directory with a config subdirectory
	projectDir := filepath.Join(tempDir, "project")
	projectConfigDir := filepath.Join(projectDir, "config")
	if err := os.MkdirAll(projectConfigDir, 0755); err != nil {
		t.Fatalf("Failed to create project config directory: %v", err)
	}

	// Create a test models.yaml file in the project config directory
	testConfigContent := "# Test config file"
	testConfigPath := filepath.Join(projectConfigDir, ModelsConfigFileName)
	if err := os.WriteFile(testConfigPath, []byte(testConfigContent), 0644); err != nil {
		t.Fatalf("Failed to write test config file: %v", err)
	}

	// Save current directory
	origWd, err := os.Getwd()
	if err != nil {
		t.Fatalf("Failed to get current working directory: %v", err)
	}

	// Change to the project directory
	if err := os.Chdir(projectDir); err != nil {
		t.Fatalf("Failed to change working directory: %v", err)
	}
	defer func() {
		if err := os.Chdir(origWd); err != nil {
			t.Logf("Failed to restore working directory: %v", err)
		}
	}() // Restore original working directory

	// Create an empty directory with no config file
	emptyDir := filepath.Join(tempDir, "empty")
	if err := os.MkdirAll(emptyDir, 0755); err != nil {
		t.Fatalf("Failed to create empty directory: %v", err)
	}

	if err := os.Chdir(emptyDir); err != nil {
		t.Fatalf("Failed to change to empty directory: %v", err)
	}

	// Create a manager and initialize it
	logger := logutil.NewLogger(logutil.InfoLevel, nil, "[test] ")
	manager := NewManager(logger)

	// This should fail, but without any hardcoded absolute paths
	err = manager.installDefaultConfig()
	if err == nil {
		t.Error("Expected installDefaultConfig to fail with no config file, but it succeeded")
	}

	// Check that the error message doesn't contain any absolute paths like "/Users/phaedrus"
	if err != nil {
		if strings.Contains(err.Error(), "/Users/phaedrus") {
			t.Errorf("Error message contains hardcoded absolute path: %v", err)
		}

		// Make sure it contains the $HOME placeholder instead
		if !strings.Contains(err.Error(), "$HOME") {
			t.Errorf("Error message should use $HOME placeholder but doesn't: %v", err)
		}
	}
}
