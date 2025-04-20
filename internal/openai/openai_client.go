// Package openai provides a client for interacting with the OpenAI API
package openai

import (
	"context"
	"errors"
	"fmt"
	"strings"

	"github.com/openai/openai-go"
	"github.com/openai/openai-go/option"
	"github.com/phrazzld/architect/internal/llm"
)

// openaiAPI defines the operations we need from the OpenAI client
type openaiAPI interface {
	createChatCompletion(ctx context.Context, messages []openai.ChatCompletionMessageParamUnion, model string) (*openai.ChatCompletion, error)
	createChatCompletionWithParams(ctx context.Context, params openai.ChatCompletionNewParams) (*openai.ChatCompletion, error)
}

// openaiClient implements the llm.LLMClient interface for OpenAI
type openaiClient struct {
	api       openaiAPI
	modelName string
	// Parameters for the OpenAI API
	temperature      *float64
	topP             *float64
	presencePenalty  *float64
	frequencyPenalty *float64
	maxTokens        *int
	reasoningEffort  *string // For O-series model reasoning parameter
}

// realOpenAIAPI implements openaiAPI using the real OpenAI client
type realOpenAIAPI struct {
	client openai.Client
}

// createChatCompletion implements the openaiAPI interface method using the real OpenAI client
func (api *realOpenAIAPI) createChatCompletion(ctx context.Context, messages []openai.ChatCompletionMessageParamUnion, model string) (*openai.ChatCompletion, error) {
	completion, err := api.client.Chat.Completions.New(ctx, openai.ChatCompletionNewParams{
		Messages: messages,
		Model:    model,
	})
	if err != nil {
		// Format the error using our error handling
		return nil, FormatAPIError(err, 0)
	}
	return completion, nil
}

// createChatCompletionWithParams makes an API call with specific parameters
func (api *realOpenAIAPI) createChatCompletionWithParams(ctx context.Context, params openai.ChatCompletionNewParams) (*openai.ChatCompletion, error) {
	completion, err := api.client.Chat.Completions.New(ctx, params)
	if err != nil {
		// Format the error using our error handling
		return nil, FormatAPIError(err, 0)
	}
	return completion, nil
}

// NewClient creates a new OpenAI client
func NewClient(apiKey, modelName, apiBase string) (llm.LLMClient, error) {
	if apiKey == "" {
		return nil, fmt.Errorf("API key is required")
	}

	if modelName == "" {
		return nil, fmt.Errorf("model name is required")
	}

	// Create API client
	var clientOptions []option.ClientOption
	clientOptions = append(clientOptions, option.WithAPIKey(apiKey))

	// Use custom API base URL if provided
	if apiBase != "" {
		clientOptions = append(clientOptions, option.WithBaseURL(apiBase))
	}

	client, err := openai.NewClient(clientOptions...)
	if err != nil {
		return nil, fmt.Errorf("failed to create OpenAI client: %w", err)
	}

	// Create the real API implementation
	api := &realOpenAIAPI{
		client: client,
	}

	// Create the client
	return &openaiClient{
		api:       api,
		modelName: modelName,
	}, nil
}

// GenerateContent implements the LLMClient interface
func (c *openaiClient) GenerateContent(ctx context.Context, prompt string, params map[string]interface{}) (*llm.ProviderResult, error) {
	if prompt == "" {
		return nil, CreateAPIError(
			llm.CategoryInvalidRequest,
			"Empty prompt",
			errors.New("prompt cannot be empty"),
			"Please provide a valid prompt",
		)
	}

	// Create message array with system role (if present) and user message
	var messages []openai.ChatCompletionMessageParamUnion

	// Add system message if detected
	if strings.Contains(prompt, "<system>") && strings.Contains(prompt, "</system>") {
		systemContent := extractTag(prompt, "system")
		if systemContent != "" {
			// Add system message
			systemMsg := openai.ChatCompletionSystemMessageParam{
				Role:    "system",
				Content: systemContent,
			}
			messages = append(messages, openai.ChatCompletionSystemMessageParam(systemMsg))

			// Remove system content from prompt
			prompt = removeTag(prompt, "system")
		}
	}

	// Add user message
	userMsg := openai.ChatCompletionUserMessageParam{
		Role: "user",
		Content: []openai.ChatCompletionUserMessageContentParam{
			openai.ChatCompletionUserMessageContentTextParam{
				Type: "text",
				Text: prompt,
			},
		},
	}
	messages = append(messages, openai.ChatCompletionUserMessageParam(userMsg))

	// Build request parameters
	requestParams := openai.ChatCompletionNewParams{
		Messages: messages,
		Model:    c.modelName,
	}

	// Apply fixed parameters from client
	if c.temperature != nil {
		requestParams.Temperature = c.temperature
	}
	if c.topP != nil {
		requestParams.TopP = c.topP
	}
	if c.presencePenalty != nil {
		requestParams.PresencePenalty = c.presencePenalty
	}
	if c.frequencyPenalty != nil {
		requestParams.FrequencyPenalty = c.frequencyPenalty
	}
	if c.maxTokens != nil {
		requestParams.MaxTokens = c.maxTokens
	}
	if c.reasoningEffort != nil {
		// For O-series models
		requestParams.ExtraParams = make(map[string]interface{})
		requestParams.ExtraParams["reasoning_effort"] = *c.reasoningEffort
	}

	// Override with parameters from request
	if params != nil {
		applyOpenAIParameters(&requestParams, params)
	}

	// Make the API call
	completion, err := c.api.createChatCompletionWithParams(ctx, requestParams)
	if err != nil {
		return nil, err
	}

	// Handle empty choices
	if len(completion.Choices) == 0 {
		return nil, fmt.Errorf("no completions returned from OpenAI API")
	}

	// Extract content
	choice := completion.Choices[0]
	content := ""

	if choice.Message.Content != nil {
		content = *choice.Message.Content
	}

	// Get finish reason
	finishReason := "unknown"
	if choice.FinishReason != nil {
		finishReason = *choice.FinishReason
	}

	// Build result
	result := &llm.ProviderResult{
		Content:      content,
		FinishReason: finishReason,
		Truncated:    finishReason == "length",
	}

	return result, nil
}

// GetModelName returns the model name
func (c *openaiClient) GetModelName() string {
	return c.modelName
}

// Close cleans up any resources (nothing to clean up for OpenAI)
func (c *openaiClient) Close() error {
	return nil
}

// Helper function to extract content between XML tags
func extractTag(text, tag string) string {
	startTag := fmt.Sprintf("<%s>", tag)
	endTag := fmt.Sprintf("</%s>", tag)

	startIdx := strings.Index(text, startTag)
	if startIdx == -1 {
		return ""
	}
	startIdx += len(startTag)

	endIdx := strings.Index(text[startIdx:], endTag)
	if endIdx == -1 {
		return ""
	}

	return text[startIdx : startIdx+endIdx]
}

// Helper function to remove content between XML tags including the tags
func removeTag(text, tag string) string {
	startTag := fmt.Sprintf("<%s>", tag)
	endTag := fmt.Sprintf("</%s>", tag)

	startIdx := strings.Index(text, startTag)
	if startIdx == -1 {
		return text
	}

	endIdx := strings.Index(text[startIdx:], endTag)
	if endIdx == -1 {
		return text
	}
	endIdx = startIdx + endIdx + len(endTag)

	return text[:startIdx] + text[endIdx:]
}

// Apply parameters from map to OpenAI request parameters
func applyOpenAIParameters(params *openai.ChatCompletionNewParams, customParams map[string]interface{}) {
	// Apply standard parameters
	if temp, ok := customParams["temperature"]; ok {
		if v, ok := temp.(float64); ok {
			params.Temperature = &v
		}
	}

	if topP, ok := customParams["top_p"]; ok {
		if v, ok := topP.(float64); ok {
			params.TopP = &v
		}
	}

	if presencePenalty, ok := customParams["presence_penalty"]; ok {
		if v, ok := presencePenalty.(float64); ok {
			params.PresencePenalty = &v
		}
	}

	if frequencyPenalty, ok := customParams["frequency_penalty"]; ok {
		if v, ok := frequencyPenalty.(float64); ok {
			params.FrequencyPenalty = &v
		}
	}

	if maxTokens, ok := customParams["max_tokens"]; ok {
		if v, ok := maxTokens.(int); ok {
			params.MaxTokens = &v
		} else if v, ok := maxTokens.(float64); ok {
			intVal := int(v)
			params.MaxTokens = &intVal
		}
	}

	// Handle special parameters for newer models
	if reasoningEffort, ok := customParams["reasoning_effort"]; ok {
		if params.ExtraParams == nil {
			params.ExtraParams = make(map[string]interface{})
		}
		params.ExtraParams["reasoning_effort"] = reasoningEffort
	}
}
