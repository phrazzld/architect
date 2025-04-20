// Package architect contains the core application logic for the architect tool
package architect

// Define token-related stubs to maintain backward compatibility
// Note: These are being maintained only temporarily as part of the T032 refactoring
// series. They will be removed in a future step.

// TokenResult represents the result of token counting and validation
// This will be removed in a future step, but is kept for now to maintain
// test compatibility
type TokenResult struct {
	TokenCount   int32
	InputLimit   int32
	ExceedsLimit bool
	LimitError   string
	Percentage   float64
}
