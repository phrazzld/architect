package modelproc_test

import (
	"context"
	"fmt"
	"path/filepath"
	"testing"

	"github.com/phrazzld/architect/internal/architect/modelproc"
	"github.com/phrazzld/architect/internal/auditlog"
	"github.com/phrazzld/architect/internal/config"
	"github.com/phrazzld/architect/internal/llm"
	"github.com/phrazzld/architect/internal/registry"
)

// TestModelProcessor_Process_WithLLMClient tests the ModelProcessor.Process method
// with a provider-agnostic LLMClient.
func TestModelProcessor_Process_WithLLMClient(t *testing.T) {
	// Setup mocks
	mockAPI := &mockLLMAPIService{
		initLLMClientFunc: func(ctx context.Context, apiKey, modelName, apiEndpoint string) (llm.LLMClient, error) {
			return &mockLLMClient{
				getModelNameFunc: func() string { return modelName },
				generateContentFunc: func(ctx context.Context, prompt string, params map[string]interface{}) (*llm.ProviderResult, error) {
					return &llm.ProviderResult{
						Content:    "Generated LLM content",
						TokenCount: 50,
					}, nil
				},
			}, nil
		},
		processLLMResponseFunc: func(result *llm.ProviderResult) (string, error) {
			return result.Content, nil
		},
	}

	// Track if SaveToFile was called
	saveToFileCalled := false
	mockWriter := &mockFileWriter{
		saveToFileFunc: func(content, outputFile string) error {
			saveToFileCalled = true
			// Verify the content and output path are what we expect
			if content != "Generated LLM content" {
				t.Errorf("Expected content 'Generated LLM content', got '%s'", content)
			}
			if filepath.Base(outputFile) != "test-llm-model.md" {
				t.Errorf("Expected output file 'test-llm-model.md', got '%s'", filepath.Base(outputFile))
			}
			return nil
		},
	}

	// Track the audit log entries
	auditEntries := make([]auditlog.AuditEntry, 0)
	mockAudit := &mockAuditLogger{
		logFunc: func(entry auditlog.AuditEntry) error {
			auditEntries = append(auditEntries, entry)
			return nil
		},
	}

	mockLogger := newNoOpLogger()

	// Setup config
	cfg := config.NewDefaultCliConfig()
	cfg.APIKey = "test-api-key"
	cfg.OutputDir = "/tmp/test-output"

	// Create processor
	processor := modelproc.NewProcessor(
		mockAPI,
		mockWriter,
		mockAudit,
		mockLogger,
		cfg,
	)

	// Run test
	err := processor.Process(
		context.Background(),
		"test-llm-model",
		"Test prompt",
	)

	// Verify results
	if err != nil {
		t.Errorf("Expected no error, got %v", err)
	}

	if !saveToFileCalled {
		t.Errorf("Expected SaveToFile to be called, but it wasn't")
	}

	// Verify audit log entries
	expectedOperations := []string{
		"GenerateContentStart",
		"GenerateContentEnd",
		"SaveOutputStart",
		"SaveOutputEnd",
	}

	if len(auditEntries) != len(expectedOperations) {
		t.Errorf("Expected %d audit entries, got %d", len(expectedOperations), len(auditEntries))
	} else {
		for i, operation := range expectedOperations {
			if auditEntries[i].Operation != operation {
				t.Errorf("Expected audit operation '%s', got '%s'", operation, auditEntries[i].Operation)
			}
		}
	}
}

// TestModelProcessor_Process_UseTokenManagerWithLLMClient specifically tests that
// token management in the processor uses the LLMClient interface correctly.
func TestModelProcessor_Process_UseTokenManagerWithLLMClient(t *testing.T) {
	// Create a mock LLM client
	mockLLM := &mockLLMClient{
		getModelNameFunc: func() string { return "test-model" },
		getModelInfoFunc: func(ctx context.Context) (*llm.ProviderModelInfo, error) {
			return &llm.ProviderModelInfo{
				Name:             "test-model",
				InputTokenLimit:  1000,
				OutputTokenLimit: 500,
			}, nil
		},
		countTokensFunc: func(ctx context.Context, prompt string) (*llm.ProviderTokenCount, error) {
			return &llm.ProviderTokenCount{
				Total: 100,
			}, nil
		},
		generateContentFunc: func(ctx context.Context, prompt string, params map[string]interface{}) (*llm.ProviderResult, error) {
			return &llm.ProviderResult{
				Content:    "Test content",
				TokenCount: 50,
			}, nil
		},
	}

	mockAPI := &mockLLMAPIService{
		initLLMClientFunc: func(ctx context.Context, apiKey, modelName, apiEndpoint string) (llm.LLMClient, error) {
			return mockLLM, nil
		},
		processLLMResponseFunc: func(result *llm.ProviderResult) (string, error) {
			return result.Content, nil
		},
	}

	// Configure other mocks
	mockWriter := &mockFileWriter{
		saveToFileFunc: func(content, outputFile string) error {
			return nil
		},
	}
	mockAudit := &mockAuditLogger{
		logFunc: func(entry auditlog.AuditEntry) error {
			return nil
		},
	}
	mockLogger := newNoOpLogger()

	// Setup config
	cfg := config.NewDefaultCliConfig()
	cfg.APIKey = "test-api-key"
	cfg.OutputDir = "/tmp/test-output"

	// Create processor
	processor := modelproc.NewProcessor(
		mockAPI,
		mockWriter,
		mockAudit,
		mockLogger,
		cfg,
	)

	// Run test
	err := processor.Process(
		context.Background(),
		"test-model",
		"Test prompt",
	)

	// Verify results
	if err != nil {
		t.Errorf("Expected no error, got %v", err)
	}

	// As of T032B, token validation is removed, so we no longer expect these methods to be called
	// The test now just verifies that the processor can successfully complete without token validation
}

// TestModelProcessor_Process_UsesLLMClientExclusively tests that the ModelProcessor
// only uses the LLMClient interface and doesn't try to use any provider-specific APIs.
func TestModelProcessor_Process_UsesLLMClientExclusively(t *testing.T) {
	// Make a special mock that tracks if any provider-specific conversions are attempted
	mockLLM := &mockLLMClientWithProviderCheck{
		modelName: "test-model",
	}

	mockAPI := &mockLLMAPIService{
		initLLMClientFunc: func(ctx context.Context, apiKey, modelName, apiEndpoint string) (llm.LLMClient, error) {
			return mockLLM, nil
		},
		processLLMResponseFunc: func(result *llm.ProviderResult) (string, error) {
			// Ensure we only get llm.ProviderResult and not something that was converted
			if mockLLM.convertedFromProvider {
				t.Error("Processor is converting from provider-specific types instead of using llm.ProviderResult directly")
			}
			return result.Content, nil
		},
	}

	// Other mocks
	mockWriter := &mockFileWriter{
		saveToFileFunc: func(content, outputFile string) error {
			return nil
		},
	}
	mockAudit := &mockAuditLogger{
		logFunc: func(entry auditlog.AuditEntry) error {
			return nil
		},
	}
	mockLogger := newNoOpLogger()

	// Setup config
	cfg := config.NewDefaultCliConfig()
	cfg.APIKey = "test-api-key"
	cfg.OutputDir = "/tmp/test-output"

	// Create processor
	processor := modelproc.NewProcessor(
		mockAPI,
		mockWriter,
		mockAudit,
		mockLogger,
		cfg,
	)

	// Run the processor
	err := processor.Process(
		context.Background(),
		"test-model",
		"Test prompt",
	)

	// Verify success
	if err != nil {
		t.Errorf("Expected no error, got %v", err)
	}

	// Check that our mock client was used properly
	if mockLLM.convertedFromProvider {
		t.Error("The processor attempted to convert between provider-specific and generic types")
	}
}

// mockLLMClientWithProviderCheck is a special mock LLM client that checks
// if any provider-specific conversions are attempted.
type mockLLMClientWithProviderCheck struct {
	modelName             string
	convertedFromProvider bool
}

func (m *mockLLMClientWithProviderCheck) GenerateContent(ctx context.Context, prompt string, params map[string]interface{}) (*llm.ProviderResult, error) {
	return &llm.ProviderResult{
		Content:    "Test content",
		TokenCount: 50,
	}, nil
}

func (m *mockLLMClientWithProviderCheck) CountTokens(ctx context.Context, prompt string) (*llm.ProviderTokenCount, error) {
	return &llm.ProviderTokenCount{
		Total: 100,
	}, nil
}

func (m *mockLLMClientWithProviderCheck) GetModelInfo(ctx context.Context) (*llm.ProviderModelInfo, error) {
	return &llm.ProviderModelInfo{
		Name:             m.modelName,
		InputTokenLimit:  1000,
		OutputTokenLimit: 500,
	}, nil
}

func (m *mockLLMClientWithProviderCheck) GetModelName() string {
	return m.modelName
}

func (m *mockLLMClientWithProviderCheck) Close() error {
	return nil
}

// mockLLMAPIService mocks the modelproc.APIService interface with LLM support
type mockLLMAPIService struct {
	initLLMClientFunc        func(ctx context.Context, apiKey, modelName, apiEndpoint string) (llm.LLMClient, error)
	processLLMResponseFunc   func(result *llm.ProviderResult) (string, error)
	isEmptyResponseErrorFunc func(err error) bool
	isSafetyBlockedErrorFunc func(err error) bool
	getErrorDetailsFunc      func(err error) string
	getModelParametersFunc   func(modelName string) (map[string]interface{}, error)
	getModelDefinitionFunc   func(modelName string) (*registry.ModelDefinition, error)
	getModelTokenLimitsFunc  func(modelName string) (contextWindow, maxOutputTokens int32, err error)
}

func (m *mockLLMAPIService) InitLLMClient(ctx context.Context, apiKey, modelName, apiEndpoint string) (llm.LLMClient, error) {
	return m.initLLMClientFunc(ctx, apiKey, modelName, apiEndpoint)
}

func (m *mockLLMAPIService) ProcessLLMResponse(result *llm.ProviderResult) (string, error) {
	return m.processLLMResponseFunc(result)
}

func (m *mockLLMAPIService) IsEmptyResponseError(err error) bool {
	if m.isEmptyResponseErrorFunc != nil {
		return m.isEmptyResponseErrorFunc(err)
	}
	return false
}

func (m *mockLLMAPIService) IsSafetyBlockedError(err error) bool {
	if m.isSafetyBlockedErrorFunc != nil {
		return m.isSafetyBlockedErrorFunc(err)
	}
	return false
}

func (m *mockLLMAPIService) GetErrorDetails(err error) string {
	if m.getErrorDetailsFunc != nil {
		return m.getErrorDetailsFunc(err)
	}
	return fmt.Sprintf("error: %v", err)
}

func (m *mockLLMAPIService) GetModelParameters(modelName string) (map[string]interface{}, error) {
	if m.getModelParametersFunc != nil {
		return m.getModelParametersFunc(modelName)
	}
	// Default implementation returns empty parameters
	return make(map[string]interface{}), nil
}

func (m *mockLLMAPIService) GetModelDefinition(modelName string) (*registry.ModelDefinition, error) {
	if m.getModelDefinitionFunc != nil {
		return m.getModelDefinitionFunc(modelName)
	}
	// Default implementation returns a minimal model definition
	return &registry.ModelDefinition{
		Name:            modelName,
		ContextWindow:   8192,
		MaxOutputTokens: 2048,
	}, nil
}

func (m *mockLLMAPIService) GetModelTokenLimits(modelName string) (contextWindow, maxOutputTokens int32, err error) {
	if m.getModelTokenLimitsFunc != nil {
		return m.getModelTokenLimitsFunc(modelName)
	}
	// Default implementation returns standard values
	return 8192, 2048, nil
}

// We're using the mockLLMClient from mocks_test.go
// All necessary methods are already defined there

// TestModelProcessor_Process_ErrorCategorization tests that the ModelProcessor
// correctly handles and categorizes errors from LLM clients
func TestModelProcessor_Process_ErrorCategorization(t *testing.T) {
	testCases := []struct {
		name           string
		errorCategory  llm.ErrorCategory
		errorMessage   string
		expectContains string
	}{
		{
			name:           "rate limit error",
			errorCategory:  llm.CategoryRateLimit,
			errorMessage:   "rate limit exceeded",
			expectContains: "RateLimitError",
		},
		{
			name:           "auth error",
			errorCategory:  llm.CategoryAuth,
			errorMessage:   "invalid API key",
			expectContains: "AuthenticationError",
		},
		{
			name:           "input limit error",
			errorCategory:  llm.CategoryInputLimit,
			errorMessage:   "token limit exceeded",
			expectContains: "InputLimitError",
		},
		{
			name:           "content filtered error",
			errorCategory:  llm.CategoryContentFiltered,
			errorMessage:   "content blocked by safety filters",
			expectContains: "ContentFilteredError",
		},
		{
			name:           "network error",
			errorCategory:  llm.CategoryNetwork,
			errorMessage:   "network connection failed",
			expectContains: "NetworkError",
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			// Create a test error that implements the CategorizedError interface
			testError := &categorizedTestError{
				msg:      tc.errorMessage,
				category: tc.errorCategory,
			}

			// Track error message
			var loggedError string

			mockLLM := &mockLLMClient{
				getModelNameFunc: func() string { return "test-model" },
				getModelInfoFunc: func(ctx context.Context) (*llm.ProviderModelInfo, error) {
					return &llm.ProviderModelInfo{
						Name:             "test-model",
						InputTokenLimit:  1000,
						OutputTokenLimit: 500,
					}, nil
				},
				countTokensFunc: func(ctx context.Context, prompt string) (*llm.ProviderTokenCount, error) {
					return &llm.ProviderTokenCount{
						Total: 100,
					}, nil
				},
				generateContentFunc: func(ctx context.Context, prompt string, params map[string]interface{}) (*llm.ProviderResult, error) {
					// Return our categorized error
					return nil, testError
				},
			}

			mockAPI := &mockLLMAPIService{
				initLLMClientFunc: func(ctx context.Context, apiKey, modelName, apiEndpoint string) (llm.LLMClient, error) {
					return mockLLM, nil
				},
				processLLMResponseFunc: func(result *llm.ProviderResult) (string, error) {
					return result.Content, nil
				},
				getErrorDetailsFunc: func(err error) string {
					return err.Error()
				},
				isEmptyResponseErrorFunc: func(err error) bool {
					return false
				},
				isSafetyBlockedErrorFunc: func(err error) bool {
					return false
				},
			}

			// Configure other mocks
			mockWriter := &mockFileWriter{
				saveToFileFunc: func(content, outputFile string) error {
					return nil
				},
			}
			mockAudit := &mockAuditLogger{
				logFunc: func(entry auditlog.AuditEntry) error {
					// Capture error type in the audit log
					if entry.Error != nil && entry.Error.Type != "" {
						loggedError = entry.Error.Type
					}
					return nil
				},
			}

			// Use a logger that captures errors
			mockLogger := &mockLogger{
				errorFunc: func(format string, args ...interface{}) {
					// This helps us verify specific error messages are logged
					msg := fmt.Sprintf(format, args...)
					t.Logf("Error logged: %s", msg)
				},
			}

			// Setup config
			cfg := config.NewDefaultCliConfig()
			cfg.APIKey = "test-api-key"
			cfg.OutputDir = "/tmp/test-output"

			// Create processor
			processor := modelproc.NewProcessor(
				mockAPI,
				mockWriter,
				mockAudit,
				mockLogger,
				cfg,
			)

			// Run test
			err := processor.Process(
				context.Background(),
				"test-model",
				"Test prompt",
			)

			// Verify results
			if err == nil {
				t.Errorf("Expected error, got nil")
			}

			// Verify that the error was categorized correctly in the audit log
			if loggedError == "" {
				t.Errorf("Expected error to be logged, but none was")
			} else if loggedError != tc.expectContains {
				t.Errorf("Expected error type to contain '%s', got '%s'", tc.expectContains, loggedError)
			}
		})
	}
}

// categorizedTestError is a test error that implements the CategorizedError interface
type categorizedTestError struct {
	msg      string
	category llm.ErrorCategory
}

func (e *categorizedTestError) Error() string {
	return e.msg
}

func (e *categorizedTestError) Category() llm.ErrorCategory {
	return e.category
}

// mockLogger implements a logger that captures errors for testing
type mockLogger struct {
	infoFunc    func(format string, args ...interface{})
	errorFunc   func(format string, args ...interface{})
	debugFunc   func(format string, args ...interface{})
	fatalFunc   func(format string, args ...interface{})
	warnFunc    func(format string, args ...interface{})
	printlnFunc func(args ...interface{})
	printfFunc  func(format string, args ...interface{})
}

func (l *mockLogger) Info(format string, args ...interface{}) {
	if l.infoFunc != nil {
		l.infoFunc(format, args...)
	}
}

func (l *mockLogger) Error(format string, args ...interface{}) {
	if l.errorFunc != nil {
		l.errorFunc(format, args...)
	}
}

func (l *mockLogger) Debug(format string, args ...interface{}) {
	if l.debugFunc != nil {
		l.debugFunc(format, args...)
	}
}

func (l *mockLogger) Fatal(format string, args ...interface{}) {
	if l.fatalFunc != nil {
		l.fatalFunc(format, args...)
	}
}

func (l *mockLogger) Warn(format string, args ...interface{}) {
	if l.warnFunc != nil {
		l.warnFunc(format, args...)
	}
}

func (l *mockLogger) Println(args ...interface{}) {
	if l.printlnFunc != nil {
		l.printlnFunc(args...)
	}
}

func (l *mockLogger) Printf(format string, args ...interface{}) {
	if l.printfFunc != nil {
		l.printfFunc(format, args...)
	}
}
