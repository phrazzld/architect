package modelproc_test

import (
	"context"

	"github.com/phrazzld/thinktank/internal/auditlog"
	"github.com/phrazzld/thinktank/internal/llm"
	"github.com/phrazzld/thinktank/internal/registry"
)

// Mock implementations
type mockAPIService struct {
	isEmptyResponseErrorFunc func(err error) bool
	isSafetyBlockedErrorFunc func(err error) bool
	getErrorDetailsFunc      func(err error) string
	initLLMClientFunc        func(ctx context.Context, apiKey, modelName, apiEndpoint string) (llm.LLMClient, error)
	processLLMResponseFunc   func(result *llm.ProviderResult) (string, error)
	getModelParametersFunc   func(modelName string) (map[string]interface{}, error)
	getModelDefinitionFunc   func(modelName string) (*registry.ModelDefinition, error)
	getModelTokenLimitsFunc  func(modelName string) (contextWindow, maxOutputTokens int32, err error)
}

func (m *mockAPIService) IsEmptyResponseError(err error) bool {
	if m.isEmptyResponseErrorFunc != nil {
		return m.isEmptyResponseErrorFunc(err)
	}
	return false
}

func (m *mockAPIService) IsSafetyBlockedError(err error) bool {
	if m.isSafetyBlockedErrorFunc != nil {
		return m.isSafetyBlockedErrorFunc(err)
	}
	return false
}

func (m *mockAPIService) GetErrorDetails(err error) string {
	if m.getErrorDetailsFunc != nil {
		return m.getErrorDetailsFunc(err)
	}
	return err.Error()
}

func (m *mockAPIService) InitLLMClient(ctx context.Context, apiKey, modelName, apiEndpoint string) (llm.LLMClient, error) {
	if m.initLLMClientFunc != nil {
		return m.initLLMClientFunc(ctx, apiKey, modelName, apiEndpoint)
	}
	// Default implementation uses a mock LLM client
	return &mockLLMClient{}, nil
}

func (m *mockAPIService) ProcessLLMResponse(result *llm.ProviderResult) (string, error) {
	if m.processLLMResponseFunc != nil {
		return m.processLLMResponseFunc(result)
	}
	// Default implementation returns the content
	return result.Content, nil
}

// Implement the new registry methods
func (m *mockAPIService) GetModelParameters(modelName string) (map[string]interface{}, error) {
	if m.getModelParametersFunc != nil {
		return m.getModelParametersFunc(modelName)
	}
	// Default implementation returns empty map
	return make(map[string]interface{}), nil
}

func (m *mockAPIService) GetModelDefinition(modelName string) (*registry.ModelDefinition, error) {
	if m.getModelDefinitionFunc != nil {
		return m.getModelDefinitionFunc(modelName)
	}
	// Default implementation returns nil and error
	return nil, nil
}

func (m *mockAPIService) GetModelTokenLimits(modelName string) (contextWindow, maxOutputTokens int32, err error) {
	if m.getModelTokenLimitsFunc != nil {
		return m.getModelTokenLimitsFunc(modelName)
	}
	// Default implementation returns zeros with no error
	return 0, 0, nil
}

// Note: TokenManager implementation and related code has been removed
// as token management is no longer part of the production code

type mockFileWriter struct {
	writeFileFunc  func(path string, content string) error
	saveToFileFunc func(content, outputFile string) error
}

func (m *mockFileWriter) WriteFile(path string, content string) error {
	if m.writeFileFunc != nil {
		return m.writeFileFunc(path, content)
	}
	return nil
}

func (m *mockFileWriter) SaveToFile(content, outputFile string) error {
	if m.saveToFileFunc != nil {
		return m.saveToFileFunc(content, outputFile)
	}
	return nil
}

type mockLLMClient struct {
	generateContentFunc func(ctx context.Context, prompt string, params map[string]interface{}) (*llm.ProviderResult, error)
	getModelNameFunc    func() string
	closeFunc           func() error
}

func (m *mockLLMClient) GenerateContent(ctx context.Context, prompt string, params map[string]interface{}) (*llm.ProviderResult, error) {
	if m.generateContentFunc != nil {
		return m.generateContentFunc(ctx, prompt, params)
	}
	return &llm.ProviderResult{Content: "mock content"}, nil
}

func (m *mockLLMClient) GetModelName() string {
	if m.getModelNameFunc != nil {
		return m.getModelNameFunc()
	}
	return "mock-model"
}

func (m *mockLLMClient) Close() error {
	if m.closeFunc != nil {
		return m.closeFunc()
	}
	return nil
}

type mockAuditLogger struct {
	logFunc   func(entry auditlog.AuditEntry) error
	logOpFunc func(operation, status string, inputs map[string]interface{}, outputs map[string]interface{}, err error) error
	closeFunc func() error
}

func (m *mockAuditLogger) Log(entry auditlog.AuditEntry) error {
	if m.logFunc != nil {
		return m.logFunc(entry)
	}
	return nil
}

func (m *mockAuditLogger) LogOp(operation, status string, inputs map[string]interface{}, outputs map[string]interface{}, err error) error {
	if m.logOpFunc != nil {
		return m.logOpFunc(operation, status, inputs, outputs, err)
	}
	return nil
}

func (m *mockAuditLogger) Close() error {
	if m.closeFunc != nil {
		return m.closeFunc()
	}
	return nil
}
