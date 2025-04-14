// internal/gemini/gemini_client.go
package gemini

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"strings"
	"sync"
	"time"

	"github.com/phrazzld/architect/internal/llm"
	"github.com/phrazzld/architect/internal/logutil"

	genai "github.com/google/generative-ai-go/genai"
	"google.golang.org/api/option"
)

// HTTPClient is an interface for an HTTP client
type HTTPClient interface {
	Do(req *http.Request) (*http.Response, error)
}

// geminiClient implements the Client interface using Google's genai SDK
type geminiClient struct {
	client      *genai.Client
	model       *genai.GenerativeModel
	modelName   string
	apiKey      string
	apiEndpoint string
	logger      logutil.LoggerInterface

	// Model info caching
	modelInfoCache map[string]*ModelInfo
	modelInfoMutex sync.RWMutex
	httpClient     HTTPClient
}

// geminiClientOption defines a function type for applying options to geminiClient
type geminiClientOption func(*geminiClient)

// newGeminiClient creates a new Gemini client with Google's genai SDK
func newGeminiClient(ctx context.Context, apiKey, modelName, apiEndpoint string, opts ...geminiClientOption) (Client, error) {
	if apiKey == "" {
		return nil, errors.New("API key cannot be empty")
	}

	if modelName == "" {
		return nil, errors.New("model name cannot be empty")
	}

	// Create standard logger for internal client use
	logger := logutil.NewLogger(logutil.InfoLevel, nil, "[gemini] ")

	// Prepare client options
	var clientOpts []option.ClientOption

	if apiEndpoint != "" {
		// Custom endpoint (likely for testing)
		logger.Debug("Using custom Gemini API endpoint: %s", apiEndpoint)
		clientOpts = append(clientOpts,
			option.WithEndpoint(apiEndpoint),
			option.WithoutAuthentication()) // Skip auth for mock server
	} else {
		// Default endpoint with API key
		clientOpts = append(clientOpts, option.WithAPIKey(apiKey))
	}

	// Initialize the Google genai client
	client, err := genai.NewClient(ctx, clientOpts...)
	if err != nil {
		return nil, fmt.Errorf("failed to create Gemini client: %w", err)
	}

	// Initialize the model with default settings
	model := client.GenerativeModel(modelName)
	config := DefaultModelConfig()
	model.SetMaxOutputTokens(config.MaxOutputTokens)
	model.SetTemperature(config.Temperature)
	model.SetTopP(config.TopP)

	// Create the client with default values
	gc := &geminiClient{
		client:         client,
		model:          model,
		modelName:      modelName,
		apiKey:         apiKey,
		apiEndpoint:    apiEndpoint,
		logger:         logger,
		modelInfoCache: make(map[string]*ModelInfo),
		modelInfoMutex: sync.RWMutex{},
		httpClient:     &http.Client{Timeout: 10 * time.Second}, // Default HTTP client
	}

	// Apply any custom options
	for _, opt := range opts {
		opt(gc)
	}

	return gc, nil
}

// GenerateContent sends a text prompt to Gemini and returns the generated content
func (c *geminiClient) GenerateContent(ctx context.Context, prompt string) (*GenerationResult, error) {
	if prompt == "" {
		return nil, &APIError{
			Original:   errors.New("prompt cannot be empty"),
			Type:       ErrorTypeInvalidRequest,
			Message:    "Cannot generate content with an empty prompt",
			Suggestion: "Provide a task description using the --task flag or --task-file option",
		}
	}

	// Generate content
	resp, err := c.model.GenerateContent(ctx, genai.Text(prompt))
	if err != nil {
		apiErr := FormatAPIError(err, 0)
		// Log detailed info for debugging
		c.logger.Debug("Gemini API Error: %s", apiErr.DebugInfo())
		return nil, apiErr
	}

	// Check for empty response
	if resp == nil {
		return nil, &APIError{
			Original:   errors.New("received nil response from Gemini API"),
			Type:       ErrorTypeUnknown,
			Message:    "Received an empty response from the Gemini API",
			Suggestion: "This is likely a temporary issue. Please try again in a few moments.",
		}
	}

	// Check for empty candidates
	if len(resp.Candidates) == 0 {
		return nil, &APIError{
			Original:   errors.New("received empty candidates from Gemini API"),
			Type:       ErrorTypeUnknown,
			Message:    "The Gemini API returned no generation candidates",
			Suggestion: "This could be due to content filtering. Try modifying your prompt or task description.",
		}
	}

	candidate := resp.Candidates[0]

	// Check for empty content
	if candidate.Content == nil || len(candidate.Content.Parts) == 0 {
		return &GenerationResult{
			Content:       "",
			FinishReason:  string(candidate.FinishReason),
			SafetyRatings: mapSafetyRatings(candidate.SafetyRatings),
			Truncated:     candidate.FinishReason == genai.FinishReasonMaxTokens,
		}, nil
	}

	// Extract text content
	var contentBuilder strings.Builder
	for _, part := range candidate.Content.Parts {
		if textPart, ok := part.(genai.Text); ok {
			contentBuilder.WriteString(string(textPart))
		}
	}

	// Get token usage if available
	var tokenCount int32
	if resp.UsageMetadata != nil {
		tokenCount = resp.UsageMetadata.TotalTokenCount
	}

	// Build result
	result := &GenerationResult{
		Content:       contentBuilder.String(),
		FinishReason:  string(candidate.FinishReason),
		SafetyRatings: mapSafetyRatings(candidate.SafetyRatings),
		TokenCount:    tokenCount,
		Truncated:     candidate.FinishReason == genai.FinishReasonMaxTokens,
	}

	return result, nil
}

// CountTokens counts the tokens in a given prompt
func (c *geminiClient) CountTokens(ctx context.Context, prompt string) (*TokenCount, error) {
	if prompt == "" {
		return &TokenCount{Total: 0}, nil
	}

	resp, err := c.model.CountTokens(ctx, genai.Text(prompt))
	if err != nil {
		apiErr := FormatAPIError(err, 0)
		apiErr.Message = "Failed to count tokens in prompt"
		apiErr.Suggestion = "Check your API key and internet connection. This operation is required before sending content to the API."

		// Log detailed info for debugging
		c.logger.Debug("Token counting error: %s", apiErr.DebugInfo())

		return nil, apiErr
	}

	return &TokenCount{
		Total: resp.TotalTokens,
	}, nil
}

// ModelDetailsResponse represents the API response for model details
type ModelDetailsResponse struct {
	Name                       string   `json:"name"`
	BaseModelID                string   `json:"baseModelId"`
	Version                    string   `json:"version"`
	DisplayName                string   `json:"displayName"`
	Description                string   `json:"description"`
	InputTokenLimit            int32    `json:"inputTokenLimit"`
	OutputTokenLimit           int32    `json:"outputTokenLimit"`
	SupportedGenerationMethods []string `json:"supportedGenerationMethods"`
	Temperature                float32  `json:"temperature"`
	TopP                       float32  `json:"topP"`
	TopK                       int32    `json:"topK"`
}

// GetModelInfo retrieves information about the current model
func (c *geminiClient) GetModelInfo(ctx context.Context) (*ModelInfo, error) {
	// Check cache first
	c.modelInfoMutex.RLock()
	if info, ok := c.modelInfoCache[c.modelName]; ok {
		c.modelInfoMutex.RUnlock()
		return info, nil
	}
	c.modelInfoMutex.RUnlock()

	// Not in cache, fetch from API
	info, err := c.fetchModelInfo(ctx, c.modelName)
	if err != nil {
		// If API fetch fails, use conservative defaults
		c.logger.Warn("Failed to fetch model info for %s: %v. Using default values.", c.modelName, err)

		info = &ModelInfo{
			Name:             c.modelName,
			InputTokenLimit:  30720, // Conservative default
			OutputTokenLimit: 8192,  // Conservative default
		}
	}

	// Cache the result (even default values to avoid repeated failures)
	c.modelInfoMutex.Lock()
	c.modelInfoCache[c.modelName] = info
	c.modelInfoMutex.Unlock()

	return info, nil
}

// fetchModelInfo calls the Generative Language API to get model details
func (c *geminiClient) fetchModelInfo(ctx context.Context, modelName string) (*ModelInfo, error) {
	var url string

	if c.apiEndpoint != "" {
		// Use custom endpoint with no authentication
		url = fmt.Sprintf("%s/v1beta/models/%s",
			strings.TrimSuffix(c.apiEndpoint, "/"), modelName)
	} else {
		// Use default endpoint with API key
		url = fmt.Sprintf("https://generativelanguage.googleapis.com/v1beta/models/%s?key=%s",
			modelName, c.apiKey)
	}

	// Create request
	req, err := http.NewRequestWithContext(ctx, "GET", url, nil)
	if err != nil {
		apiErr := &APIError{
			Original:   err,
			Type:       ErrorTypeNetwork,
			Message:    "Failed to create HTTP request for model information",
			Suggestion: "This is likely a temporary issue with network connectivity. Check your internet connection and try again.",
		}
		return nil, apiErr
	}

	// Make the request
	resp, err := c.httpClient.Do(req)
	if err != nil {
		apiErr := &APIError{
			Original:   err,
			Type:       ErrorTypeNetwork,
			Message:    "Failed to connect to Gemini API to fetch model information",
			Suggestion: "Check your internet connection and try again. If the issue persists, the API might be experiencing downtime.",
			Details:    err.Error(),
		}
		return nil, apiErr
	}
	defer func() { _ = resp.Body.Close() }()

	// Check status code
	if resp.StatusCode != http.StatusOK {
		body, _ := io.ReadAll(resp.Body)
		bodyStr := string(body)

		apiErr := FormatAPIError(
			fmt.Errorf("API returned error: %s", bodyStr),
			resp.StatusCode,
		)

		// Add model-specific details
		apiErr.Details = fmt.Sprintf("Model: %s, Status: %d, Response: %s",
			modelName, resp.StatusCode, bodyStr)

		// For 404 errors, provide more context about the model
		if resp.StatusCode == http.StatusNotFound {
			apiErr.Message = fmt.Sprintf("Model '%s' not found or not accessible", modelName)
			apiErr.Suggestion = "Check that the model name is correct and that you have access to it. Consider using a different model."
		}

		// Log detailed error for debugging
		c.logger.Debug("Model info error: %s", apiErr.DebugInfo())

		return nil, apiErr
	}

	// Parse response
	var modelDetails ModelDetailsResponse
	if err := json.NewDecoder(resp.Body).Decode(&modelDetails); err != nil {
		apiErr := &APIError{
			Original:   err,
			Type:       ErrorTypeInvalidRequest,
			Message:    "Failed to parse model information response from Gemini API",
			Suggestion: "This is likely a temporary API issue or a change in the API response format. Try again later.",
			Details:    err.Error(),
		}
		return nil, apiErr
	}

	// Convert to our internal model
	info := &ModelInfo{
		Name:             modelDetails.Name,
		InputTokenLimit:  modelDetails.InputTokenLimit,
		OutputTokenLimit: modelDetails.OutputTokenLimit,
	}

	c.logger.Debug("Fetched model info for %s: input limit=%d, output limit=%d",
		modelName, info.InputTokenLimit, info.OutputTokenLimit)

	return info, nil
}

// Close releases resources used by the client
func (c *geminiClient) Close() error {
	if c.client != nil {
		return c.client.Close()
	}
	return nil
}

// GetModelName returns the name of the model being used
func (c *geminiClient) GetModelName() string {
	return c.modelName
}

// GetTemperature returns the temperature setting for the model
func (c *geminiClient) GetTemperature() float32 {
	if c.model != nil && c.model.Temperature != nil {
		return *c.model.Temperature
	}
	return DefaultModelConfig().Temperature
}

// GetMaxOutputTokens returns the max output tokens setting for the model
func (c *geminiClient) GetMaxOutputTokens() int32 {
	if c.model != nil && c.model.MaxOutputTokens != nil {
		return int32(*c.model.MaxOutputTokens)
	}
	return DefaultModelConfig().MaxOutputTokens
}

// GetTopP returns the topP setting for the model
func (c *geminiClient) GetTopP() float32 {
	if c.model != nil && c.model.TopP != nil {
		return *c.model.TopP
	}
	return DefaultModelConfig().TopP
}

// mapSafetyRatings converts genai safety ratings to our internal format
func mapSafetyRatings(ratings []*genai.SafetyRating) []SafetyRating {
	if ratings == nil {
		return nil
	}

	result := make([]SafetyRating, len(ratings))
	for i, rating := range ratings {
		result[i] = SafetyRating{
			Category: string(rating.Category),
			Blocked:  rating.Blocked,
			Score:    float32(rating.Probability),
		}
	}

	return result
}

// Implement llm.LLMClient interface alongside the original Client interface

// toProviderSafety converts Gemini safety ratings to provider-agnostic safety info
func toProviderSafety(ratings []SafetyRating) []llm.Safety {
	if ratings == nil {
		return nil
	}

	safetyInfo := make([]llm.Safety, len(ratings))
	for i, rating := range ratings {
		safetyInfo[i] = llm.Safety{
			Category: rating.Category,
			Blocked:  rating.Blocked,
			Score:    rating.Score,
		}
	}
	return safetyInfo
}

// Adapter methods for implementing llm.LLMClient interface

// llmGenerateContent implements the llm.LLMClient interface for geminiClient
// This is an adapter method that calls the original GenerateContent method
// and converts the result to the provider-agnostic format
func (c *geminiClient) llmGenerateContent(ctx context.Context, prompt string) (*llm.ProviderResult, error) {
	// Call the original method
	result, err := c.GenerateContent(ctx, prompt)
	if err != nil {
		return nil, err
	}

	// Convert to provider-agnostic format
	return &llm.ProviderResult{
		Content:      result.Content,
		FinishReason: result.FinishReason,
		TokenCount:   result.TokenCount,
		Truncated:    result.Truncated,
		SafetyInfo:   toProviderSafety(result.SafetyRatings),
	}, nil
}

// llmCountTokens implements the llm.LLMClient interface for geminiClient
// This is an adapter method that calls the original CountTokens method
// and converts the result to the provider-agnostic format
func (c *geminiClient) llmCountTokens(ctx context.Context, prompt string) (*llm.ProviderTokenCount, error) {
	// Call the original method
	result, err := c.CountTokens(ctx, prompt)
	if err != nil {
		return nil, err
	}

	// Convert to provider-agnostic format
	return &llm.ProviderTokenCount{
		Total: result.Total,
	}, nil
}

// llmGetModelInfo implements the llm.LLMClient interface for geminiClient
// This is an adapter method that calls the original GetModelInfo method
// and converts the result to the provider-agnostic format
func (c *geminiClient) llmGetModelInfo(ctx context.Context) (*llm.ProviderModelInfo, error) {
	// Call the original method
	info, err := c.GetModelInfo(ctx)
	if err != nil {
		return nil, err
	}

	// Convert to provider-agnostic format
	return &llm.ProviderModelInfo{
		Name:             info.Name,
		InputTokenLimit:  info.InputTokenLimit,
		OutputTokenLimit: info.OutputTokenLimit,
	}, nil
}

// AsLLMClient returns a gemini.Client as an llm.LLMClient
// This is a constructor function that adapts the gemini.Client to the llm.LLMClient interface
func AsLLMClient(client Client) llm.LLMClient {
	// If it's already a geminiClient, return it directly
	if gc, ok := client.(*geminiClient); ok {
		return &geminiLLMAdapter{gc}
	}

	// Otherwise, wrap it in an adapter
	return &geminiLLMAdapter{client}
}

// geminiLLMAdapter adapts the gemini.Client to the llm.LLMClient interface
type geminiLLMAdapter struct {
	client Client
}

// GenerateContent implements the llm.LLMClient interface for the adapter
func (a *geminiLLMAdapter) GenerateContent(ctx context.Context, prompt string) (*llm.ProviderResult, error) {
	// If the client is a geminiClient, use its method directly
	if gc, ok := a.client.(*geminiClient); ok {
		return gc.llmGenerateContent(ctx, prompt)
	}

	// Otherwise, call the client method and convert the result
	result, err := a.client.GenerateContent(ctx, prompt)
	if err != nil {
		return nil, err
	}

	return &llm.ProviderResult{
		Content:      result.Content,
		FinishReason: result.FinishReason,
		TokenCount:   result.TokenCount,
		Truncated:    result.Truncated,
		SafetyInfo:   toProviderSafety(result.SafetyRatings),
	}, nil
}

// CountTokens implements the llm.LLMClient interface for the adapter
func (a *geminiLLMAdapter) CountTokens(ctx context.Context, prompt string) (*llm.ProviderTokenCount, error) {
	// If the client is a geminiClient, use its method directly
	if gc, ok := a.client.(*geminiClient); ok {
		return gc.llmCountTokens(ctx, prompt)
	}

	// Otherwise, call the client method and convert the result
	result, err := a.client.CountTokens(ctx, prompt)
	if err != nil {
		return nil, err
	}

	return &llm.ProviderTokenCount{
		Total: result.Total,
	}, nil
}

// GetModelInfo implements the llm.LLMClient interface for the adapter
func (a *geminiLLMAdapter) GetModelInfo(ctx context.Context) (*llm.ProviderModelInfo, error) {
	// If the client is a geminiClient, use its method directly
	if gc, ok := a.client.(*geminiClient); ok {
		return gc.llmGetModelInfo(ctx)
	}

	// Otherwise, call the client method and convert the result
	info, err := a.client.GetModelInfo(ctx)
	if err != nil {
		return nil, err
	}

	return &llm.ProviderModelInfo{
		Name:             info.Name,
		InputTokenLimit:  info.InputTokenLimit,
		OutputTokenLimit: info.OutputTokenLimit,
	}, nil
}

// GetModelName implements the llm.LLMClient interface for the adapter
func (a *geminiLLMAdapter) GetModelName() string {
	return a.client.GetModelName()
}

// Close implements the llm.LLMClient interface for the adapter
func (a *geminiLLMAdapter) Close() error {
	return a.client.Close()
}
