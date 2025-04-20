package registry

import (
	"regexp"
	"strings"
	"testing"

	"github.com/phrazzld/architect/internal/logutil"
)

// setupTestRegistryWithExtendedModels creates a test registry with more edge cases
func setupTestRegistryWithExtendedModels(t *testing.T) *Manager {
	logger := logutil.NewLogger(logutil.DebugLevel, nil, "[test] ")
	manager := NewManager(logger)

	// Create a test registry with models
	registry := manager.GetRegistry()

	// Add test providers
	registry.providers = map[string]ProviderDefinition{
		"openai":     {Name: "openai"},
		"gemini":     {Name: "gemini"},
		"openrouter": {Name: "openrouter"},
		"anthropic":  {Name: "anthropic"},
		"mistral":    {Name: "mistral"},
		"custom":     {Name: "custom"},
	}

	// Add test models with various edge cases
	registry.models = map[string]ModelDefinition{
		// Regular models
		"gpt-4": {
			Name:            "gpt-4",
			Provider:        "openai",
			APIModelID:      "gpt-4",
			ContextWindow:   8192,
			MaxOutputTokens: 4096,
		},
		"gemini-pro": {
			Name:            "gemini-pro",
			Provider:        "gemini",
			APIModelID:      "gemini-pro",
			ContextWindow:   32768,
			MaxOutputTokens: 8192,
		},
		// Models with similar prefixes
		"gpt-4-turbo": {
			Name:            "gpt-4-turbo",
			Provider:        "openai",
			APIModelID:      "gpt-4-turbo",
			ContextWindow:   128000,
			MaxOutputTokens: 4096,
		},
		"gpt-4-vision": {
			Name:            "gpt-4-vision",
			Provider:        "openai",
			APIModelID:      "gpt-4-vision-preview",
			ContextWindow:   128000,
			MaxOutputTokens: 4096,
		},
		// Models with different cases
		"GPT-3.5-TURBO": {
			Name:            "GPT-3.5-TURBO",
			Provider:        "openai",
			APIModelID:      "gpt-3.5-turbo",
			ContextWindow:   16385,
			MaxOutputTokens: 4096,
		},
		// Models with special characters
		"mistral-7b@dev": {
			Name:            "mistral-7b@dev",
			Provider:        "mistral",
			APIModelID:      "mistral-7b-v2.0-dev",
			ContextWindow:   32000,
			MaxOutputTokens: 8000,
		},
		// Models with very long names
		"a-very-long-model-name-that-exceeds-typical-length-limits-but-should-still-work-fine-in-our-system": {
			Name:            "a-very-long-model-name-that-exceeds-typical-length-limits-but-should-still-work-fine-in-our-system",
			Provider:        "custom",
			APIModelID:      "long-name-model",
			ContextWindow:   4096,
			MaxOutputTokens: 1024,
		},
		// OpenRouter models with nested naming structure
		"openrouter/anthropic/claude-3-opus": {
			Name:            "openrouter/anthropic/claude-3-opus",
			Provider:        "openrouter",
			APIModelID:      "anthropic/claude-3-opus",
			ContextWindow:   200000,
			MaxOutputTokens: 25000,
		},
		"openrouter/anthropic/claude-3-sonnet": {
			Name:            "openrouter/anthropic/claude-3-sonnet",
			Provider:        "openrouter",
			APIModelID:      "anthropic/claude-3-sonnet",
			ContextWindow:   180000,
			MaxOutputTokens: 20000,
		},
	}

	// Mark as loaded
	manager.loaded = true

	return manager
}

// TestCaseSensitivity tests if model lookup is case-sensitive
func TestCaseSensitivity(t *testing.T) {
	manager := setupTestRegistryWithExtendedModels(t)

	tests := []struct {
		name      string
		modelName string
		expected  bool
	}{
		{
			name:      "Exact case match",
			modelName: "GPT-3.5-TURBO",
			expected:  true,
		},
		{
			name:      "Different case",
			modelName: "gpt-3.5-turbo",
			expected:  false, // Should be false since lookup is case-sensitive
		},
		{
			name:      "Mixed case",
			modelName: "GpT-3.5-TuRbO",
			expected:  false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := manager.IsModelSupported(tt.modelName)
			if result != tt.expected {
				t.Errorf("IsModelSupported(%q) = %v, want %v", tt.modelName, result, tt.expected)
			}
		})
	}
}

// TestSimilarModelNames tests models with similar naming patterns
func TestSimilarModelNames(t *testing.T) {
	manager := setupTestRegistryWithExtendedModels(t)

	tests := []struct {
		name         string
		modelName    string
		wantProvider string
		wantErr      bool
	}{
		{
			name:         "Base model",
			modelName:    "gpt-4",
			wantProvider: "openai",
			wantErr:      false,
		},
		{
			name:         "Specific variant",
			modelName:    "gpt-4-turbo",
			wantProvider: "openai",
			wantErr:      false,
		},
		{
			name:         "Another specific variant",
			modelName:    "gpt-4-vision",
			wantProvider: "openai",
			wantErr:      false,
		},
		{
			name:         "Non-existent variant",
			modelName:    "gpt-4-nonexistent",
			wantProvider: "",
			wantErr:      true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			provider, err := manager.GetProviderForModel(tt.modelName)
			if (err != nil) != tt.wantErr {
				t.Errorf("GetProviderForModel() error = %v, wantErr %v", err, tt.wantErr)
				return
			}
			if provider != tt.wantProvider {
				t.Errorf("GetProviderForModel() got = %v, want %v", provider, tt.wantProvider)
			}
		})
	}
}

// TestModelInfoProperties tests if GetModelInfo returns the correct model properties
func TestModelInfoProperties(t *testing.T) {
	manager := setupTestRegistryWithExtendedModels(t)

	tests := []struct {
		name            string
		modelName       string
		wantAPIModelID  string
		wantProvider    string
		wantContextSize int32
		wantMaxOutput   int32
		wantErr         bool
	}{
		{
			name:            "OpenAI model information",
			modelName:       "gpt-4",
			wantAPIModelID:  "gpt-4",
			wantProvider:    "openai",
			wantContextSize: 8192,
			wantMaxOutput:   4096,
			wantErr:         false,
		},
		{
			name:            "OpenRouter model information",
			modelName:       "openrouter/anthropic/claude-3-opus",
			wantAPIModelID:  "anthropic/claude-3-opus",
			wantProvider:    "openrouter",
			wantContextSize: 200000,
			wantMaxOutput:   25000,
			wantErr:         false,
		},
		{
			name:            "Model with special characters",
			modelName:       "mistral-7b@dev",
			wantAPIModelID:  "mistral-7b-v2.0-dev",
			wantProvider:    "mistral",
			wantContextSize: 32000,
			wantMaxOutput:   8000,
			wantErr:         false,
		},
		{
			name:            "Very long model name",
			modelName:       "a-very-long-model-name-that-exceeds-typical-length-limits-but-should-still-work-fine-in-our-system",
			wantAPIModelID:  "long-name-model",
			wantProvider:    "custom",
			wantContextSize: 4096,
			wantMaxOutput:   1024,
			wantErr:         false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			modelInfo, err := manager.GetModelInfo(tt.modelName)
			if (err != nil) != tt.wantErr {
				t.Errorf("GetModelInfo() error = %v, wantErr %v", err, tt.wantErr)
				return
			}

			if err != nil {
				return
			}

			// Verify model properties
			if modelInfo.APIModelID != tt.wantAPIModelID {
				t.Errorf("GetModelInfo().APIModelID = %v, want %v", modelInfo.APIModelID, tt.wantAPIModelID)
			}
			if modelInfo.Provider != tt.wantProvider {
				t.Errorf("GetModelInfo().Provider = %v, want %v", modelInfo.Provider, tt.wantProvider)
			}
			if modelInfo.ContextWindow != tt.wantContextSize {
				t.Errorf("GetModelInfo().ContextWindow = %v, want %v", modelInfo.ContextWindow, tt.wantContextSize)
			}
			if modelInfo.MaxOutputTokens != tt.wantMaxOutput {
				t.Errorf("GetModelInfo().MaxOutputTokens = %v, want %v", modelInfo.MaxOutputTokens, tt.wantMaxOutput)
			}
		})
	}
}

// TestMalformedModelNames tests handling of malformed model names
func TestMalformedModelNames(t *testing.T) {
	manager := setupTestRegistryWithExtendedModels(t)

	tests := []struct {
		name      string
		modelName string
		wantErr   bool
	}{
		{
			name:      "Empty string",
			modelName: "",
			wantErr:   true,
		},
		{
			name:      "Only whitespace",
			modelName: "   ",
			wantErr:   true,
		},
		{
			name:      "Special characters only",
			modelName: "!@#$%^&*()",
			wantErr:   true,
		},
		{
			name:      "SQL injection attempt",
			modelName: "gpt-4'; DROP TABLE models; --",
			wantErr:   true,
		},
		{
			name:      "Very long model name that does not exist",
			modelName: strings.Repeat("a", 1000),
			wantErr:   true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			_, err := manager.GetProviderForModel(tt.modelName)
			if (err != nil) != tt.wantErr {
				t.Errorf("GetProviderForModel() with malformed name error = %v, wantErr %v", err, tt.wantErr)
			}
		})
	}
}

// TestOpenRouterModels tests handling of OpenRouter models with nested names
func TestOpenRouterModels(t *testing.T) {
	manager := setupTestRegistryWithExtendedModels(t)

	// Test listing models for OpenRouter provider
	models := manager.GetModelsForProvider("openrouter")
	if len(models) != 2 {
		t.Errorf("GetModelsForProvider(\"openrouter\") returned %d models, want 2", len(models))
	}

	// Verify that OpenRouter models are correctly identified
	modelPatternRegex := regexp.MustCompile(`^openrouter/anthropic/claude-3-(opus|sonnet)$`)

	for _, model := range models {
		if !modelPatternRegex.MatchString(model) {
			t.Errorf("Unexpected OpenRouter model name format: %s", model)
		}

		// Check that we can get correct provider for each model
		provider, err := manager.GetProviderForModel(model)
		if err != nil {
			t.Errorf("GetProviderForModel(%q) returned error: %v", model, err)
		}
		if provider != "openrouter" {
			t.Errorf("GetProviderForModel(%q) = %s, want \"openrouter\"", model, provider)
		}
	}
}

// TestModelRegistryWithEmptyRegistry tests model detection with an empty registry
func TestModelRegistryWithEmptyRegistry(t *testing.T) {
	logger := logutil.NewLogger(logutil.DebugLevel, nil, "[test] ")
	manager := NewManager(logger)

	// Set up empty registry but mark as loaded
	registry := manager.GetRegistry()
	registry.models = make(map[string]ModelDefinition)
	registry.providers = make(map[string]ProviderDefinition)
	manager.loaded = true

	// Verify empty GetAllModels
	models := manager.GetAllModels()
	if len(models) != 0 {
		t.Errorf("GetAllModels() on empty registry returned %d models, want 0", len(models))
	}

	// Verify GetModelsForProvider on empty registry
	providerModels := manager.GetModelsForProvider("any-provider")
	if len(providerModels) != 0 {
		t.Errorf("GetModelsForProvider() on empty registry returned %d models, want 0", len(providerModels))
	}

	// Verify IsModelSupported on empty registry
	supported := manager.IsModelSupported("any-model")
	if supported {
		t.Errorf("IsModelSupported() on empty registry returned true, want false")
	}

	// Verify GetProviderForModel on empty registry
	_, err := manager.GetProviderForModel("any-model")
	if err == nil {
		t.Errorf("GetProviderForModel() on empty registry did not return an error")
	}
}
