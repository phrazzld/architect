// Package architect contains the core application logic for the architect tool
package architect

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"time"

	"github.com/phrazzld/architect/internal/architect/interfaces"
	"github.com/phrazzld/architect/internal/architect/orchestrator"
	"github.com/phrazzld/architect/internal/auditlog"
	"github.com/phrazzld/architect/internal/config"
	"github.com/phrazzld/architect/internal/gemini"
	"github.com/phrazzld/architect/internal/llm"
	"github.com/phrazzld/architect/internal/logutil"
	"github.com/phrazzld/architect/internal/ratelimit"
	"github.com/phrazzld/architect/internal/runutil"
)

// Execute is the main entry point for the core application logic.
// It handles initial setup, logging, dependency initialization, and orchestration.
func Execute(
	ctx context.Context,
	cliConfig *config.CliConfig,
	logger logutil.LoggerInterface,
	auditLogger auditlog.AuditLogger,
	apiService APIService,
) (err error) {
	// Use a deferred function to ensure ExecuteEnd is always logged
	defer func() {
		status := "Success"
		var errorInfo *auditlog.ErrorInfo
		if err != nil {
			status = "Failure"
			errorInfo = &auditlog.ErrorInfo{
				Message: err.Error(),
				Type:    "ExecutionError",
			}

			// Check if it's a categorized error for more detailed information
			if catErr, ok := llm.IsCategorizedError(err); ok {
				category := catErr.Category()
				// Update the error type to include the category
				errorInfo.Type = fmt.Sprintf("ExecutionError:%s", category.String())

				// For certain error categories, add more context to the message
				switch category {
				case llm.CategoryAuth:
					errorInfo.Message = "Authentication failed. Check your API key."
				case llm.CategoryRateLimit:
					errorInfo.Message = "Rate limit exceeded. Try again later or adjust rate limiting parameters."
				case llm.CategoryInputLimit:
					errorInfo.Message = "Input token limit exceeded. Try reducing the context size."
				case llm.CategoryContentFiltered:
					errorInfo.Message = "Content was filtered by safety settings."
				}
			}
		}

		if logErr := auditLogger.Log(auditlog.AuditEntry{
			Timestamp: time.Now().UTC(),
			Operation: "ExecuteEnd",
			Status:    status,
			Error:     errorInfo,
			Message:   fmt.Sprintf("Execution completed with status: %s", status),
		}); logErr != nil {
			logger.Error("Failed to write audit log: %v", logErr)
		}
	}()

	// 1. Set up the output directory
	if err := setupOutputDirectory(cliConfig, logger); err != nil {
		return err
	}

	// 2. Log the start of the Execute operation
	inputs := map[string]interface{}{
		"instructions_file": cliConfig.InstructionsFile,
		"output_dir":        cliConfig.OutputDir,
		"audit_log_file":    cliConfig.AuditLogFile,
		"format":            cliConfig.Format,
		"paths_count":       len(cliConfig.Paths),
		"include":           cliConfig.Include,
		"exclude":           cliConfig.Exclude,
		"exclude_names":     cliConfig.ExcludeNames,
		"dry_run":           cliConfig.DryRun,
		"verbose":           cliConfig.Verbose,
		"model_names":       cliConfig.ModelNames,
		"confirm_tokens":    cliConfig.ConfirmTokens,
		"log_level":         cliConfig.LogLevel,
	}

	if err := auditLogger.Log(auditlog.AuditEntry{
		Timestamp: time.Now().UTC(),
		Operation: "ExecuteStart",
		Status:    "InProgress",
		Inputs:    inputs,
		Message:   "Starting execution of architect tool",
	}); err != nil {
		logger.Error("Failed to write audit log: %v", err)
	}

	// 3. Read instructions from file
	instructionsContent, err := os.ReadFile(cliConfig.InstructionsFile)
	if err != nil {
		logger.Error("Failed to read instructions file %s: %v", cliConfig.InstructionsFile, err)

		// Log the failure to read the instructions file to the audit log
		if logErr := auditLogger.Log(auditlog.AuditEntry{
			Timestamp: time.Now().UTC(),
			Operation: "ReadInstructions",
			Status:    "Failure",
			Inputs: map[string]interface{}{
				"path": cliConfig.InstructionsFile,
			},
			Error: &auditlog.ErrorInfo{
				Message: fmt.Sprintf("Failed to read instructions file: %v", err),
				Type:    "FileIOError",
			},
		}); logErr != nil {
			logger.Error("Failed to write audit log: %v", logErr)
		}

		return fmt.Errorf("failed to read instructions file %s: %w", cliConfig.InstructionsFile, err)
	}
	instructions := string(instructionsContent)
	logger.Info("Successfully read instructions from %s", cliConfig.InstructionsFile)

	// Log the successful reading of the instructions file to the audit log
	if logErr := auditLogger.Log(auditlog.AuditEntry{
		Timestamp: time.Now().UTC(),
		Operation: "ReadInstructions",
		Status:    "Success",
		Inputs: map[string]interface{}{
			"path": cliConfig.InstructionsFile,
		},
		Outputs: map[string]interface{}{
			"content_length": len(instructions),
		},
		Message: "Successfully read instructions file",
	}); logErr != nil {
		logger.Error("Failed to write audit log: %v", logErr)
	}

	// 4. Use the injected APIService

	// Create a reference client for token counting in context gathering
	referenceClientLLM, err := apiService.InitLLMClient(ctx, cliConfig.APIKey, cliConfig.ModelNames[0], cliConfig.APIEndpoint)
	if err != nil {
		// Check if this is a categorized error to provide better error messages
		if catErr, ok := llm.IsCategorizedError(err); ok {
			category := catErr.Category()

			// Log with category information
			logger.Error("Failed to initialize reference client for context gathering: %v (category: %s)",
				err, category.String())

			// Use error category to give more specific error messages
			switch category {
			case llm.CategoryAuth:
				return fmt.Errorf("API authentication failed for model %s: %w", cliConfig.ModelNames[0], err)
			case llm.CategoryRateLimit:
				return fmt.Errorf("API rate limit exceeded for model %s: %w", cliConfig.ModelNames[0], err)
			case llm.CategoryNotFound:
				return fmt.Errorf("model %s not found or not available: %w", cliConfig.ModelNames[0], err)
			case llm.CategoryInputLimit:
				return fmt.Errorf("input token limit exceeded for model %s: %w", cliConfig.ModelNames[0], err)
			case llm.CategoryContentFiltered:
				return fmt.Errorf("content was filtered by safety settings: %w", err)
			default:
				return fmt.Errorf("failed to initialize reference client for model %s: %w", cliConfig.ModelNames[0], err)
			}
		} else {
			// If not a categorized error, use the standard error handling
			logger.Error("Failed to initialize reference client for context gathering: %v", err)
			return fmt.Errorf("failed to initialize reference client for context gathering: %w", err)
		}
	}
	defer func() { _ = referenceClientLLM.Close() }()

	// Create TokenManager with the LLM client reference
	tokenManager, tokenManagerErr := NewTokenManager(logger, auditLogger, referenceClientLLM)
	if tokenManagerErr != nil {
		// Check if this is a categorized error for better error messages
		if catErr, ok := llm.IsCategorizedError(tokenManagerErr); ok {
			category := catErr.Category()
			logger.Error("Failed to create token manager: %v (category: %s)",
				tokenManagerErr, category.String())

			// For token manager, the most likely issues are related to model info
			if category == llm.CategoryNotFound {
				return fmt.Errorf("token counting unavailable for model %s: %w", cliConfig.ModelNames[0], tokenManagerErr)
			}
		} else {
			logger.Error("Failed to create token manager: %v", tokenManagerErr)
		}
		return fmt.Errorf("failed to create token manager: %w", tokenManagerErr)
	}

	// Create an adapter to convert LLMClient to gemini.Client
	// This is temporary until ContextGatherer is updated to use LLMClient directly
	adapter := &llmToGeminiAdapter{
		referenceClientLLM: referenceClientLLM,
	}

	contextGatherer := NewContextGatherer(logger, cliConfig.DryRun, tokenManager, adapter, auditLogger)
	fileWriter := NewFileWriter(logger, auditLogger)

	// Create rate limiter from configuration
	rateLimiter := ratelimit.NewRateLimiter(
		cliConfig.MaxConcurrentRequests,
		cliConfig.RateLimitRequestsPerMinute,
	)

	// 5. Create and run the orchestrator
	// Create adapters for the interfaces
	apiServiceAdapter := &APIServiceAdapter{APIService: apiService}
	tokenManagerAdapter := &TokenManagerAdapter{TokenManager: tokenManager}
	contextGathererAdapter := &ContextGathererAdapter{ContextGatherer: contextGatherer}
	fileWriterAdapter := &FileWriterAdapter{FileWriter: fileWriter}

	orch := orchestratorConstructor(
		apiServiceAdapter,
		contextGathererAdapter,
		tokenManagerAdapter,
		fileWriterAdapter,
		auditLogger,
		rateLimiter,
		cliConfig,
		logger,
	)

	return orch.Run(ctx, instructions)
}

// Note: RunInternal has been removed as part of the refactoring.
// The Execute function now properly handles dependency injection and can be
// used directly for testing by providing appropriate mocks.

// Note: processModel, processModelConcurrently, sanitizeFilename, and saveOutputToFile functions
// have been removed as part of the refactoring. Their functionality has been moved to the
// ModelProcessor in the modelproc package.

// Orchestrator defines the interface for the orchestration component.
// This interface is defined here to allow for testing without introducing import cycles.
type Orchestrator interface {
	Run(ctx context.Context, instructions string) error
}

// llmToGeminiAdapter is a temporary adapter between LLMClient and gemini.Client interfaces
// This is needed until ContextGatherer is refactored to use LLMClient directly
type llmToGeminiAdapter struct {
	referenceClientLLM llm.LLMClient
}

func (m *llmToGeminiAdapter) GenerateContent(ctx context.Context, prompt string) (*gemini.GenerationResult, error) {
	result, err := m.referenceClientLLM.GenerateContent(ctx, prompt)
	if err != nil {
		return nil, err
	}

	// Convert from llm.ProviderResult to gemini.GenerationResult
	return &gemini.GenerationResult{
		Content:      result.Content,
		FinishReason: result.FinishReason,
		TokenCount:   result.TokenCount,
		Truncated:    result.Truncated,
	}, nil
}

func (m *llmToGeminiAdapter) CountTokens(ctx context.Context, prompt string) (*gemini.TokenCount, error) {
	result, err := m.referenceClientLLM.CountTokens(ctx, prompt)
	if err != nil {
		return nil, err
	}

	return &gemini.TokenCount{
		Total: result.Total,
	}, nil
}

func (m *llmToGeminiAdapter) GetModelInfo(ctx context.Context) (*gemini.ModelInfo, error) {
	result, err := m.referenceClientLLM.GetModelInfo(ctx)
	if err != nil {
		return nil, err
	}

	return &gemini.ModelInfo{
		Name:             result.Name,
		InputTokenLimit:  result.InputTokenLimit,
		OutputTokenLimit: result.OutputTokenLimit,
	}, nil
}

func (m *llmToGeminiAdapter) GetModelName() string {
	return m.referenceClientLLM.GetModelName()
}

func (m *llmToGeminiAdapter) GetTemperature() float32 {
	return 0.7 // Default value
}

func (m *llmToGeminiAdapter) GetMaxOutputTokens() int32 {
	return 1024 // Default value
}

func (m *llmToGeminiAdapter) GetTopP() float32 {
	return 0.95 // Default value
}

func (m *llmToGeminiAdapter) Close() error {
	return nil // Don't close the underlying client since that's handled elsewhere
}

// orchestratorConstructor is the function used to create an Orchestrator.
// This can be overridden in tests to return a mock orchestrator.
var orchestratorConstructor = func(
	apiService APIService,
	contextGatherer interfaces.ContextGatherer,
	tokenManager interfaces.TokenManager,
	fileWriter interfaces.FileWriter,
	auditLogger auditlog.AuditLogger,
	rateLimiter *ratelimit.RateLimiter,
	config *config.CliConfig,
	logger logutil.LoggerInterface,
) Orchestrator {
	return orchestrator.NewOrchestrator(
		apiService,
		contextGatherer,
		tokenManager,
		fileWriter,
		auditLogger,
		rateLimiter,
		config,
		logger,
	)
}

// setupOutputDirectory ensures that the output directory is set and exists.
// If outputDir in cliConfig is empty, it generates a unique directory name.
func setupOutputDirectory(cliConfig *config.CliConfig, logger logutil.LoggerInterface) error {
	if cliConfig.OutputDir == "" {
		// Generate a unique run name (e.g., "curious-panther")
		runName := runutil.GenerateRunName()
		// Get the current working directory
		cwd, err := os.Getwd()
		if err != nil {
			logger.Error("Error getting current working directory: %v", err)
			return fmt.Errorf("error getting current working directory: %w", err)
		}
		// Set the output directory to the run name in the current working directory
		cliConfig.OutputDir = filepath.Join(cwd, runName)
		logger.Info("Generated output directory: %s", cliConfig.OutputDir)
	}

	// Ensure the output directory exists
	if err := os.MkdirAll(cliConfig.OutputDir, 0755); err != nil {
		logger.Error("Error creating output directory %s: %v", cliConfig.OutputDir, err)
		return fmt.Errorf("error creating output directory %s: %w", cliConfig.OutputDir, err)
	}

	logger.Info("Using output directory: %s", cliConfig.OutputDir)
	return nil
}
