// Package architect contains the core application logic for the architect tool
package architect

import (
	"context"
	"errors"
	"fmt"
	"strings"

	"github.com/phrazzld/architect/internal/architect/interfaces"
	"github.com/phrazzld/architect/internal/gemini"
	"github.com/phrazzld/architect/internal/llm"
	"github.com/phrazzld/architect/internal/logutil"
	"github.com/phrazzld/architect/internal/openai"
	"github.com/phrazzld/architect/internal/registry"
)

// Define package-level error types for better error handling
var (
	// ErrEmptyResponse indicates the API returned an empty response
	ErrEmptyResponse = errors.New("received empty response from LLM")

	// ErrWhitespaceContent indicates the API returned only whitespace content
	ErrWhitespaceContent = errors.New("LLM returned an empty output text")

	// ErrSafetyBlocked indicates content was blocked by safety filters
	ErrSafetyBlocked = errors.New("content blocked by LLM safety filters")

	// ErrAPICall indicates a general API call error
	ErrAPICall = errors.New("error calling LLM API")

	// ErrClientInitialization indicates client initialization failed
	ErrClientInitialization = errors.New("error creating LLM client")

	// ErrUnsupportedModel indicates an unsupported model was requested
	ErrUnsupportedModel = errors.New("unsupported model type")

	// ErrModelNotFound indicates a model definition was not found in the registry
	ErrModelNotFound = errors.New("model definition not found in registry")
)

// APIService is defined in the interfaces package
// See github.com/phrazzld/architect/internal/architect/interfaces

// ProviderType represents the type of LLM provider
type ProviderType string

const (
	// ProviderGemini represents the Gemini provider
	ProviderGemini ProviderType = "gemini"
	// ProviderOpenAI represents the OpenAI provider
	ProviderOpenAI ProviderType = "openai"
	// ProviderUnknown represents an unknown provider
	ProviderUnknown ProviderType = "unknown"
)

// apiService implements the APIService interface
type apiService struct {
	logger              logutil.LoggerInterface
	newGeminiClientFunc func(ctx context.Context, apiKey, modelName, apiEndpoint string) (llm.LLMClient, error)
	newOpenAIClientFunc func(modelName string) (llm.LLMClient, error)
}

// DetectProviderFromModel detects the provider type from the model name
//
// Deprecated: This function uses hardcoded model name prefixes to detect providers.
// It is maintained only for backward compatibility with the legacy APIService.
//
// New code should use the Registry instead, which provides a more flexible, configuration-driven
// approach where models and providers are defined in a configuration file.
//
// The Registry is initialized in cmd/architect/main.go and is accessible via the
// registry.GetGlobalManager() function. Use NewRegistryAPIService instead of NewAPIService
// to take advantage of the registry pattern.
func DetectProviderFromModel(modelName string) ProviderType {
	// First attempt to use registry if available
	regManager := registry.GetGlobalManager(nil)
	if regManager != nil {
		provider, err := regManager.GetProviderForModel(modelName)
		if err == nil {
			// Convert provider string to ProviderType
			switch provider {
			case "gemini":
				return ProviderGemini
			case "openai":
				return ProviderOpenAI
			case "openrouter":
				// OpenRouter is not explicitly defined in ProviderType,
				// so we need to handle it in the calling code
				return ProviderUnknown
			}
		}
	}

	// Fall back to the original string matching logic
	if modelName == "" {
		return ProviderUnknown
	}

	// Check for Gemini models
	if len(modelName) >= 6 && modelName[:6] == "gemini" {
		return ProviderGemini
	}

	// Check for OpenAI GPT models
	if len(modelName) >= 3 && modelName[:3] == "gpt" {
		return ProviderOpenAI
	}

	// Check for other OpenAI models
	otherOpenAIModels := []string{
		"text-davinci",
		"davinci",
		"curie",
		"babbage",
		"ada",
		"text-embedding",
		"text-moderation",
		"whisper",
	}

	for _, prefix := range otherOpenAIModels {
		if len(modelName) >= len(prefix) && modelName[:len(prefix)] == prefix {
			return ProviderOpenAI
		}
	}

	// Unknown model type
	return ProviderUnknown
}

// newGeminiClientWrapper adapts the original Gemini client creation function to return llm.LLMClient
func newGeminiClientWrapper(ctx context.Context, apiKey, modelName, apiEndpoint string) (llm.LLMClient, error) {
	// Create the Gemini client
	client, err := gemini.NewClient(ctx, apiKey, modelName, apiEndpoint)
	if err != nil {
		return nil, err
	}

	// Convert to LLMClient
	return gemini.AsLLMClient(client), nil
}

// newOpenAIClientWrapper wraps the OpenAI client creation to match function signature
func newOpenAIClientWrapper(modelName string) (llm.LLMClient, error) {
	return openai.NewClient(modelName)
}

// NewAPIService creates a new instance of APIService
//
// Deprecated: Use NewRegistryAPIService instead, which provides more flexible
// and configurable model handling through the registry system.
func NewAPIService(logger logutil.LoggerInterface) interfaces.APIService {
	return &apiService{
		logger:              logger,
		newGeminiClientFunc: newGeminiClientWrapper,
		newOpenAIClientFunc: newOpenAIClientWrapper,
	}
}

// Internal helper to create the actual client (avoids duplication)
func (s *apiService) createLLMClient(ctx context.Context, apiKey, modelName, apiEndpoint string) (llm.LLMClient, error) {
	// Check for empty required parameters
	if apiKey == "" {
		return nil, fmt.Errorf("%w: API key is required", ErrClientInitialization)
	}
	if modelName == "" {
		return nil, fmt.Errorf("%w: model name is required", ErrClientInitialization)
	}

	// Check for context cancellation
	if ctx.Err() != nil {
		return nil, fmt.Errorf("%w: %v", ErrClientInitialization, ctx.Err())
	}

	// Log custom endpoint if provided
	if apiEndpoint != "" {
		s.logger.Debug("Using custom API endpoint: %s", apiEndpoint)
	}

	// Special case for testing with error-model
	if modelName == "error-model" {
		return nil, errors.New("test model error")
	}

	// Try to use the registry first if available
	regManager := registry.GetGlobalManager(nil)
	if regManager != nil {
		provider, err := regManager.GetProviderForModel(modelName)
		if err == nil {
			// Successfully got provider from registry
			s.logger.Debug("Registry model detection found provider '%s' for model '%s'", provider, modelName)

			var client llm.LLMClient
			var err error

			// Create client based on provider from registry
			switch provider {
			case "gemini":
				s.logger.Debug("Using Gemini provider for model %s (registry-based)", modelName)
				client, err = s.newGeminiClientFunc(ctx, apiKey, modelName, apiEndpoint)
			case "openai":
				s.logger.Debug("Using OpenAI provider for model %s (registry-based)", modelName)
				client, err = s.newOpenAIClientFunc(modelName)
			default:
				// Unsupported provider type in legacy client
				s.logger.Warn("Provider '%s' found in registry but not supported by legacy API service", provider)
				return nil, fmt.Errorf("%w: provider '%s' not supported by legacy API service", ErrUnsupportedModel, provider)
			}

			// Return client or error
			return client, err
		} else {
			s.logger.Debug("Model '%s' not found in registry, falling back to string matching", modelName)
		}
	} else {
		s.logger.Debug("Registry not available, using string matching for model '%s'", modelName)
	}

	// Fall back to legacy string matching approach
	providerType := DetectProviderFromModel(modelName)

	// Initialize the appropriate client based on provider type
	var client llm.LLMClient
	var err error

	switch providerType {
	case ProviderGemini:
		s.logger.Debug("Using Gemini provider for model %s (string matching)", modelName)
		client, err = s.newGeminiClientFunc(ctx, apiKey, modelName, apiEndpoint)
	case ProviderOpenAI:
		s.logger.Debug("Using OpenAI provider for model %s (string matching)", modelName)
		client, err = s.newOpenAIClientFunc(modelName)
	case ProviderUnknown:
		return nil, fmt.Errorf("%w: %s", ErrUnsupportedModel, modelName)
	}

	// Handle client creation error
	if err != nil {
		// Check if it's already an API error with enhanced details from Gemini
		if apiErr, ok := gemini.IsAPIError(err); ok {
			return nil, fmt.Errorf("%w: %s", ErrClientInitialization, apiErr.UserFacingError())
		}

		// Check if it's an OpenAI API error
		if apiErr, ok := openai.IsOpenAIError(err); ok {
			return nil, fmt.Errorf("%w: %s", ErrClientInitialization, apiErr.UserFacingError())
		}

		// Wrap the original error
		return nil, fmt.Errorf("%w: %v", ErrClientInitialization, err)
	}

	return client, nil
}

// InitLLMClient initializes and returns an LLM client based on the model name
func (s *apiService) InitLLMClient(ctx context.Context, apiKey, modelName, apiEndpoint string) (llm.LLMClient, error) {
	return s.createLLMClient(ctx, apiKey, modelName, apiEndpoint)
}

// ProcessLLMResponse processes a provider-agnostic API response and extracts content
func (s *apiService) ProcessLLMResponse(result *llm.ProviderResult) (string, error) {
	// Check for nil result
	if result == nil {
		return "", fmt.Errorf("%w: result is nil", ErrEmptyResponse)
	}

	// Check for empty content
	if result.Content == "" {
		var errDetails strings.Builder

		// Add finish reason if available
		if result.FinishReason != "" {
			fmt.Fprintf(&errDetails, " (Finish Reason: %s)", result.FinishReason)
		}

		// Check for safety blocks
		if len(result.SafetyInfo) > 0 {
			blocked := false
			safetyInfo := ""
			for _, safety := range result.SafetyInfo {
				if safety.Blocked {
					blocked = true
					safetyInfo += fmt.Sprintf(" Blocked by Safety Category: %s;", safety.Category)
				}
			}

			if blocked {
				if errDetails.Len() > 0 {
					errDetails.WriteString(" ")
				}
				errDetails.WriteString("Safety Blocking:")
				errDetails.WriteString(safetyInfo)

				// If we have safety blocks, use the specific safety error
				return "", fmt.Errorf("%w%s", ErrSafetyBlocked, errDetails.String())
			}
		}

		// If we don't have safety blocks, use the generic empty response error
		return "", fmt.Errorf("%w%s", ErrEmptyResponse, errDetails.String())
	}

	// Check for whitespace-only content
	if strings.TrimSpace(result.Content) == "" {
		return "", ErrWhitespaceContent
	}

	return result.Content, nil
}

// GetModelParameters retrieves parameter values from the registry for a given model
// For the legacy API service, this returns an empty map since it doesn't use the registry
func (s *apiService) GetModelParameters(modelName string) (map[string]interface{}, error) {
	// The legacy API service doesn't use the registry, so we return an empty map
	// This will be overridden by the registry-based implementation
	return make(map[string]interface{}), nil
}

// GetModelDefinition retrieves the full model definition from the registry
// For the legacy API service, this returns an error since it doesn't use the registry
func (s *apiService) GetModelDefinition(modelName string) (*registry.ModelDefinition, error) {
	// The legacy API service doesn't use the registry
	return nil, fmt.Errorf("model definitions not available in legacy API service")
}

// GetModelTokenLimits retrieves token limits from the registry for a given model
// For the legacy API service, this returns zero values with an error
func (s *apiService) GetModelTokenLimits(modelName string) (contextWindow, maxOutputTokens int32, err error) {
	// The legacy API service doesn't use the registry
	return 0, 0, fmt.Errorf("token limits not available in legacy API service")
}

// IsEmptyResponseError checks if an error is related to empty API responses.
//
// This method uses two strategies for identifying empty response errors:
// 1. Type checking: Using errors.Is() with known error types (preferred method)
// 2. String matching: Checking for key phrases in the error message
//
// The string matching approach is necessary to handle errors from different providers
// that may not use the same error types but still indicate empty responses.
// This provides a provider-agnostic way to identify common error conditions.
//
// IMPORTANT: String matching introduces fragility - if a provider changes their
// error message format, this method may fail to correctly identify errors.
// When adding a new provider, consider:
// - First using errors.Is() with proper error types when possible
// - Adding provider-specific error detection before falling back to string matching
// - Documenting common error message patterns in provider-specific error docs
// - Adding test cases for new provider error messages
func (s *apiService) IsEmptyResponseError(err error) bool {
	if err == nil {
		return false
	}

	// Check for specific error types using errors.Is
	if errors.Is(err, ErrEmptyResponse) || errors.Is(err, ErrWhitespaceContent) {
		return true
	}

	// Convert the error message to lowercase for case-insensitive matching
	errMsg := strings.ToLower(err.Error())

	// Check for common empty response phrases
	if strings.Contains(errMsg, "empty response") ||
		strings.Contains(errMsg, "empty content") ||
		strings.Contains(errMsg, "empty output") ||
		strings.Contains(errMsg, "empty result") {
		return true
	}

	// Check for provider-specific empty response patterns
	if strings.Contains(errMsg, "zero candidates") ||
		strings.Contains(errMsg, "empty candidates") ||
		strings.Contains(errMsg, "no output") {
		return true
	}

	return false
}

// IsSafetyBlockedError checks if an error is related to safety filters.
//
// This method uses a dual approach for identifying safety-related errors:
// 1. Type checking: Using errors.Is() with ErrSafetyBlocked (preferred method)
// 2. String matching: Examining error messages for safety-related keywords
//
// The string matching approach serves several purposes:
// - Provides a provider-agnostic way to detect content policy violations
// - Handles cases where different LLM providers use different error structures
// - Offers a safety net for errors that don't wrap the standard error types
//
// RISKS:
// - Provider error message changes could cause safety errors to go undetected
// - Overly generic string matching might cause false positives
// - Different providers may use different terminology for safety violations
//
// MAINTENANCE NOTES:
// - When integrating new LLM providers, update this method with their safety error patterns
// - Attempt to standardize wrapped error types first before falling back to string matching
// - Consider adding provider-specific prefix checks to reduce false positives
// - Keep the test suite updated with examples of safety errors from all supported providers
func (s *apiService) IsSafetyBlockedError(err error) bool {
	if err == nil {
		return false
	}

	// Check for specific error types using errors.Is
	if errors.Is(err, ErrSafetyBlocked) {
		return true
	}

	// Convert to lowercase for case-insensitive matching
	errMsg := strings.ToLower(err.Error())

	// Check for common safety-related phrases
	if strings.Contains(errMsg, "safety") ||
		strings.Contains(errMsg, "content policy") ||
		strings.Contains(errMsg, "content filter") ||
		strings.Contains(errMsg, "content_filter") {
		return true
	}

	// Check for provider-specific moderation terminology
	if strings.Contains(errMsg, "moderation") ||
		strings.Contains(errMsg, "blocked") ||
		strings.Contains(errMsg, "filtered") ||
		strings.Contains(errMsg, "harm_category") {
		return true
	}

	return false
}

// GetErrorDetails extracts detailed information from an error.
//
// This method provides a provider-agnostic way to extract user-friendly error details
// from provider-specific error types. It handles:
// - Gemini API errors: Using gemini.IsAPIError and its UserFacingError method
// - OpenAI API errors: Using openai.IsOpenAIError and its UserFacingError method
// - Generic errors: Falling back to standard Error() method
//
// The approach balances provider-specific error handling with a consistent interface,
// allowing the application to present detailed error information without exposing
// provider implementation details to higher layers.
//
// When adding a new provider:
// - Implement an IsAPIError function for the provider
// - Ensure provider errors expose a UserFacingError() method
// - Add a provider-specific check in this method
// - Update tests to verify correct error detail extraction
func (s *apiService) GetErrorDetails(err error) string {
	// Handle nil error case
	if err == nil {
		return "no error"
	}

	// Check if it's a Gemini API error with enhanced details
	if apiErr, ok := gemini.IsAPIError(err); ok {
		return apiErr.UserFacingError()
	}

	// Check if it's an OpenAI API error with enhanced details
	if apiErr, ok := openai.IsOpenAIError(err); ok {
		return apiErr.UserFacingError()
	}

	// Return the error string for other error types
	return err.Error()
}

// Legacy APIService method for parameter validation

// ValidateModelParameter is a stub implementation for the legacy apiService
// It always returns true since the legacy implementation doesn't support parameter validation
func (s *apiService) ValidateModelParameter(modelName, paramName string, value interface{}) (bool, error) {
	// Log that this is a stub implementation
	s.logger.Debug("ValidateModelParameter called on legacy apiService, always returns true")

	// Always return true for legacy implementation
	return true, nil
}
