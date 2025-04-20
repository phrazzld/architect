// Package modelproc provides model processing functionality for the architect tool.
// It encapsulates the logic for interacting with AI models, managing tokens,
// writing outputs, and logging operations.
package modelproc

import (
	"context"
	"fmt"
	"path/filepath"
	"strings"
	"time"

	"github.com/phrazzld/architect/internal/auditlog"
	"github.com/phrazzld/architect/internal/config"
	"github.com/phrazzld/architect/internal/llm"
	"github.com/phrazzld/architect/internal/logutil"
	"github.com/phrazzld/architect/internal/registry"
)

// APIService defines the interface for API-related operations
type APIService interface {
	// InitLLMClient initializes and returns a provider-agnostic LLM client
	InitLLMClient(ctx context.Context, apiKey, modelName, apiEndpoint string) (llm.LLMClient, error)

	// GetModelParameters retrieves parameter values from the registry for a given model
	GetModelParameters(modelName string) (map[string]interface{}, error)

	// GetModelDefinition retrieves the full model definition from the registry
	GetModelDefinition(modelName string) (*registry.ModelDefinition, error)

	// GetModelTokenLimits retrieves token limits from the registry for a given model
	GetModelTokenLimits(modelName string) (contextWindow, maxOutputTokens int32, err error)

	// ProcessLLMResponse processes a provider-agnostic response and extracts content
	ProcessLLMResponse(result *llm.ProviderResult) (string, error)

	// IsEmptyResponseError checks if an error is related to empty API responses
	IsEmptyResponseError(err error) bool

	// IsSafetyBlockedError checks if an error is related to safety filters
	IsSafetyBlockedError(err error) bool

	// GetErrorDetails extracts detailed information from an error
	GetErrorDetails(err error) string
}

// FileWriter defines the interface for file output writing
type FileWriter interface {
	// SaveToFile writes content to the specified file
	SaveToFile(content, outputFile string) error
}

// ModelProcessor handles all interactions with AI models including initialization,
// token management, request generation, response processing, and output handling.
type ModelProcessor struct {
	// Dependencies
	apiService  APIService
	fileWriter  FileWriter
	auditLogger auditlog.AuditLogger
	logger      logutil.LoggerInterface
	config      *config.CliConfig
}

// NewProcessor creates a new ModelProcessor with all required dependencies.
// Note: TokenManagers are created per-model in the Process method.
// This is necessary to avoid import cycles and to handle the multi-model architecture.
func NewProcessor(
	apiService APIService,
	fileWriter FileWriter,
	auditLogger auditlog.AuditLogger,
	logger logutil.LoggerInterface,
	config *config.CliConfig,
) *ModelProcessor {
	return &ModelProcessor{
		apiService:  apiService,
		fileWriter:  fileWriter,
		auditLogger: auditLogger,
		logger:      logger,
		config:      config,
	}
}

// Process handles the entire model processing workflow for a single model.
// It implements the logic from the previous processModel/processModelConcurrently functions,
// including initialization, token checking, generation, response processing, and output saving.
func (p *ModelProcessor) Process(ctx context.Context, modelName string, stitchedPrompt string) error {
	p.logger.Info("Processing model: %s", modelName)

	// 1. Initialize model-specific LLM client
	llmClient, err := p.apiService.InitLLMClient(ctx, p.config.APIKey, modelName, p.config.APIEndpoint)
	if err != nil {
		// Use the APIService interface for consistent error detail extraction
		errorDetails := p.apiService.GetErrorDetails(err)
		p.logger.Error("Error creating LLM client for model %s: %s", modelName, errorDetails)
		return fmt.Errorf("failed to initialize API client for model %s: %w", modelName, err)
	}

	// BUGFIX: Ensure llmClient is not nil before attempting to close it
	// CAUSE: There was a race condition in tests where client could be nil
	//        when concurrent tests interact with rate limiting, leading to nil pointer dereference
	// FIX: Add safety check in defer to prevent a panic if client is nil for any reason
	defer func() {
		if llmClient != nil {
			_ = llmClient.Close()
		}
	}()

	// Token validation has been completely removed as part of tasks T032A and T032B
	// We now rely on provider APIs to enforce their own token limits

	// 3. Generate content with this model
	p.logger.Info("Generating output with model %s...", modelName)

	// Log the start of content generation
	generateStartTime := time.Now()
	if logErr := p.auditLogger.Log(auditlog.AuditEntry{
		Timestamp: generateStartTime,
		Operation: "GenerateContentStart",
		Status:    "InProgress",
		Inputs: map[string]interface{}{
			"model_name":    modelName,
			"prompt_length": len(stitchedPrompt),
		},
		Message: "Starting content generation with model " + modelName,
	}); logErr != nil {
		p.logger.Error("Failed to write audit log: %v", logErr)
	}

	// Get model parameters from the APIService
	params, err := p.apiService.GetModelParameters(modelName)
	if err != nil {
		p.logger.Debug("Failed to get model parameters for %s: %v. Using defaults.", modelName, err)
		// Continue with empty parameters if there's an error
		params = make(map[string]interface{})
	}

	// Log parameters being used (at debug level)
	if len(params) > 0 {
		p.logger.Debug("Using model parameters for %s:", modelName)
		for k, v := range params {
			p.logger.Debug("  %s: %v", k, v)
		}
	}

	// Generate content with parameters
	result, err := llmClient.GenerateContent(ctx, stitchedPrompt, params)

	// Calculate duration in milliseconds
	generateDurationMs := time.Since(generateStartTime).Milliseconds()

	if err != nil {
		p.logger.Error("Generation failed for model %s", modelName)

		// Get detailed error information using APIService
		errorDetails := p.apiService.GetErrorDetails(err)
		p.logger.Error("Error generating content with model %s: %s", modelName, errorDetails)

		errorType := "ContentGenerationError"
		errorMessage := fmt.Sprintf("Failed to generate content with model %s: %v", modelName, err)

		// Check if it's a safety-blocked error
		if p.apiService.IsSafetyBlockedError(err) {
			errorType = "SafetyBlockedError"
		} else {
			// Use the new error categorization if available
			if catErr, isCat := llm.IsCategorizedError(err); isCat {
				// Get more specific error category information
				switch catErr.Category() {
				case llm.CategoryRateLimit:
					errorType = "RateLimitError"
					p.logger.Error("Rate limit or quota exceeded. Consider adjusting --max-concurrent and --rate-limit flags.")
				case llm.CategoryAuth:
					errorType = "AuthenticationError"
					p.logger.Error("Authentication failed. Check that your API key is valid and has not expired.")
				case llm.CategoryInputLimit:
					errorType = "InputLimitError"
					p.logger.Error("Input token limit exceeded. Try reducing context with --include/--exclude flags.")
				case llm.CategoryContentFiltered:
					errorType = "ContentFilteredError"
					p.logger.Error("Content was filtered by safety settings. Review and modify your input.")
				case llm.CategoryNetwork:
					errorType = "NetworkError"
					p.logger.Error("Network error occurred. Check your internet connection and try again.")
				case llm.CategoryServer:
					errorType = "ServerError"
					p.logger.Error("Server error occurred. This is typically a temporary issue. Wait and try again.")
				case llm.CategoryCancelled:
					errorType = "CancelledError"
					p.logger.Error("Request was cancelled. Try again with a longer timeout if needed.")
				}
			}
		}

		// Log the content generation failure
		if logErr := p.auditLogger.Log(auditlog.AuditEntry{
			Timestamp:  time.Now().UTC(),
			Operation:  "GenerateContentEnd",
			Status:     "Failure",
			DurationMs: &generateDurationMs,
			Inputs: map[string]interface{}{
				"model_name":    modelName,
				"prompt_length": len(stitchedPrompt),
			},
			Error: &auditlog.ErrorInfo{
				Message: errorMessage,
				Type:    errorType,
			},
			Message: "Content generation failed for model " + modelName,
		}); logErr != nil {
			p.logger.Error("Failed to write audit log: %v", logErr)
		}

		return fmt.Errorf("output generation failed for model %s: %w", modelName, err)
	}

	// Log successful content generation
	if logErr := p.auditLogger.Log(auditlog.AuditEntry{
		Timestamp:  time.Now().UTC(),
		Operation:  "GenerateContentEnd",
		Status:     "Success",
		DurationMs: &generateDurationMs,
		Inputs: map[string]interface{}{
			"model_name":    modelName,
			"prompt_length": len(stitchedPrompt),
		},
		Outputs: map[string]interface{}{
			"finish_reason":      result.FinishReason,
			"has_safety_ratings": len(result.SafetyInfo) > 0,
		},
		Message: "Content generation completed successfully for model " + modelName,
	}); logErr != nil {
		p.logger.Error("Failed to write audit log: %v", logErr)
	}

	// 4. Process API response
	generatedOutput, err := p.apiService.ProcessLLMResponse(result)
	if err != nil {
		// Get detailed error information
		errorDetails := p.apiService.GetErrorDetails(err)

		// Provide specific error messages based on error type
		if p.apiService.IsEmptyResponseError(err) {
			p.logger.Error("Received empty or invalid response from API for model %s", modelName)
			p.logger.Error("Error details: %s", errorDetails)
			return fmt.Errorf("failed to process API response for model %s due to empty content: %w", modelName, err)
		} else if p.apiService.IsSafetyBlockedError(err) {
			p.logger.Error("Content was blocked by safety filters for model %s", modelName)
			p.logger.Error("Error details: %s", errorDetails)
			return fmt.Errorf("failed to process API response for model %s due to safety restrictions: %w", modelName, err)
		} else if catErr, isCat := llm.IsCategorizedError(err); isCat {
			// Use the new error categorization for more specific messages
			switch catErr.Category() {
			case llm.CategoryContentFiltered:
				p.logger.Error("Content was filtered by safety settings for model %s", modelName)
				p.logger.Error("Error details: %s", errorDetails)
				return fmt.Errorf("failed to process API response for model %s due to content filtering: %w", modelName, err)
			case llm.CategoryRateLimit:
				p.logger.Error("Rate limit exceeded while processing response for model %s", modelName)
				p.logger.Error("Error details: %s", errorDetails)
				return fmt.Errorf("failed to process API response for model %s due to rate limiting: %w", modelName, err)
			case llm.CategoryInputLimit:
				p.logger.Error("Input limit exceeded during response processing for model %s", modelName)
				p.logger.Error("Error details: %s", errorDetails)
				return fmt.Errorf("failed to process API response for model %s due to input limits: %w", modelName, err)
			default:
				// Other categorized errors
				p.logger.Error("Error processing response for model %s (%s category)", modelName, catErr.Category())
				p.logger.Error("Error details: %s", errorDetails)
				return fmt.Errorf("failed to process API response for model %s (%s error): %w", modelName, catErr.Category(), err)
			}
		} else {
			// Generic API error handling
			return fmt.Errorf("failed to process API response for model %s: %w", modelName, err)
		}
	}
	contentLength := len(generatedOutput)
	p.logger.Info("Output generated successfully with model %s (content length: %d characters)",
		modelName, contentLength)

	// 5. Sanitize model name for use in filename
	sanitizedModelName := sanitizeFilename(modelName)

	// 6. Construct output file path
	outputFilePath := filepath.Join(p.config.OutputDir, sanitizedModelName+".md")

	// 7. Save the output to file
	if err := p.saveOutputToFile(outputFilePath, generatedOutput); err != nil {
		return fmt.Errorf("failed to save output for model %s: %w", modelName, err)
	}

	p.logger.Info("Successfully processed model: %s", modelName)
	return nil
}

// sanitizeFilename replaces characters that are not valid in filenames
func sanitizeFilename(filename string) string {
	// Replace slashes and other problematic characters with hyphens
	replacer := strings.NewReplacer(
		"/", "-",
		"\\", "-",
		":", "-",
		"*", "-",
		"?", "-",
		"\"", "-",
		"<", "-",
		">", "-",
		"|", "-",
	)
	return replacer.Replace(filename)
}

// saveOutputToFile is a helper method that saves the generated output to a file
// and includes audit logging around the file writing operation.
func (p *ModelProcessor) saveOutputToFile(outputFilePath, content string) error {
	// Log the start of output saving
	saveStartTime := time.Now()
	if logErr := p.auditLogger.Log(auditlog.AuditEntry{
		Timestamp: saveStartTime,
		Operation: "SaveOutputStart",
		Status:    "InProgress",
		Inputs: map[string]interface{}{
			"output_path":    outputFilePath,
			"content_length": len(content),
		},
		Message: "Starting to save output to file",
	}); logErr != nil {
		p.logger.Error("Failed to write audit log: %v", logErr)
	}

	// Save output file
	p.logger.Info("Writing output to %s...", outputFilePath)
	err := p.fileWriter.SaveToFile(content, outputFilePath)

	// Calculate duration in milliseconds
	saveDurationMs := time.Since(saveStartTime).Milliseconds()

	if err != nil {
		// Log failure to save output
		p.logger.Error("Error saving output to file %s: %v", outputFilePath, err)

		if logErr := p.auditLogger.Log(auditlog.AuditEntry{
			Timestamp:  time.Now().UTC(),
			Operation:  "SaveOutputEnd",
			Status:     "Failure",
			DurationMs: &saveDurationMs,
			Inputs: map[string]interface{}{
				"output_path": outputFilePath,
			},
			Error: &auditlog.ErrorInfo{
				Message: fmt.Sprintf("Failed to save output to file: %v", err),
				Type:    "FileIOError",
			},
			Message: "Failed to save output to file",
		}); logErr != nil {
			p.logger.Error("Failed to write audit log: %v", logErr)
		}

		return fmt.Errorf("error saving output to file %s: %w", outputFilePath, err)
	}

	// Log successful saving of output
	if logErr := p.auditLogger.Log(auditlog.AuditEntry{
		Timestamp:  time.Now().UTC(),
		Operation:  "SaveOutputEnd",
		Status:     "Success",
		DurationMs: &saveDurationMs,
		Inputs: map[string]interface{}{
			"output_path": outputFilePath,
		},
		Outputs: map[string]interface{}{
			"content_length": len(content),
		},
		Message: "Successfully saved output to file",
	}); logErr != nil {
		p.logger.Error("Failed to write audit log: %v", logErr)
	}

	p.logger.Info("Output successfully generated and saved to %s", outputFilePath)
	return nil
}
