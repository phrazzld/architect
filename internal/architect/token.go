// Package architect contains the core application logic for the architect tool
package architect

import (
	"bufio"
	"context"
	"fmt"
	"os"
	"strings"
	"time"

	"github.com/phrazzld/architect/internal/auditlog"
	"github.com/phrazzld/architect/internal/llm"
	"github.com/phrazzld/architect/internal/logutil"
	"github.com/phrazzld/architect/internal/registry"
)

// TokenResult holds information about token counts and limits
type TokenResult struct {
	TokenCount   int32
	InputLimit   int32
	ExceedsLimit bool
	LimitError   string
	Percentage   float64
}

// TokenManager defines the interface for token counting and management
type TokenManager interface {
	// GetTokenInfo retrieves token count information and checks limits
	GetTokenInfo(ctx context.Context, prompt string) (*TokenResult, error)

	// CheckTokenLimit verifies the prompt doesn't exceed the model's token limit
	CheckTokenLimit(ctx context.Context, prompt string) error

	// PromptForConfirmation asks for user confirmation to proceed if token count exceeds threshold
	PromptForConfirmation(tokenCount int32, threshold int) bool
}

// tokenManager implements the TokenManager interface
type tokenManager struct {
	logger      logutil.LoggerInterface
	auditLogger auditlog.AuditLogger
	client      llm.LLMClient
	registry    *registry.Registry // Registry for model configurations
}

// NewTokenManager creates a new TokenManager instance
func NewTokenManager(logger logutil.LoggerInterface, auditLogger auditlog.AuditLogger, client llm.LLMClient, reg *registry.Registry) (TokenManager, error) {
	if client == nil {
		return nil, fmt.Errorf("client cannot be nil for TokenManager")
	}
	return NewTokenManagerWithClient(logger, auditLogger, client, reg), nil
}

// NewTokenManagerWithClient creates a TokenManager with a specific client.
// This is defined as a variable to allow it to be mocked in tests.
var NewTokenManagerWithClient = func(logger logutil.LoggerInterface, auditLogger auditlog.AuditLogger, client llm.LLMClient, reg *registry.Registry) TokenManager {
	return &tokenManager{
		logger:      logger,
		auditLogger: auditLogger,
		client:      client,
		registry:    reg,
	}
}

// GetTokenInfo retrieves token count information and checks limits
func (tm *tokenManager) GetTokenInfo(ctx context.Context, prompt string) (*TokenResult, error) {
	// Get the model name from the injected client
	modelName := tm.client.GetModelName()

	// Log the start of token checking
	checkStartTime := time.Now()
	if logErr := tm.auditLogger.Log(auditlog.AuditEntry{
		Timestamp: checkStartTime,
		Operation: "CheckTokensStart",
		Status:    "InProgress",
		Inputs: map[string]interface{}{
			"prompt_length": len(prompt),
			"model_name":    modelName,
		},
		Message: "Starting token count check for model " + modelName,
	}); logErr != nil {
		tm.logger.Error("Failed to write audit log: %v", logErr)
	}

	// Create result structure
	result := &TokenResult{
		ExceedsLimit: false,
	}

	// First, try to get model info from the registry if available
	var tokenSource = "client"
	var registryAttempted bool

	if tm.registry != nil {
		registryAttempted = true
		// Try to get model definition from registry
		modelDef, err := tm.registry.GetModel(modelName)
		if err == nil && modelDef != nil && modelDef.ContextWindow > 0 {
			// We found the model in the registry with a valid context window
			tm.logger.Info("Using token limits from registry for model %s: %d tokens (registry values take precedence)",
				modelName, modelDef.ContextWindow)

			// Store input limit from registry
			result.InputLimit = modelDef.ContextWindow
			tokenSource = "registry"
		} else {
			if err != nil {
				tm.logger.Debug("Model %s lookup in registry failed: %v", modelName, err)
			} else if modelDef == nil {
				tm.logger.Debug("Model %s found in registry but model definition is nil", modelName)
			} else if modelDef.ContextWindow <= 0 {
				tm.logger.Debug("Model %s found in registry but has invalid context window: %d",
					modelName, modelDef.ContextWindow)
			}
			tm.logger.Info("Model %s not properly configured in registry, falling back to client-provided token limits",
				modelName)
		}
	}

	// If registry lookup failed or registry is not available, fall back to client GetModelInfo
	if tokenSource != "registry" {
		// Get model information (limits) from LLM client
		modelInfo, err := tm.client.GetModelInfo(ctx)
		if err != nil {
			// Handle provider-agnostic error logging
			errorType := "TokenCheckError"
			// Using a separate variable declaration to avoid ineffectual assignment
			var errorMessage string

			// Get specific error details if we can recognize provider-specific errors
			// Note: This approach allows us to handle Gemini or other provider-specific errors
			// without direct dependency on provider-specific error types
			if apiService, ok := tm.client.(interface {
				IsSafetyBlockedError(err error) bool
				GetErrorDetails(err error) string
			}); ok && apiService != nil {
				if apiService.IsSafetyBlockedError(err) {
					errorType = "SafetyBlockedError"
				}
				errorMessage = apiService.GetErrorDetails(err)
			} else {
				// Just use the error message directly
				errorMessage = err.Error()
			}

			// Log the token check failure with additional context about registry attempt
			var registryAttemptInfo string
			if registryAttempted {
				registryAttemptInfo = " after registry lookup failed"
			}

			if logErr := tm.auditLogger.Log(auditlog.AuditEntry{
				Timestamp: time.Now().UTC(),
				Operation: "CheckTokens",
				Status:    "Failure",
				Inputs: map[string]interface{}{
					"prompt_length":      len(prompt),
					"model_name":         modelName,
					"registry_attempted": registryAttempted,
				},
				Error: &auditlog.ErrorInfo{
					Message: errorMessage,
					Type:    errorType,
				},
				Message: "Token count check failed for model " + modelName + registryAttemptInfo,
			}); logErr != nil {
				tm.logger.Error("Failed to write audit log: %v", logErr)
			}

			// Return the original error
			return nil, fmt.Errorf("failed to get model info for token limit check: %w", err)
		}

		// Store input limit from model info
		result.InputLimit = modelInfo.InputTokenLimit
		tm.logger.Info("Using token limits from client for model %s: %d tokens",
			modelName, modelInfo.InputTokenLimit)
	}

	// Count tokens in the prompt
	tokenResult, err := tm.client.CountTokens(ctx, prompt)
	if err != nil {
		// Handle provider-agnostic error logging
		errorType := "TokenCheckError"
		// Using a separate variable declaration to avoid ineffectual assignment
		var errorMessage string

		// Get specific error details if we can recognize provider-specific errors
		if apiService, ok := tm.client.(interface {
			IsSafetyBlockedError(err error) bool
			GetErrorDetails(err error) string
		}); ok && apiService != nil {
			if apiService.IsSafetyBlockedError(err) {
				errorType = "SafetyBlockedError"
			}
			errorMessage = apiService.GetErrorDetails(err)
		} else {
			// Just use the error message directly
			errorMessage = err.Error()
		}

		// Log the token check failure
		if logErr := tm.auditLogger.Log(auditlog.AuditEntry{
			Timestamp: time.Now().UTC(),
			Operation: "CheckTokens",
			Status:    "Failure",
			Inputs: map[string]interface{}{
				"prompt_length": len(prompt),
				"model_name":    modelName,
			},
			Error: &auditlog.ErrorInfo{
				Message: errorMessage,
				Type:    errorType,
			},
			Message: "Token count check failed for model " + modelName,
		}); logErr != nil {
			tm.logger.Error("Failed to write audit log: %v", logErr)
		}

		// Return the original error
		return nil, fmt.Errorf("failed to count tokens for token limit check: %w", err)
	}

	// Store token count
	result.TokenCount = tokenResult.Total

	// Calculate percentage of limit
	result.Percentage = float64(result.TokenCount) / float64(result.InputLimit) * 100

	// Log token usage information
	tm.logger.Debug("Token usage: %d / %d (%.1f%%)",
		result.TokenCount,
		result.InputLimit,
		result.Percentage)

	// Check if the prompt exceeds the token limit
	if result.TokenCount > result.InputLimit {
		result.ExceedsLimit = true
		result.LimitError = fmt.Sprintf("prompt exceeds token limit (%d tokens > %d token limit)",
			result.TokenCount, result.InputLimit)

		// Log the token limit exceeded case with token source info
		if logErr := tm.auditLogger.Log(auditlog.AuditEntry{
			Timestamp: time.Now().UTC(),
			Operation: "CheckTokens",
			Status:    "Failure",
			Inputs: map[string]interface{}{
				"prompt_length":      len(prompt),
				"model_name":         modelName,
				"registry_attempted": registryAttempted,
			},
			Outputs: map[string]interface{}{
				"token_source": tokenSource,
			},
			TokenCounts: &auditlog.TokenCountInfo{
				PromptTokens: result.TokenCount,
				TotalTokens:  result.TokenCount,
				Limit:        result.InputLimit,
			},
			Error: &auditlog.ErrorInfo{
				Message: result.LimitError,
				Type:    "TokenLimitExceededError",
			},
			Message: fmt.Sprintf("Token limit exceeded for model %s (using %s token limits)",
				modelName, tokenSource),
		}); logErr != nil {
			tm.logger.Error("Failed to write audit log: %v", logErr)
		}
	} else {
		// Log the successful token check with token source info
		if logErr := tm.auditLogger.Log(auditlog.AuditEntry{
			Timestamp: time.Now().UTC(),
			Operation: "CheckTokens",
			Status:    "Success",
			Inputs: map[string]interface{}{
				"prompt_length":      len(prompt),
				"model_name":         modelName,
				"registry_attempted": registryAttempted,
			},
			Outputs: map[string]interface{}{
				"percentage":   result.Percentage,
				"token_source": tokenSource,
			},
			TokenCounts: &auditlog.TokenCountInfo{
				PromptTokens: result.TokenCount,
				TotalTokens:  result.TokenCount,
				Limit:        result.InputLimit,
			},
			Message: fmt.Sprintf("Token check passed for model %s: %d / %d tokens (%.1f%% of limit, using %s token limits)",
				modelName, result.TokenCount, result.InputLimit, result.Percentage, tokenSource),
		}); logErr != nil {
			tm.logger.Error("Failed to write audit log: %v", logErr)
		}
	}

	return result, nil
}

// CheckTokenLimit verifies the prompt doesn't exceed the model's token limit
func (tm *tokenManager) CheckTokenLimit(ctx context.Context, prompt string) error {
	tokenInfo, err := tm.GetTokenInfo(ctx, prompt)
	if err != nil {
		return err
	}

	if tokenInfo.ExceedsLimit {
		return fmt.Errorf("%s", tokenInfo.LimitError)
	}

	return nil
}

// PromptForConfirmation asks for user confirmation to proceed
func (tm *tokenManager) PromptForConfirmation(tokenCount int32, threshold int) bool {
	if threshold <= 0 || int32(threshold) > tokenCount {
		// No confirmation needed if threshold is disabled (0) or token count is below threshold
		tm.logger.Debug("No confirmation needed: threshold=%d, tokenCount=%d", threshold, tokenCount)
		return true
	}

	tm.logger.Info("Token count (%d) exceeds confirmation threshold (%d).", tokenCount, threshold)
	tm.logger.Info("Do you want to proceed with the API call? [y/N]: ")

	reader := bufio.NewReader(os.Stdin)
	response, err := reader.ReadString('\n')
	if err != nil {
		tm.logger.Error("Error reading input: %v", err)
		return false
	}

	// Log the raw response for debugging
	tm.logger.Debug("User confirmation response (raw): %q", response)

	// Trim whitespace and convert to lowercase
	response = strings.ToLower(strings.TrimSpace(response))
	tm.logger.Debug("User confirmation response (processed): %q", response)

	// Only proceed if the user explicitly confirms with 'y' or 'yes'
	result := response == "y" || response == "yes"
	tm.logger.Debug("User confirmation result: %v", result)
	return result
}
