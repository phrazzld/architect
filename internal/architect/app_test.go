// Package architect contains the core application logic for the architect tool
package architect

import (
	"context"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"testing"

	"github.com/phrazzld/architect/internal/architect/interfaces"
	"github.com/phrazzld/architect/internal/auditlog"
	"github.com/phrazzld/architect/internal/config"
	"github.com/phrazzld/architect/internal/llm"
	"github.com/phrazzld/architect/internal/logutil"
	"github.com/phrazzld/architect/internal/ratelimit"
	"github.com/phrazzld/architect/internal/registry"
)

// ----- Mock Implementations -----

// MockAuditLogger mocks the auditlog.AuditLogger interface for testing
type MockAuditLogger struct {
	mu      sync.Mutex
	entries []auditlog.AuditEntry
	logErr  error
}

func NewMockAuditLogger() *MockAuditLogger {
	return &MockAuditLogger{
		entries: []auditlog.AuditEntry{},
		logErr:  nil,
	}
}

func (m *MockAuditLogger) Log(entry auditlog.AuditEntry) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.entries = append(m.entries, entry)
	return m.logErr
}

func (m *MockAuditLogger) Close() error {
	return nil
}

func (m *MockAuditLogger) GetEntries() []auditlog.AuditEntry {
	m.mu.Lock()
	defer m.mu.Unlock()
	result := make([]auditlog.AuditEntry, len(m.entries))
	copy(result, m.entries)
	return result
}

func (m *MockAuditLogger) FindEntry(operation string) *auditlog.AuditEntry {
	m.mu.Lock()
	defer m.mu.Unlock()
	for i := len(m.entries) - 1; i >= 0; i-- {
		if m.entries[i].Operation == operation {
			return &m.entries[i]
		}
	}
	return nil
}

func (m *MockAuditLogger) SetLogError(err error) {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.logErr = err
}

// MockLogger mocks the logutil.LoggerInterface for testing
type MockLogger struct {
	debugMessages []string
	infoMessages  []string
	warnMessages  []string
	errorMessages []string
}

func NewMockLogger() *MockLogger {
	return &MockLogger{
		debugMessages: []string{},
		infoMessages:  []string{},
		warnMessages:  []string{},
		errorMessages: []string{},
	}
}

func (m *MockLogger) Debug(format string, args ...interface{}) {
	m.debugMessages = append(m.debugMessages, fmt.Sprintf(format, args...))
}

func (m *MockLogger) Info(format string, args ...interface{}) {
	m.infoMessages = append(m.infoMessages, fmt.Sprintf(format, args...))
}

func (m *MockLogger) Warn(format string, args ...interface{}) {
	m.warnMessages = append(m.warnMessages, fmt.Sprintf(format, args...))
}

func (m *MockLogger) Error(format string, args ...interface{}) {
	m.errorMessages = append(m.errorMessages, fmt.Sprintf(format, args...))
}

func (m *MockLogger) Fatal(format string, args ...interface{}) {
	m.errorMessages = append(m.errorMessages, "FATAL: "+fmt.Sprintf(format, args...))
}

func (m *MockLogger) Println(args ...interface{}) {}

func (m *MockLogger) Printf(format string, args ...interface{}) {}

// MockAPIService mocks the APIService interface
type MockAPIService struct {
	isEmptyResponseErrorFunc func(err error) bool
	isSafetyBlockedErrorFunc func(err error) bool
	getErrorDetailsFunc      func(err error) string
	initLLMClientErr         error
	mockLLMClient            llm.LLMClient
	processLLMResponseErr    error
	processedContent         string
}

func NewMockAPIService() *MockAPIService {
	return &MockAPIService{
		processedContent:         "Test Generated Plan",
		initLLMClientErr:         nil,
		mockLLMClient:            nil,
		processLLMResponseErr:    nil,
		isEmptyResponseErrorFunc: nil,
		isSafetyBlockedErrorFunc: nil,
		getErrorDetailsFunc:      nil,
	}
}

func (m *MockAPIService) IsEmptyResponseError(err error) bool {
	if m.isEmptyResponseErrorFunc != nil {
		return m.isEmptyResponseErrorFunc(err)
	}
	return strings.Contains(err.Error(), "empty content")
}

func (m *MockAPIService) IsSafetyBlockedError(err error) bool {
	if m.isSafetyBlockedErrorFunc != nil {
		return m.isSafetyBlockedErrorFunc(err)
	}
	return strings.Contains(err.Error(), "safety")
}

func (m *MockAPIService) GetErrorDetails(err error) string {
	if m.getErrorDetailsFunc != nil {
		return m.getErrorDetailsFunc(err)
	}
	return err.Error()
}

func (m *MockAPIService) InitLLMClient(ctx context.Context, apiKey, modelName, apiEndpoint string) (llm.LLMClient, error) {
	if m.initLLMClientErr != nil {
		return nil, m.initLLMClientErr
	}
	if m.mockLLMClient != nil {
		return m.mockLLMClient, nil
	}
	return &llm.MockLLMClient{}, nil
}

func (m *MockAPIService) ProcessLLMResponse(result *llm.ProviderResult) (string, error) {
	if m.processLLMResponseErr != nil {
		return "", m.processLLMResponseErr
	}
	if result == nil {
		return "", errors.New("nil result")
	}
	return m.processedContent, nil
}

func (m *MockAPIService) GetModelParameters(modelName string) (map[string]interface{}, error) {
	return map[string]interface{}{}, nil
}

func (m *MockAPIService) ValidateModelParameter(modelName, paramName string, value interface{}) (bool, error) {
	return true, nil
}

func (m *MockAPIService) GetModelDefinition(modelName string) (*registry.ModelDefinition, error) {
	return nil, errors.New("not implemented")
}

func (m *MockAPIService) GetModelTokenLimits(modelName string) (contextWindow, maxOutputTokens int32, err error) {
	return 8192, 8192, nil
}

// MockOrchestrator mocks the orchestrator for testing
type MockOrchestrator struct {
	runErr error
}

func NewMockOrchestrator() *MockOrchestrator {
	return &MockOrchestrator{
		runErr: nil,
	}
}

func (m *MockOrchestrator) Run(ctx context.Context, instructions string) error {
	return m.runErr
}

// ----- Test Helper Functions -----

// setupTestEnvironment creates a temporary directory for testing
func setupTestEnvironment(t *testing.T) (string, func()) {
	testDir, err := os.MkdirTemp("", "architect-test-*")
	if err != nil {
		t.Fatalf("Failed to create test directory: %v", err)
	}

	cleanup := func() {
		err := os.RemoveAll(testDir)
		if err != nil {
			t.Logf("Warning: Failed to clean up test directory: %v", err)
		}
	}

	return testDir, cleanup
}

// createTestFile creates a test file with the given content
func createTestFile(t *testing.T, path, content string) string {
	err := os.MkdirAll(filepath.Dir(path), 0755)
	if err != nil {
		t.Fatalf("Failed to create directory for test file: %v", err)
	}

	err = os.WriteFile(path, []byte(content), 0644)
	if err != nil {
		t.Fatalf("Failed to write test file: %v", err)
	}

	return path
}

// MockLLMClient implements the LLMClient interface for testing
type MockLLMClient struct {
	modelName       string
	tokenCount      int32
	generationErr   error
	generatedOutput string
}

func NewMockLLMClient(modelName string) *MockLLMClient {
	return &MockLLMClient{
		modelName:       modelName,
		tokenCount:      100,
		generatedOutput: "Test Generated Plan",
	}
}

func (m *MockLLMClient) GenerateContent(ctx context.Context, prompt string, params map[string]interface{}) (*llm.ProviderResult, error) {
	if m.generationErr != nil {
		return nil, m.generationErr
	}
	return &llm.ProviderResult{
		Content:      m.generatedOutput,
		TokenCount:   100,
		FinishReason: "STOP",
	}, nil
}

func (m *MockLLMClient) CountTokens(ctx context.Context, text string) (*llm.ProviderTokenCount, error) {
	return &llm.ProviderTokenCount{
		Total: 100,
	}, nil
}

func (m *MockLLMClient) GetModelInfo(ctx context.Context) (*llm.ProviderModelInfo, error) {
	return &llm.ProviderModelInfo{
		Name:             m.modelName,
		InputTokenLimit:  8192,
		OutputTokenLimit: 8192,
	}, nil
}

func (m *MockLLMClient) Close() error {
	return nil
}

func (m *MockLLMClient) GetModelName() string {
	return m.modelName
}

// ----- Test Cases -----

// TestExecuteHappyPath tests the happy path of the Execute function
func TestExecuteHappyPath(t *testing.T) {
	// Set up test environment
	testDir, cleanup := setupTestEnvironment(t)
	defer cleanup()

	// Create instruction file
	instructionsContent := "Test instructions for plan generation"
	instructionsFile := createTestFile(t, filepath.Join(testDir, "instructions.md"), instructionsContent)

	// Set up output directory
	outputDir := filepath.Join(testDir, "output")

	// Create configuration
	cliConfig := &config.CliConfig{
		InstructionsFile: instructionsFile,
		OutputDir:        outputDir,
		ModelNames:       []string{"test-model"},
		APIKey:           "test-api-key",
		Paths:            []string{testDir},
		LogLevel:         logutil.InfoLevel,
	}

	// Create mocks
	mockLogger := NewMockLogger()
	mockAuditLogger := NewMockAuditLogger()
	mockLLMClient := NewMockLLMClient("test-model")
	mockAPIService := NewMockAPIService()
	mockAPIService.mockLLMClient = mockLLMClient
	mockOrchestrator := NewMockOrchestrator()

	// Save original constructor for orchestrator
	originalOrchestrator := orchestratorConstructor

	// Override orchestrator constructor
	orchestratorConstructor = func(apiService interfaces.APIService, contextGatherer interfaces.ContextGatherer, fileWriter interfaces.FileWriter, auditLogger auditlog.AuditLogger, rateLimiter *ratelimit.RateLimiter, config *config.CliConfig, logger logutil.LoggerInterface) Orchestrator {
		return mockOrchestrator
	}

	// Restore original constructor when test finishes
	defer func() {
		orchestratorConstructor = originalOrchestrator
	}()

	// Execute the function - pass mockAPIService directly
	err := Execute(context.Background(), cliConfig, mockLogger, mockAuditLogger, mockAPIService)

	// Verify results
	if err != nil {
		t.Errorf("Execute returned an error: %v", err)
	}

	// Verify output directory was created
	if _, err := os.Stat(outputDir); os.IsNotExist(err) {
		t.Errorf("Output directory was not created at %s", outputDir)
	}

	// Verify audit log entries
	executeStartEntry := mockAuditLogger.FindEntry("ExecuteStart")
	if executeStartEntry == nil {
		t.Error("No ExecuteStart entry found in audit log")
		return
	}

	readInstructionsEntry := mockAuditLogger.FindEntry("ReadInstructions")
	if readInstructionsEntry == nil {
		t.Error("No ReadInstructions entry found in audit log")
		return
	}
	if readInstructionsEntry.Status != "Success" {
		t.Errorf("ReadInstructions status was %s, expected Success", readInstructionsEntry.Status)
	}

	executeEndEntry := mockAuditLogger.FindEntry("ExecuteEnd")
	if executeEndEntry == nil {
		t.Error("No ExecuteEnd entry found in audit log")
		return
	}
	if executeEndEntry.Status != "Success" {
		t.Errorf("ExecuteEnd status was %s, expected Success", executeEndEntry.Status)
	}
}

// TestExecuteDryRun tests the dry run mode
func TestExecuteDryRun(t *testing.T) {
	// Set up test environment
	testDir, cleanup := setupTestEnvironment(t)
	defer cleanup()

	// Create instruction file
	instructionsContent := "Test instructions for plan generation"
	instructionsFile := createTestFile(t, filepath.Join(testDir, "instructions.md"), instructionsContent)

	// Set up output directory
	outputDir := filepath.Join(testDir, "output")

	// Create configuration with dry run enabled
	cliConfig := &config.CliConfig{
		InstructionsFile: instructionsFile,
		OutputDir:        outputDir,
		ModelNames:       []string{"test-model"},
		APIKey:           "test-api-key",
		Paths:            []string{testDir},
		LogLevel:         logutil.InfoLevel,
		DryRun:           true, // Enable dry run mode
	}

	// Create mocks
	mockLogger := NewMockLogger()
	mockAuditLogger := NewMockAuditLogger()
	mockLLMClient := NewMockLLMClient("test-model")
	mockAPIService := NewMockAPIService()
	mockAPIService.mockLLMClient = mockLLMClient
	mockOrchestrator := NewMockOrchestrator()

	// Save original constructor for orchestrator
	originalOrchestrator := orchestratorConstructor

	// Override orchestrator constructor
	orchestratorConstructor = func(apiService interfaces.APIService, contextGatherer interfaces.ContextGatherer, fileWriter interfaces.FileWriter, auditLogger auditlog.AuditLogger, rateLimiter *ratelimit.RateLimiter, config *config.CliConfig, logger logutil.LoggerInterface) Orchestrator {
		return mockOrchestrator
	}

	// Restore original constructor when test finishes
	defer func() {
		orchestratorConstructor = originalOrchestrator
	}()

	// Execute the function - pass mockAPIService directly
	err := Execute(context.Background(), cliConfig, mockLogger, mockAuditLogger, mockAPIService)

	// Verify results
	if err != nil {
		t.Errorf("Execute returned an error in dry run mode: %v", err)
	}

	// Verify output directory was created (even in dry run mode, we create the directory)
	if _, err := os.Stat(outputDir); os.IsNotExist(err) {
		t.Errorf("Output directory was not created at %s", outputDir)
	}

	// Verify audit log entries
	executeStartEntry := mockAuditLogger.FindEntry("ExecuteStart")
	if executeStartEntry == nil {
		t.Error("No ExecuteStart entry found in audit log")
		return
	}

	// Check if dry_run is true in the inputs
	foundDryRun := false
	if executeStartEntry.Inputs != nil {
		if dryRun, ok := executeStartEntry.Inputs["dry_run"].(bool); ok && dryRun {
			foundDryRun = true
		}
	}
	if !foundDryRun {
		t.Error("ExecuteStart entry doesn't show dry_run = true")
	}

	readInstructionsEntry := mockAuditLogger.FindEntry("ReadInstructions")
	if readInstructionsEntry == nil {
		t.Error("No ReadInstructions entry found in audit log")
		return
	}
	if readInstructionsEntry.Status != "Success" {
		t.Errorf("ReadInstructions status was %s, expected Success", readInstructionsEntry.Status)
	}

	executeEndEntry := mockAuditLogger.FindEntry("ExecuteEnd")
	if executeEndEntry == nil {
		t.Error("No ExecuteEnd entry found in audit log")
		return
	}
	if executeEndEntry.Status != "Success" {
		t.Errorf("ExecuteEnd status was %s, expected Success", executeEndEntry.Status)
	}
}

// TestExecuteInstructionsFileError tests error handling when instructions file can't be read
func TestExecuteInstructionsFileError(t *testing.T) {
	// Set up test environment
	testDir, cleanup := setupTestEnvironment(t)
	defer cleanup()

	// Set up a non-existent instructions file
	instructionsFile := filepath.Join(testDir, "nonexistent-instructions.md")

	// Set up output directory
	outputDir := filepath.Join(testDir, "output")

	// Create configuration
	cliConfig := &config.CliConfig{
		InstructionsFile: instructionsFile,
		OutputDir:        outputDir,
		ModelNames:       []string{"test-model"},
		APIKey:           "test-api-key",
		Paths:            []string{testDir},
		LogLevel:         logutil.InfoLevel,
	}

	// Create mocks
	mockLogger := NewMockLogger()
	mockAuditLogger := NewMockAuditLogger()
	mockLLMClient := NewMockLLMClient("test-model")
	mockAPIService := NewMockAPIService()
	mockAPIService.mockLLMClient = mockLLMClient
	mockOrchestrator := NewMockOrchestrator()

	// Save original constructor for orchestrator
	originalNewOrchestrator := orchestratorConstructor

	// Override orchestrator constructor
	orchestratorConstructor = func(apiService interfaces.APIService, contextGatherer interfaces.ContextGatherer, fileWriter interfaces.FileWriter, auditLogger auditlog.AuditLogger, rateLimiter *ratelimit.RateLimiter, config *config.CliConfig, logger logutil.LoggerInterface) Orchestrator {
		return mockOrchestrator
	}

	// Restore original constructor when test finishes
	defer func() {
		orchestratorConstructor = originalNewOrchestrator
	}()

	// Execute the function
	err := Execute(context.Background(), cliConfig, mockLogger, mockAuditLogger, mockAPIService)

	// Verify results
	if err == nil {
		t.Error("Execute did not return an error for nonexistent instructions file")
	}
	if !strings.Contains(err.Error(), "failed to read instructions file") {
		t.Errorf("Unexpected error message: %v", err)
	}

	// Verify audit log entries
	executeStartEntry := mockAuditLogger.FindEntry("ExecuteStart")
	if executeStartEntry == nil {
		t.Error("No ExecuteStart entry found in audit log")
		return
	}

	readInstructionsEntry := mockAuditLogger.FindEntry("ReadInstructions")
	if readInstructionsEntry == nil {
		t.Error("No ReadInstructions entry found in audit log")
		return
	}
	if readInstructionsEntry.Status != "Failure" {
		t.Errorf("ReadInstructions status was %s, expected Failure", readInstructionsEntry.Status)
	}

	executeEndEntry := mockAuditLogger.FindEntry("ExecuteEnd")
	if executeEndEntry == nil {
		t.Error("No ExecuteEnd entry found in audit log")
		return
	}
	if executeEndEntry.Status != "Failure" {
		t.Errorf("ExecuteEnd status was %s, expected Failure", executeEndEntry.Status)
	}
}

// TestExecuteClientInitializationError tests error handling when API client initialization fails
func TestExecuteClientInitializationError(t *testing.T) {
	// Set up test environment
	testDir, cleanup := setupTestEnvironment(t)
	defer cleanup()

	// Create instruction file
	instructionsContent := "Test instructions for plan generation"
	instructionsFile := createTestFile(t, filepath.Join(testDir, "instructions.md"), instructionsContent)

	// Set up output directory
	outputDir := filepath.Join(testDir, "output")

	// Create configuration
	cliConfig := &config.CliConfig{
		InstructionsFile: instructionsFile,
		OutputDir:        outputDir,
		ModelNames:       []string{"test-model"},
		APIKey:           "test-api-key",
		Paths:            []string{testDir},
		LogLevel:         logutil.InfoLevel,
	}

	// Create mocks
	mockLogger := NewMockLogger()
	mockAuditLogger := NewMockAuditLogger()
	mockAPIService := NewMockAPIService()
	mockAPIService.initLLMClientErr = errors.New("API client initialization error")
	mockOrchestrator := NewMockOrchestrator()

	// Save original constructor for orchestrator
	originalNewOrchestrator := orchestratorConstructor

	// Override orchestrator constructor
	orchestratorConstructor = func(apiService interfaces.APIService, contextGatherer interfaces.ContextGatherer, fileWriter interfaces.FileWriter, auditLogger auditlog.AuditLogger, rateLimiter *ratelimit.RateLimiter, config *config.CliConfig, logger logutil.LoggerInterface) Orchestrator {
		return mockOrchestrator
	}

	// Restore original constructor when test finishes
	defer func() {
		orchestratorConstructor = originalNewOrchestrator
	}()

	// Execute the function
	err := Execute(context.Background(), cliConfig, mockLogger, mockAuditLogger, mockAPIService)

	// Verify results
	if err == nil {
		t.Error("Execute did not return an error for API client initialization failure")
	}
	if !strings.Contains(err.Error(), "failed to initialize reference client") {
		t.Errorf("Unexpected error message: %v", err)
	}

	// Verify audit log entries
	executeStartEntry := mockAuditLogger.FindEntry("ExecuteStart")
	if executeStartEntry == nil {
		t.Error("No ExecuteStart entry found in audit log")
		return
	}

	readInstructionsEntry := mockAuditLogger.FindEntry("ReadInstructions")
	if readInstructionsEntry == nil {
		t.Error("No ReadInstructions entry found in audit log")
		return
	}
	if readInstructionsEntry.Status != "Success" {
		t.Errorf("ReadInstructions status was %s, expected Success", readInstructionsEntry.Status)
	}

	executeEndEntry := mockAuditLogger.FindEntry("ExecuteEnd")
	if executeEndEntry == nil {
		t.Error("No ExecuteEnd entry found in audit log")
		return
	}
	if executeEndEntry.Status != "Failure" {
		t.Errorf("ExecuteEnd status was %s, expected Failure", executeEndEntry.Status)
	}
}

// TestExecuteOrchestratorError tests error handling when orchestrator.Run returns an error
func TestExecuteOrchestratorError(t *testing.T) {
	// Set up test environment
	testDir, cleanup := setupTestEnvironment(t)
	defer cleanup()

	// Create instruction file
	instructionsContent := "Test instructions for plan generation"
	instructionsFile := createTestFile(t, filepath.Join(testDir, "instructions.md"), instructionsContent)

	// Set up output directory
	outputDir := filepath.Join(testDir, "output")

	// Create configuration
	cliConfig := &config.CliConfig{
		InstructionsFile: instructionsFile,
		OutputDir:        outputDir,
		ModelNames:       []string{"test-model"},
		APIKey:           "test-api-key",
		Paths:            []string{testDir},
		LogLevel:         logutil.InfoLevel,
	}

	// Create mocks
	mockLogger := NewMockLogger()
	mockAuditLogger := NewMockAuditLogger()
	mockLLMClient := NewMockLLMClient("test-model")
	mockAPIService := NewMockAPIService()
	mockAPIService.mockLLMClient = mockLLMClient
	mockOrchestrator := NewMockOrchestrator()
	mockOrchestrator.runErr = errors.New("orchestrator run error")

	// Save original constructor for orchestrator
	originalNewOrchestrator := orchestratorConstructor

	// Override orchestrator constructor
	orchestratorConstructor = func(apiService interfaces.APIService, contextGatherer interfaces.ContextGatherer, fileWriter interfaces.FileWriter, auditLogger auditlog.AuditLogger, rateLimiter *ratelimit.RateLimiter, config *config.CliConfig, logger logutil.LoggerInterface) Orchestrator {
		return mockOrchestrator
	}

	// Restore original constructor when test finishes
	defer func() {
		orchestratorConstructor = originalNewOrchestrator
	}()

	// Execute the function
	err := Execute(context.Background(), cliConfig, mockLogger, mockAuditLogger, mockAPIService)

	// Verify results
	if err == nil {
		t.Error("Execute did not return an error when orchestrator.Run failed")
	}
	if !strings.Contains(err.Error(), "orchestrator run error") {
		t.Errorf("Unexpected error message: %v", err)
	}

	// Verify audit log entries
	executeStartEntry := mockAuditLogger.FindEntry("ExecuteStart")
	if executeStartEntry == nil {
		t.Error("No ExecuteStart entry found in audit log")
		return
	}

	readInstructionsEntry := mockAuditLogger.FindEntry("ReadInstructions")
	if readInstructionsEntry == nil {
		t.Error("No ReadInstructions entry found in audit log")
		return
	}
	if readInstructionsEntry.Status != "Success" {
		t.Errorf("ReadInstructions status was %s, expected Success", readInstructionsEntry.Status)
	}

	executeEndEntry := mockAuditLogger.FindEntry("ExecuteEnd")
	if executeEndEntry == nil {
		t.Error("No ExecuteEnd entry found in audit log")
		return
	}
	if executeEndEntry.Status != "Failure" {
		t.Errorf("ExecuteEnd status was %s, expected Failure", executeEndEntry.Status)
	}
}

// TestSetupOutputDirectoryError tests error handling when output directory creation fails
func TestSetupOutputDirectoryError(t *testing.T) {
	// Create a temporary test directory
	parentDir, err := os.MkdirTemp("", "architect-test-*")
	if err != nil {
		t.Fatalf("Failed to create test directory: %v", err)
	}
	defer func() { _ = os.RemoveAll(parentDir) }()

	// Create a file with the same name where we will try to create a directory
	invalidDirPath := filepath.Join(parentDir, "cannot-be-dir")
	err = os.WriteFile(invalidDirPath, []byte("this is a file"), 0644)
	if err != nil {
		t.Fatalf("Failed to create test file: %v", err)
	}

	// Create a valid instructions file
	instructionsContent := "Test instructions for plan generation"
	instructionsFile := filepath.Join(parentDir, "instructions.md")
	err = os.WriteFile(instructionsFile, []byte(instructionsContent), 0644)
	if err != nil {
		t.Fatalf("Failed to create instructions file: %v", err)
	}

	// Create configuration with the problematic output directory
	cliConfig := &config.CliConfig{
		InstructionsFile: instructionsFile,
		OutputDir:        filepath.Join(invalidDirPath, "subdir"), // This will fail because parent is a file
		ModelNames:       []string{"test-model"},
		APIKey:           "test-api-key",
		Paths:            []string{parentDir},
		LogLevel:         logutil.InfoLevel,
	}

	// Create mocks
	mockLogger := NewMockLogger()
	mockAuditLogger := NewMockAuditLogger()
	mockLLMClient := NewMockLLMClient("test-model")
	mockAPIService := NewMockAPIService()
	mockAPIService.mockLLMClient = mockLLMClient

	// No need to override any constructors here, since we're passing mockAPIService directly

	// Execute the function (should fail when creating output directory)
	err = Execute(context.Background(), cliConfig, mockLogger, mockAuditLogger, mockAPIService)

	// Verify results
	if err == nil {
		t.Error("Execute did not return an error for output directory creation failure")
	}
	if !strings.Contains(err.Error(), "error creating output directory") {
		t.Errorf("Unexpected error message: %v", err)
	}

	// We're mostly concerned with the error return in this test
	// The specific audit log entries might vary depending on exactly when the error occurs
	// So we don't verify them in detail here
}
