// Package interfaces provides shared interfaces used across the architect tool.
// It helps prevent import cycles between packages that depend on each other.
package interfaces

import (
	"context"

	"github.com/phrazzld/architect/internal/auditlog"
	"github.com/phrazzld/architect/internal/fileutil"
	"github.com/phrazzld/architect/internal/llm"
	"github.com/phrazzld/architect/internal/logutil"
	"github.com/phrazzld/architect/internal/registry"
)

// APIService defines the interface for API-related operations
// It provides methods for initializing LLM clients, processing responses,
// handling errors, and accessing model configuration from the registry.
type APIService interface {
	// InitLLMClient initializes and returns a provider-agnostic LLM client
	// Parameters:
	//   - ctx: The context for client initialization
	//   - apiKey: The API key for authentication with the provider
	//   - modelName: The name of the model to use
	//   - apiEndpoint: Optional custom API endpoint (if empty, default is used)
	// Returns:
	//   - An initialized LLM client
	//   - An error if initialization fails
	InitLLMClient(ctx context.Context, apiKey, modelName, apiEndpoint string) (llm.LLMClient, error)

	// GetModelParameters retrieves parameter values from the registry for a given model
	// It returns a map of parameter name to parameter value, applying defaults from the model definition
	// Parameters:
	//   - modelName: The name of the model to get parameters for
	// Returns:
	//   - A map of parameter name to parameter value
	//   - An error if retrieval fails
	GetModelParameters(modelName string) (map[string]interface{}, error)

	// ValidateModelParameter validates a parameter value against its constraints
	// It checks type validation and constraint validation (min/max for numeric, enum values for string)
	// Parameters:
	//   - modelName: The name of the model the parameter belongs to
	//   - paramName: The name of the parameter to validate
	//   - value: The value to validate
	// Returns:
	//   - true if the parameter is valid, false otherwise
	//   - An error with details about why validation failed
	ValidateModelParameter(modelName, paramName string, value interface{}) (bool, error)

	// GetModelDefinition retrieves the full model definition from the registry
	// Parameters:
	//   - modelName: The name of the model to get the definition for
	// Returns:
	//   - The model definition
	//   - An error if retrieval fails
	GetModelDefinition(modelName string) (*registry.ModelDefinition, error)

	// GetModelTokenLimits retrieves token limits from the registry for a given model
	// Parameters:
	//   - modelName: The name of the model to get token limits for
	// Returns:
	//   - contextWindow: The maximum number of tokens the model can accept as input
	//   - maxOutputTokens: The maximum number of tokens the model can generate as output
	//   - An error if retrieval fails
	GetModelTokenLimits(modelName string) (contextWindow, maxOutputTokens int32, err error)

	// ProcessLLMResponse processes a provider-agnostic response and extracts content
	// Parameters:
	//   - result: The provider result to process
	// Returns:
	//   - The processed content
	//   - An error if processing fails
	ProcessLLMResponse(result *llm.ProviderResult) (string, error)

	// IsEmptyResponseError checks if an error is related to empty API responses
	// Parameters:
	//   - err: The error to check
	// Returns:
	//   - true if the error is related to empty API responses, false otherwise
	IsEmptyResponseError(err error) bool

	// IsSafetyBlockedError checks if an error is related to safety filters
	// Parameters:
	//   - err: The error to check
	// Returns:
	//   - true if the error is related to safety filters, false otherwise
	IsSafetyBlockedError(err error) bool

	// GetErrorDetails extracts detailed information from an error
	// Parameters:
	//   - err: The error to get details for
	// Returns:
	//   - A user-friendly error message with details
	GetErrorDetails(err error) string
}

// ContextStats holds information about processed files and context size
type ContextStats struct {
	ProcessedFilesCount int
	CharCount           int
	LineCount           int
	// TokenCount field removed as part of T032F - token handling refactoring
	ProcessedFiles []string
}

// GatherConfig holds parameters needed for gathering context
type GatherConfig struct {
	Paths        []string
	Include      string
	Exclude      string
	ExcludeNames string
	Format       string
	Verbose      bool
	LogLevel     logutil.LogLevel
}

// ContextGatherer defines the interface for gathering project context
type ContextGatherer interface {
	// GatherContext collects and processes files based on configuration
	GatherContext(ctx context.Context, config GatherConfig) ([]fileutil.FileMeta, *ContextStats, error)

	// DisplayDryRunInfo shows detailed information for dry run mode
	DisplayDryRunInfo(ctx context.Context, stats *ContextStats) error
}

// FileWriter defines the interface for file output writing
type FileWriter interface {
	// SaveToFile writes content to the specified file
	SaveToFile(content, outputFile string) error
}

// AuditLogger defines the interface for writing audit logs
type AuditLogger interface {
	// Log writes an audit entry to the log
	Log(entry auditlog.AuditEntry) error

	// Close closes the audit logger
	Close() error
}
