// Package architect provides the command-line interface for the architect tool
package architect

import (
	"github.com/phrazzld/architect/internal/architect"
)

// Re-export error types from internal/architect for backward compatibility with tests
var (
	ErrEmptyResponse        = architect.ErrEmptyResponse
	ErrWhitespaceContent    = architect.ErrWhitespaceContent
	ErrSafetyBlocked        = architect.ErrSafetyBlocked
	ErrAPICall              = architect.ErrAPICall
	ErrClientInitialization = architect.ErrClientInitialization
)

// APIService is an alias to the internal one
type APIService = architect.APIService

// NewAPIService is a wrapper for the internal one
var NewAPIService = architect.NewAPIService
