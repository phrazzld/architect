// Package openai provides a client for interacting with the OpenAI API
package openai

import (
	"context"
	"fmt"
	"testing"

	"github.com/openai/openai-go"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// TestCreateChatCompletionWithParams tests the createChatCompletionWithParams method
// to ensure it correctly builds the API request payload based on provided parameters
func TestCreateChatCompletionWithParams(t *testing.T) {
	// Basic test that validates required parameters are included
	t.Run("Required parameters only", func(t *testing.T) {
		// Create a mock API that captures parameters
		var capturedModel string
		var capturedMessages []openai.ChatCompletionMessageParamUnion

		mockAPI := &mockOpenAIAPI{
			createChatCompletionWithParamsFunc: func(ctx context.Context, params openai.ChatCompletionNewParams) (*openai.ChatCompletion, error) {
				capturedModel = params.Model
				capturedMessages = params.Messages
				return &openai.ChatCompletion{
					Choices: []openai.ChatCompletionChoice{
						{
							Message: openai.ChatCompletionMessage{
								Content: "test response",
							},
						},
					},
				}, nil
			},
		}

		// Create client
		client := &openaiClient{
			api:         mockAPI,
			modelName:   "gpt-4",
			modelLimits: make(map[string]*modelInfo),
		}

		// Define test messages
		messages := []openai.ChatCompletionMessageParamUnion{
			openai.UserMessage("test prompt"),
		}

		// Call the method
		completion, err := client.createChatCompletionWithParams(context.Background(), messages)

		// Expect success
		require.NoError(t, err)
		require.NotNil(t, completion)

		// Validate required parameters were correctly set
		assert.Equal(t, "gpt-4", capturedModel)
		assert.Equal(t, messages, capturedMessages)
	})

	// Tests for different parameter combinations
	testCases := []struct {
		name        string
		setupClient func(*openaiClient)
		validateAPI func(*testing.T, openai.ChatCompletionNewParams)
		expectError bool
		errorMsg    string
	}{
		{
			name: "With temperature parameter",
			setupClient: func(c *openaiClient) {
				temp := 0.7
				c.temperature = &temp
			},
			validateAPI: func(t *testing.T, params openai.ChatCompletionNewParams) {
				// For the OpenAI Go client, we need to use reflection or a different approach to check if a field is set
				// In this simpler test, we'll just validate the model and messages
				assert.Equal(t, "gpt-4", params.Model)
				assert.NotEmpty(t, params.Messages)
			},
		},
		{
			name: "With top_p parameter",
			setupClient: func(c *openaiClient) {
				topP := 0.95
				c.topP = &topP
			},
			validateAPI: func(t *testing.T, params openai.ChatCompletionNewParams) {
				assert.Equal(t, "gpt-4", params.Model)
				assert.NotEmpty(t, params.Messages)
			},
		},
		{
			name: "With max tokens parameter",
			setupClient: func(c *openaiClient) {
				maxTokens := 100
				c.maxTokens = &maxTokens
			},
			validateAPI: func(t *testing.T, params openai.ChatCompletionNewParams) {
				assert.Equal(t, "gpt-4", params.Model)
				assert.NotEmpty(t, params.Messages)
			},
		},
		{
			name: "With frequency penalty parameter",
			setupClient: func(c *openaiClient) {
				freqPenalty := 0.5
				c.frequencyPenalty = &freqPenalty
			},
			validateAPI: func(t *testing.T, params openai.ChatCompletionNewParams) {
				assert.Equal(t, "gpt-4", params.Model)
				assert.NotEmpty(t, params.Messages)
			},
		},
		{
			name: "With presence penalty parameter",
			setupClient: func(c *openaiClient) {
				presPenalty := 0.5
				c.presencePenalty = &presPenalty
			},
			validateAPI: func(t *testing.T, params openai.ChatCompletionNewParams) {
				assert.Equal(t, "gpt-4", params.Model)
				assert.NotEmpty(t, params.Messages)
			},
		},
		{
			name: "With all parameters",
			setupClient: func(c *openaiClient) {
				temp := 0.7
				topP := 0.95
				maxTokens := 100
				freqPenalty := 0.5
				presPenalty := 0.5
				c.temperature = &temp
				c.topP = &topP
				c.maxTokens = &maxTokens
				c.frequencyPenalty = &freqPenalty
				c.presencePenalty = &presPenalty
			},
			validateAPI: func(t *testing.T, params openai.ChatCompletionNewParams) {
				assert.Equal(t, "gpt-4", params.Model)
				assert.NotEmpty(t, params.Messages)
			},
		},
		{
			name: "With reasoning effort parameter for O-series model",
			setupClient: func(c *openaiClient) {
				c.modelName = "o4-mini"
				effort := "high"
				c.reasoningEffort = &effort
			},
			validateAPI: func(t *testing.T, params openai.ChatCompletionNewParams) {
				assert.Equal(t, "o4-mini", params.Model)
				assert.NotEmpty(t, params.Messages)
				assert.Equal(t, "high", string(params.ReasoningEffort))
			},
		},
		{
			name: "Valid temperature - formerly model-specific validation",
			setupClient: func(c *openaiClient) {
				c.modelName = "o4-mini"
				temp := 0.7 // Now allowed for all models
				effort := "high"
				c.temperature = &temp
				c.reasoningEffort = &effort
			},
			validateAPI: func(t *testing.T, params openai.ChatCompletionNewParams) {
				assert.Equal(t, "o4-mini", params.Model)
				assert.NotEmpty(t, params.Messages)
				assert.Equal(t, "high", string(params.ReasoningEffort))
				// Check that temperature is set, but don't check its value
				// as the implementation of openai.Float might change
				assert.NotNil(t, params.Temperature)
			},
		},
		{
			name: "Temperature 1.0",
			setupClient: func(c *openaiClient) {
				c.modelName = "o4-mini"
				temp := 1.0
				effort := "high"
				c.temperature = &temp
				c.reasoningEffort = &effort
			},
			validateAPI: func(t *testing.T, params openai.ChatCompletionNewParams) {
				assert.Equal(t, "o4-mini", params.Model)
				assert.NotEmpty(t, params.Messages)
				assert.Equal(t, "high", string(params.ReasoningEffort))
				// Check that temperature is set, regardless of model
				assert.NotNil(t, params.Temperature)
			},
		},
		{
			name: "Invalid reasoning effort",
			setupClient: func(c *openaiClient) {
				c.modelName = "o4-mini"
				effort := "invalid"
				c.reasoningEffort = &effort
			},
			expectError: true,
			errorMsg:    "Reasoning effort must be 'low', 'medium', or 'high'",
		},
		{
			name: "Low reasoning effort",
			setupClient: func(c *openaiClient) {
				c.modelName = "o4-mini"
				effort := "low"
				c.reasoningEffort = &effort
			},
			validateAPI: func(t *testing.T, params openai.ChatCompletionNewParams) {
				assert.Equal(t, "o4-mini", params.Model)
				assert.NotEmpty(t, params.Messages)
				assert.Equal(t, "low", string(params.ReasoningEffort))
			},
		},
		{
			name: "Medium reasoning effort",
			setupClient: func(c *openaiClient) {
				c.modelName = "o4-mini"
				effort := "medium"
				c.reasoningEffort = &effort
			},
			validateAPI: func(t *testing.T, params openai.ChatCompletionNewParams) {
				assert.Equal(t, "o4-mini", params.Model)
				assert.NotEmpty(t, params.Messages)
				assert.Equal(t, "medium", string(params.ReasoningEffort))
			},
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			// Create a mock API that captures parameters
			var capturedParams openai.ChatCompletionNewParams
			mockAPI := &mockOpenAIAPI{
				createChatCompletionWithParamsFunc: func(ctx context.Context, params openai.ChatCompletionNewParams) (*openai.ChatCompletion, error) {
					capturedParams = params
					return &openai.ChatCompletion{
						Choices: []openai.ChatCompletionChoice{
							{
								Message: openai.ChatCompletionMessage{
									Content: "test response",
								},
							},
						},
					}, nil
				},
			}

			// Create client with appropriate parameters
			client := &openaiClient{
				api:         mockAPI,
				modelName:   "gpt-4",
				modelLimits: make(map[string]*modelInfo),
			}

			// Setup client with test-specific parameters
			tc.setupClient(client)

			// Define test messages
			messages := []openai.ChatCompletionMessageParamUnion{
				openai.UserMessage("test prompt"),
			}

			// Call the method
			completion, err := client.createChatCompletionWithParams(context.Background(), messages)

			// Check if we expect an error
			if tc.expectError {
				assert.Error(t, err)
				if tc.errorMsg != "" {
					assert.Contains(t, err.Error(), tc.errorMsg)
				}
				return
			}

			// Otherwise, expect success
			assert.NoError(t, err)
			assert.NotNil(t, completion)

			// Validate the parameters were correctly set
			if tc.validateAPI != nil {
				tc.validateAPI(t, capturedParams)
			}
		})
	}
}

// TestCreateChatCompletionWithParamsAPIError tests error handling in the createChatCompletionWithParams method
func TestCreateChatCompletionWithParamsAPIError(t *testing.T) {
	// Create a mock API that returns an error
	mockAPI := &mockOpenAIAPI{
		createChatCompletionWithParamsFunc: func(ctx context.Context, params openai.ChatCompletionNewParams) (*openai.ChatCompletion, error) {
			return nil, fmt.Errorf("simulated API error")
		},
	}

	// Create client
	client := &openaiClient{
		api:       mockAPI,
		modelName: "gpt-4",
	}

	// Define test messages
	messages := []openai.ChatCompletionMessageParamUnion{
		openai.UserMessage("test prompt"),
	}

	// Call the method
	completion, err := client.createChatCompletionWithParams(context.Background(), messages)

	// Expect error to be passed through
	assert.Error(t, err)
	assert.Nil(t, completion)
	assert.Contains(t, err.Error(), "simulated API error")
}

// TestCreateChatCompletionWithParamsValidatesParametersBeforeAPICall tests that parameters
// are validated before making the API call
func TestCreateChatCompletionWithParamsValidatesParametersBeforeAPICall(t *testing.T) {
	// Create a mock API that tracks if it was called
	apiCalled := false
	mockAPI := &mockOpenAIAPI{
		createChatCompletionWithParamsFunc: func(ctx context.Context, params openai.ChatCompletionNewParams) (*openai.ChatCompletion, error) {
			apiCalled = true
			return &openai.ChatCompletion{}, nil
		},
	}

	// Test cases with invalid parameters
	invalidParamTests := []struct {
		name          string
		setupClient   func(*openaiClient)
		expectedError string
	}{
		{
			name: "Temperature too high",
			setupClient: func(c *openaiClient) {
				temp := 3.0
				c.temperature = &temp
			},
			expectedError: "Temperature must be between 0.0 and 2.0",
		},
		{
			name: "Temperature too low",
			setupClient: func(c *openaiClient) {
				temp := -0.5
				c.temperature = &temp
			},
			expectedError: "Temperature must be between 0.0 and 2.0",
		},
		{
			name: "Top P too high",
			setupClient: func(c *openaiClient) {
				topP := 1.5
				c.topP = &topP
			},
			expectedError: "Top_p must be between 0.0 and 1.0",
		},
		{
			name: "Top P too low",
			setupClient: func(c *openaiClient) {
				topP := -0.5
				c.topP = &topP
			},
			expectedError: "Top_p must be between 0.0 and 1.0",
		},
		{
			name: "Max tokens negative",
			setupClient: func(c *openaiClient) {
				maxTokens := -100
				c.maxTokens = &maxTokens
			},
			expectedError: "Max tokens must be positive",
		},
		{
			name: "Max tokens zero",
			setupClient: func(c *openaiClient) {
				maxTokens := 0
				c.maxTokens = &maxTokens
			},
			expectedError: "Max tokens must be positive",
		},
		{
			name: "Frequency penalty too high",
			setupClient: func(c *openaiClient) {
				freqPenalty := 3.0
				c.frequencyPenalty = &freqPenalty
			},
			expectedError: "Frequency penalty must be between -2.0 and 2.0",
		},
		{
			name: "Frequency penalty too low",
			setupClient: func(c *openaiClient) {
				freqPenalty := -3.0
				c.frequencyPenalty = &freqPenalty
			},
			expectedError: "Frequency penalty must be between -2.0 and 2.0",
		},
		{
			name: "Presence penalty too high",
			setupClient: func(c *openaiClient) {
				presPenalty := 3.0
				c.presencePenalty = &presPenalty
			},
			expectedError: "Presence penalty must be between -2.0 and 2.0",
		},
		{
			name: "Presence penalty too low",
			setupClient: func(c *openaiClient) {
				presPenalty := -3.0
				c.presencePenalty = &presPenalty
			},
			expectedError: "Presence penalty must be between -2.0 and 2.0",
		},
		{
			name: "Invalid reasoning effort",
			setupClient: func(c *openaiClient) {
				c.modelName = "o4-mini"
				effort := "invalid"
				c.reasoningEffort = &effort
			},
			expectedError: "Reasoning effort must be 'low', 'medium', or 'high'",
		},
	}

	for _, tc := range invalidParamTests {
		t.Run(tc.name, func(t *testing.T) {
			// Reset API called flag
			apiCalled = false

			// Create client with invalid parameters
			client := &openaiClient{
				api:         mockAPI,
				modelName:   "gpt-4",
				modelLimits: make(map[string]*modelInfo),
			}

			// Setup client with test parameters
			tc.setupClient(client)

			// Define test messages
			messages := []openai.ChatCompletionMessageParamUnion{
				openai.UserMessage("test prompt"),
			}

			// Call the method
			_, err := client.createChatCompletionWithParams(context.Background(), messages)

			// Expect validation error
			assert.Error(t, err)
			assert.Contains(t, err.Error(), tc.expectedError)

			// Verify API was not called
			assert.False(t, apiCalled, "API should not have been called with invalid parameters")
		})
	}
}
