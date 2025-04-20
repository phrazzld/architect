package registry

import (
	"strings"
	"testing"

	"github.com/phrazzld/architect/internal/logutil"
)

// setupTestRegistryWithModels creates a test registry manager with predefined models.
func setupTestRegistryWithModels(t *testing.T) *Manager {
	logger := logutil.NewLogger(logutil.DebugLevel, nil, "[test] ")
	manager := NewManager(logger)

	// Create a test registry with models
	registry := manager.GetRegistry()

	// Add test providers
	registry.providers = map[string]ProviderDefinition{
		"openai":     {Name: "openai"},
		"gemini":     {Name: "gemini"},
		"openrouter": {Name: "openrouter"},
	}

	// Add test models
	registry.models = map[string]ModelDefinition{
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
		"openrouter/anthropic/claude-3-opus": {
			Name:            "openrouter/anthropic/claude-3-opus",
			Provider:        "openrouter",
			APIModelID:      "anthropic/claude-3-opus",
			ContextWindow:   200000,
			MaxOutputTokens: 25000,
		},
	}

	// Mark as loaded
	manager.loaded = true

	return manager
}

func TestGetProviderForModel(t *testing.T) {
	manager := setupTestRegistryWithModels(t)

	tests := []struct {
		name          string
		modelName     string
		wantProvider  string
		wantErr       bool
		wantErrPrefix string
	}{
		{
			name:         "OpenAI model",
			modelName:    "gpt-4",
			wantProvider: "openai",
			wantErr:      false,
		},
		{
			name:         "Gemini model",
			modelName:    "gemini-pro",
			wantProvider: "gemini",
			wantErr:      false,
		},
		{
			name:         "OpenRouter model",
			modelName:    "openrouter/anthropic/claude-3-opus",
			wantProvider: "openrouter",
			wantErr:      false,
		},
		{
			name:          "Unknown model",
			modelName:     "nonexistent-model",
			wantProvider:  "",
			wantErr:       true,
			wantErrPrefix: "failed to determine provider for model 'nonexistent-model'",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			gotProvider, err := manager.GetProviderForModel(tt.modelName)

			// Check error
			if (err != nil) != tt.wantErr {
				t.Errorf("GetProviderForModel() error = %v, wantErr %v", err, tt.wantErr)
				return
			}

			if err != nil && tt.wantErrPrefix != "" && !strings.HasPrefix(err.Error(), tt.wantErrPrefix) {
				t.Errorf("GetProviderForModel() error = %v, want error with prefix %v", err, tt.wantErrPrefix)
			}

			// Check provider
			if gotProvider != tt.wantProvider {
				t.Errorf("GetProviderForModel() = %v, want %v", gotProvider, tt.wantProvider)
			}
		})
	}
}

func TestIsModelSupported(t *testing.T) {
	manager := setupTestRegistryWithModels(t)

	tests := []struct {
		name      string
		modelName string
		want      bool
	}{
		{
			name:      "OpenAI model",
			modelName: "gpt-4",
			want:      true,
		},
		{
			name:      "Gemini model",
			modelName: "gemini-pro",
			want:      true,
		},
		{
			name:      "OpenRouter model",
			modelName: "openrouter/anthropic/claude-3-opus",
			want:      true,
		},
		{
			name:      "Unknown model",
			modelName: "nonexistent-model",
			want:      false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := manager.IsModelSupported(tt.modelName); got != tt.want {
				t.Errorf("IsModelSupported() = %v, want %v", got, tt.want)
			}
		})
	}
}

func TestGetModelInfo(t *testing.T) {
	manager := setupTestRegistryWithModels(t)

	tests := []struct {
		name          string
		modelName     string
		wantProvider  string
		wantAPIID     string
		wantErr       bool
		wantErrPrefix string
	}{
		{
			name:         "OpenAI model",
			modelName:    "gpt-4",
			wantProvider: "openai",
			wantAPIID:    "gpt-4",
			wantErr:      false,
		},
		{
			name:         "Gemini model",
			modelName:    "gemini-pro",
			wantProvider: "gemini",
			wantAPIID:    "gemini-pro",
			wantErr:      false,
		},
		{
			name:         "OpenRouter model",
			modelName:    "openrouter/anthropic/claude-3-opus",
			wantProvider: "openrouter",
			wantAPIID:    "anthropic/claude-3-opus",
			wantErr:      false,
		},
		{
			name:          "Unknown model",
			modelName:     "nonexistent-model",
			wantProvider:  "",
			wantAPIID:     "",
			wantErr:       true,
			wantErrPrefix: "model 'nonexistent-model' not found in registry",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			gotModel, err := manager.GetModelInfo(tt.modelName)

			// Check error
			if (err != nil) != tt.wantErr {
				t.Errorf("GetModelInfo() error = %v, wantErr %v", err, tt.wantErr)
				return
			}

			if err != nil {
				if tt.wantErrPrefix != "" && !strings.HasPrefix(err.Error(), tt.wantErrPrefix) {
					t.Errorf("GetModelInfo() error = %v, want error with prefix %v", err, tt.wantErrPrefix)
				}
				return
			}

			// Check model info
			if gotModel.Provider != tt.wantProvider {
				t.Errorf("GetModelInfo().Provider = %v, want %v", gotModel.Provider, tt.wantProvider)
			}

			if gotModel.APIModelID != tt.wantAPIID {
				t.Errorf("GetModelInfo().APIModelID = %v, want %v", gotModel.APIModelID, tt.wantAPIID)
			}
		})
	}
}

func TestGetAllModels(t *testing.T) {
	manager := setupTestRegistryWithModels(t)

	models := manager.GetAllModels()

	if len(models) != 3 {
		t.Errorf("GetAllModels() returned %d models, want 3", len(models))
	}

	// Check if all test models are in the result
	expectedModels := map[string]bool{
		"gpt-4":                              true,
		"gemini-pro":                         true,
		"openrouter/anthropic/claude-3-opus": true,
	}

	for _, model := range models {
		if _, ok := expectedModels[model]; !ok {
			t.Errorf("GetAllModels() returned unexpected model: %s", model)
		}
		delete(expectedModels, model)
	}

	if len(expectedModels) > 0 {
		missing := make([]string, 0, len(expectedModels))
		for model := range expectedModels {
			missing = append(missing, model)
		}
		t.Errorf("GetAllModels() is missing expected models: %v", missing)
	}
}

func TestGetModelsForProvider(t *testing.T) {
	manager := setupTestRegistryWithModels(t)

	tests := []struct {
		name           string
		providerName   string
		wantModels     []string
		wantModelCount int
	}{
		{
			name:           "OpenAI provider",
			providerName:   "openai",
			wantModels:     []string{"gpt-4"},
			wantModelCount: 1,
		},
		{
			name:           "Gemini provider",
			providerName:   "gemini",
			wantModels:     []string{"gemini-pro"},
			wantModelCount: 1,
		},
		{
			name:           "OpenRouter provider",
			providerName:   "openrouter",
			wantModels:     []string{"openrouter/anthropic/claude-3-opus"},
			wantModelCount: 1,
		},
		{
			name:           "Unknown provider",
			providerName:   "nonexistent-provider",
			wantModels:     []string{},
			wantModelCount: 0,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			gotModels := manager.GetModelsForProvider(tt.providerName)

			if len(gotModels) != tt.wantModelCount {
				t.Errorf("GetModelsForProvider() returned %d models, want %d", len(gotModels), tt.wantModelCount)
			}

			for _, wantModel := range tt.wantModels {
				found := false
				for _, gotModel := range gotModels {
					if gotModel == wantModel {
						found = true
						break
					}
				}
				if !found {
					t.Errorf("GetModelsForProvider() did not return expected model: %s", wantModel)
				}
			}
		})
	}
}

func TestRegistryNotInitialized(t *testing.T) {
	logger := logutil.NewLogger(logutil.DebugLevel, nil, "[test] ")
	manager := NewManager(logger)
	// Note: manager.loaded = false by default

	// Test GetProviderForModel
	provider, err := manager.GetProviderForModel("any-model")
	if err == nil {
		t.Errorf("GetProviderForModel() should return error when registry not initialized")
	}
	if provider != "" {
		t.Errorf("GetProviderForModel() returned provider %s when registry not initialized", provider)
	}

	// Test IsModelSupported
	if manager.IsModelSupported("any-model") {
		t.Errorf("IsModelSupported() should return false when registry not initialized")
	}

	// Test GetModelInfo
	model, err := manager.GetModelInfo("any-model")
	if err == nil {
		t.Errorf("GetModelInfo() should return error when registry not initialized")
	}
	if model != nil {
		t.Errorf("GetModelInfo() returned model when registry not initialized")
	}

	// Test GetAllModels
	models := manager.GetAllModels()
	if len(models) != 0 {
		t.Errorf("GetAllModels() should return empty slice when registry not initialized")
	}

	// Test GetModelsForProvider
	models = manager.GetModelsForProvider("any-provider")
	if len(models) != 0 {
		t.Errorf("GetModelsForProvider() should return empty slice when registry not initialized")
	}
}
