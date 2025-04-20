// internal/providers/openai/provider.go
package openai

import (
	"context"
	"fmt"
	"os"
	"strings"
	"sync"

	"github.com/phrazzld/architect/internal/llm"
	"github.com/phrazzld/architect/internal/logutil"
	"github.com/phrazzld/architect/internal/openai"
	"github.com/phrazzld/architect/internal/providers"
)

// OpenAIProvider implements the Provider interface for OpenAI models.
type OpenAIProvider struct {
	logger logutil.LoggerInterface
	apiKey string
}

// NewProvider creates a new instance of OpenAIProvider.
func NewProvider(logger logutil.LoggerInterface) providers.Provider {
	// If no logger provided, create a default one
	if logger == nil {
		logger = logutil.NewLogger(logutil.InfoLevel, nil, "[openai-provider] ")
	}

	return &OpenAIProvider{
		logger: logger,
	}
}

// CreateClient implements the Provider interface.
func (p *OpenAIProvider) CreateClient(
	ctx context.Context,
	apiKey string,
	modelID string,
	apiEndpoint string,
) (llm.LLMClient, error) {
	p.logger.Debug("Creating OpenAI client for model: %s", modelID)

	// For OpenAI, we should ALWAYS use the OPENAI_API_KEY environment variable
	// or a valid OpenAI API key provided as an argument

	// Initialize the effective API key variable
	var effectiveAPIKey string

	// Check if the provided API key is a valid OpenAI key
	if apiKey != "" && strings.HasPrefix(apiKey, "sk-") {
		// Use the provided key since it looks like a valid OpenAI key
		effectiveAPIKey = apiKey
		p.logger.Debug("Using provided API key which matches OpenAI key format")
	} else {
		// Always fall back to the OPENAI_API_KEY environment variable
		effectiveAPIKey = os.Getenv("OPENAI_API_KEY")
		if effectiveAPIKey == "" {
			return nil, fmt.Errorf("no valid OpenAI API key provided and OPENAI_API_KEY environment variable not set")
		}

		// Verify this looks like an OpenAI key
		if !strings.HasPrefix(effectiveAPIKey, "sk-") {
			p.logger.Warn("OpenAI API key does not have expected format (should start with 'sk-'). This will likely fail.")
		} else {
			p.logger.Debug("Using API key from OPENAI_API_KEY environment variable")
		}
	}

	// Store API key for later use
	p.apiKey = effectiveAPIKey

	// We need to set the environment variable temporarily for the openai client
	// Since the OpenAI client directly reads from the environment
	oldAPIKey := os.Getenv("OPENAI_API_KEY")
	if err := os.Setenv("OPENAI_API_KEY", effectiveAPIKey); err != nil {
		return nil, fmt.Errorf("failed to set OpenAI API key in environment: %w", err)
	}
	defer func() {
		var err error
		if oldAPIKey != "" {
			err = os.Setenv("OPENAI_API_KEY", oldAPIKey)
		} else {
			err = os.Unsetenv("OPENAI_API_KEY")
		}
		if err != nil {
			p.logger.Warn("Failed to restore original OpenAI API key environment variable: %v", err)
		}
	}()

	// Create the client using the existing OpenAI implementation
	baseClient, err := openai.NewClient(modelID)
	if err != nil {
		return nil, fmt.Errorf("failed to create OpenAI client: %w", err)
	}

	// Wrap the client in our parameter-aware adapter
	return NewOpenAIClientAdapter(baseClient), nil
}

// OpenAIClientAdapter wraps the standard OpenAI client to provide
// parameter-aware capabilities
type OpenAIClientAdapter struct {
	client llm.LLMClient
	params map[string]interface{}
	mu     sync.RWMutex
}

// NewOpenAIClientAdapter creates a new adapter for the OpenAI client
func NewOpenAIClientAdapter(client llm.LLMClient) *OpenAIClientAdapter {
	return &OpenAIClientAdapter{
		client: client,
		params: make(map[string]interface{}),
		mu:     sync.RWMutex{},
	}
}

// SetParameters sets the parameters to use for API calls
func (a *OpenAIClientAdapter) SetParameters(params map[string]interface{}) {
	a.mu.Lock()
	defer a.mu.Unlock()
	a.params = params
}

// GenerateContent implements the llm.LLMClient interface and applies parameters
func (a *OpenAIClientAdapter) GenerateContent(ctx context.Context, prompt string, params map[string]interface{}) (*llm.ProviderResult, error) {
	a.mu.Lock()
	defer a.mu.Unlock()

	// Apply parameters if provided
	if len(params) > 0 {
		a.params = params
	}

	// We need to convert the generic parameters from the Registry to OpenAI-specific settings
	// We'll try to apply common parameters to the underlying client if it supports them

	// For OpenAI adapter, we use safer type assertions with specific interfaces
	// Since we don't control the underlying OpenAI client implementation directly

	// Apply temperature parameter if client supports it
	if client, ok := a.client.(interface{ SetTemperature(temp float32) }); ok && a.params != nil {
		if temp, ok := a.getFloatParam("temperature"); ok {
			client.SetTemperature(temp)
		}
	}

	// Apply top_p parameter if client supports it
	if client, ok := a.client.(interface{ SetTopP(topP float32) }); ok && a.params != nil {
		if topP, ok := a.getFloatParam("top_p"); ok {
			client.SetTopP(topP)
		}
	}

	// Apply max_tokens parameter if client supports it
	if client, ok := a.client.(interface{ SetMaxTokens(tokens int32) }); ok && a.params != nil {
		if maxTokens, ok := a.getIntParam("max_tokens"); ok {
			client.SetMaxTokens(maxTokens)
		} else if maxTokens, ok := a.getIntParam("max_output_tokens"); ok {
			// Try the Gemini-style parameter name as a fallback
			client.SetMaxTokens(maxTokens)
		}
	}

	// Apply frequency_penalty parameter if client supports it
	if client, ok := a.client.(interface{ SetFrequencyPenalty(penalty float32) }); ok && a.params != nil {
		if penalty, ok := a.getFloatParam("frequency_penalty"); ok {
			client.SetFrequencyPenalty(penalty)
		}
	}

	// Apply presence_penalty parameter if client supports it
	if client, ok := a.client.(interface{ SetPresencePenalty(penalty float32) }); ok && a.params != nil {
		if penalty, ok := a.getFloatParam("presence_penalty"); ok {
			client.SetPresencePenalty(penalty)
		}
	}

	// Call the underlying client's implementation
	// After updating the llm.LLMClient interface, all clients should implement
	// the new interface with parameters. The adapter's main purpose is to convert
	// between parameter formats and apply them to the wrapped client.
	return a.client.GenerateContent(ctx, prompt, a.params)
}

// getFloatParam safely extracts a float parameter
func (a *OpenAIClientAdapter) getFloatParam(name string) (float32, bool) {
	if a.params == nil {
		return 0, false
	}

	if val, ok := a.params[name]; ok {
		switch v := val.(type) {
		case float64:
			return float32(v), true
		case float32:
			return v, true
		case int:
			return float32(v), true
		case int32:
			return float32(v), true
		case int64:
			return float32(v), true
		}
	}
	return 0, false
}

// getIntParam safely extracts an integer parameter
func (a *OpenAIClientAdapter) getIntParam(name string) (int32, bool) {
	if a.params == nil {
		return 0, false
	}

	if val, ok := a.params[name]; ok {
		switch v := val.(type) {
		case int:
			return int32(v), true
		case int32:
			return v, true
		case int64:
			return int32(v), true
		case float64:
			return int32(v), true
		case float32:
			return int32(v), true
		}
	}
	return 0, false
}

// GetModelName implements the llm.LLMClient interface
func (a *OpenAIClientAdapter) GetModelName() string {
	return a.client.GetModelName()
}

// Close implements the llm.LLMClient interface
func (a *OpenAIClientAdapter) Close() error {
	return a.client.Close()
}
