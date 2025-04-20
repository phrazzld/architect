// Package registry provides a configuration-driven registry
// for LLM providers and models, allowing for flexible configuration
// and easier addition of new models and providers.
package registry

import (
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"sync"

	"github.com/phrazzld/architect/internal/logutil"
	"github.com/phrazzld/architect/internal/providers/gemini"
	"github.com/phrazzld/architect/internal/providers/openai"
	"github.com/phrazzld/architect/internal/providers/openrouter"
)

// Manager provides a singleton-like access to the registry.
// It handles initialization, configuration loading, and provider registration.
type Manager struct {
	registry *Registry
	logger   logutil.LoggerInterface
	mu       sync.RWMutex
	loaded   bool
}

var (
	// globalManager is the singleton instance of the registry manager
	globalManager *Manager
	// managerMu protects access to the global manager
	managerMu sync.Mutex
)

// NewManager creates a new registry manager.
func NewManager(logger logutil.LoggerInterface) *Manager {
	if logger == nil {
		logger = logutil.NewLogger(logutil.InfoLevel, nil, "[registry-manager] ")
	}

	return &Manager{
		registry: NewRegistry(logger),
		logger:   logger,
		loaded:   false,
	}
}

// GetGlobalManager returns the global registry manager instance,
// creating it if necessary.
func GetGlobalManager(logger logutil.LoggerInterface) *Manager {
	managerMu.Lock()
	defer managerMu.Unlock()

	if globalManager == nil {
		globalManager = NewManager(logger)
	}

	return globalManager
}

// Initialize loads the registry configuration and registers
// provider implementations.
func (m *Manager) Initialize() error {
	m.mu.Lock()
	defer m.mu.Unlock()

	if m.loaded {
		m.logger.Debug("Registry already initialized, skipping")
		return nil
	}

	m.logger.Info("Initializing registry")

	// Load configuration
	configLoader := NewConfigLoader()
	if err := m.registry.LoadConfig(configLoader); err != nil {
		// Check if the error is due to missing config file
		if os.IsNotExist(err) {
			m.logger.Warn("Configuration file not found. Attempting to install default configuration.")
			if err := m.installDefaultConfig(); err != nil {
				homeDir, _ := os.UserHomeDir()
				configPath := filepath.Join(homeDir, ConfigDirName, ModelsConfigFileName)
				return fmt.Errorf("failed to install default configuration: %w\nPlease ensure the config file exists at: %s, or run config/install.sh to set up the configuration", err, configPath)
			}

			// Try loading again after installation
			if err := m.registry.LoadConfig(configLoader); err != nil {
				return fmt.Errorf("failed to load configuration after installation: %w\nThe configuration was installed but could not be loaded - please check file permissions and format", err)
			}

			m.logger.Info("Successfully installed and loaded default configuration")
		} else {
			return fmt.Errorf("failed to load configuration: %w", err)
		}
	}

	// Register provider implementations
	if err := m.registerProviders(); err != nil {
		return fmt.Errorf("failed to register provider implementations: %w", err)
	}

	m.loaded = true
	m.logger.Info("Registry initialized successfully")
	return nil
}

// GetRegistry returns the underlying registry.
func (m *Manager) GetRegistry() *Registry {
	m.mu.RLock()
	defer m.mu.RUnlock()
	return m.registry
}

// registerProviders registers the provider implementations with the registry.
func (m *Manager) registerProviders() error {
	// Register Gemini provider implementation
	geminiProvider := gemini.NewProvider(m.logger)
	if err := m.registry.RegisterProviderImplementation("gemini", geminiProvider); err != nil {
		return fmt.Errorf("failed to register Gemini provider: %w", err)
	}
	m.logger.Debug("Registered Gemini provider implementation")

	// Register OpenAI provider implementation
	openaiProvider := openai.NewProvider(m.logger)
	if err := m.registry.RegisterProviderImplementation("openai", openaiProvider); err != nil {
		return fmt.Errorf("failed to register OpenAI provider: %w", err)
	}
	m.logger.Debug("Registered OpenAI provider implementation")

	// Register OpenRouter provider implementation
	openrouterProvider := openrouter.NewProvider(m.logger)
	if err := m.registry.RegisterProviderImplementation("openrouter", openrouterProvider); err != nil {
		return fmt.Errorf("failed to register OpenRouter provider: %w", err)
	}
	m.logger.Debug("Registered OpenRouter provider implementation")

	return nil
}

// installDefaultConfig creates the config directory and copies the default models.yaml file.
func (m *Manager) installDefaultConfig() error {
	// Get home directory
	homeDir, err := os.UserHomeDir()
	if err != nil {
		return fmt.Errorf("failed to get user home directory: %w", err)
	}

	// Create config directory
	configDir := filepath.Join(homeDir, ConfigDirName)
	if err := os.MkdirAll(configDir, 0755); err != nil {
		return fmt.Errorf("failed to create config directory: %w", err)
	}

	// Path to the default models.yaml in the repository
	// Try different paths to find the default config file
	possiblePaths := []string{
		// Current directory + config/models.yaml (for running in project root)
		filepath.Join("config", ModelsConfigFileName),
		// One directory up + config/models.yaml (for running in subdirectory)
		filepath.Join("..", "config", ModelsConfigFileName),
		// Two directories up + config/models.yaml (for deeper nesting)
		filepath.Join("..", "..", "config", ModelsConfigFileName),
	}

	// Try to find the config file
	var defaultConfigPath string
	var found bool

	for _, path := range possiblePaths {
		if _, err := os.Stat(path); err == nil {
			defaultConfigPath = path
			found = true
			m.logger.Debug("Found default configuration at %s", defaultConfigPath)
			break
		}
	}

	if !found {
		// Use relative paths in the error message to avoid hardcoding absolute paths
		configRelPath := filepath.Join("$HOME", ".config", "architect", ModelsConfigFileName)
		return fmt.Errorf("default configuration file not found. Please run setup script or manually install models.yaml to %s",
			configRelPath)
	}

	// Read the default configuration
	defaultConfig, err := os.ReadFile(defaultConfigPath)
	if err != nil {
		return fmt.Errorf("failed to read default configuration file: %w", err)
	}

	// Write to the user's config directory
	targetConfigPath := filepath.Join(configDir, ModelsConfigFileName)
	if err := os.WriteFile(targetConfigPath, defaultConfig, 0644); err != nil {
		return fmt.Errorf("failed to write configuration file: %w", err)
	}

	m.logger.Info("Default configuration installed to %s", targetConfigPath)
	return nil
}

// GetProviderForModel returns the provider name for a given model name.
// It looks up the model in the registry and returns the associated provider.
// If the model is not found, an error is returned.
func (m *Manager) GetProviderForModel(modelName string) (string, error) {
	if !m.loaded {
		return "", errors.New("registry not initialized, call Initialize() first")
	}

	m.logger.Debug("Looking up provider for model '%s'", modelName)

	model, err := m.registry.GetModel(modelName)
	if err != nil {
		m.logger.Warn("Failed to determine provider for model '%s': %v", modelName, err)
		return "", fmt.Errorf("failed to determine provider for model '%s': %w", modelName, err)
	}

	m.logger.Debug("Model '%s' uses provider '%s'", modelName, model.Provider)
	return model.Provider, nil
}

// IsModelSupported checks if a model is defined in the registry.
// Returns true if the model exists, false otherwise.
func (m *Manager) IsModelSupported(modelName string) bool {
	if !m.loaded {
		m.logger.Warn("Registry not initialized when checking model support")
		return false
	}

	m.logger.Debug("Checking if model '%s' is supported", modelName)

	_, err := m.registry.GetModel(modelName)
	supported := err == nil

	if supported {
		m.logger.Debug("Model '%s' is supported", modelName)
	} else {
		m.logger.Debug("Model '%s' is not supported", modelName)
	}

	return supported
}

// GetModelInfo returns detailed information about a model.
// If the model is not found, an error is returned.
func (m *Manager) GetModelInfo(modelName string) (*ModelDefinition, error) {
	if !m.loaded {
		return nil, errors.New("registry not initialized, call Initialize() first")
	}

	m.logger.Debug("Getting model info for '%s'", modelName)

	return m.registry.GetModel(modelName)
}

// GetAllModels returns a list of all model names registered in the registry.
func (m *Manager) GetAllModels() []string {
	if !m.loaded {
		m.logger.Warn("Registry not initialized when requesting all models")
		return []string{}
	}

	return m.registry.GetAllModelNames()
}

// GetModelsForProvider returns a list of model names for a specific provider.
func (m *Manager) GetModelsForProvider(providerName string) []string {
	if !m.loaded {
		m.logger.Warn("Registry not initialized when requesting models for provider '%s'", providerName)
		return []string{}
	}

	return m.registry.GetModelNamesByProvider(providerName)
}
