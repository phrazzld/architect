// Package architect provides core functionality for the Architect application
package architect

import (
	"context"
	"fmt"
	"strings"

	"github.com/phrazzld/architect/internal/architect/interfaces"
	"github.com/phrazzld/architect/internal/fileutil"
	"github.com/phrazzld/architect/internal/llm"
	"github.com/phrazzld/architect/internal/registry"
)

// APIServiceAdapter provides an adapter for different APIService implementations
// It allows using different implementations of APIService while maintaining
// backward compatibility with older code.
type APIServiceAdapter struct {
	// The underlying APIService implementation
	APIService interfaces.APIService
}

// InitLLMClient delegates to the underlying APIService implementation
func (a *APIServiceAdapter) InitLLMClient(ctx context.Context, apiKey, modelName, apiEndpoint string) (llm.LLMClient, error) {
	return a.APIService.InitLLMClient(ctx, apiKey, modelName, apiEndpoint)
}

// ProcessLLMResponse delegates to the underlying APIService implementation
func (a *APIServiceAdapter) ProcessLLMResponse(result *llm.ProviderResult) (string, error) {
	return a.APIService.ProcessLLMResponse(result)
}

// GetErrorDetails delegates to the underlying APIService implementation
func (a *APIServiceAdapter) GetErrorDetails(err error) string {
	return a.APIService.GetErrorDetails(err)
}

// IsEmptyResponseError delegates to the underlying APIService implementation
func (a *APIServiceAdapter) IsEmptyResponseError(err error) bool {
	return a.APIService.IsEmptyResponseError(err)
}

// IsSafetyBlockedError delegates to the underlying APIService implementation
func (a *APIServiceAdapter) IsSafetyBlockedError(err error) bool {
	return a.APIService.IsSafetyBlockedError(err)
}

// GetModelParameters delegates to the underlying APIService implementation
func (a *APIServiceAdapter) GetModelParameters(modelName string) (map[string]interface{}, error) {
	// Check if the underlying implementation supports this method
	if apiService, ok := a.APIService.(interface {
		GetModelParameters(string) (map[string]interface{}, error)
	}); ok {
		return apiService.GetModelParameters(modelName)
	}
	// Return empty map for implementations that don't support this method
	return make(map[string]interface{}), nil
}

// GetModelDefinition delegates to the underlying APIService implementation
func (a *APIServiceAdapter) GetModelDefinition(modelName string) (*registry.ModelDefinition, error) {
	// Check if the underlying implementation supports this method
	if apiService, ok := a.APIService.(interface {
		GetModelDefinition(string) (*registry.ModelDefinition, error)
	}); ok {
		return apiService.GetModelDefinition(modelName)
	}
	// Return error for implementations that don't support this method
	return nil, fmt.Errorf("model definition not available")
}

// GetModelTokenLimits delegates to the underlying APIService implementation
func (a *APIServiceAdapter) GetModelTokenLimits(modelName string) (contextWindow, maxOutputTokens int32, err error) {
	// Check if the underlying implementation supports this method
	if apiService, ok := a.APIService.(interface {
		GetModelTokenLimits(string) (int32, int32, error)
	}); ok {
		return apiService.GetModelTokenLimits(modelName)
	}

	// We'll skip logging during tests to avoid noise

	// Convert model name to lowercase for case-insensitive matching
	modelNameLower := strings.ToLower(modelName)

	// Try to determine token limits based on model name
	// Use hardcoded values for well-known models
	switch {
	// Gemini models
	case strings.HasPrefix(modelNameLower, "gemini-1.5-pro"):
		return 1000000, 8192, nil
	case strings.HasPrefix(modelNameLower, "gemini-1.5-flash"):
		return 1000000, 8192, nil
	case strings.HasPrefix(modelNameLower, "gemini-1.0-pro"):
		return 32768, 8192, nil
	case strings.HasPrefix(modelNameLower, "gemini-1.0-ultra"):
		return 32768, 8192, nil

	// OpenAI models - Claude 3 / GPT-4 1M Token models
	case strings.HasPrefix(modelNameLower, "gpt-4.1"),
		strings.HasPrefix(modelNameLower, "o4-mini"),
		strings.HasPrefix(modelNameLower, "o4-"):
		return 1000000, 32768, nil

	// OpenAI models - GPT-4 Turbo and GPT-4o models (128k)
	case strings.HasPrefix(modelNameLower, "gpt-4o"),
		strings.HasPrefix(modelNameLower, "gpt-4-turbo"):
		return 128000, 4096, nil

	// OpenAI models - GPT-4 (8k)
	case strings.HasPrefix(modelNameLower, "gpt-4"):
		return 8192, 4096, nil

	// OpenAI models - GPT-3.5 Turbo
	case strings.HasPrefix(modelNameLower, "gpt-3.5-turbo"):
		return 16385, 4096, nil

	// Default fallback
	default:
		// Use a more generous default for unknown models
		// This matches the default in openai_client.go
		return 200000, 4096, nil
	}
}

// ValidateModelParameter validates a parameter value against its constraints.
// It delegates to the underlying APIService implementation.
func (a *APIServiceAdapter) ValidateModelParameter(modelName, paramName string, value interface{}) (bool, error) {
	if apiService, ok := a.APIService.(interface {
		ValidateModelParameter(string, string, interface{}) (bool, error)
	}); ok {
		return apiService.ValidateModelParameter(modelName, paramName, value)
	}
	// Return true if the underlying implementation doesn't support this method
	return true, nil
}

// Note: TokenManagerAdapter was removed as part of T032A
// to remove token handling from the application.

// ContextGathererAdapter provides an adapter for different ContextGatherer implementations
// It adapts the internal ContextGatherer interface to the interfaces.ContextGatherer interface
type ContextGathererAdapter struct {
	// The underlying ContextGatherer implementation from the internal package
	ContextGatherer ContextGatherer
}

// Convert internal GatherConfig to interfaces.GatherConfig
func internalToInterfacesGatherConfig(config interfaces.GatherConfig) GatherConfig {
	return GatherConfig{
		Paths:        config.Paths,
		Include:      config.Include,
		Exclude:      config.Exclude,
		ExcludeNames: config.ExcludeNames,
		Format:       config.Format,
		Verbose:      config.Verbose,
		LogLevel:     config.LogLevel,
	}
}

// Convert internal ContextStats to interfaces.ContextStats
func internalToInterfacesContextStats(stats *ContextStats) *interfaces.ContextStats {
	if stats == nil {
		return nil
	}
	return &interfaces.ContextStats{
		ProcessedFilesCount: stats.ProcessedFilesCount,
		CharCount:           stats.CharCount,
		LineCount:           stats.LineCount,
		// TokenCount field removed as part of T032F - token handling refactoring
		ProcessedFiles: stats.ProcessedFiles,
	}
}

// Convert interfaces.ContextStats to internal ContextStats
func interfacesToInternalContextStats(stats *interfaces.ContextStats) *ContextStats {
	if stats == nil {
		return nil
	}
	return &ContextStats{
		ProcessedFilesCount: stats.ProcessedFilesCount,
		CharCount:           stats.CharCount,
		LineCount:           stats.LineCount,
		// TokenCount field removed as part of T032F - token handling refactoring
		ProcessedFiles: stats.ProcessedFiles,
	}
}

// GatherContext adapts between interfaces.GatherConfig and internal GatherConfig
func (c *ContextGathererAdapter) GatherContext(ctx context.Context, config interfaces.GatherConfig) ([]fileutil.FileMeta, *interfaces.ContextStats, error) {
	// Convert interfaces.GatherConfig to internal GatherConfig
	internalConfig := internalToInterfacesGatherConfig(config)

	// Call the underlying internal implementation
	files, stats, err := c.ContextGatherer.GatherContext(ctx, internalConfig)

	// Convert internal ContextStats to interfaces.ContextStats if no error
	if err != nil {
		return nil, nil, err
	}

	return files, internalToInterfacesContextStats(stats), nil
}

// DisplayDryRunInfo adapts between interfaces.ContextStats and internal ContextStats
func (c *ContextGathererAdapter) DisplayDryRunInfo(ctx context.Context, stats *interfaces.ContextStats) error {
	// Convert interfaces.ContextStats to internal ContextStats
	internalStats := interfacesToInternalContextStats(stats)

	// Call the underlying internal implementation
	return c.ContextGatherer.DisplayDryRunInfo(ctx, internalStats)
}

// FileWriterAdapter provides an adapter for different FileWriter implementations
// It adapts the internal FileWriter interface to the interfaces.FileWriter interface
type FileWriterAdapter struct {
	// The underlying FileWriter implementation from the internal package
	FileWriter FileWriter
}

// SaveToFile delegates to the underlying FileWriter implementation
func (f *FileWriterAdapter) SaveToFile(content, outputFile string) error {
	return f.FileWriter.SaveToFile(content, outputFile)
}
