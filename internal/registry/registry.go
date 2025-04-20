// Package registry provides a configuration-driven registry
// for LLM providers and models, allowing for flexible configuration
// and easier addition of new models and providers.
package registry

import (
	"context"
	"fmt"
	"strings"
	"sync"

	"github.com/phrazzld/architect/internal/llm"
	"github.com/phrazzld/architect/internal/logutil"
	"github.com/phrazzld/architect/internal/providers"
)

// Registry holds loaded provider and model definitions and their implementations.
// It provides thread-safe access to the configuration data and registered provider implementations.
type Registry struct {
	models          map[string]ModelDefinition
	providers       map[string]ProviderDefinition
	implementations map[string]providers.Provider
	mu              sync.RWMutex
	logger          logutil.LoggerInterface
}

// NewRegistry creates a new Registry instance with initialized maps and the provided logger.
func NewRegistry(logger logutil.LoggerInterface) *Registry {
	return &Registry{
		models:          make(map[string]ModelDefinition),
		providers:       make(map[string]ProviderDefinition),
		implementations: make(map[string]providers.Provider),
		logger:          logger,
	}
}

// ConfigLoaderInterface defines the interface for configuration loaders
type ConfigLoaderInterface interface {
	Load() (*ModelsConfig, error)
}

// LoadConfig loads and validates the models configuration using the provided ConfigLoader.
// It populates the Registry with models and providers from the configuration.
func (r *Registry) LoadConfig(loader ConfigLoaderInterface) error {
	r.mu.Lock()
	defer r.mu.Unlock()

	r.logger.Debug("Loading models configuration using provided loader")

	// Load the configuration
	config, err := loader.Load()
	if err != nil {
		r.logger.Error("Failed to load configuration: %v", err)
		return fmt.Errorf("failed to load configuration: %w", err)
	}

	// Clear existing maps before populating with new data
	r.logger.Debug("Clearing existing registry data before loading new configuration")
	r.providers = make(map[string]ProviderDefinition)
	r.models = make(map[string]ModelDefinition)

	// Load providers
	r.logger.Debug("Loading %d providers into registry", len(config.Providers))
	for _, p := range config.Providers {
		r.providers[p.Name] = p
		r.logger.Debug("Registered provider '%s'%s", p.Name,
			getBaseURLLogSuffix(p.BaseURL))
	}

	// Load models with detailed logging
	r.logger.Debug("Loading %d models into registry", len(config.Models))
	for _, m := range config.Models {
		r.models[m.Name] = m
		r.logger.Debug("Registered model '%s' (provider: '%s', %d parameters)",
			m.Name, m.Provider, len(m.Parameters))
	}

	r.logger.Info("Models configuration loaded: %d providers, %d models",
		len(r.providers), len(r.models))

	// Log summary of loaded models
	if len(r.models) > 0 {
		r.logModelsSummary()
	}

	return nil
}

// Helper function to format base URL for logging
func getBaseURLLogSuffix(baseURL string) string {
	if baseURL != "" {
		return fmt.Sprintf(" with custom base URL: %s", baseURL)
	}
	return " with default base URL"
}

// logModelsSummary logs a summary of the models in the registry
func (r *Registry) logModelsSummary() {
	// Log providers with their model counts
	providerModels := make(map[string]int)
	for _, model := range r.models {
		providerModels[model.Provider]++
	}

	for provider, count := range providerModels {
		r.logger.Info("Provider '%s' has %d registered models", provider, count)
	}
}

// GetModel retrieves a model definition by name.
// Returns an error if the model is not found.
func (r *Registry) GetModel(name string) (*ModelDefinition, error) {
	r.mu.RLock()
	defer r.mu.RUnlock()

	r.logger.Debug("Looking up model '%s' in registry", name)

	model, ok := r.models[name]
	if !ok {
		r.logger.Warn("Model '%s' not found in registry (available models: %s)",
			name, r.getAvailableModelsList())
		return nil, fmt.Errorf("model '%s' not found in registry", name)
	}

	r.logger.Debug("Found model '%s' in registry (provider: '%s')",
		name, model.Provider)

	return &model, nil
}

// getAvailableModelsList returns a comma-separated list of available models
func (r *Registry) getAvailableModelsList() string {
	if len(r.models) == 0 {
		return "no models available"
	}

	// Get a slice of model names
	modelNames := make([]string, 0, len(r.models))
	for name := range r.models {
		modelNames = append(modelNames, name)
	}

	// If there are too many models, just show a few and the count
	if len(modelNames) > 5 {
		return fmt.Sprintf("%s and %d others",
			strings.Join(modelNames[:5], ", "), len(modelNames)-5)
	}

	return strings.Join(modelNames, ", ")
}

// GetProvider retrieves a provider definition by name.
// Returns an error if the provider is not found.
func (r *Registry) GetProvider(name string) (*ProviderDefinition, error) {
	r.mu.RLock()
	defer r.mu.RUnlock()

	r.logger.Debug("Looking up provider '%s' in registry", name)

	provider, ok := r.providers[name]
	if !ok {
		r.logger.Warn("Provider '%s' not found in registry (available providers: %s)",
			name, r.getAvailableProvidersList())
		return nil, fmt.Errorf("provider '%s' not found in registry", name)
	}

	baseURLInfo := "default base URL"
	if provider.BaseURL != "" {
		baseURLInfo = fmt.Sprintf("custom base URL: %s", provider.BaseURL)
	}

	r.logger.Debug("Found provider '%s' in registry (%s)", name, baseURLInfo)

	return &provider, nil
}

// getAvailableProvidersList returns a comma-separated list of available providers
func (r *Registry) getAvailableProvidersList() string {
	if len(r.providers) == 0 {
		return "no providers available"
	}

	// Get a slice of provider names
	providerNames := make([]string, 0, len(r.providers))
	for name := range r.providers {
		providerNames = append(providerNames, name)
	}

	return strings.Join(providerNames, ", ")
}

// RegisterProviderImplementation registers a provider implementation.
// The implementation must correspond to a provider defined in the configuration.
func (r *Registry) RegisterProviderImplementation(name string, impl providers.Provider) error {
	r.mu.Lock()
	defer r.mu.Unlock()

	// Verify the provider exists in the configuration
	_, ok := r.providers[name]
	if !ok {
		return fmt.Errorf("provider '%s' not defined in configuration", name)
	}

	r.implementations[name] = impl
	r.logger.Debug("Registered provider implementation for '%s'", name)
	return nil
}

// GetProviderImplementation retrieves a registered provider implementation.
// Returns an error if the implementation is not registered.
func (r *Registry) GetProviderImplementation(name string) (providers.Provider, error) {
	r.mu.RLock()
	defer r.mu.RUnlock()

	impl, ok := r.implementations[name]
	if !ok {
		return nil, fmt.Errorf("provider implementation for '%s' not registered", name)
	}
	return impl, nil
}

// CreateLLMClient creates an LLM client for the specified model using the registry.
// This is a convenience method that coordinates the retrieval of model and provider information
// and uses the registered provider implementation to create an appropriate client.
func (r *Registry) CreateLLMClient(ctx context.Context, apiKey, modelName string) (llm.LLMClient, error) {
	r.logger.Info("Creating LLM client for model '%s'", modelName)

	// Validate API key (basic check)
	if apiKey == "" {
		r.logger.Error("Empty API key provided when creating client for model '%s'", modelName)
		return nil, fmt.Errorf("empty API key provided for model '%s'", modelName)
	}

	// Get the model definition
	r.logger.Debug("Retrieving model definition for '%s'", modelName)
	model, err := r.GetModel(modelName)
	if err != nil {
		r.logger.Error("Failed to get model definition for '%s': %v", modelName, err)
		return nil, fmt.Errorf("failed to get model definition: %w", err)
	}

	// Log token limits for reference
	r.logger.Debug("Model '%s' has context window of %d tokens and max output of %d tokens",
		modelName, model.ContextWindow, model.MaxOutputTokens)

	// Get the provider definition
	providerName := model.Provider
	r.logger.Debug("Retrieving provider definition for '%s'", providerName)
	provider, err := r.GetProvider(providerName)
	if err != nil {
		r.logger.Error("Failed to get provider definition for '%s': %v", providerName, err)
		return nil, fmt.Errorf("failed to get provider definition: %w", err)
	}

	// Get the provider implementation
	r.logger.Debug("Retrieving provider implementation for '%s'", providerName)
	impl, err := r.GetProviderImplementation(providerName)
	if err != nil {
		r.logger.Error("Failed to get provider implementation for '%s': %v", providerName, err)
		return nil, fmt.Errorf("failed to get provider implementation: %w", err)
	}

	// Create the client using the provider implementation
	r.logger.Info("Creating LLM client for model '%s' (API ID: '%s') using provider '%s'%s",
		modelName, model.APIModelID, providerName, getBaseURLLogSuffix(provider.BaseURL))

	client, err := impl.CreateClient(ctx, apiKey, model.APIModelID, provider.BaseURL)
	if err != nil {
		r.logger.Error("Failed to create client for model '%s': %v", modelName, err)
		return nil, fmt.Errorf("provider '%s' failed to create client: %w", providerName, err)
	}

	r.logger.Debug("Successfully created LLM client for model '%s'", modelName)
	return client, nil
}

// GetAllModelNames returns a list of all model names in the registry.
func (r *Registry) GetAllModelNames() []string {
	r.mu.RLock()
	defer r.mu.RUnlock()

	r.logger.Debug("Getting all model names from registry")
	modelNames := make([]string, 0, len(r.models))
	for name := range r.models {
		modelNames = append(modelNames, name)
	}

	r.logger.Debug("Found %d models in registry", len(modelNames))
	return modelNames
}

// GetModelNamesByProvider returns a list of model names for a specific provider.
func (r *Registry) GetModelNamesByProvider(providerName string) []string {
	r.mu.RLock()
	defer r.mu.RUnlock()

	r.logger.Debug("Getting model names for provider '%s'", providerName)

	var modelNames []string
	for name, model := range r.models {
		if model.Provider == providerName {
			modelNames = append(modelNames, name)
		}
	}

	r.logger.Debug("Found %d models for provider '%s'", len(modelNames), providerName)
	return modelNames
}
