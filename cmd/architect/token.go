// Package architect provides the command-line interface for the architect tool
package architect

import (
	"bufio"
	"context"
	"fmt"
	"os"
	"strings"

	"github.com/phrazzld/architect/internal/gemini"
	"github.com/phrazzld/architect/internal/logutil"
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
	logger logutil.LoggerInterface
	client gemini.Client
}

// NewTokenManager creates a new TokenManager instance
func NewTokenManager(logger logutil.LoggerInterface, client gemini.Client) (TokenManager, error) {
	if client == nil {
		return nil, fmt.Errorf("client cannot be nil for TokenManager")
	}
	return &tokenManager{
		logger: logger,
		client: client,
	}, nil
}

// GetTokenInfo retrieves token count information and checks limits
func (tm *tokenManager) GetTokenInfo(ctx context.Context, prompt string) (*TokenResult, error) {
	// Create result structure
	result := &TokenResult{
		ExceedsLimit: false,
	}

	// Get model information (limits)
	modelInfo, err := tm.client.GetModelInfo(ctx)
	if err != nil {
		// Pass through API errors directly for better error messages
		if apiErr, ok := gemini.IsGeminiError(err); ok {
			return nil, apiErr
		}

		// Wrap other errors
		return nil, fmt.Errorf("failed to get model info for token limit check: %w", err)
	}

	// Store input limit
	result.InputLimit = modelInfo.InputTokenLimit

	// Count tokens in the prompt
	tokenResult, err := tm.client.CountTokens(ctx, prompt)
	if err != nil {
		// Pass through API errors directly for better error messages
		if apiErr, ok := gemini.IsGeminiError(err); ok {
			return nil, apiErr
		}

		// Wrap other errors
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
		return fmt.Errorf(tokenInfo.LimitError)
	}

	return nil
}

// PromptForConfirmation asks for user confirmation to proceed
func (tm *tokenManager) PromptForConfirmation(tokenCount int32, threshold int) bool {
	if threshold <= 0 || int32(threshold) > tokenCount {
		// No confirmation needed if threshold is disabled (0) or token count is below threshold
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

	// Trim whitespace and convert to lowercase
	response = strings.ToLower(strings.TrimSpace(response))

	// Only proceed if the user explicitly confirms with 'y' or 'yes'
	return response == "y" || response == "yes"
}
