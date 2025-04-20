// Package architect provides the command-line interface for the architect tool
package architect

import (
	"flag"
	"fmt"
	"io"
	"os"
	"strings"

	"github.com/phrazzld/architect/internal/config"
	"github.com/phrazzld/architect/internal/logutil"
	"github.com/phrazzld/architect/internal/registry"
)

// stringSliceFlag is a slice of strings that implements flag.Value interface
// to handle repeatable flags for multiple values
type stringSliceFlag []string

// String returns a comma-separated string representation of the flag values
// This method is required by the flag.Value interface
func (s *stringSliceFlag) String() string {
	return strings.Join(*s, ",")
}

// Set appends the value to the slice of values
// This method is required by the flag.Value interface
func (s *stringSliceFlag) Set(value string) error {
	*s = append(*s, value)
	return nil
}

// Constants referencing the config package defaults
const (
	defaultOutputFile   = config.DefaultOutputFile
	defaultModel        = config.DefaultModel
	apiKeyEnvVar        = config.APIKeyEnvVar
	apiEndpointEnvVar   = config.APIEndpointEnvVar
	openaiAPIKeyEnvVar  = config.OpenAIAPIKeyEnvVar
	defaultFormat       = config.DefaultFormat
	defaultExcludes     = config.DefaultExcludes
	defaultExcludeNames = config.DefaultExcludeNames
)

// ValidateInputs checks if the configuration is valid and returns an error if not
func ValidateInputs(config *config.CliConfig, logger logutil.LoggerInterface) error {
	return ValidateInputsWithEnv(config, logger, os.Getenv)
}

// ValidateInputsWithEnv checks if the configuration is valid and returns an error if not
// This version takes a getenv function for easier testing
func ValidateInputsWithEnv(config *config.CliConfig, logger logutil.LoggerInterface, getenv func(string) string) error {
	// Check for instructions file
	if config.InstructionsFile == "" && !config.DryRun {
		logger.Error("The required --instructions flag is missing.")
		return fmt.Errorf("missing required --instructions flag")
	}

	// Check for input paths
	if len(config.Paths) == 0 {
		logger.Error("At least one file or directory path must be provided as an argument.")
		return fmt.Errorf("no paths specified")
	}

	// Check for API key based on model configuration
	modelNeedsOpenAIKey := false
	modelNeedsGeminiKey := false
	modelNeedsOpenRouterKey := false

	// Check models using registry if available
	regManager := registry.GetGlobalManager(nil)

	// Use registry for provider detection if available
	if regManager != nil {
		for _, model := range config.ModelNames {
			// Use the registry to determine the provider
			provider, err := regManager.GetProviderForModel(model)
			if err != nil {
				// If model not found in registry, fallback to string matching
				logger.Debug("Model %s not found in registry, using string matching fallback", model)
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
				continue
			}

			// Set flag based on provider
			switch provider {
			case "openai":
				modelNeedsOpenAIKey = true
			case "openrouter":
				modelNeedsOpenRouterKey = true
			case "gemini":
				modelNeedsGeminiKey = true
			default:
				logger.Warn("Unknown provider %s for model %s", provider, model)
			}
		}
	} else {
		// Registry not available, use string matching fallback
		logger.Debug("Registry not available, using string matching fallback for model detection")
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
	}

	// API key validation based on model requirements
	if config.APIKey == "" && modelNeedsGeminiKey {
		logger.Error("%s environment variable not set.", apiKeyEnvVar)
		return fmt.Errorf("gemini API key not set")
	}

	// If any OpenAI model is used, check for OpenAI API key
	if modelNeedsOpenAIKey {
		openAIKey := getenv(openaiAPIKeyEnvVar)
		if openAIKey == "" {
			logger.Error("%s environment variable not set.", openaiAPIKeyEnvVar)
			return fmt.Errorf("openAI API key not set")
		}
	}

	// If any OpenRouter model is used, check for OpenRouter API key
	if modelNeedsOpenRouterKey {
		openRouterKey := getenv("OPENROUTER_API_KEY")
		if openRouterKey == "" {
			logger.Error("OPENROUTER_API_KEY environment variable not set.")
			return fmt.Errorf("openRouter API key not set")
		}
	}

	// Check for model names
	if len(config.ModelNames) == 0 && !config.DryRun {
		logger.Error("At least one model must be specified with --model flag.")
		return fmt.Errorf("no models specified")
	}

	return nil
}

// ParseFlags handles command line argument parsing and returns the configuration
func ParseFlags() (*config.CliConfig, error) {
	return ParseFlagsWithEnv(flag.CommandLine, os.Args[1:], os.Getenv)
}

// ParseFlagsWithEnv handles command-line flag parsing with custom flag set and environment lookup
// This improves testability by allowing tests to provide mock flag sets and environment functions
func ParseFlagsWithEnv(flagSet *flag.FlagSet, args []string, getenv func(string) string) (*config.CliConfig, error) {
	cfg := config.NewDefaultCliConfig()

	// Define flags
	instructionsFileFlag := flagSet.String("instructions", "", "Path to a file containing the static instructions for the LLM.")
	outputDirFlag := flagSet.String("output-dir", "", "Directory path to store generated plans (one per model).")
	verboseFlag := flagSet.Bool("verbose", false, "Enable verbose logging output (shorthand for --log-level=debug).")
	logLevelFlag := flagSet.String("log-level", "info", "Set logging level (debug, info, warn, error).")
	includeFlag := flagSet.String("include", "", "Comma-separated list of file extensions to include (e.g., .go,.md)")
	excludeFlag := flagSet.String("exclude", defaultExcludes, "Comma-separated list of file extensions to exclude.")
	excludeNamesFlag := flagSet.String("exclude-names", defaultExcludeNames, "Comma-separated list of file/dir names to exclude.")
	formatFlag := flagSet.String("format", defaultFormat, "Format string for each file. Use {path} and {content}.")
	dryRunFlag := flagSet.Bool("dry-run", false, "Show files that would be included and token count, but don't call the API.")
	// confirm-tokens flag removed as part of T032E - token management refactoring
	auditLogFileFlag := flagSet.String("audit-log-file", "", "Path to write structured audit logs (JSON Lines). Disabled if empty.")

	// Rate limiting flags
	maxConcurrentFlag := flagSet.Int("max-concurrent", 5, // Use hardcoded default for backward compatibility with tests
		"Maximum number of concurrent API requests (0 = no limit)")
	rateLimitRPMFlag := flagSet.Int("rate-limit", 60, // Use hardcoded default for backward compatibility with tests
		"Maximum requests per minute (RPM) per model (0 = no limit)")

	// Define the model flag using our custom stringSliceFlag type to support multiple values
	modelFlag := &stringSliceFlag{}
	flagSet.Var(modelFlag, "model", fmt.Sprintf("Model to use for generation (repeatable). Can be Gemini (e.g., %s) or OpenAI (e.g., gpt-4) models. Default: %s", defaultModel, defaultModel))

	// Set custom usage message
	flagSet.Usage = func() {
		fmt.Fprintf(os.Stderr, "Usage: %s --instructions <file> [options] <path1> [path2...]\n\n", os.Args[0])

		fmt.Fprintf(os.Stderr, "Arguments:\n")
		fmt.Fprintf(os.Stderr, "  <path1> [path2...]   One or more file or directory paths for project context.\n\n")

		fmt.Fprintf(os.Stderr, "Example Commands:\n")
		fmt.Fprintf(os.Stderr, "  %s --instructions instructions.txt ./src                        Generate plan using default model\n", os.Args[0])
		fmt.Fprintf(os.Stderr, "  %s --instructions instructions.txt --output-dir custom-dir ./       Generate plans in custom directory\n", os.Args[0])
		fmt.Fprintf(os.Stderr, "  %s --instructions instructions.txt --model model1 --model model2 ./  Generate plans for multiple models\n", os.Args[0])
		fmt.Fprintf(os.Stderr, "  %s --dry-run ./                                                 Show files without generating plan\n\n", os.Args[0])

		fmt.Fprintf(os.Stderr, "Options:\n")
		flagSet.PrintDefaults()

		fmt.Fprintf(os.Stderr, "\nEnvironment Variables:\n")
		fmt.Fprintf(os.Stderr, "  %s: Required for Gemini models. Your Google AI Gemini API key.\n", apiKeyEnvVar)
		fmt.Fprintf(os.Stderr, "  %s: Required for OpenAI models. Your OpenAI API key.\n", openaiAPIKeyEnvVar)
		fmt.Fprintf(os.Stderr, "  OPENROUTER_API_KEY: Required for OpenRouter models. Your OpenRouter API key.\n")
	}

	// Parse the flags
	if err := flagSet.Parse(args); err != nil {
		return nil, fmt.Errorf("error parsing flags: %w", err)
	}

	// Store flag values in configuration
	cfg.InstructionsFile = *instructionsFileFlag

	// Set output directory
	cfg.OutputDir = *outputDirFlag

	cfg.AuditLogFile = *auditLogFileFlag
	cfg.Verbose = *verboseFlag
	cfg.Include = *includeFlag
	cfg.Exclude = *excludeFlag
	cfg.ExcludeNames = *excludeNamesFlag
	cfg.Format = *formatFlag
	cfg.DryRun = *dryRunFlag
	// ConfirmTokens field assignment removed as part of T032E - token management refactoring
	cfg.Paths = flagSet.Args()

	// Store rate limiting configuration
	cfg.MaxConcurrentRequests = *maxConcurrentFlag
	cfg.RateLimitRequestsPerMinute = *rateLimitRPMFlag

	// Set model names from the flag, defaulting to a single default model if none provided
	if len(*modelFlag) > 0 {
		cfg.ModelNames = *modelFlag
	} else {
		// If no models were specified on the command line, use the default model
		cfg.ModelNames = []string{defaultModel}
	}

	// Determine initial log level from flag
	parsedLogLevel := logutil.InfoLevel // Default
	if *logLevelFlag != "info" {
		ll, err := logutil.ParseLogLevel(*logLevelFlag)
		if err == nil {
			parsedLogLevel = ll
		}
	}
	cfg.LogLevel = parsedLogLevel

	// Apply verbose override *after* parsing the specific level
	if cfg.Verbose {
		cfg.LogLevel = logutil.DebugLevel
	}
	cfg.APIKey = getenv(apiKeyEnvVar)
	cfg.APIEndpoint = getenv(apiEndpointEnvVar)

	// ParseFlagsWithEnv no longer does logical validation (just parsing errors)
	// Validation is now exclusively handled by ValidateInputs
	return cfg, nil
}

// setupLoggingCustomImpl is the implementation of SetupLoggingCustom
func setupLoggingCustomImpl(config *config.CliConfig, _ *flag.Flag, output io.Writer) logutil.LoggerInterface {
	// Apply verbose override if set
	if config.Verbose {
		config.LogLevel = logutil.DebugLevel
	}

	// Use the LogLevel set in the config
	logger := logutil.NewLogger(config.LogLevel, output, "[architect] ")
	return logger
}

// SetupLoggingCustom is a variable holding the implementation for easier testing
var SetupLoggingCustom = setupLoggingCustomImpl

// SetupLogging initializes the logger based on configuration
func SetupLogging(config *config.CliConfig) logutil.LoggerInterface {
	return SetupLoggingCustom(config, flag.Lookup("log-level"), os.Stderr)
}
