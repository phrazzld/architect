// Package registry provides a configuration-driven registry
// for LLM providers and models, allowing for flexible configuration
// and easier addition of new models and providers.
package registry

// ProviderDefinition represents a provider entry from the configuration.
// It contains information about a specific LLM provider.
type ProviderDefinition struct {
	// Name is the unique identifier for the provider
	Name string `yaml:"name" json:"name"`

	// BaseURL is the optional API endpoint base URL
	// If not provided, the default URL for the provider will be used
	BaseURL string `yaml:"base_url,omitempty" json:"base_url,omitempty"`
}

// ModelDefinition represents a model entry from the configuration.
// It contains information about a specific LLM model including its
// provider, API identifier, and default parameters.
type ModelDefinition struct {
	// Name is the user-facing alias for the model (e.g., "gpt-4-turbo")
	Name string `yaml:"name" json:"name"`

	// Provider links to a defined provider (e.g., "openai")
	Provider string `yaml:"provider" json:"provider"`

	// APIModelID is the actual ID used in API calls (e.g., "gpt-4-turbo")
	APIModelID string `yaml:"api_model_id" json:"api_model_id"`

	// Parameters is a map defining supported parameters for the model
	// (e.g., temperature, top_p, reasoning_effort)
	Parameters map[string]ParameterDefinition `yaml:"parameters" json:"parameters"`
}

// ParameterDefinition represents a parameter definition from the configuration.
// It defines the type, default value, and constraints for a model parameter.
type ParameterDefinition struct {
	// Type specifies the parameter type (float, int, string)
	Type string `yaml:"type" json:"type"`

	// Default specifies the default value for the parameter
	Default interface{} `yaml:"default" json:"default"`

	// Min specifies the minimum allowed value for numeric types (float, int)
	Min interface{} `yaml:"min,omitempty" json:"min,omitempty"`

	// Max specifies the maximum allowed value for numeric types (float, int)
	Max interface{} `yaml:"max,omitempty" json:"max,omitempty"`

	// EnumValues specifies the allowed values for string type parameters
	EnumValues []string `yaml:"enum_values,omitempty" json:"enum_values,omitempty"`
}

// ModelsConfig represents the full models configuration loaded from YAML.
type ModelsConfig struct {
	// APIKeySources maps provider names to environment variable names
	// for API keys (e.g., {"openai": "OPENAI_API_KEY"})
	APIKeySources map[string]string `yaml:"api_key_sources" json:"api_key_sources"`

	// Providers is a list of available LLM providers
	Providers []ProviderDefinition `yaml:"providers" json:"providers"`

	// Models is a list of available LLM models
	Models []ModelDefinition `yaml:"models" json:"models"`
}
