// Package openai contains tests for the OpenAI client adapter
package openai

import (
	"context"
	"errors"
	"testing"

	"github.com/phrazzld/architect/internal/llm"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// MockLLMClientWithParamChecking is a specialized mock for tracking parameter-related calls
type MockLLMClientWithParamChecking struct {
	// Mock function implementations
	GenerateContentFunc func(ctx context.Context, prompt string, params map[string]interface{}) (*llm.ProviderResult, error)
	CountTokensFunc     func(ctx context.Context, prompt string) (*llm.ProviderTokenCount, error)
	GetModelInfoFunc    func(ctx context.Context) (*llm.ProviderModelInfo, error)
	GetModelNameFunc    func() string
	CloseFunc           func() error

	// Parameter tracking
	TemperatureSet       bool
	TemperatureValue     float32
	TopPSet              bool
	TopPValue            float32
	MaxTokensSet         bool
	MaxTokensValue       int32
	FreqPenaltySet       bool
	FreqPenaltyValue     float32
	PresencePenaltySet   bool
	PresencePenaltyValue float32
	GenerateParams       map[string]interface{}

	// Method call tracking
	GenerateContentCalled bool
	CountTokensCalled     bool
	GetModelInfoCalled    bool
	GetModelNameCalled    bool
	CloseCalled           bool
}

// SetTemperature implementation for parameter checking
func (m *MockLLMClientWithParamChecking) SetTemperature(temp float32) {
	m.TemperatureSet = true
	m.TemperatureValue = temp
}

// SetTopP implementation for parameter checking
func (m *MockLLMClientWithParamChecking) SetTopP(topP float32) {
	m.TopPSet = true
	m.TopPValue = topP
}

// SetMaxTokens implementation for parameter checking
func (m *MockLLMClientWithParamChecking) SetMaxTokens(tokens int32) {
	m.MaxTokensSet = true
	m.MaxTokensValue = tokens
}

// SetFrequencyPenalty implementation for parameter checking
func (m *MockLLMClientWithParamChecking) SetFrequencyPenalty(penalty float32) {
	m.FreqPenaltySet = true
	m.FreqPenaltyValue = penalty
}

// SetPresencePenalty implementation for parameter checking
func (m *MockLLMClientWithParamChecking) SetPresencePenalty(penalty float32) {
	m.PresencePenaltySet = true
	m.PresencePenaltyValue = penalty
}

// GenerateContent overrides the method to track calls
func (m *MockLLMClientWithParamChecking) GenerateContent(ctx context.Context, prompt string, params map[string]interface{}) (*llm.ProviderResult, error) {
	m.GenerateContentCalled = true
	m.GenerateParams = params

	if m.GenerateContentFunc != nil {
		return m.GenerateContentFunc(ctx, prompt, params)
	}
	return &llm.ProviderResult{Content: "Mock response"}, nil
}

// CountTokens overrides the method to track calls
func (m *MockLLMClientWithParamChecking) CountTokens(ctx context.Context, prompt string) (*llm.ProviderTokenCount, error) {
	m.CountTokensCalled = true

	if m.CountTokensFunc != nil {
		return m.CountTokensFunc(ctx, prompt)
	}
	return &llm.ProviderTokenCount{Total: int32(len(prompt) / 4)}, nil
}

// GetModelInfo overrides the method to track calls
func (m *MockLLMClientWithParamChecking) GetModelInfo(ctx context.Context) (*llm.ProviderModelInfo, error) {
	m.GetModelInfoCalled = true

	if m.GetModelInfoFunc != nil {
		return m.GetModelInfoFunc(ctx)
	}
	return &llm.ProviderModelInfo{
		Name:             "gpt-4",
		InputTokenLimit:  8192,
		OutputTokenLimit: 2048,
	}, nil
}

// GetModelName overrides the method to track calls
func (m *MockLLMClientWithParamChecking) GetModelName() string {
	m.GetModelNameCalled = true

	if m.GetModelNameFunc != nil {
		return m.GetModelNameFunc()
	}
	return "gpt-4"
}

// Close overrides the method to track calls
func (m *MockLLMClientWithParamChecking) Close() error {
	m.CloseCalled = true

	if m.CloseFunc != nil {
		return m.CloseFunc()
	}
	return nil
}

// TestNewOpenAIClientAdapter verifies that NewOpenAIClientAdapter correctly initializes
// the adapter with all required fields
func TestNewOpenAIClientAdapter(t *testing.T) {
	// Create mock client
	mockClient := &MockLLMClient{}

	// Create adapter
	adapter := NewOpenAIClientAdapter(mockClient)

	// Verify adapter is properly initialized
	require.NotNil(t, adapter, "Adapter should not be nil")
	assert.Equal(t, mockClient, adapter.client, "Adapter should store the provided client")
	assert.NotNil(t, adapter.params, "Adapter should initialize params map")
	assert.Empty(t, adapter.params, "Params map should be empty initially")

	// Verify adapter implements LLMClient interface
	var _ llm.LLMClient = adapter
}

// TestSetParameters verifies that SetParameters correctly updates the parameter map
func TestSetParameters(t *testing.T) {
	// Create adapter with mock client
	mockClient := &MockLLMClient{}
	adapter := NewOpenAIClientAdapter(mockClient)

	// Initial params should be empty
	assert.Empty(t, adapter.params, "Initial params should be empty")

	// Set parameters
	testParams := map[string]interface{}{
		"temperature": 0.7,
		"top_p":       0.8,
		"max_tokens":  100,
	}
	adapter.SetParameters(testParams)

	// Verify parameters are stored
	assert.Equal(t, testParams, adapter.params, "SetParameters should store the provided map")

	// Update parameters
	updatedParams := map[string]interface{}{
		"temperature":       0.5,
		"frequency_penalty": 0.2,
	}
	adapter.SetParameters(updatedParams)

	// Verify parameters are completely replaced, not merged
	assert.Equal(t, updatedParams, adapter.params, "SetParameters should replace the entire params map")
}

// TestGenerateContentDelegation verifies that GenerateContent correctly delegates to the
// underlying client and applies parameters
func TestGenerateContentDelegation(t *testing.T) {
	tests := []struct {
		name                string
		adapterParams       map[string]interface{}
		requestParams       map[string]interface{}
		expectedTemperature float32
		expectedTopP        float32
		expectedMaxTokens   int32
		expectedFreqPenalty float32
		expectedPresPenalty float32
		mockResponse        *llm.ProviderResult
		mockError           error
	}{
		{
			name:                "No parameters",
			adapterParams:       nil,
			requestParams:       nil,
			expectedTemperature: 0,
			expectedTopP:        0,
			expectedMaxTokens:   0,
			expectedFreqPenalty: 0,
			expectedPresPenalty: 0,
			mockResponse:        &llm.ProviderResult{Content: "Response with no parameters"},
			mockError:           nil,
		},
		{
			name: "Adapter-level parameters",
			adapterParams: map[string]interface{}{
				"temperature":       0.7,
				"top_p":             0.8,
				"max_tokens":        100,
				"frequency_penalty": 0.2,
				"presence_penalty":  0.3,
			},
			requestParams:       nil,
			expectedTemperature: 0.7,
			expectedTopP:        0.8,
			expectedMaxTokens:   100,
			expectedFreqPenalty: 0.2,
			expectedPresPenalty: 0.3,
			mockResponse:        &llm.ProviderResult{Content: "Response with adapter parameters"},
			mockError:           nil,
		},
		{
			name: "Request-level parameters override adapter parameters",
			adapterParams: map[string]interface{}{
				"temperature":       0.7,
				"top_p":             0.8,
				"max_tokens":        100,
				"frequency_penalty": 0.2,
				"presence_penalty":  0.3,
			},
			requestParams: map[string]interface{}{
				"temperature":      0.5,
				"max_tokens":       200,
				"presence_penalty": 0.1,
			},
			expectedTemperature: 0.5,
			expectedTopP:        0, // Not checking this parameter in this test
			expectedMaxTokens:   200,
			expectedFreqPenalty: 0, // Not checking this parameter in this test
			expectedPresPenalty: 0.1,
			mockResponse:        &llm.ProviderResult{Content: "Response with overridden parameters"},
			mockError:           nil,
		},
		{
			name:                "Error case",
			adapterParams:       nil,
			requestParams:       nil,
			expectedTemperature: 0,
			expectedTopP:        0,
			expectedMaxTokens:   0,
			expectedFreqPenalty: 0,
			expectedPresPenalty: 0,
			mockResponse:        nil,
			mockError:           errors.New("mock API error"),
		},
		{
			name:          "Alternative parameter names (Gemini-style)",
			adapterParams: nil,
			requestParams: map[string]interface{}{
				"max_output_tokens": 150, // Gemini-style parameter
			},
			expectedTemperature: 0,
			expectedTopP:        0,
			expectedMaxTokens:   150, // Should be converted to max_tokens
			expectedFreqPenalty: 0,
			expectedPresPenalty: 0,
			mockResponse:        &llm.ProviderResult{Content: "Response with Gemini-style parameters"},
			mockError:           nil,
		},
		{
			name:          "Type conversion test",
			adapterParams: nil,
			requestParams: map[string]interface{}{
				"temperature":       float32(0.6),
				"top_p":             int(1),
				"max_tokens":        float64(300),
				"frequency_penalty": int32(2),
				"presence_penalty":  int64(1),
			},
			expectedTemperature: 0.6,
			expectedTopP:        1.0,
			expectedMaxTokens:   300,
			expectedFreqPenalty: 2.0,
			expectedPresPenalty: 1.0,
			mockResponse:        &llm.ProviderResult{Content: "Response with type-converted parameters"},
			mockError:           nil,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Create mock client with parameter checking
			mockClient := &MockLLMClientWithParamChecking{}
			mockClient.GenerateContentFunc = func(ctx context.Context, prompt string, params map[string]interface{}) (*llm.ProviderResult, error) {
				return tt.mockResponse, tt.mockError
			}

			// Create adapter
			adapter := NewOpenAIClientAdapter(mockClient)

			// Set adapter parameters if provided
			if tt.adapterParams != nil {
				adapter.SetParameters(tt.adapterParams)
			}

			// Call GenerateContent
			prompt := "Test prompt"
			result, err := adapter.GenerateContent(context.Background(), prompt, tt.requestParams)

			// Verify error behavior
			if tt.mockError != nil {
				assert.Error(t, err)
				assert.Equal(t, tt.mockError, err)
				assert.Nil(t, result)
			} else {
				assert.NoError(t, err)
				assert.Equal(t, tt.mockResponse, result)
			}

			// Verify GenerateContent was called on the underlying client
			assert.True(t, mockClient.GenerateContentCalled, "GenerateContent should be called on the underlying client")

			// Verify parameters were properly set on the client
			if tt.expectedTemperature > 0 {
				assert.True(t, mockClient.TemperatureSet, "Temperature should be set on the client")
				assert.Equal(t, tt.expectedTemperature, mockClient.TemperatureValue)
			}

			if tt.expectedTopP > 0 {
				assert.True(t, mockClient.TopPSet, "TopP should be set on the client")
				assert.Equal(t, tt.expectedTopP, mockClient.TopPValue)
			}

			if tt.expectedMaxTokens > 0 {
				assert.True(t, mockClient.MaxTokensSet, "MaxTokens should be set on the client")
				assert.Equal(t, tt.expectedMaxTokens, mockClient.MaxTokensValue)
			}

			if tt.expectedFreqPenalty > 0 {
				assert.True(t, mockClient.FreqPenaltySet, "FrequencyPenalty should be set on the client")
				assert.Equal(t, tt.expectedFreqPenalty, mockClient.FreqPenaltyValue)
			}

			if tt.expectedPresPenalty > 0 {
				assert.True(t, mockClient.PresencePenaltySet, "PresencePenalty should be set on the client")
				assert.Equal(t, tt.expectedPresPenalty, mockClient.PresencePenaltyValue)
			}

			// Verify params were passed to the underlying client
			if tt.requestParams != nil {
				// The adapter should replace its params with the request params
				assert.Equal(t, tt.requestParams, mockClient.GenerateParams)
			} else if tt.adapterParams != nil {
				// The adapter should use its pre-set params
				assert.Equal(t, tt.adapterParams, mockClient.GenerateParams)
			}
		})
	}
}

// TestDelegationMethods verifies that the adapter correctly delegates
// all LLMClient interface methods to the underlying client
func TestDelegationMethods(t *testing.T) {
	// Create mock responses for each method
	mockTokenCount := &llm.ProviderTokenCount{Total: 42}
	mockModelInfo := &llm.ProviderModelInfo{
		Name:             "test-model",
		InputTokenLimit:  10000,
		OutputTokenLimit: 2000,
	}
	mockModelName := "custom-gpt-model"

	// Create error responses for each method
	tokenCountError := errors.New("token count error")
	modelInfoError := errors.New("model info error")
	closeError := errors.New("close error")

	// Test successful delegation
	t.Run("Successful delegation", func(t *testing.T) {
		// Create mock client with predefined responses
		mockClient := &MockLLMClientWithParamChecking{}
		mockClient.CountTokensFunc = func(ctx context.Context, prompt string) (*llm.ProviderTokenCount, error) {
			return mockTokenCount, nil
		}
		mockClient.GetModelInfoFunc = func(ctx context.Context) (*llm.ProviderModelInfo, error) {
			return mockModelInfo, nil
		}
		mockClient.GetModelNameFunc = func() string {
			return mockModelName
		}
		mockClient.CloseFunc = func() error {
			return nil
		}

		// Create adapter
		adapter := NewOpenAIClientAdapter(mockClient)

		// Test CountTokens delegation
		tokenCount, err := adapter.CountTokens(context.Background(), "test prompt")
		assert.NoError(t, err)
		assert.Equal(t, mockTokenCount, tokenCount)
		assert.True(t, mockClient.CountTokensCalled)

		// Test GetModelInfo delegation
		modelInfo, err := adapter.GetModelInfo(context.Background())
		assert.NoError(t, err)
		assert.Equal(t, mockModelInfo, modelInfo)
		assert.True(t, mockClient.GetModelInfoCalled)

		// Test GetModelName delegation
		modelName := adapter.GetModelName()
		assert.Equal(t, mockModelName, modelName)
		assert.True(t, mockClient.GetModelNameCalled)

		// Test Close delegation
		err = adapter.Close()
		assert.NoError(t, err)
		assert.True(t, mockClient.CloseCalled)
	})

	// Test error delegation
	t.Run("Error delegation", func(t *testing.T) {
		// Create mock client with error responses
		mockClient := &MockLLMClientWithParamChecking{}
		mockClient.CountTokensFunc = func(ctx context.Context, prompt string) (*llm.ProviderTokenCount, error) {
			return nil, tokenCountError
		}
		mockClient.GetModelInfoFunc = func(ctx context.Context) (*llm.ProviderModelInfo, error) {
			return nil, modelInfoError
		}
		mockClient.CloseFunc = func() error {
			return closeError
		}

		// Create adapter
		adapter := NewOpenAIClientAdapter(mockClient)

		// Test CountTokens error delegation
		tokenCount, err := adapter.CountTokens(context.Background(), "test prompt")
		assert.Error(t, err)
		assert.Equal(t, tokenCountError, err)
		assert.Nil(t, tokenCount)

		// Test GetModelInfo error delegation
		modelInfo, err := adapter.GetModelInfo(context.Background())
		assert.Error(t, err)
		assert.Equal(t, modelInfoError, err)
		assert.Nil(t, modelInfo)

		// Test Close error delegation
		err = adapter.Close()
		assert.Error(t, err)
		assert.Equal(t, closeError, err)
	})
}

// TestGetModelInfoTokenLimitOverrides verifies the adapter's token limit handling logic
func TestGetModelInfoTokenLimitOverrides(t *testing.T) {
	testCases := []struct {
		name           string
		modelName      string
		clientResponse *llm.ProviderModelInfo
		clientError    error
		expectError    bool
		expectedLimits *llm.ProviderModelInfo
	}{
		{
			name:      "Client response is used when valid",
			modelName: "gpt-4",
			clientResponse: &llm.ProviderModelInfo{
				Name:             "gpt-4",
				InputTokenLimit:  8192,
				OutputTokenLimit: 2048,
			},
			clientError: nil,
			expectError: false,
			expectedLimits: &llm.ProviderModelInfo{
				Name:             "gpt-4",
				InputTokenLimit:  8192,
				OutputTokenLimit: 2048,
			},
		},
		{
			name:           "Error from client is propagated",
			modelName:      "gpt-4",
			clientResponse: nil,
			clientError:    errors.New("model info error"),
			expectError:    true,
			expectedLimits: nil,
		},
		{
			name:      "Zero limits are replaced with defaults for known models (gpt-4)",
			modelName: "gpt-4",
			clientResponse: &llm.ProviderModelInfo{
				Name:             "gpt-4",
				InputTokenLimit:  0, // Invalid limit
				OutputTokenLimit: 0, // Invalid limit
			},
			clientError: nil,
			expectError: false,
			expectedLimits: &llm.ProviderModelInfo{
				Name:             "gpt-4",
				InputTokenLimit:  8192, // Default for gpt-4
				OutputTokenLimit: 2048, // Default for gpt-4
			},
		},
		{
			name:      "Zero limits are replaced with defaults for known models (gpt-4-turbo)",
			modelName: "gpt-4-turbo",
			clientResponse: &llm.ProviderModelInfo{
				Name:             "gpt-4-turbo",
				InputTokenLimit:  0, // Invalid limit
				OutputTokenLimit: 0, // Invalid limit
			},
			clientError: nil,
			expectError: false,
			expectedLimits: &llm.ProviderModelInfo{
				Name:             "gpt-4-turbo",
				InputTokenLimit:  128000, // Default for gpt-4-turbo
				OutputTokenLimit: 4096,   // Default for gpt-4-turbo
			},
		},
		{
			name:      "Zero limits are replaced with defaults for known models (gpt-3.5-turbo)",
			modelName: "gpt-3.5-turbo",
			clientResponse: &llm.ProviderModelInfo{
				Name:             "gpt-3.5-turbo",
				InputTokenLimit:  0, // Invalid limit
				OutputTokenLimit: 0, // Invalid limit
			},
			clientError: nil,
			expectError: false,
			expectedLimits: &llm.ProviderModelInfo{
				Name:             "gpt-3.5-turbo",
				InputTokenLimit:  16385, // Default for gpt-3.5-turbo
				OutputTokenLimit: 4096,  // Default for gpt-3.5-turbo
			},
		},
		{
			name:      "Zero limits are replaced with defaults for unknown models",
			modelName: "unknown-model",
			clientResponse: &llm.ProviderModelInfo{
				Name:             "unknown-model",
				InputTokenLimit:  0, // Invalid limit
				OutputTokenLimit: 0, // Invalid limit
			},
			clientError: nil,
			expectError: false,
			expectedLimits: &llm.ProviderModelInfo{
				Name:             "unknown-model",
				InputTokenLimit:  4096, // Default for unknown models
				OutputTokenLimit: 2048, // Default for unknown models
			},
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			// Create mock client
			mockClient := &MockLLMClientWithParamChecking{}
			mockClient.GetModelInfoFunc = func(ctx context.Context) (*llm.ProviderModelInfo, error) {
				return tc.clientResponse, tc.clientError
			}
			mockClient.GetModelNameFunc = func() string {
				return tc.modelName
			}

			// Create adapter
			adapter := NewOpenAIClientAdapter(mockClient)

			// Call GetModelInfo
			result, err := adapter.GetModelInfo(context.Background())

			// Verify error behavior
			if tc.expectError {
				assert.Error(t, err)
				assert.Nil(t, result)
				return
			}

			assert.NoError(t, err)
			assert.NotNil(t, result)

			// Verify limits
			assert.Equal(t, tc.expectedLimits.Name, result.Name)
			assert.Equal(t, tc.expectedLimits.InputTokenLimit, result.InputTokenLimit)
			assert.Equal(t, tc.expectedLimits.OutputTokenLimit, result.OutputTokenLimit)
		})
	}
}

// MockLLMClientExtended is an extended mock for tracking parameter setting
type MockLLMClientExtended struct {
	// Mock function implementations
	GenerateContentFunc func(ctx context.Context, prompt string, params map[string]interface{}) (*llm.ProviderResult, error)
	CountTokensFunc     func(ctx context.Context, prompt string) (*llm.ProviderTokenCount, error)
	GetModelInfoFunc    func(ctx context.Context) (*llm.ProviderModelInfo, error)
	GetModelNameFunc    func() string
	CloseFunc           func() error

	// Parameter tracking
	SetTemperatureCalls     []float32
	SetTopPCalls            []float32
	SetMaxTokensCalls       []int32
	SetFreqPenaltyCalls     []float32
	SetPresencePenaltyCalls []float32
	GenerateContentParams   []map[string]interface{}
}

// SetTemperature implementation
func (m *MockLLMClientExtended) SetTemperature(temp float32) {
	m.SetTemperatureCalls = append(m.SetTemperatureCalls, temp)
}

// SetTopP implementation
func (m *MockLLMClientExtended) SetTopP(topP float32) {
	m.SetTopPCalls = append(m.SetTopPCalls, topP)
}

// SetMaxTokens implementation
func (m *MockLLMClientExtended) SetMaxTokens(tokens int32) {
	m.SetMaxTokensCalls = append(m.SetMaxTokensCalls, tokens)
}

// SetFrequencyPenalty implementation
func (m *MockLLMClientExtended) SetFrequencyPenalty(penalty float32) {
	m.SetFreqPenaltyCalls = append(m.SetFreqPenaltyCalls, penalty)
}

// SetPresencePenalty implementation
func (m *MockLLMClientExtended) SetPresencePenalty(penalty float32) {
	m.SetPresencePenaltyCalls = append(m.SetPresencePenaltyCalls, penalty)
}

// GenerateContent implementation
func (m *MockLLMClientExtended) GenerateContent(ctx context.Context, prompt string, params map[string]interface{}) (*llm.ProviderResult, error) {
	if params != nil {
		m.GenerateContentParams = append(m.GenerateContentParams, params)
	}
	if m.GenerateContentFunc != nil {
		return m.GenerateContentFunc(ctx, prompt, params)
	}
	return &llm.ProviderResult{Content: "Mock response"}, nil
}

// CountTokens implementation
func (m *MockLLMClientExtended) CountTokens(ctx context.Context, prompt string) (*llm.ProviderTokenCount, error) {
	if m.CountTokensFunc != nil {
		return m.CountTokensFunc(ctx, prompt)
	}
	return &llm.ProviderTokenCount{Total: int32(len(prompt) / 4)}, nil
}

// GetModelInfo implementation
func (m *MockLLMClientExtended) GetModelInfo(ctx context.Context) (*llm.ProviderModelInfo, error) {
	if m.GetModelInfoFunc != nil {
		return m.GetModelInfoFunc(ctx)
	}
	return &llm.ProviderModelInfo{
		Name:             "mock-model",
		InputTokenLimit:  8192,
		OutputTokenLimit: 2048,
	}, nil
}

// GetModelName implementation
func (m *MockLLMClientExtended) GetModelName() string {
	if m.GetModelNameFunc != nil {
		return m.GetModelNameFunc()
	}
	return "mock-model"
}

// Close implementation
func (m *MockLLMClientExtended) Close() error {
	if m.CloseFunc != nil {
		return m.CloseFunc()
	}
	return nil
}

// TestParameterPassingWithExtendedMock uses a more tracking-focused mock to verify parameter passing
func TestParameterPassingWithExtendedMock(t *testing.T) {
	t.Run("Parameters set via SetParameters are passed to GenerateContent", func(t *testing.T) {
		mockClient := &MockLLMClientExtended{
			GenerateContentFunc: func(ctx context.Context, prompt string, params map[string]interface{}) (*llm.ProviderResult, error) {
				return &llm.ProviderResult{Content: "mock response"}, nil
			},
		}
		adapter := NewOpenAIClientAdapter(mockClient)

		// Set parameters via adapter
		adapter.SetParameters(map[string]interface{}{
			"temperature": 0.7,
			"top_p":       0.8,
			"max_tokens":  100,
		})

		// Call GenerateContent - parameters should be applied
		result, err := adapter.GenerateContent(context.Background(), "test prompt", nil)

		// Verify
		assert.NoError(t, err)
		assert.NotNil(t, result)
		assert.Equal(t, "mock response", result.Content)

		// Check that parameters were set
		assert.Len(t, mockClient.SetTemperatureCalls, 1)
		assert.Equal(t, float32(0.7), mockClient.SetTemperatureCalls[0])

		assert.Len(t, mockClient.SetTopPCalls, 1)
		assert.Equal(t, float32(0.8), mockClient.SetTopPCalls[0])

		assert.Len(t, mockClient.SetMaxTokensCalls, 1)
		assert.Equal(t, int32(100), mockClient.SetMaxTokensCalls[0])
	})

	t.Run("Parameters passed to GenerateContent override adapter parameters", func(t *testing.T) {
		mockClient := &MockLLMClientExtended{
			GenerateContentFunc: func(ctx context.Context, prompt string, params map[string]interface{}) (*llm.ProviderResult, error) {
				return &llm.ProviderResult{Content: "mock response with overrides"}, nil
			},
		}
		adapter := NewOpenAIClientAdapter(mockClient)

		// Set adapter parameters first
		adapter.SetParameters(map[string]interface{}{
			"temperature": 0.7,
			"top_p":       0.8,
			"max_tokens":  100,
		})

		// Call GenerateContent with some parameters that override adapter parameters
		requestParams := map[string]interface{}{
			"temperature": 0.5,
			"max_tokens":  200,
		}
		result, err := adapter.GenerateContent(context.Background(), "test prompt", requestParams)

		// Verify
		assert.NoError(t, err)
		assert.NotNil(t, result)
		assert.Equal(t, "mock response with overrides", result.Content)

		// Check that parameters were set correctly (the overridden ones)
		assert.Len(t, mockClient.SetTemperatureCalls, 1)
		assert.Equal(t, float32(0.5), mockClient.SetTemperatureCalls[0])

		// top_p should not be set since it was not in the override params
		assert.Len(t, mockClient.SetTopPCalls, 0)

		assert.Len(t, mockClient.SetMaxTokensCalls, 1)
		assert.Equal(t, int32(200), mockClient.SetMaxTokensCalls[0])

		// Verify the params were passed to GenerateContent
		assert.Len(t, mockClient.GenerateContentParams, 1)
		assert.Equal(t, requestParams, mockClient.GenerateContentParams[0])
	})
}

// TestParamPersistenceAcrossCalls verifies that parameters persist across multiple calls
func TestParamPersistenceAcrossCalls(t *testing.T) {
	mockClient := &MockLLMClientWithParamChecking{}
	adapter := NewOpenAIClientAdapter(mockClient)

	// Set parameters
	initialParams := map[string]interface{}{
		"temperature": 0.7,
		"top_p":       0.8,
		"max_tokens":  100,
	}
	adapter.SetParameters(initialParams)

	// First call should use the parameters
	_, err := adapter.GenerateContent(context.Background(), "prompt 1", nil)
	assert.NoError(t, err)
	assert.True(t, mockClient.TemperatureSet)
	assert.Equal(t, float32(0.7), mockClient.TemperatureValue)
	assert.True(t, mockClient.TopPSet)
	assert.Equal(t, float32(0.8), mockClient.TopPValue)
	assert.True(t, mockClient.MaxTokensSet)
	assert.Equal(t, int32(100), mockClient.MaxTokensValue)

	// Reset mock tracking for the next test
	mockClient.TemperatureSet = false
	mockClient.TopPSet = false
	mockClient.MaxTokensSet = false

	// Second call without setting parameters again should reuse the same parameters
	_, err = adapter.GenerateContent(context.Background(), "prompt 2", nil)
	assert.NoError(t, err)
	assert.True(t, mockClient.TemperatureSet)
	assert.Equal(t, float32(0.7), mockClient.TemperatureValue)
	assert.True(t, mockClient.TopPSet)
	assert.Equal(t, float32(0.8), mockClient.TopPValue)
	assert.True(t, mockClient.MaxTokensSet)
	assert.Equal(t, int32(100), mockClient.MaxTokensValue)

	// Reset mock tracking for the next test
	mockClient.TemperatureSet = false
	mockClient.TopPSet = false
	mockClient.MaxTokensSet = false

	// Third call with explicit parameters should override adapter parameters for this call
	callParams := map[string]interface{}{
		"temperature": 0.5,
		"max_tokens":  200,
	}
	_, err = adapter.GenerateContent(context.Background(), "prompt 3", callParams)
	assert.NoError(t, err)
	assert.True(t, mockClient.TemperatureSet)
	assert.Equal(t, float32(0.5), mockClient.TemperatureValue) // overridden
	assert.False(t, mockClient.TopPSet)                        // Not set in this call
	assert.True(t, mockClient.MaxTokensSet)
	assert.Equal(t, int32(200), mockClient.MaxTokensValue) // overridden

	// Reset mock tracking for the next test
	mockClient.TemperatureSet = false
	mockClient.TopPSet = false
	mockClient.MaxTokensSet = false

	// Fourth call after the override should use the new parameters
	// because adapter.params was replaced with callParams
	_, err = adapter.GenerateContent(context.Background(), "prompt 4", nil)
	assert.NoError(t, err)
	assert.True(t, mockClient.TemperatureSet)
	assert.Equal(t, float32(0.5), mockClient.TemperatureValue) // From the override
	assert.False(t, mockClient.TopPSet)                        // Not in the new params
	assert.True(t, mockClient.MaxTokensSet)
	assert.Equal(t, int32(200), mockClient.MaxTokensValue) // From the override

	// Reset params and tracking for the final test
	adapter.SetParameters(nil)
	mockClient.TemperatureSet = false
	mockClient.TopPSet = false
	mockClient.MaxTokensSet = false

	// Fifth call with no parameters should not set any parameters
	_, err = adapter.GenerateContent(context.Background(), "prompt 5", nil)
	assert.NoError(t, err)
	assert.False(t, mockClient.TemperatureSet) // No params to set
	assert.False(t, mockClient.TopPSet)        // No params to set
	assert.False(t, mockClient.MaxTokensSet)   // No params to set
}

// TestParameterTypeConversions verifies that different parameter types are correctly converted
func TestParameterTypeConversions(t *testing.T) {
	testCases := []struct {
		name        string
		paramName   string
		paramValue  interface{}
		paramType   string // "float" or "int"
		expectedSet bool
		expected    interface{} // expected value after conversion
	}{
		{"Float as float64", "temperature", float64(0.75), "float", true, float32(0.75)},
		{"Float as float32", "temperature", float32(0.8), "float", true, float32(0.8)},
		{"Float as int", "temperature", 1, "float", true, float32(1.0)},
		{"Float as int32", "temperature", int32(1), "float", true, float32(1.0)},
		{"Float as int64", "temperature", int64(1), "float", true, float32(1.0)},
		{"Int as int", "max_tokens", 100, "int", true, int32(100)},
		{"Int as int32", "max_tokens", int32(200), "int", true, int32(200)},
		{"Int as int64", "max_tokens", int64(300), "int", true, int32(300)},
		{"Int as float64", "max_tokens", float64(400.5), "int", true, int32(400)},
		{"Int as float32", "max_tokens", float32(500.5), "int", true, int32(500)},
		{"Invalid type string", "temperature", "invalid", "float", false, nil},
		{"Invalid type bool", "max_tokens", true, "int", false, nil},
		{"Invalid type nil", "max_tokens", nil, "int", false, nil},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			mockClient := &MockLLMClientWithParamChecking{}
			adapter := NewOpenAIClientAdapter(mockClient)

			// Set up the parameter
			adapter.SetParameters(map[string]interface{}{
				tc.paramName: tc.paramValue,
			})

			// Call GenerateContent to trigger parameter processing
			_, err := adapter.GenerateContent(context.Background(), "test prompt", nil)
			assert.NoError(t, err)

			// Check if parameter was correctly set
			switch tc.paramType {
			case "float":
				switch tc.paramName {
				case "temperature":
					assert.Equal(t, tc.expectedSet, mockClient.TemperatureSet)
					if tc.expectedSet {
						assert.Equal(t, tc.expected, mockClient.TemperatureValue)
					}
				case "top_p":
					assert.Equal(t, tc.expectedSet, mockClient.TopPSet)
					if tc.expectedSet {
						assert.Equal(t, tc.expected, mockClient.TopPValue)
					}
				case "frequency_penalty":
					assert.Equal(t, tc.expectedSet, mockClient.FreqPenaltySet)
					if tc.expectedSet {
						assert.Equal(t, tc.expected, mockClient.FreqPenaltyValue)
					}
				case "presence_penalty":
					assert.Equal(t, tc.expectedSet, mockClient.PresencePenaltySet)
					if tc.expectedSet {
						assert.Equal(t, tc.expected, mockClient.PresencePenaltyValue)
					}
				}
			case "int":
				assert.Equal(t, tc.expectedSet, mockClient.MaxTokensSet)
				if tc.expectedSet {
					assert.Equal(t, tc.expected, mockClient.MaxTokensValue)
				}
			}
		})
	}
}

// TestParameterPassingThroughAdapter verifies that parameters are passed correctly through the adapter
func TestParameterPassingThroughAdapter(t *testing.T) {
	t.Run("Parameters are passed through to the underlying client", func(t *testing.T) {
		// Create a mock client that tracks parameter passes
		mockClient := &MockLLMClientWithParamChecking{
			GenerateContentFunc: func(ctx context.Context, prompt string, params map[string]interface{}) (*llm.ProviderResult, error) {
				return &llm.ProviderResult{Content: "Success"}, nil
			},
		}

		// Create the adapter
		adapter := NewOpenAIClientAdapter(mockClient)

		// Set adapter parameters
		adapterParams := map[string]interface{}{
			"temperature":       0.7,
			"top_p":             0.8,
			"max_tokens":        100,
			"frequency_penalty": 0.2,
			"presence_penalty":  0.3,
			"custom_param":      "value", // This should pass through unchanged
		}
		adapter.SetParameters(adapterParams)

		// Call GenerateContent
		_, err := adapter.GenerateContent(context.Background(), "test prompt", nil)
		assert.NoError(t, err)

		// Verify typed parameters were applied
		assert.True(t, mockClient.TemperatureSet)
		assert.Equal(t, float32(0.7), mockClient.TemperatureValue)
		assert.True(t, mockClient.TopPSet)
		assert.Equal(t, float32(0.8), mockClient.TopPValue)
		assert.True(t, mockClient.MaxTokensSet)
		assert.Equal(t, int32(100), mockClient.MaxTokensValue)
		assert.True(t, mockClient.FreqPenaltySet)
		assert.Equal(t, float32(0.2), mockClient.FreqPenaltyValue)
		assert.True(t, mockClient.PresencePenaltySet)
		assert.Equal(t, float32(0.3), mockClient.PresencePenaltyValue)

		// Verify original parameter map was passed to GenerateContent
		assert.Equal(t, adapterParams, mockClient.GenerateParams)
		assert.Equal(t, "value", mockClient.GenerateParams["custom_param"])
	})

	t.Run("Alternative parameter names (Gemini compatibility)", func(t *testing.T) {
		// Create a mock client
		mockClient := &MockLLMClientWithParamChecking{
			GenerateContentFunc: func(ctx context.Context, prompt string, params map[string]interface{}) (*llm.ProviderResult, error) {
				return &llm.ProviderResult{Content: "Success"}, nil
			},
		}

		// Create the adapter
		adapter := NewOpenAIClientAdapter(mockClient)

		// Set parameters using Gemini-style parameter names
		adapter.SetParameters(map[string]interface{}{
			"max_output_tokens": 200, // This should be recognized as max_tokens
		})

		// Call GenerateContent
		_, err := adapter.GenerateContent(context.Background(), "test prompt", nil)
		assert.NoError(t, err)

		// Verify the parameter was recognized and converted
		assert.True(t, mockClient.MaxTokensSet)
		assert.Equal(t, int32(200), mockClient.MaxTokensValue)
	})
}
