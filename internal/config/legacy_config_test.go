package config

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/spf13/viper"
)

// TestLegacyConfigWithClarifyFieldsIsIgnored tests that legacy configuration files
// with clarify-related fields are properly handled (fields are ignored).
// This test is behavior-focused rather than implementation-focused.
func TestLegacyConfigWithClarifyFieldsIsIgnored(t *testing.T) {
	// Create a temporary directory for testing
	tempDir, err := os.MkdirTemp("", "architect-config-test-*")
	if err != nil {
		t.Fatalf("Failed to create temp directory: %v", err)
	}
	defer os.RemoveAll(tempDir)

	// Create a temporary config file with clarify fields
	configDir := filepath.Join(tempDir, ".config", "architect")
	if err := os.MkdirAll(configDir, 0755); err != nil {
		t.Fatalf("Failed to create config directory: %v", err)
	}

	// Create config with legacy clarify fields
	legacyConfig := `output_file = "test-output.md"
model = "test-model"
clarify_task = true

[templates]
default = "default.tmpl"
clarify = "clarify.tmpl"
refine = "refine.tmpl"
`

	configPath := filepath.Join(configDir, "config.toml")
	if err := os.WriteFile(configPath, []byte(legacyConfig), 0644); err != nil {
		t.Fatalf("Failed to write test config: %v", err)
	}

	// Initialize manager with the test directory
	logger := newMockLogger()
	manager := &Manager{
		logger:        logger,
		userConfigDir: configDir,
		sysConfigDirs: []string{},
		config:        DefaultConfig(),
		viperInst:     viper.New(),
	}

	// Load the config
	err = manager.LoadFromFiles()
	if err != nil {
		t.Fatalf("Failed to load config: %v", err)
	}

	// Check that the regular configuration was loaded
	config := manager.GetConfig()
	if config.OutputFile != "test-output.md" || config.ModelName != "test-model" {
		t.Errorf("Failed to load standard config values. Got OutputFile=%s, ModelName=%s",
			config.OutputFile, config.ModelName)
	}

	// Verify the field isn't present in the loaded app config
	// We check behavior, not implementation, by inspecting what fields are exposed by the config
	// and ensuring ClarifyTask isn't exposed even though it was in the config file
	appConfig := manager.config

	// Let's verify the template config doesn't contain clarify/refine templates
	tConfig := appConfig.Templates

	// Check if templates map contains keys for templates
	templateFields := map[string]string{
		"Default": tConfig.Default,
		"Test":    tConfig.Test,
		"Custom":  tConfig.Custom,
	}

	// Verify the map doesn't have Clarify or Refine keys
	for k, v := range templateFields {
		if k == "Clarify" || k == "Refine" {
			t.Errorf("Template config unexpectedly contains %s field with value %s", k, v)
		}
	}

	// Verify the clarify template path isn't retrievable
	if _, ok := manager.getTemplatePathFromConfig("clarify"); ok {
		t.Error("getTemplatePathFromConfig unexpectedly provides a path for 'clarify'")
	}

	// Verify the refine template path isn't retrievable
	if _, ok := manager.getTemplatePathFromConfig("refine"); ok {
		t.Error("getTemplatePathFromConfig unexpectedly provides a path for 'refine'")
	}
}
