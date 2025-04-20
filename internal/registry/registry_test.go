package registry

import (
	"context"
	"errors"
	"sync"
	"testing"

	"github.com/phrazzld/architect/internal/llm"
	"github.com/phrazzld/architect/internal/logutil"
	"github.com/phrazzld/architect/internal/providers"
)

// MockConfigLoader implements a test double for the ConfigLoader
type MockConfigLoader struct {
	LoadFunc func() (*ModelsConfig, error)
}

// Load calls the mock implementation
func (m *MockConfigLoader) Load() (*ModelsConfig, error) {
	if m.LoadFunc != nil {
		return m.LoadFunc()
	}
	return nil, errors.New("LoadFunc not implemented")
}

// MockProvider implements the providers.Provider interface for testing
type MockProvider struct {
	CreateClientFunc func(ctx context.Context, apiKey string, modelID string, apiEndpoint string) (llm.LLMClient, error)
}

// CreateClient delegates to the mock implementation
func (m *MockProvider) CreateClient(ctx context.Context, apiKey string, modelID string, apiEndpoint string) (llm.LLMClient, error) {
	if m.CreateClientFunc != nil {
		return m.CreateClientFunc(ctx, apiKey, modelID, apiEndpoint)
	}
	return nil, errors.New("CreateClientFunc not implemented")
}

// MockLLMClient implements the llm.LLMClient interface for testing
type MockLLMClient struct {
	GenerateContentFunc func(ctx context.Context, prompt string, params map[string]interface{}) (*llm.ProviderResult, error)
	GetModelNameFunc    func() string
	CloseFunc           func() error
}

func (m *MockLLMClient) GenerateContent(ctx context.Context, prompt string, params map[string]interface{}) (*llm.ProviderResult, error) {
	if m.GenerateContentFunc != nil {
		return m.GenerateContentFunc(ctx, prompt, params)
	}
	return nil, errors.New("GenerateContentFunc not implemented")
}

func (m *MockLLMClient) GetModelName() string {
	if m.GetModelNameFunc != nil {
		return m.GetModelNameFunc()
	}
	return "mock-model"
}

func (m *MockLLMClient) Close() error {
	if m.CloseFunc != nil {
		return m.CloseFunc()
	}
	return nil
}

// TestNewRegistry verifies that NewRegistry creates a properly initialized instance
func TestNewRegistry(t *testing.T) {
	logger := logutil.NewLogger(logutil.InfoLevel, nil, "[test] ")
	registry := NewRegistry(logger)

	if registry == nil {
		t.Fatal("Expected non-nil registry")
	}

	if registry.models == nil {
		t.Error("Expected non-nil models map")
	}

	if registry.providers == nil {
		t.Error("Expected non-nil providers map")
	}

	if registry.implementations == nil {
		t.Error("Expected non-nil implementations map")
	}

	if registry.logger == nil {
		t.Error("Expected non-nil logger")
	}
}

// TestLoadConfig verifies the behavior of the LoadConfig method
func TestLoadConfig(t *testing.T) {
	// Test cases
	tests := []struct {
		name    string
		config  *ModelsConfig
		loadErr error
		wantErr bool
	}{
		{
			name: "successful load",
			config: &ModelsConfig{
				APIKeySources: map[string]string{"openai": "OPENAI_API_KEY"},
				Providers: []ProviderDefinition{
					{Name: "openai"},
					{Name: "gemini", BaseURL: "https://custom.endpoint"},
				},
				Models: []ModelDefinition{
					{
						Name:       "gpt-4",
						Provider:   "openai",
						APIModelID: "gpt-4",
						Parameters: map[string]ParameterDefinition{
							"temperature": {Type: "float", Default: 0.7},
						},
					},
				},
			},
			loadErr: nil,
			wantErr: false,
		},
		{
			name:    "load error",
			config:  nil,
			loadErr: errors.New("failed to load configuration"),
			wantErr: true,
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			// Setup
			logger := logutil.NewLogger(logutil.InfoLevel, nil, "[test] ")
			registry := NewRegistry(logger)

			// Create mock loader
			mockLoader := &MockConfigLoader{
				LoadFunc: func() (*ModelsConfig, error) {
					return tc.config, tc.loadErr
				},
			}

			// Call method under test
			err := registry.LoadConfig(mockLoader)

			// Check results
			if tc.wantErr {
				if err == nil {
					t.Error("Expected error but got nil")
				}
			} else {
				if err != nil {
					t.Errorf("Expected no error but got: %v", err)
				}

				// Check that maps are correctly populated
				if len(tc.config.Providers) != len(registry.providers) {
					t.Errorf("Expected %d providers, got %d", len(tc.config.Providers), len(registry.providers))
				}

				if len(tc.config.Models) != len(registry.models) {
					t.Errorf("Expected %d models, got %d", len(tc.config.Models), len(registry.models))
				}

				// Verify specific entries were added properly
				if _, ok := registry.providers["openai"]; !ok {
					t.Error("Expected 'openai' provider to be in the map")
				}

				if _, ok := registry.models["gpt-4"]; !ok {
					t.Error("Expected 'gpt-4' model to be in the map")
				}

				// Check that a provider with BaseURL was added correctly
				if provider, ok := registry.providers["gemini"]; !ok || provider.BaseURL != "https://custom.endpoint" {
					t.Error("Provider with BaseURL not correctly added")
				}
			}
		})
	}
}

// TestGetModel verifies the behavior of the GetModel method
func TestGetModel(t *testing.T) {
	// Create a test registry with sample models
	logger := logutil.NewLogger(logutil.InfoLevel, nil, "[test] ")
	registry := NewRegistry(logger)

	// Add test models
	registry.models = map[string]ModelDefinition{
		"model1": {
			Name:       "model1",
			Provider:   "provider1",
			APIModelID: "api-model-1",
		},
	}

	// Test cases
	tests := []struct {
		name      string
		modelName string
		wantErr   bool
	}{
		{
			name:      "existing model",
			modelName: "model1",
			wantErr:   false,
		},
		{
			name:      "non-existent model",
			modelName: "model2",
			wantErr:   true,
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			// Call method under test
			model, err := registry.GetModel(tc.modelName)

			// Check results
			if tc.wantErr {
				if err == nil {
					t.Error("Expected error but got nil")
				}
			} else {
				if err != nil {
					t.Errorf("Expected no error but got: %v", err)
				}
				if model == nil {
					t.Fatal("Expected non-nil model")
				}
				if model.Name != tc.modelName {
					t.Errorf("Expected model name %s, got %s", tc.modelName, model.Name)
				}
			}
		})
	}
}

// TestGetProvider verifies the behavior of the GetProvider method
func TestGetProvider(t *testing.T) {
	// Create a test registry with sample providers
	logger := logutil.NewLogger(logutil.InfoLevel, nil, "[test] ")
	registry := NewRegistry(logger)

	// Add test providers
	registry.providers = map[string]ProviderDefinition{
		"provider1": {
			Name:    "provider1",
			BaseURL: "https://api.provider1.com",
		},
	}

	// Test cases
	tests := []struct {
		name         string
		providerName string
		wantErr      bool
	}{
		{
			name:         "existing provider",
			providerName: "provider1",
			wantErr:      false,
		},
		{
			name:         "non-existent provider",
			providerName: "provider2",
			wantErr:      true,
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			// Call method under test
			provider, err := registry.GetProvider(tc.providerName)

			// Check results
			if tc.wantErr {
				if err == nil {
					t.Error("Expected error but got nil")
				}
			} else {
				if err != nil {
					t.Errorf("Expected no error but got: %v", err)
				}
				if provider == nil {
					t.Fatal("Expected non-nil provider")
				}
				if provider.Name != tc.providerName {
					t.Errorf("Expected provider name %s, got %s", tc.providerName, provider.Name)
				}
			}
		})
	}
}

// TestRegisterProviderImplementation verifies the behavior of the RegisterProviderImplementation method
func TestRegisterProviderImplementation(t *testing.T) {
	// Create a test registry with sample providers
	logger := logutil.NewLogger(logutil.InfoLevel, nil, "[test] ")
	registry := NewRegistry(logger)

	// Add test providers
	registry.providers = map[string]ProviderDefinition{
		"provider1": {Name: "provider1"},
	}

	// Create mock provider
	mockProvider := &MockProvider{}

	// Test cases
	tests := []struct {
		name         string
		providerName string
		impl         providers.Provider
		wantErr      bool
	}{
		{
			name:         "existing provider",
			providerName: "provider1",
			impl:         mockProvider,
			wantErr:      false,
		},
		{
			name:         "non-existent provider",
			providerName: "provider2",
			impl:         mockProvider,
			wantErr:      true,
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			// Call method under test
			err := registry.RegisterProviderImplementation(tc.providerName, tc.impl)

			// Check results
			if tc.wantErr {
				if err == nil {
					t.Error("Expected error but got nil")
				}
			} else {
				if err != nil {
					t.Errorf("Expected no error but got: %v", err)
				}

				// Verify implementation was registered
				impl, ok := registry.implementations[tc.providerName]
				if !ok {
					t.Errorf("Implementation for %s not found in registry", tc.providerName)
				}
				if impl != tc.impl {
					t.Errorf("Implementation for %s does not match expected value", tc.providerName)
				}
			}
		})
	}
}

// TestGetProviderImplementation verifies the behavior of the GetProviderImplementation method
func TestGetProviderImplementation(t *testing.T) {
	// Create a test registry
	logger := logutil.NewLogger(logutil.InfoLevel, nil, "[test] ")
	registry := NewRegistry(logger)

	// Create and register a mock provider
	mockProvider := &MockProvider{}
	registry.implementations = map[string]providers.Provider{
		"provider1": mockProvider,
	}

	// Test cases
	tests := []struct {
		name         string
		providerName string
		wantErr      bool
	}{
		{
			name:         "existing implementation",
			providerName: "provider1",
			wantErr:      false,
		},
		{
			name:         "non-existent implementation",
			providerName: "provider2",
			wantErr:      true,
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			// Call method under test
			impl, err := registry.GetProviderImplementation(tc.providerName)

			// Check results
			if tc.wantErr {
				if err == nil {
					t.Error("Expected error but got nil")
				}
			} else {
				if err != nil {
					t.Errorf("Expected no error but got: %v", err)
				}
				if impl == nil {
					t.Fatal("Expected non-nil implementation")
				}
				if impl != mockProvider {
					t.Error("Expected implementation to match the registered mock")
				}
			}
		})
	}
}

// TestCreateLLMClient verifies the behavior of the CreateLLMClient method
func TestCreateLLMClient(t *testing.T) {
	// Create a test registry
	logger := logutil.NewLogger(logutil.InfoLevel, nil, "[test] ")
	registry := NewRegistry(logger)

	// Set up test data
	mockLLMClient := &MockLLMClient{
		GetModelNameFunc: func() string { return "test-model" },
	}

	// Add a test model
	registry.models = map[string]ModelDefinition{
		"test-model": {
			Name:       "test-model",
			Provider:   "test-provider",
			APIModelID: "test-api-model",
		},
	}

	// Add a test provider
	registry.providers = map[string]ProviderDefinition{
		"test-provider": {
			Name:    "test-provider",
			BaseURL: "https://test.api",
		},
	}

	// Create and register a mock provider implementation
	mockProvider := &MockProvider{
		CreateClientFunc: func(ctx context.Context, apiKey string, modelID string, apiEndpoint string) (llm.LLMClient, error) {
			// Verify the correct parameters are passed
			if modelID != "test-api-model" {
				t.Errorf("Expected modelID 'test-api-model', got '%s'", modelID)
			}
			if apiEndpoint != "https://test.api" {
				t.Errorf("Expected apiEndpoint 'https://test.api', got '%s'", apiEndpoint)
			}
			return mockLLMClient, nil
		},
	}
	registry.implementations = map[string]providers.Provider{
		"test-provider": mockProvider,
	}

	// Test cases
	tests := []struct {
		name      string
		modelName string
		apiKey    string
		wantErr   bool
		errFunc   func(*Registry)
	}{
		{
			name:      "successful client creation",
			modelName: "test-model",
			apiKey:    "test-api-key",
			wantErr:   false,
		},
		{
			name:      "model not found",
			modelName: "non-existent-model",
			apiKey:    "test-api-key",
			wantErr:   true,
		},
		{
			name:      "provider not found",
			modelName: "test-model",
			apiKey:    "test-api-key",
			wantErr:   true,
			errFunc: func(r *Registry) {
				// Temporarily remove the provider to simulate provider not found
				delete(r.providers, "test-provider")
			},
		},
		{
			name:      "implementation not found",
			modelName: "test-model",
			apiKey:    "test-api-key",
			wantErr:   true,
			errFunc: func(r *Registry) {
				// Temporarily remove the implementation to simulate implementation not found
				delete(r.implementations, "test-provider")
			},
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			// Apply error setup function if provided
			if tc.errFunc != nil {
				tc.errFunc(registry)
				defer func() {
					// Reset the registry for the next test
					registry.providers = map[string]ProviderDefinition{
						"test-provider": {
							Name:    "test-provider",
							BaseURL: "https://test.api",
						},
					}
					registry.implementations = map[string]providers.Provider{
						"test-provider": mockProvider,
					}
				}()
			}

			// Call method under test
			client, err := registry.CreateLLMClient(context.Background(), tc.apiKey, tc.modelName)

			// Check results
			if tc.wantErr {
				if err == nil {
					t.Error("Expected error but got nil")
				}
			} else {
				if err != nil {
					t.Errorf("Expected no error but got: %v", err)
				}
				if client == nil {
					t.Fatal("Expected non-nil client")
				}
			}
		})
	}
}

// TestConcurrentAccess verifies that the Registry is safe for concurrent access
func TestConcurrentAccess(t *testing.T) {
	// Create a registry
	logger := logutil.NewLogger(logutil.InfoLevel, nil, "[test] ")
	registry := NewRegistry(logger)

	// Add some test data
	registry.models = map[string]ModelDefinition{
		"model1": {Name: "model1", Provider: "provider1"},
	}
	registry.providers = map[string]ProviderDefinition{
		"provider1": {Name: "provider1"},
	}

	// Create a mock provider
	mockProvider := &MockProvider{}

	// Number of concurrent goroutines
	n := 10
	var wg sync.WaitGroup
	wg.Add(n * 4) // 4 operations per goroutine

	// Start concurrent operations
	for i := 0; i < n; i++ {
		go func() {
			defer wg.Done()
			_, _ = registry.GetModel("model1")
		}()

		go func() {
			defer wg.Done()
			_, _ = registry.GetProvider("provider1")
		}()

		go func() {
			defer wg.Done()
			_ = registry.RegisterProviderImplementation("provider1", mockProvider)
		}()

		go func() {
			defer wg.Done()
			_, _ = registry.GetProviderImplementation("provider1")
		}()
	}

	// Wait for all goroutines to complete
	wg.Wait()

	// If we reach here without panics or deadlocks, the test passes
	// We're primarily testing that concurrent access doesn't cause panics or deadlocks
}
