// Package modelproc provides model processing functionality for the architect tool.
package modelproc

import (
	"context"

	"github.com/phrazzld/architect/internal/auditlog"
	"github.com/phrazzld/architect/internal/llm"
	"github.com/phrazzld/architect/internal/logutil"
	"github.com/phrazzld/architect/internal/registry"
)

// TokenResult is a stub structure that was used for token count information
// It has been retained as a placeholder for compatibility with existing tests
// This will be completely removed in subsequent tasks
type TokenResult struct {
	TokenCount   int32
	InputLimit   int32
	ExceedsLimit bool
	LimitError   string
	Percentage   float64
}

// TokenManager is a stub interface that was used for token management
// It has been retained as a placeholder for compatibility with existing tests
// This will be completely removed in subsequent tasks
type TokenManager interface {
	// GetTokenInfo retrieves token count information and checks limits
	GetTokenInfo(ctx context.Context, prompt string) (*TokenResult, error)

	// CheckTokenLimit verifies the prompt doesn't exceed the model's token limit
	CheckTokenLimit(ctx context.Context, prompt string) error

	// PromptForConfirmation asks for user confirmation to proceed if token count exceeds threshold
	PromptForConfirmation(tokenCount int32, threshold int) bool
}

// NewTokenManagerWithClient is a stub function that returns a no-op TokenManager
// It has been retained as a placeholder for compatibility with existing tests
// This will be completely removed in subsequent tasks
var NewTokenManagerWithClient = func(logger logutil.LoggerInterface, auditLogger auditlog.AuditLogger, client llm.LLMClient, reg *registry.Registry) TokenManager {
	return &noOpTokenManager{
		logger:      logger,
		auditLogger: auditLogger,
		client:      client,
	}
}

// noOpTokenManager is a no-op implementation of TokenManager
// It has been retained as a placeholder for compatibility with existing tests
type noOpTokenManager struct {
	logger      logutil.LoggerInterface
	auditLogger auditlog.AuditLogger
	client      llm.LLMClient
}

// GetTokenInfo always returns a dummy TokenResult with no errors
func (m *noOpTokenManager) GetTokenInfo(ctx context.Context, prompt string) (*TokenResult, error) {
	return &TokenResult{
		TokenCount:   100,
		InputLimit:   1000,
		ExceedsLimit: false,
		Percentage:   10.0,
	}, nil
}

// CheckTokenLimit always returns nil (no error)
func (m *noOpTokenManager) CheckTokenLimit(ctx context.Context, prompt string) error {
	return nil
}

// PromptForConfirmation always returns true (proceed)
func (m *noOpTokenManager) PromptForConfirmation(tokenCount int32, threshold int) bool {
	return true
}
