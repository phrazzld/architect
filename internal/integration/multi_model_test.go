// internal/integration/multi_model_test.go
package integration

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/phrazzld/architect/internal/architect"
	"github.com/phrazzld/architect/internal/config"
	"github.com/phrazzld/architect/internal/gemini"
	"github.com/phrazzld/architect/internal/llm"
	"github.com/phrazzld/architect/internal/logutil"
	"github.com/phrazzld/architect/internal/registry"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// Use the shared ContextKey and ModelNameKey from test_utils.go

// mockModelTrackingAPIService extends mockIntAPIService to track model names
type mockModelTrackingAPIService struct {
	logger        logutil.LoggerInterface
	mockClient    gemini.Client
	mockLLMClient llm.LLMClient
}

// InitLLMClient returns a mock LLM client and stores model name in context
func (s *mockModelTrackingAPIService) InitLLMClient(ctx context.Context, apiKey, modelName, apiEndpoint string) (llm.LLMClient, error) {
	// Create a new context with the model name
	ctx = context.WithValue(ctx, ModelNameKey, modelName)

	// If a specific mock LLM client was provided, use it
	if s.mockLLMClient != nil {
		// We need to wrap it in a model-aware client to carry the context
		return &modelAwareLLMClient{
			delegateClient: s.mockLLMClient,
			ctx:            ctx,
		}, nil
	}

	// Create adapter that wraps the mock gemini client to implement llm.LLMClient
	llmAdapter := NewLLMClientAdapter(s.mockClient, modelName)

	// Then wrap it in a model-aware client to carry the context
	return &modelAwareLLMClient{
		delegateClient: llmAdapter,
		ctx:            ctx,
	}, nil
}

func (s *mockModelTrackingAPIService) IsEmptyResponseError(err error) bool {
	if err == nil {
		return false
	}
	return strings.Contains(err.Error(), "empty")
}

func (s *mockModelTrackingAPIService) IsSafetyBlockedError(err error) bool {
	if err == nil {
		return false
	}
	return strings.Contains(err.Error(), "safety")
}

func (s *mockModelTrackingAPIService) GetErrorDetails(err error) string {
	if err == nil {
		return ""
	}
	return err.Error()
}

// ProcessLLMResponse processes provider-agnostic responses
func (s *mockModelTrackingAPIService) ProcessLLMResponse(result *llm.ProviderResult) (string, error) {
	if result == nil {
		return "", fmt.Errorf("empty response from API")
	}
	if result.Content == "" {
		return "", fmt.Errorf("empty content from API")
	}
	return result.Content, nil
}

// GetModelDefinition retrieves the full model definition from the registry
func (s *mockModelTrackingAPIService) GetModelDefinition(modelName string) (*registry.ModelDefinition, error) {
	// Return a basic model definition for testing
	return &registry.ModelDefinition{
		Name:            modelName,
		Provider:        "gemini",
		APIModelID:      modelName,
		ContextWindow:   8192,
		MaxOutputTokens: 8192,
		Parameters: map[string]registry.ParameterDefinition{
			"temperature": {
				Type:    "float",
				Default: 0.7,
				Min:     0.0,
				Max:     1.0,
			},
			"max_tokens": {
				Type:    "integer",
				Default: 8192,
				Min:     1,
				Max:     8192,
			},
		},
	}, nil
}

// GetModelParameters retrieves parameter values from the registry for a given model
func (s *mockModelTrackingAPIService) GetModelParameters(modelName string) (map[string]interface{}, error) {
	// Return default parameters for testing
	return map[string]interface{}{
		"temperature": 0.7,
		"max_tokens":  8192,
	}, nil
}

// ValidateModelParameter validates a parameter value against its constraints
func (s *mockModelTrackingAPIService) ValidateModelParameter(modelName, paramName string, value interface{}) (bool, error) {
	// Basic validation for common parameters
	switch paramName {
	case "temperature":
		if temp, ok := value.(float64); ok {
			return temp >= 0.0 && temp <= 1.0, nil
		}
		return false, fmt.Errorf("temperature must be a float between 0 and 1")
	case "max_tokens":
		if tokens, ok := value.(int); ok {
			return tokens >= 1 && tokens <= 8192, nil
		}
		return false, fmt.Errorf("max_tokens must be an integer between 1 and 8192")
	default:
		// By default, accept unknown parameters
		return true, nil
	}
}

// GetModelTokenLimits retrieves token limits from the registry for a given model
func (s *mockModelTrackingAPIService) GetModelTokenLimits(modelName string) (contextWindow, maxOutputTokens int32, err error) {
	// Return standard limits for testing
	return 8192, 8192, nil
}

// modelAwareLLMClient wraps an LLMClient to carry context with model information
type modelAwareLLMClient struct {
	delegateClient llm.LLMClient
	ctx            context.Context
}

// GenerateContent passes the model-aware context to the delegate
func (m *modelAwareLLMClient) GenerateContent(ctx context.Context, prompt string, params map[string]interface{}) (*llm.ProviderResult, error) {
	// Use the context with model information instead of the one provided
	return m.delegateClient.GenerateContent(m.ctx, prompt, params)
}

// CountTokens implements the LLMClient interface
func (m *modelAwareLLMClient) CountTokens(ctx context.Context, prompt string) (*llm.ProviderTokenCount, error) {
	return m.delegateClient.CountTokens(m.ctx, prompt)
}

// GetModelInfo implements the LLMClient interface
func (m *modelAwareLLMClient) GetModelInfo(ctx context.Context) (*llm.ProviderModelInfo, error) {
	return m.delegateClient.GetModelInfo(m.ctx)
}

// GetModelName implements the LLMClient interface
func (m *modelAwareLLMClient) GetModelName() string {
	return m.delegateClient.GetModelName()
}

// Close implements the LLMClient interface
func (m *modelAwareLLMClient) Close() error {
	return m.delegateClient.Close()
}

// Note: The modelAwareClient has been removed as it's no longer needed.
// All tests now use the provider-agnostic modelAwareLLMClient instead.

// multiModelTestCase defines a table-driven test case for multi-model testing
type multiModelTestCase struct {
	name                string
	instructionsContent string
	modelNames          []string
	configureMock       func(t *testing.T, env *TestEnv, modelProcessData *modelProcessingData)
	expectedError       bool
	verifyResults       func(t *testing.T, env *TestEnv, err error, modelProcessData *modelProcessingData)
	concurrencyTest     bool
	disableRateLimit    bool
}

// modelProcessingData holds test data for tracking model processing
type modelProcessingData struct {
	sync.Mutex
	started    map[string]bool
	failed     map[string]bool
	intervals  []TimeInterval
	completed  map[string]bool
	waitGroups struct {
		started   *sync.WaitGroup
		barrier   *sync.WaitGroup
		completed *sync.WaitGroup
	}
}

// newModelProcessingData creates initialized tracking data for tests
func newModelProcessingData(modelCount int, trackIntervals bool, setupWaitGroups bool) *modelProcessingData {
	data := &modelProcessingData{
		started:   make(map[string]bool),
		failed:    make(map[string]bool),
		completed: make(map[string]bool),
	}

	if trackIntervals {
		data.intervals = make([]TimeInterval, 0, modelCount)
	}

	if setupWaitGroups {
		data.waitGroups.started = &sync.WaitGroup{}
		data.waitGroups.barrier = &sync.WaitGroup{}
		data.waitGroups.completed = &sync.WaitGroup{}

		data.waitGroups.started.Add(modelCount)
		data.waitGroups.barrier.Add(1)
		data.waitGroups.completed.Add(modelCount)
	}

	return data
}

// TestMultiModelFeatures tests various multi-model features using a table-driven approach
func TestMultiModelFeatures(t *testing.T) {
	// Skip in short mode to reduce CI time
	if testing.Short() {
		t.Skip("Skipping test in short mode")
	}

	testCases := []multiModelTestCase{
		{
			name:                "BasicExecution",
			instructionsContent: "Test multi-model generation",
			modelNames:          []string{"model1", "model2", "model3"},
			configureMock: func(t *testing.T, env *TestEnv, modelData *modelProcessingData) {
				env.MockClient.GenerateContentFunc = func(ctx context.Context, prompt string, params map[string]interface{}) (*gemini.GenerationResult, error) {
					// Extract the model name from the context
					modelName := ctx.Value(ModelNameKey).(string)

					// Use mutex to protect concurrent map access
					modelData.Lock()
					modelData.started[modelName] = true
					modelData.Unlock()

					return &gemini.GenerationResult{
						Content:      "# Plan Generated by " + modelName + "\n\nThis is a test plan generated for the model.",
						TokenCount:   1000,
						FinishReason: "STOP",
					}, nil
				}
			},
			verifyResults: func(t *testing.T, env *TestEnv, err error, modelData *modelProcessingData) {
				// Verify no errors
				require.NoError(t, err)

				outputDir := filepath.Join(env.TestDir, "output")
				// Check output files for all models
				for _, modelName := range []string{"model1", "model2", "model3"} {
					outputFile := filepath.Join(outputDir, modelName+".md")

					// Check that the file exists
					if _, err := os.Stat(outputFile); os.IsNotExist(err) {
						t.Errorf("Output file for model %s was not created at %s", modelName, outputFile)
						continue
					}

					// Read and verify the content
					content, err := os.ReadFile(outputFile)
					if err != nil {
						t.Errorf("Failed to read output file for model %s: %v", modelName, err)
						continue
					}

					if !strings.Contains(string(content), "Plan Generated by "+modelName) {
						t.Errorf("Output file for model %s does not contain model-specific content", modelName)
					}
				}

				// Verify all models were processed
				modelData.Lock()
				defer modelData.Unlock()
				for _, modelName := range []string{"model1", "model2", "model3"} {
					if !modelData.started[modelName] {
						t.Errorf("Model %s was not processed", modelName)
					}
				}
			},
		},
		{
			name:                "ModelFailureHandling",
			instructionsContent: "Test multi-model generation with error handling",
			modelNames:          []string{"model1", "model2", "model3"},
			configureMock: func(t *testing.T, env *TestEnv, modelData *modelProcessingData) {
				env.MockClient.GenerateContentFunc = func(ctx context.Context, prompt string, params map[string]interface{}) (*gemini.GenerationResult, error) {
					// Extract the model name
					modelName := ctx.Value(ModelNameKey).(string)

					// Protect concurrent map access
					modelData.Lock()
					modelData.started[modelName] = true
					modelData.Unlock()

					// Make model2 fail
					if modelName == "model2" {
						modelData.Lock()
						modelData.failed[modelName] = true
						modelData.Unlock()
						return nil, fmt.Errorf("simulated error for model %s", modelName)
					}

					return &gemini.GenerationResult{
						Content:      "# Plan Generated by " + modelName + "\n\nThis is a test plan generated for the model.",
						TokenCount:   1000,
						FinishReason: "STOP",
					}, nil
				}
			},
			expectedError: true,
			verifyResults: func(t *testing.T, env *TestEnv, err error, modelData *modelProcessingData) {
				// Should have an error because model2 failed
				require.Error(t, err)

				// Error should mention model2
				assert.Contains(t, err.Error(), "model2", "Error message should mention the failed model")

				outputDir := filepath.Join(env.TestDir, "output")

				// Check output files for successful models
				for _, modelName := range []string{"model1", "model3"} {
					outputFile := filepath.Join(outputDir, modelName+".md")

					// Check that the file exists
					if _, err := os.Stat(outputFile); os.IsNotExist(err) {
						t.Errorf("Output file for successful model %s was not created at %s", modelName, outputFile)
						continue
					}

					content, err := os.ReadFile(outputFile)
					if err != nil {
						t.Errorf("Failed to read output file for model %s: %v", modelName, err)
						continue
					}

					if !strings.Contains(string(content), "Plan Generated by "+modelName) {
						t.Errorf("Output file for model %s does not contain model-specific content", modelName)
					}
				}

				// Check that the failed model doesn't have an output file
				failedOutputFile := filepath.Join(outputDir, "model2.md")
				if _, err := os.Stat(failedOutputFile); !os.IsNotExist(err) {
					t.Errorf("Output file for failed model was created at %s (it shouldn't exist)", failedOutputFile)
				}

				// Verify that all models were attempted
				modelData.Lock()
				defer modelData.Unlock()

				for _, modelName := range []string{"model1", "model2", "model3"} {
					if !modelData.started[modelName] {
						t.Errorf("Model %s was not attempted", modelName)
					}
				}

				// Verify that only model2 failed
				if !modelData.failed["model2"] {
					t.Errorf("Expected model2 to fail, but it didn't")
				}

				failedModelCount := len(modelData.failed)
				if failedModelCount != 1 {
					t.Errorf("Expected exactly one model to fail, but got %d failed models", failedModelCount)
				}
			},
		},
		{
			name:                "ConcurrentModelProcessing",
			instructionsContent: "Test concurrent model processing",
			modelNames:          []string{"model1", "model2", "model3"},
			configureMock: func(t *testing.T, env *TestEnv, modelData *modelProcessingData) {
				env.MockClient.GenerateContentFunc = func(ctx context.Context, prompt string, params map[string]interface{}) (*gemini.GenerationResult, error) {
					// Extract the model name from the context
					modelName := ctx.Value(ModelNameKey).(string)

					// Record that this model started processing
					modelData.Lock()
					modelData.started[modelName] = true
					modelData.Unlock()

					// Signal that processing has started for this model
					modelData.waitGroups.started.Done()

					// Wait for all models to reach this point (ensures concurrent processing)
					modelData.waitGroups.barrier.Wait()

					// Simulate work with a much smaller sleep (reduced for test optimization)
					time.Sleep(5 * time.Millisecond)

					// Record completion
					modelData.Lock()
					modelData.completed[modelName] = true
					modelData.Unlock()

					// Signal completion
					modelData.waitGroups.completed.Done()

					// Return a result
					return &gemini.GenerationResult{
						Content:      "# Plan Generated by " + modelName + "\n\nThis is a test plan.",
						TokenCount:   1000,
						FinishReason: "STOP",
					}, nil
				}
			},
			concurrencyTest: true,
			verifyResults: func(t *testing.T, env *TestEnv, err error, modelData *modelProcessingData) {
				// Verify execution succeeded
				require.NoError(t, err)

				// Wait for all models to complete
				modelData.waitGroups.completed.Wait()

				// Verify all models were started and completed
				modelData.Lock()
				defer modelData.Unlock()

				// Check started models
				startedCount := len(modelData.started)
				completedCount := len(modelData.completed)
				modelCount := 3

				if startedCount != modelCount {
					t.Errorf("Expected %d models to start processing, but got %d", modelCount, startedCount)
				}

				if completedCount != modelCount {
					t.Errorf("Expected %d models to complete processing, but got %d", modelCount, completedCount)
				}

				// Verify that output files were created for all models
				outputDir := filepath.Join(env.TestDir, "output")
				for _, modelName := range []string{"model1", "model2", "model3"} {
					outputFile := filepath.Join(outputDir, modelName+".md")
					if _, err := os.Stat(outputFile); os.IsNotExist(err) {
						t.Errorf("Output file for model %s was not created at %s", modelName, outputFile)
					}
				}
			},
		},
		{
			name:                "ConcurrentModelFailureHandling",
			instructionsContent: "Test concurrent model failure handling",
			modelNames:          []string{"model1", "model2", "model3"},
			configureMock: func(t *testing.T, env *TestEnv, modelData *modelProcessingData) {
				env.MockClient.GenerateContentFunc = func(ctx context.Context, prompt string, params map[string]interface{}) (*gemini.GenerationResult, error) {
					// Extract the model name from context
					modelName := ctx.Value(ModelNameKey).(string)

					// Record that this model started processing
					modelData.Lock()
					modelData.started[modelName] = true
					modelData.Unlock()

					// Signal that processing has started for this model
					modelData.waitGroups.started.Done()

					// Wait for all models to reach this point (ensures concurrent processing)
					modelData.waitGroups.barrier.Wait()

					// Simulate work with a much smaller sleep (reduced for test optimization)
					time.Sleep(5 * time.Millisecond)

					// Make models 1 and 3 fail with different errors
					switch modelName {
					case "model1":
						modelData.Lock()
						modelData.failed[modelName] = true
						modelData.Unlock()

						return nil, fmt.Errorf("simulated error 1 for model %s", modelName)
					case "model3":
						modelData.Lock()
						modelData.failed[modelName] = true
						modelData.Unlock()

						return nil, fmt.Errorf("simulated error 3 for model %s", modelName)
					}

					// Return a successful result for model2
					return &gemini.GenerationResult{
						Content:      "# Plan Generated by " + modelName + "\n\nThis is a test plan.",
						TokenCount:   1000,
						FinishReason: "STOP",
					}, nil
				}
			},
			concurrencyTest: true,
			expectedError:   true,
			verifyResults: func(t *testing.T, env *TestEnv, err error, modelData *modelProcessingData) {
				// Should have errors from model1 and model3
				require.Error(t, err)

				// Verify that the error contains information about both failed models
				errorMsg := err.Error()
				assert.Contains(t, errorMsg, "model1", "Error message should mention model1")
				assert.Contains(t, errorMsg, "model3", "Error message should mention model3")

				outputDir := filepath.Join(env.TestDir, "output")

				// Verify all models were started
				for _, modelName := range []string{"model1", "model2", "model3"} {
					assert.True(t, modelData.started[modelName], "Model %s should have been started", modelName)
				}

				// Verify that models 1 and 3 failed
				assert.True(t, modelData.failed["model1"], "model1 should have failed")
				assert.True(t, modelData.failed["model3"], "model3 should have failed")
				assert.False(t, modelData.failed["model2"], "model2 should have succeeded")

				// Verify that output files were created only for successful models
				// Model2 should have an output file
				model2OutputFile := filepath.Join(outputDir, "model2.md")
				if _, err := os.Stat(model2OutputFile); os.IsNotExist(err) {
					t.Errorf("Output file for successful model2 was not created at %s", model2OutputFile)
				}

				// Models 1 and 3 should not have output files since they failed
				model1OutputFile := filepath.Join(outputDir, "model1.md")
				if _, err := os.Stat(model1OutputFile); !os.IsNotExist(err) {
					t.Errorf("Output file for failed model1 was created at %s (it shouldn't exist)", model1OutputFile)
				}

				model3OutputFile := filepath.Join(outputDir, "model3.md")
				if _, err := os.Stat(model3OutputFile); !os.IsNotExist(err) {
					t.Errorf("Output file for failed model3 was created at %s (it shouldn't exist)", model3OutputFile)
				}
			},
		},
		{
			name:                "EnhancedConcurrentModelProcessing",
			instructionsContent: "Test enhanced concurrent model processing",
			modelNames:          []string{"model1", "model2", "model3", "model4", "model5"},
			configureMock: func(t *testing.T, env *TestEnv, modelData *modelProcessingData) {
				env.MockClient.GenerateContentFunc = func(ctx context.Context, prompt string, params map[string]interface{}) (*gemini.GenerationResult, error) {
					// Extract the model name from the context
					modelName := ctx.Value(ModelNameKey).(string)

					// Record start time
					startTime := time.Now()

					// Signal that processing has started for this model
					modelData.waitGroups.started.Done()

					// Wait for all models to reach this point (ensures concurrent processing)
					modelData.waitGroups.barrier.Wait()

					// Simulate work with a variable sleep time (50-150ms) to create realistic variation
					sleepTime := 5 + (time.Duration(len(modelName)%3) * 5)
					time.Sleep(sleepTime * time.Millisecond)

					// Record end time
					endTime := time.Now()

					// Record the interval
					modelData.Lock()
					modelData.intervals = append(modelData.intervals, TimeInterval{Start: startTime, End: endTime})
					modelData.Unlock()

					// Signal completion
					modelData.waitGroups.completed.Done()

					// Return a result
					return &gemini.GenerationResult{
						Content:      "# Plan Generated by " + modelName + "\n\nThis is a test plan.",
						TokenCount:   1000,
						FinishReason: "STOP",
					}, nil
				}
			},
			concurrencyTest:  true,
			disableRateLimit: true,
			verifyResults: func(t *testing.T, env *TestEnv, err error, modelData *modelProcessingData) {
				// Verify execution succeeded
				require.NoError(t, err)

				// Wait for all models to complete
				modelData.waitGroups.completed.Wait()

				outputDir := filepath.Join(env.TestDir, "output")

				// Verify that output files were created for all models
				for _, modelName := range []string{"model1", "model2", "model3", "model4", "model5"} {
					outputFile := filepath.Join(outputDir, modelName+".md")
					if _, err := os.Stat(outputFile); os.IsNotExist(err) {
						t.Errorf("Output file for model %s was not created at %s", modelName, outputFile)
					} else {
						// Check file content contains expected model name
						content, err := os.ReadFile(outputFile)
						if err != nil {
							t.Errorf("Failed to read output file for model %s: %v", modelName, err)
						} else {
							if !strings.Contains(string(content), "Plan Generated by "+modelName) {
								t.Errorf("Output file for model %s does not contain expected content", modelName)
							}
						}
					}
				}

				// Verify intervals are concurrent
				modelData.Lock()
				intervals := modelData.intervals
				modelData.Unlock()

				// Ensure we got intervals for all models
				modelCount := 5
				require.Equal(t, modelCount, len(intervals), "Should have captured time intervals for all models")

				// Check if intervals show concurrent execution
				assert.True(t, areIntervalsConcurrent(intervals),
					"Model processing intervals should overlap, indicating concurrent execution")
			},
		},
		{
			name:                "ModelSpecificOutput",
			instructionsContent: "Test model-specific output",
			modelNames:          []string{"gemini-pro"},
			configureMock: func(t *testing.T, env *TestEnv, modelData *modelProcessingData) {
				env.SetupMockGeminiClient()
			},
			verifyResults: func(t *testing.T, env *TestEnv, err error, modelData *modelProcessingData) {
				require.NoError(t, err)

				outputDir := filepath.Join(env.TestDir, "output")

				// Check that the output directory exists
				if _, err := os.Stat(outputDir); os.IsNotExist(err) {
					t.Errorf("Output directory was not created at %s", outputDir)
				}

				// Check that the model-specific file exists
				modelOutputFile := filepath.Join(outputDir, "gemini-pro.md")
				if _, err := os.Stat(modelOutputFile); os.IsNotExist(err) {
					t.Errorf("Model-specific output file was not created at %s", modelOutputFile)
				}

				// Verify that the legacy output.md file does NOT exist
				legacyOutputFile := filepath.Join(outputDir, "output.md")
				if _, err := os.Stat(legacyOutputFile); !os.IsNotExist(err) {
					t.Errorf("Legacy output.md file still exists at %s but it should not", legacyOutputFile)
				}
			},
		},
	}

	for _, tc := range testCases {
		tc := tc // Capture range variable
		t.Run(tc.name, func(t *testing.T) {
			// Set up test environment
			env := NewTestEnv(t)
			defer env.Cleanup()

			// Create tracking data for model processing
			modelData := newModelProcessingData(len(tc.modelNames), tc.concurrencyTest, tc.concurrencyTest)

			// Configure mock client
			if tc.configureMock != nil {
				tc.configureMock(t, env, modelData)
			}

			// Create test files
			env.CreateTestFile(t, "src/main.go", `package main

func main() {}`)

			// Create instructions file
			instructionsFile := env.CreateTestFile(t, "instructions.md", tc.instructionsContent)

			// Set up the output directory
			outputDir := filepath.Join(env.TestDir, "output")

			// Create test configuration
			testConfig := &config.CliConfig{
				InstructionsFile: instructionsFile,
				OutputDir:        outputDir,
				ModelNames:       tc.modelNames,
				APIKey:           "test-api-key",
				Paths:            []string{env.TestDir + "/src"},
				LogLevel:         logutil.InfoLevel,
			}

			// Disable rate limiting for concurrency tests if needed
			if tc.disableRateLimit {
				testConfig.MaxConcurrentRequests = 0
				testConfig.RateLimitRequestsPerMinute = 0
			}

			// Create API service
			mockAPIService := &mockModelTrackingAPIService{
				logger:     env.Logger,
				mockClient: env.MockClient,
			}

			// Set up concurrency testing if needed
			if tc.concurrencyTest && modelData.waitGroups.barrier != nil {
				// Release the barrier after all models have started processing
				go func() {
					// Wait for all models to start
					modelData.waitGroups.started.Wait()
					// Release the barrier to let all continue
					modelData.waitGroups.barrier.Done()
				}()
			}

			// Run the application
			ctx := context.Background()
			err := architect.Execute(
				ctx,
				testConfig,
				env.Logger,
				env.AuditLogger,
				mockAPIService,
			)

			// Verify results based on test case
			if tc.verifyResults != nil {
				tc.verifyResults(t, env, err, modelData)
			} else if tc.expectedError {
				require.Error(t, err)
			} else {
				require.NoError(t, err)
			}
		})
	}
}
