// Package config handles loading and managing application configuration.
// It defines a canonical set of configuration parameters used throughout
// the application, consolidating configuration from CLI flags, environment
// variables, and default values. This centralized approach ensures
// consistent configuration handling and reduces duplication.
package config

import (
	"fmt"
	"github.com/phrazzld/architect/internal/logutil"
	"os"
	"strings"
)

// Configuration constants
const (
	// Default values
	DefaultOutputFile      = "PLAN.md"
	DefaultModel           = "gemini-2.5-pro-exp-03-25"
	APIKeyEnvVar           = "GEMINI_API_KEY"
	APIEndpointEnvVar      = "GEMINI_API_URL"
	OpenAIAPIKeyEnvVar     = "OPENAI_API_KEY"
	OpenRouterAPIKeyEnvVar = "OPENROUTER_API_KEY"
	DefaultFormat          = "<{path}>\n```\n{content}\n```\n</{path}>\n\n"

	// Default rate limiting values
	DefaultMaxConcurrentRequests      = 5  // Default maximum concurrent API requests
	DefaultRateLimitRequestsPerMinute = 60 // Default requests per minute per model

	// Default excludes for file extensions
	DefaultExcludes = ".exe,.bin,.obj,.o,.a,.lib,.so,.dll,.dylib,.class,.jar,.pyc,.pyo,.pyd," +
		".zip,.tar,.gz,.rar,.7z,.pdf,.doc,.docx,.xls,.xlsx,.ppt,.pptx,.odt,.ods,.odp," +
		".jpg,.jpeg,.png,.gif,.bmp,.tiff,.svg,.mp3,.wav,.ogg,.mp4,.avi,.mov,.wmv,.flv," +
		".iso,.img,.dmg,.db,.sqlite,.log"

	// Default excludes for file and directory names
	DefaultExcludeNames = ".git,.hg,.svn,node_modules,bower_components,vendor,target,dist,build," +
		"out,tmp,coverage,__pycache__,*.pyc,*.pyo,.DS_Store,~$*,desktop.ini,Thumbs.db," +
		"package-lock.json,yarn.lock,go.sum,go.work"
)

// ExcludeConfig defines file exclusion configuration
type ExcludeConfig struct {
	// File extensions to exclude
	Extensions string
	// File and directory names to exclude
	Names string
}

// AppConfig holds essential configuration settings with defaults
type AppConfig struct {
	// Core settings with defaults
	OutputFile string
	ModelName  string
	Format     string

	// File handling settings
	Include string
	// ConfirmTokens field removed as part of T032E

	// Logging and display settings
	Verbose  bool
	LogLevel logutil.LogLevel

	// Exclude settings (hierarchical)
	Excludes ExcludeConfig
}

// DefaultConfig returns a new AppConfig instance with default values
func DefaultConfig() *AppConfig {
	return &AppConfig{
		OutputFile: DefaultOutputFile,
		ModelName:  DefaultModel,
		Format:     DefaultFormat,
		LogLevel:   logutil.InfoLevel,
		// ConfirmTokens removed as part of T032E - token management refactoring
		Excludes: ExcludeConfig{
			Extensions: DefaultExcludes,
			Names:      DefaultExcludeNames,
		},
	}
}

// CliConfig holds the parsed command-line options for the application.
// It serves as the canonical configuration structure used throughout the
// application, combining user inputs from CLI flags, environment variables,
// and default values. This struct is passed to components that need
// configuration parameters rather than having them parse flags directly.
type CliConfig struct {
	// Instructions configuration
	InstructionsFile string

	// Output configuration
	OutputDir    string
	AuditLogFile string // Path to write structured audit logs (JSON Lines)
	Format       string

	// Context gathering options
	Paths        []string
	Include      string
	Exclude      string
	ExcludeNames string
	DryRun       bool
	Verbose      bool

	// API configuration
	APIKey      string
	APIEndpoint string
	ModelNames  []string

	// Token management field removed as part of T032E

	// Logging
	LogLevel logutil.LogLevel

	// Rate limiting configuration
	MaxConcurrentRequests      int // Maximum number of concurrent API requests (0 = no limit)
	RateLimitRequestsPerMinute int // Maximum requests per minute per model (0 = no limit)
}

// NewDefaultCliConfig returns a CliConfig with default values.
// This is used as a starting point before parsing CLI flags, ensuring
// that all fields have sensible defaults even if not explicitly set
// by the user.
func NewDefaultCliConfig() *CliConfig {
	return &CliConfig{
		Format:                     DefaultFormat,
		Exclude:                    DefaultExcludes,
		ExcludeNames:               DefaultExcludeNames,
		ModelNames:                 []string{DefaultModel},
		LogLevel:                   logutil.InfoLevel,
		MaxConcurrentRequests:      DefaultMaxConcurrentRequests,
		RateLimitRequestsPerMinute: DefaultRateLimitRequestsPerMinute,
	}
}

// ValidateConfig checks if the configuration is valid and returns an error if not.
// It performs validation beyond simple type-checking, such as verifying that
// required fields are present, paths exist, and values are within acceptable ranges.
// This helps catch configuration errors early before they cause runtime failures.
func ValidateConfig(config *CliConfig, logger logutil.LoggerInterface) error {
	return ValidateConfigWithEnv(config, logger, os.Getenv)
}

// ValidateConfigWithEnv checks if the configuration is valid and returns an error if not.
// This version takes a getenv function for easier testing by allowing environment variables
// to be mocked.
func ValidateConfigWithEnv(config *CliConfig, logger logutil.LoggerInterface, getenv func(string) string) error {
	// Handle nil config
	if config == nil {
		if logger != nil {
			logger.Error("Configuration is nil")
		}
		return fmt.Errorf("nil config provided")
	}

	// Define a safe logger function that won't panic if logger is nil
	logError := func(format string, args ...interface{}) {
		if logger != nil {
			logger.Error(format, args...)
		}
	}

	// Check for valid paths (always required)
	validPaths := false
	if len(config.Paths) > 0 {
		for _, path := range config.Paths {
			if len(strings.TrimSpace(path)) > 0 {
				validPaths = true
				break
			}
		}
	}

	if !validPaths {
		logError("At least one file or directory path must be provided as an argument.")
		return fmt.Errorf("no paths specified")
	}

	// Check for instructions file (required unless in dry run mode)
	if config.InstructionsFile == "" && !config.DryRun {
		logError("The required --instructions flag is missing.")
		return fmt.Errorf("missing required --instructions flag")
	}

	// Check for API key based on model configuration
	modelNeedsOpenAIKey := false
	modelNeedsGeminiKey := false
	modelNeedsOpenRouterKey := false

	// Check if any model is OpenAI, Gemini, or OpenRouter
	for _, model := range config.ModelNames {
		if strings.HasPrefix(strings.ToLower(model), "gpt-") ||
			strings.HasPrefix(strings.ToLower(model), "text-") ||
			strings.Contains(strings.ToLower(model), "openai") {
			modelNeedsOpenAIKey = true
		} else if strings.Contains(strings.ToLower(model), "openrouter") {
			modelNeedsOpenRouterKey = true
		} else {
			// Default to Gemini for any other model
			modelNeedsGeminiKey = true
		}
	}

	// API key validation based on model requirements
	if config.APIKey == "" && modelNeedsGeminiKey {
		logError("%s environment variable not set.", APIKeyEnvVar)
		return fmt.Errorf("gemini API key not set")
	}

	// If any OpenAI model is used, check for OpenAI API key
	if modelNeedsOpenAIKey {
		openAIKey := getenv(OpenAIAPIKeyEnvVar)
		if openAIKey == "" {
			logError("%s environment variable not set.", OpenAIAPIKeyEnvVar)
			return fmt.Errorf("openAI API key not set")
		}
	}

	// If any OpenRouter model is used, check for OpenRouter API key
	if modelNeedsOpenRouterKey {
		openRouterKey := getenv(OpenRouterAPIKeyEnvVar)
		if openRouterKey == "" {
			logError("%s environment variable not set.", OpenRouterAPIKeyEnvVar)
			return fmt.Errorf("openRouter API key not set")
		}
	}

	// Check for model names (required unless in dry run mode)
	if len(config.ModelNames) == 0 && !config.DryRun {
		logError("At least one model must be specified with --model flag.")
		return fmt.Errorf("no models specified")
	}

	return nil
}
