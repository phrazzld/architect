package gemini

import (
	"context"
	"errors"
	"fmt"
	"strings"

	"github.com/phrazzld/architect/internal/llm"
	"github.com/phrazzld/architect/internal/logutil"

	genai "github.com/google/generative-ai-go/genai"
	"google.golang.org/api/option"
)

// geminiClient implements the llm.LLMClient interface using Google's genai SDK
type geminiClient struct {
	client      *genai.Client
	model       *genai.GenerativeModel
	modelName   string
	apiKey      string
	apiEndpoint string
	logger      logutil.LoggerInterface
}

// geminiClientOption defines a function type for applying options to geminiClient
type geminiClientOption func(*geminiClient)

// newGeminiClient creates a new Gemini client with Google's genai SDK
func newGeminiClient(ctx context.Context, apiKey, modelName, apiEndpoint string, opts ...geminiClientOption) (llm.LLMClient, error) {
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
		client:      client,
		model:       model,
		modelName:   modelName,
		apiKey:      apiKey,
		apiEndpoint: apiEndpoint,
		logger:      logger,
	}

	// Apply any custom options
	for _, opt := range opts {
		opt(gc)
	}

	return gc, nil
}

// GenerateContent implements the llm.LLMClient interface
func (c *geminiClient) GenerateContent(ctx context.Context, prompt string, params map[string]interface{}) (*llm.ProviderResult, error) {
	if prompt == "" {
		return nil, CreateAPIError(
			llm.CategoryInvalidRequest,
			"Cannot generate content with an empty prompt",
			errors.New("prompt cannot be empty"),
			"Provide a task description using the --instructions flag",
		)
	}

	// Apply parameters if provided
	if params != nil {
		// Temperature
		if temp, ok := params["temperature"]; ok {
			switch v := temp.(type) {
			case float64:
				c.model.SetTemperature(float32(v))
			case float32:
				c.model.SetTemperature(v)
			case int:
				c.model.SetTemperature(float32(v))
			}
		}

		// TopP
		if topP, ok := params["top_p"]; ok {
			switch v := topP.(type) {
			case float64:
				c.model.SetTopP(float32(v))
			case float32:
				c.model.SetTopP(v)
			case int:
				c.model.SetTopP(float32(v))
			}
		}

		// TopK
		if topK, ok := params["top_k"]; ok {
			switch v := topK.(type) {
			case int:
				c.model.SetTopK(int32(v))
			case int32:
				c.model.SetTopK(v)
			case int64:
				c.model.SetTopK(int32(v))
			case float64:
				c.model.SetTopK(int32(v))
			}
		}

		// MaxOutputTokens
		if maxTokens, ok := params["max_output_tokens"]; ok {
			switch v := maxTokens.(type) {
			case int:
				c.model.SetMaxOutputTokens(int32(v))
			case int32:
				c.model.SetMaxOutputTokens(v)
			case int64:
				c.model.SetMaxOutputTokens(int32(v))
			case float64:
				c.model.SetMaxOutputTokens(int32(v))
			}
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
		return nil, CreateAPIError(
			llm.CategoryUnknown,
			"Received an empty response from the Gemini API",
			errors.New("received nil response from Gemini API"),
			"This is likely a temporary issue. Please try again in a few moments.",
		)
	}

	// Check for empty candidates
	if len(resp.Candidates) == 0 {
		return nil, CreateAPIError(
			llm.CategoryUnknown,
			"The Gemini API returned no generation candidates",
			errors.New("received empty candidates from Gemini API"),
			"This could be due to content filtering. Try modifying your prompt or task description.",
		)
	}

	candidate := resp.Candidates[0]

	// Check for empty content
	if candidate.Content == nil || len(candidate.Content.Parts) == 0 {
		return &llm.ProviderResult{
			Content:      "",
			FinishReason: string(candidate.FinishReason),
			SafetyInfo:   toProviderSafety(mapSafetyRatings(candidate.SafetyRatings)),
			Truncated:    candidate.FinishReason == genai.FinishReasonMaxTokens,
		}, nil
	}

	// Extract text content
	var contentBuilder strings.Builder
	for _, part := range candidate.Content.Parts {
		if textPart, ok := part.(genai.Text); ok {
			contentBuilder.WriteString(string(textPart))
		}
	}

	// Build provider-agnostic result
	result := &llm.ProviderResult{
		Content:      contentBuilder.String(),
		FinishReason: string(candidate.FinishReason),
		Truncated:    candidate.FinishReason == genai.FinishReasonMaxTokens,
		SafetyInfo:   toProviderSafety(mapSafetyRatings(candidate.SafetyRatings)),
	}

	return result, nil
}

// Close implements the llm.LLMClient interface by releasing resources
func (c *geminiClient) Close() error {
	if c.client != nil {
		return c.client.Close()
	}
	return nil
}

// GetModelName implements the llm.LLMClient interface by returning the model name
func (c *geminiClient) GetModelName() string {
	return c.modelName
}

// Helper functions to support the code

// WithLogger allows injecting a custom logger
func WithLogger(logger logutil.LoggerInterface) geminiClientOption {
	return func(c *geminiClient) {
		c.logger = logger
	}
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
