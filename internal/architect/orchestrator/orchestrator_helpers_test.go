package orchestrator

import (
	"context"
	"errors"
	"fmt"
	"strings"
	"sync"
	"testing"

	"github.com/phrazzld/architect/internal/architect/interfaces"
	"github.com/phrazzld/architect/internal/auditlog"
	"github.com/phrazzld/architect/internal/config"
	"github.com/phrazzld/architect/internal/fileutil"
	"github.com/phrazzld/architect/internal/llm"
	"github.com/phrazzld/architect/internal/ratelimit"
	"github.com/phrazzld/architect/internal/registry"
)

// orchestratorTestDeps holds all the test dependencies for orchestrator tests
type orchestratorTestDeps struct {
	apiService      *mockAPIService
	contextGatherer *mockContextGatherer
	tokenManager    *mockTokenManager
	fileWriter      *mockFileWriter
	auditLogger     *mockAuditLogger
	rateLimiter     *ratelimit.RateLimiter
	config          *config.CliConfig
	logger          *mockLogger
	instructions    string
}

// newTestDeps creates a new set of test dependencies
func newTestDeps() *orchestratorTestDeps {
	return &orchestratorTestDeps{
		apiService:      &mockAPIService{},
		contextGatherer: &mockContextGatherer{},
		tokenManager:    &mockTokenManager{},
		fileWriter:      &mockFileWriter{},
		auditLogger:     &mockAuditLogger{},
		rateLimiter:     ratelimit.NewRateLimiter(1, 1),
		config:          &config.CliConfig{},
		logger:          &mockLogger{},
		instructions:    "test instructions",
	}
}

// createOrchestrator creates a new Orchestrator with the test dependencies
func (d *orchestratorTestDeps) createOrchestrator() *Orchestrator {
	return NewOrchestrator(
		d.apiService,
		d.contextGatherer,
		d.tokenManager,
		d.fileWriter,
		d.auditLogger,
		d.rateLimiter,
		d.config,
		d.logger,
	)
}

// setupBasicContext sets up basic context gathering behavior
func (d *orchestratorTestDeps) setupBasicContext() {
	var testFiles []fileutil.FileMeta
	testFile := fileutil.FileMeta{
		Path:    "test.go",
		Content: "package test",
	}
	testFiles = append(testFiles, testFile)

	d.contextGatherer.GatherContextFunc = func(ctx context.Context, config interfaces.GatherConfig) ([]fileutil.FileMeta, *interfaces.ContextStats, error) {
		return testFiles, &interfaces.ContextStats{ProcessedFilesCount: 1}, nil
	}
}

// setupLLMClient sets up mock clients using the provider-agnostic interface
func (d *orchestratorTestDeps) setupGeminiClient() *mockLLMClient {
	// Create a shared mock client with default implementation
	client := &mockLLMClient{}

	// Configure the API service to return a model-specific provider-agnostic client
	d.apiService.InitLLMClientFunc = func(ctx context.Context, apiKey, modelName, apiEndpoint string) (llm.LLMClient, error) {
		// Create a new client for each model
		return &mockLLMClient{
			modelName: modelName,
		}, nil
	}

	return client
}

// setupMultiModelConfig configures multiple model names
func (d *orchestratorTestDeps) setupMultiModelConfig(modelNames []string) {
	d.config.ModelNames = modelNames
	d.config.OutputDir = "test_output"
}

// setupDryRunConfig configures dry run mode
func (d *orchestratorTestDeps) setupDryRunConfig() {
	d.config.DryRun = true
}

// runOrchestrator runs the orchestrator with the given instructions
func (d *orchestratorTestDeps) runOrchestrator(ctx context.Context, instructions string) error {
	orch := d.createOrchestrator()
	return orch.Run(ctx, instructions)
}

// verifyBasicWorkflow checks that a basic workflow executed correctly
func (d *orchestratorTestDeps) verifyBasicWorkflow(t *testing.T, expectedModelNames []string) {
	// Verify API client initialization
	if len(d.apiService.InitLLMClientCalls) != len(expectedModelNames) {
		t.Errorf("Expected %d calls to InitLLMClient, got %d",
			len(expectedModelNames),
			len(d.apiService.InitLLMClientCalls))
	}

	// Verify that the file writer was called with all model outputs
	if len(d.fileWriter.SaveToFileCalls) != len(expectedModelNames) {
		t.Errorf("Expected %d calls to SaveToFile, got %d", len(expectedModelNames), len(d.fileWriter.SaveToFileCalls))
	}
}

// verifyDryRunWorkflow checks that a dry run workflow executed correctly
func (d *orchestratorTestDeps) verifyDryRunWorkflow(t *testing.T) {
	// In dry run mode, should not call InitLLMClient or SaveToFile
	if len(d.apiService.InitLLMClientCalls) > 0 {
		t.Errorf("Should not call InitLLMClient in dry run mode, got %d calls",
			len(d.apiService.InitLLMClientCalls))
	}

	if len(d.fileWriter.SaveToFileCalls) > 0 {
		t.Errorf("Should not call SaveToFile in dry run mode, got %d calls", len(d.fileWriter.SaveToFileCalls))
	}
}

// Mock implementations for dependencies

// mockAPIService mocks the interfaces.APIService
type mockAPIService struct {
	InitLLMClientFunc          func(ctx context.Context, apiKey, modelName, apiEndpoint string) (llm.LLMClient, error)
	ProcessLLMResponseFunc     func(result *llm.ProviderResult) (string, error)
	IsEmptyResponseErrorFunc   func(err error) bool
	IsSafetyBlockedErrorFunc   func(err error) bool
	GetErrorDetailsFunc        func(err error) string
	GetModelParametersFunc     func(modelName string) (map[string]interface{}, error)
	ValidateModelParameterFunc func(modelName, paramName string, value interface{}) (bool, error)
	GetModelDefinitionFunc     func(modelName string) (*registry.ModelDefinition, error)
	GetModelTokenLimitsFunc    func(modelName string) (contextWindow, maxOutputTokens int32, err error)

	InitLLMClientCalls          []struct{ ApiKey, ModelName, ApiEndpoint string }
	ProcessLLMResponseCalls     []struct{ Result *llm.ProviderResult }
	IsEmptyResponseErrorCalls   []struct{ Err error }
	IsSafetyBlockedErrorCalls   []struct{ Err error }
	GetErrorDetailsCalls        []struct{ Err error }
	GetModelParametersCalls     []struct{ ModelName string }
	ValidateModelParameterCalls []struct {
		ModelName, ParamName string
		Value                interface{}
	}
	GetModelDefinitionCalls  []struct{ ModelName string }
	GetModelTokenLimitsCalls []struct{ ModelName string }

	mu sync.Mutex
}

func (m *mockAPIService) InitLLMClient(ctx context.Context, apiKey, modelName, apiEndpoint string) (llm.LLMClient, error) {
	m.mu.Lock()
	defer m.mu.Unlock()

	call := struct{ ApiKey, ModelName, ApiEndpoint string }{
		ApiKey:      apiKey,
		ModelName:   modelName,
		ApiEndpoint: apiEndpoint,
	}
	m.InitLLMClientCalls = append(m.InitLLMClientCalls, call)

	if m.InitLLMClientFunc != nil {
		return m.InitLLMClientFunc(ctx, apiKey, modelName, apiEndpoint)
	}
	return &mockLLMClient{modelName: modelName}, nil
}

func (m *mockAPIService) ProcessLLMResponse(result *llm.ProviderResult) (string, error) {
	m.mu.Lock()
	defer m.mu.Unlock()

	call := struct{ Result *llm.ProviderResult }{Result: result}
	m.ProcessLLMResponseCalls = append(m.ProcessLLMResponseCalls, call)

	if m.ProcessLLMResponseFunc != nil {
		return m.ProcessLLMResponseFunc(result)
	}

	if result == nil {
		return "", errors.New("nil result error")
	}

	if result.Content == "" {
		return "", errors.New("empty content error")
	}

	return result.Content, nil
}

func (m *mockAPIService) IsEmptyResponseError(err error) bool {
	m.mu.Lock()
	defer m.mu.Unlock()

	call := struct{ Err error }{Err: err}
	m.IsEmptyResponseErrorCalls = append(m.IsEmptyResponseErrorCalls, call)

	if m.IsEmptyResponseErrorFunc != nil {
		return m.IsEmptyResponseErrorFunc(err)
	}

	return err != nil && strings.Contains(err.Error(), "empty")
}

func (m *mockAPIService) IsSafetyBlockedError(err error) bool {
	m.mu.Lock()
	defer m.mu.Unlock()

	call := struct{ Err error }{Err: err}
	m.IsSafetyBlockedErrorCalls = append(m.IsSafetyBlockedErrorCalls, call)

	if m.IsSafetyBlockedErrorFunc != nil {
		return m.IsSafetyBlockedErrorFunc(err)
	}

	return err != nil && strings.Contains(err.Error(), "safety") || strings.Contains(err.Error(), "blocked")
}

func (m *mockAPIService) GetErrorDetails(err error) string {
	m.mu.Lock()
	defer m.mu.Unlock()

	call := struct{ Err error }{Err: err}
	m.GetErrorDetailsCalls = append(m.GetErrorDetailsCalls, call)

	if m.GetErrorDetailsFunc != nil {
		return m.GetErrorDetailsFunc(err)
	}

	if err == nil {
		return ""
	}

	return fmt.Sprintf("Error details: %v", err)
}

// mockContextGatherer mocks the interfaces.ContextGatherer
type mockContextGatherer struct {
	GatherContextFunc     func(ctx context.Context, config interfaces.GatherConfig) ([]fileutil.FileMeta, *interfaces.ContextStats, error)
	DisplayDryRunInfoFunc func(ctx context.Context, stats *interfaces.ContextStats) error

	GatherContextCalls     []struct{ Config interfaces.GatherConfig }
	DisplayDryRunInfoCalls []struct{ Stats *interfaces.ContextStats }

	mu sync.Mutex
}

func (m *mockContextGatherer) GatherContext(ctx context.Context, config interfaces.GatherConfig) ([]fileutil.FileMeta, *interfaces.ContextStats, error) {
	m.mu.Lock()
	defer m.mu.Unlock()

	call := struct{ Config interfaces.GatherConfig }{Config: config}
	m.GatherContextCalls = append(m.GatherContextCalls, call)

	if m.GatherContextFunc != nil {
		return m.GatherContextFunc(ctx, config)
	}

	// Default implementation returns a single test file
	testFile := fileutil.FileMeta{
		Path:    "test.txt",
		Content: "test content",
	}
	stats := &interfaces.ContextStats{
		ProcessedFilesCount: 1,
		CharCount:           len(testFile.Content),
		LineCount:           1,
		TokenCount:          2,
		ProcessedFiles:      []string{testFile.Path},
	}
	return []fileutil.FileMeta{testFile}, stats, nil
}

func (m *mockContextGatherer) DisplayDryRunInfo(ctx context.Context, stats *interfaces.ContextStats) error {
	m.mu.Lock()
	defer m.mu.Unlock()

	call := struct{ Stats *interfaces.ContextStats }{Stats: stats}
	m.DisplayDryRunInfoCalls = append(m.DisplayDryRunInfoCalls, call)

	if m.DisplayDryRunInfoFunc != nil {
		return m.DisplayDryRunInfoFunc(ctx, stats)
	}

	// Default implementation does nothing
	return nil
}

// mockTokenManager mocks the interfaces.TokenManager
type mockTokenManager struct {
	CheckTokenLimitFunc       func(ctx context.Context, prompt string) error
	GetTokenInfoFunc          func(ctx context.Context, prompt string) (*interfaces.TokenResult, error)
	PromptForConfirmationFunc func(tokenCount int32, threshold int) bool

	CheckTokenLimitCalls       []struct{ Prompt string }
	GetTokenInfoCalls          []struct{ Prompt string }
	PromptForConfirmationCalls []struct {
		TokenCount int32
		Threshold  int
	}

	mu sync.Mutex
}

func (m *mockTokenManager) CheckTokenLimit(ctx context.Context, prompt string) error {
	m.mu.Lock()
	defer m.mu.Unlock()

	call := struct{ Prompt string }{Prompt: prompt}
	m.CheckTokenLimitCalls = append(m.CheckTokenLimitCalls, call)

	if m.CheckTokenLimitFunc != nil {
		return m.CheckTokenLimitFunc(ctx, prompt)
	}

	// Default implementation - no token limit issues
	return nil
}

func (m *mockTokenManager) GetTokenInfo(ctx context.Context, prompt string) (*interfaces.TokenResult, error) {
	m.mu.Lock()
	defer m.mu.Unlock()

	call := struct{ Prompt string }{Prompt: prompt}
	m.GetTokenInfoCalls = append(m.GetTokenInfoCalls, call)

	if m.GetTokenInfoFunc != nil {
		return m.GetTokenInfoFunc(ctx, prompt)
	}

	// Default implementation - token count is 1/4 of the prompt length (rough approximation)
	tokenCount := int32(len(prompt) / 4)
	return &interfaces.TokenResult{
		TokenCount:   tokenCount,
		InputLimit:   4000,
		ExceedsLimit: tokenCount > 4000,
		LimitError:   "",
		Percentage:   float64(tokenCount) / 4000 * 100,
	}, nil
}

func (m *mockTokenManager) PromptForConfirmation(tokenCount int32, threshold int) bool {
	m.mu.Lock()
	defer m.mu.Unlock()

	call := struct {
		TokenCount int32
		Threshold  int
	}{TokenCount: tokenCount, Threshold: threshold}
	m.PromptForConfirmationCalls = append(m.PromptForConfirmationCalls, call)

	if m.PromptForConfirmationFunc != nil {
		return m.PromptForConfirmationFunc(tokenCount, threshold)
	}

	// Default implementation - always confirm
	return true
}

// mockFileWriter mocks the interfaces.FileWriter
type mockFileWriter struct {
	SaveToFileFunc func(content, outputFile string) error

	SaveToFileCalls []struct{ Content, OutputFile string }

	mu sync.Mutex
}

func (m *mockFileWriter) SaveToFile(content, outputFile string) error {
	m.mu.Lock()
	defer m.mu.Unlock()

	call := struct{ Content, OutputFile string }{Content: content, OutputFile: outputFile}
	m.SaveToFileCalls = append(m.SaveToFileCalls, call)

	if m.SaveToFileFunc != nil {
		return m.SaveToFileFunc(content, outputFile)
	}

	// Default implementation - successful save
	return nil
}

// mockAuditLogger mocks the auditlog.AuditLogger
type mockAuditLogger struct {
	LogFunc   func(entry auditlog.AuditEntry) error
	CloseFunc func() error

	LogCalls   []struct{ Entry auditlog.AuditEntry }
	CloseCalls int

	mu sync.Mutex
}

func (m *mockAuditLogger) Log(entry auditlog.AuditEntry) error {
	m.mu.Lock()
	defer m.mu.Unlock()

	call := struct{ Entry auditlog.AuditEntry }{Entry: entry}
	m.LogCalls = append(m.LogCalls, call)

	if m.LogFunc != nil {
		return m.LogFunc(entry)
	}

	// Default implementation - successful log
	return nil
}

func (m *mockAuditLogger) Close() error {
	m.mu.Lock()
	defer m.mu.Unlock()

	m.CloseCalls++

	if m.CloseFunc != nil {
		return m.CloseFunc()
	}

	// Default implementation - successful close
	return nil
}

func (m *mockAuditLogger) GetLogPath() string {
	return "mock_audit.log"
}

// mockLogger mocks the logutil.LoggerInterface
type mockLogger struct {
	DebugFunc func(format string, args ...interface{})
	InfoFunc  func(format string, args ...interface{})
	WarnFunc  func(format string, args ...interface{})
	ErrorFunc func(format string, args ...interface{})
	FatalFunc func(format string, args ...interface{})

	DebugCalls []struct {
		Format string
		Args   []interface{}
	}
	InfoCalls []struct {
		Format string
		Args   []interface{}
	}
	WarnCalls []struct {
		Format string
		Args   []interface{}
	}
	ErrorCalls []struct {
		Format string
		Args   []interface{}
	}
	FatalCalls []struct {
		Format string
		Args   []interface{}
	}

	mu sync.Mutex
}

func (m *mockLogger) Debug(format string, args ...interface{}) {
	m.mu.Lock()
	defer m.mu.Unlock()

	call := struct {
		Format string
		Args   []interface{}
	}{Format: format, Args: args}
	m.DebugCalls = append(m.DebugCalls, call)

	if m.DebugFunc != nil {
		m.DebugFunc(format, args...)
	}
}

func (m *mockLogger) Info(format string, args ...interface{}) {
	m.mu.Lock()
	defer m.mu.Unlock()

	call := struct {
		Format string
		Args   []interface{}
	}{Format: format, Args: args}
	m.InfoCalls = append(m.InfoCalls, call)

	if m.InfoFunc != nil {
		m.InfoFunc(format, args...)
	}
}

func (m *mockLogger) Warn(format string, args ...interface{}) {
	m.mu.Lock()
	defer m.mu.Unlock()

	call := struct {
		Format string
		Args   []interface{}
	}{Format: format, Args: args}
	m.WarnCalls = append(m.WarnCalls, call)

	if m.WarnFunc != nil {
		m.WarnFunc(format, args...)
	}
}

func (m *mockLogger) Error(format string, args ...interface{}) {
	m.mu.Lock()
	defer m.mu.Unlock()

	call := struct {
		Format string
		Args   []interface{}
	}{Format: format, Args: args}
	m.ErrorCalls = append(m.ErrorCalls, call)

	if m.ErrorFunc != nil {
		m.ErrorFunc(format, args...)
	}
}

func (m *mockLogger) Fatal(format string, args ...interface{}) {
	m.mu.Lock()
	defer m.mu.Unlock()

	call := struct {
		Format string
		Args   []interface{}
	}{Format: format, Args: args}
	m.FatalCalls = append(m.FatalCalls, call)

	if m.FatalFunc != nil {
		m.FatalFunc(format, args...)
	}
}

// Println implements LoggerInterface
func (m *mockLogger) Println(v ...interface{}) {
	m.Info(fmt.Sprint(v...))
}

// Printf implements LoggerInterface
func (m *mockLogger) Printf(format string, v ...interface{}) {
	m.Info(format, v...)
}

// mockLLMClient implements llm.LLMClient for tests
type mockLLMClient struct {
	GenerateContentFunc func(ctx context.Context, prompt string, params map[string]interface{}) (*llm.ProviderResult, error)
	CountTokensFunc     func(ctx context.Context, prompt string) (*llm.ProviderTokenCount, error)
	GetModelInfoFunc    func(ctx context.Context) (*llm.ProviderModelInfo, error)
	GetModelNameFunc    func() string
	CloseFunc           func() error

	modelName string
	mu        sync.Mutex
}

func (m *mockLLMClient) GenerateContent(ctx context.Context, prompt string, params map[string]interface{}) (*llm.ProviderResult, error) {
	m.mu.Lock()
	defer m.mu.Unlock()

	if m.GenerateContentFunc != nil {
		return m.GenerateContentFunc(ctx, prompt, params)
	}

	// Default mock implementation
	return &llm.ProviderResult{
		Content:    "Mock response for " + prompt,
		TokenCount: int32(len(prompt) / 4),
	}, nil
}

func (m *mockLLMClient) CountTokens(ctx context.Context, prompt string) (*llm.ProviderTokenCount, error) {
	m.mu.Lock()
	defer m.mu.Unlock()

	if m.CountTokensFunc != nil {
		return m.CountTokensFunc(ctx, prompt)
	}

	// Default mock implementation
	return &llm.ProviderTokenCount{
		Total: int32(len(prompt) / 4),
	}, nil
}

func (m *mockLLMClient) GetModelInfo(ctx context.Context) (*llm.ProviderModelInfo, error) {
	m.mu.Lock()
	defer m.mu.Unlock()

	if m.GetModelInfoFunc != nil {
		return m.GetModelInfoFunc(ctx)
	}

	// Default mock implementation
	return &llm.ProviderModelInfo{
		Name:             m.modelName,
		InputTokenLimit:  8192,
		OutputTokenLimit: 2048,
	}, nil
}

func (m *mockLLMClient) GetModelName() string {
	m.mu.Lock()
	defer m.mu.Unlock()

	if m.GetModelNameFunc != nil {
		return m.GetModelNameFunc()
	}

	if m.modelName != "" {
		return m.modelName
	}

	return "mock-model"
}

func (m *mockLLMClient) Close() error {
	m.mu.Lock()
	defer m.mu.Unlock()

	if m.CloseFunc != nil {
		return m.CloseFunc()
	}

	return nil
}

// Implement the registry-related methods for mockAPIService

func (m *mockAPIService) GetModelParameters(modelName string) (map[string]interface{}, error) {
	m.mu.Lock()
	defer m.mu.Unlock()

	call := struct{ ModelName string }{ModelName: modelName}
	m.GetModelParametersCalls = append(m.GetModelParametersCalls, call)

	if m.GetModelParametersFunc != nil {
		return m.GetModelParametersFunc(modelName)
	}

	// Default implementation returns empty parameters
	return make(map[string]interface{}), nil
}

func (m *mockAPIService) GetModelDefinition(modelName string) (*registry.ModelDefinition, error) {
	m.mu.Lock()
	defer m.mu.Unlock()

	call := struct{ ModelName string }{ModelName: modelName}
	m.GetModelDefinitionCalls = append(m.GetModelDefinitionCalls, call)

	if m.GetModelDefinitionFunc != nil {
		return m.GetModelDefinitionFunc(modelName)
	}

	// Default implementation returns a minimal model definition
	return &registry.ModelDefinition{
		Name:            modelName,
		ContextWindow:   8192,
		MaxOutputTokens: 2048,
	}, nil
}

func (m *mockAPIService) GetModelTokenLimits(modelName string) (contextWindow, maxOutputTokens int32, err error) {
	m.mu.Lock()
	defer m.mu.Unlock()

	call := struct{ ModelName string }{ModelName: modelName}
	m.GetModelTokenLimitsCalls = append(m.GetModelTokenLimitsCalls, call)

	if m.GetModelTokenLimitsFunc != nil {
		return m.GetModelTokenLimitsFunc(modelName)
	}

	// Default implementation returns standard values
	return 8192, 2048, nil
}

func (m *mockAPIService) ValidateModelParameter(modelName, paramName string, value interface{}) (bool, error) {
	m.mu.Lock()
	defer m.mu.Unlock()

	call := struct {
		ModelName, ParamName string
		Value                interface{}
	}{
		ModelName: modelName,
		ParamName: paramName,
		Value:     value,
	}
	m.ValidateModelParameterCalls = append(m.ValidateModelParameterCalls, call)

	if m.ValidateModelParameterFunc != nil {
		return m.ValidateModelParameterFunc(modelName, paramName, value)
	}

	// Default implementation - accept common parameters with basic validation
	switch paramName {
	case "temperature":
		if temp, ok := value.(float64); ok {
			return temp >= 0 && temp <= 1, nil
		}
		return false, fmt.Errorf("temperature must be a float between 0 and 1")
	case "top_p":
		if topP, ok := value.(float64); ok {
			return topP >= 0 && topP <= 1, nil
		}
		return false, fmt.Errorf("top_p must be a float between 0 and 1")
	case "max_tokens":
		if _, ok := value.(int); ok {
			return true, nil
		}
		return false, fmt.Errorf("max_tokens must be an integer")
	default:
		// By default, accept unknown parameters
		return true, nil
	}
}
