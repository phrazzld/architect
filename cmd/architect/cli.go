// Package architect provides the command-line interface for the architect tool
package architect

import (
	"flag"
	"fmt"
	"io"
	"os"

	"github.com/phrazzld/architect/internal/config"
	"github.com/phrazzld/architect/internal/logutil"
)

// Constants referencing the config package defaults
const (
	defaultOutputFile   = config.DefaultOutputFile
	defaultModel        = config.DefaultModel
	apiKeyEnvVar        = config.APIKeyEnvVar
	defaultFormat       = config.DefaultFormat
	defaultExcludes     = config.DefaultExcludes
	defaultExcludeNames = config.DefaultExcludeNames
)

// CliConfig holds the parsed command-line options
type CliConfig struct {
	TaskFile       string
	OutputFile     string
	ModelName      string
	Verbose        bool
	LogLevel       logutil.LogLevel
	Include        string
	Exclude        string
	ExcludeNames   string
	Format         string
	DryRun         bool
	ConfirmTokens  int
	PromptTemplate string
	// ClarifyTask field removed as part of the feature removal
	ListExamples bool
	ShowExample  string
	Paths        []string
	ApiKey       string
}

// ValidateInputs checks if the configuration is valid and returns an error if not
func ValidateInputs(config *CliConfig, logger logutil.LoggerInterface) error {
	// Check if we're in list examples or show example mode
	if config.ListExamples || config.ShowExample != "" {
		return nil // Skip validation for example commands
	}

	// Check for task file
	if config.TaskFile == "" && !config.DryRun {
		logger.Error("The required --task-file flag is missing.")
		return fmt.Errorf("missing required --task-file flag")
	}

	// Check for input paths
	if len(config.Paths) == 0 {
		logger.Error("At least one file or directory path must be provided as an argument.")
		return fmt.Errorf("no paths specified")
	}

	// Check for API key
	if config.ApiKey == "" {
		logger.Error("%s environment variable not set.", apiKeyEnvVar)
		return fmt.Errorf("API key not set")
	}

	return nil
}

// ParseFlags handles command line argument parsing and returns the configuration
func ParseFlags() (*CliConfig, error) {
	return ParseFlagsWithEnv(flag.CommandLine, os.Args[1:], os.Getenv)
}

// ParseFlagsWithEnv handles command-line flag parsing with custom flag set and environment lookup
// This improves testability by allowing tests to provide mock flag sets and environment functions
func ParseFlagsWithEnv(flagSet *flag.FlagSet, args []string, getenv func(string) string) (*CliConfig, error) {
	config := &CliConfig{}

	// Define flags
	taskFileFlag := flagSet.String("task-file", "", "Path to a file containing the task description.")
	outputFileFlag := flagSet.String("output", defaultOutputFile, "Output file path for the generated plan.")
	modelNameFlag := flagSet.String("model", defaultModel, "Gemini model to use for generation.")
	verboseFlag := flagSet.Bool("verbose", false, "Enable verbose logging output (shorthand for --log-level=debug).")
	flagSet.String("log-level", "info", "Set logging level (debug, info, warn, error).")
	includeFlag := flagSet.String("include", "", "Comma-separated list of file extensions to include (e.g., .go,.md)")
	excludeFlag := flagSet.String("exclude", defaultExcludes, "Comma-separated list of file extensions to exclude.")
	excludeNamesFlag := flagSet.String("exclude-names", defaultExcludeNames, "Comma-separated list of file/dir names to exclude.")
	formatFlag := flagSet.String("format", defaultFormat, "Format string for each file. Use {path} and {content}.")
	dryRunFlag := flagSet.Bool("dry-run", false, "Show files that would be included and token count, but don't call the API.")
	confirmTokensFlag := flagSet.Int("confirm-tokens", 0, "Prompt for confirmation if token count exceeds this value (0 = never prompt)")
	promptTemplateFlag := flagSet.String("prompt-template", "", "Path to a custom prompt template file (.tmpl)")
	listExamplesFlag := flagSet.Bool("list-examples", false, "List available example prompt template files")
	showExampleFlag := flagSet.String("show-example", "", "Display the content of a specific example template")

	// Set custom usage message
	flagSet.Usage = func() {
		fmt.Fprintf(os.Stderr, "Usage: %s [--list-examples | --show-example NAME | --task-file <path> [options] <path1> [path2...]]\n\n", os.Args[0])

		fmt.Fprintf(os.Stderr, "Arguments:\n")
		fmt.Fprintf(os.Stderr, "  <path1> [path2...]   One or more file or directory paths for project context.\n\n")

		fmt.Fprintf(os.Stderr, "Example Commands:\n")
		fmt.Fprintf(os.Stderr, "  %s --list-examples                 List available example templates\n", os.Args[0])
		fmt.Fprintf(os.Stderr, "  %s --show-example basic.tmpl       Display the content of a specific example template\n", os.Args[0])
		fmt.Fprintf(os.Stderr, "  %s --show-example basic > my.tmpl  Save an example template to a file\n\n", os.Args[0])

		fmt.Fprintf(os.Stderr, "Options:\n")
		flagSet.PrintDefaults()

		fmt.Fprintf(os.Stderr, "\nEnvironment Variables:\n")
		fmt.Fprintf(os.Stderr, "  %s: Required. Your Google AI Gemini API key.\n", apiKeyEnvVar)
	}

	// Parse the flags
	if err := flagSet.Parse(args); err != nil {
		return nil, fmt.Errorf("error parsing flags: %w", err)
	}

	// Store flag values in configuration
	config.TaskFile = *taskFileFlag
	config.OutputFile = *outputFileFlag
	config.ModelName = *modelNameFlag
	config.Verbose = *verboseFlag
	config.Include = *includeFlag
	config.Exclude = *excludeFlag
	config.ExcludeNames = *excludeNamesFlag
	config.Format = *formatFlag
	config.DryRun = *dryRunFlag
	config.ConfirmTokens = *confirmTokensFlag
	config.PromptTemplate = *promptTemplateFlag
	config.ListExamples = *listExamplesFlag
	config.ShowExample = *showExampleFlag
	config.Paths = flagSet.Args()
	config.ApiKey = getenv(apiKeyEnvVar)

	// Basic validation for non-special commands
	if !config.ListExamples && config.ShowExample == "" {
		if config.TaskFile == "" {
			return nil, fmt.Errorf("missing required flag --task-file")
		}

		if len(config.Paths) == 0 {
			return nil, fmt.Errorf("no paths specified for project context")
		}
	}

	return config, nil
}

// SetupLogging initializes the logger based on configuration
func SetupLogging(config *CliConfig) logutil.LoggerInterface {
	return SetupLoggingCustom(config, flag.Lookup("log-level"), os.Stderr)
}

// SetupLoggingCustom initializes the logger with custom flag and writer for testing
func SetupLoggingCustom(config *CliConfig, logLevelFlag *flag.Flag, output io.Writer) logutil.LoggerInterface {
	var logLevel logutil.LogLevel

	// Determine log level
	if config.Verbose {
		logLevel = logutil.DebugLevel
	} else if logLevelFlag != nil {
		// Get the log level from the configuration
		logLevelValue := logLevelFlag.Value.String()
		var err error
		logLevel, err = logutil.ParseLogLevel(logLevelValue)
		if err != nil {
			fmt.Fprintf(os.Stderr, "Warning: %v. Defaulting to 'info' level.\n", err)
			logLevel = logutil.InfoLevel
		}
	} else {
		logLevel = logutil.InfoLevel
	}

	// Store the log level in the config
	config.LogLevel = logLevel

	// Create structured logger
	logger := logutil.NewLogger(logLevel, output, "[architect] ")

	return logger
}

// ConvertConfigToMap converts a CliConfig to a map for use with config.Manager.MergeWithFlags
func ConvertConfigToMap(cliConfig *CliConfig) map[string]interface{} {
	// The temporary clarifyTaskValue variable and clarify key have been removed
	return map[string]interface{}{
		"taskFile":       cliConfig.TaskFile,
		"output":         cliConfig.OutputFile,
		"model":          cliConfig.ModelName,
		"verbose":        cliConfig.Verbose,
		"logLevel":       cliConfig.LogLevel.String(),
		"include":        cliConfig.Include,
		"exclude":        cliConfig.Exclude,
		"excludeNames":   cliConfig.ExcludeNames,
		"format":         cliConfig.Format,
		"dryRun":         cliConfig.DryRun,
		"confirmTokens":  cliConfig.ConfirmTokens,
		"promptTemplate": cliConfig.PromptTemplate,
		"listExamples":   cliConfig.ListExamples,
		"showExample":    cliConfig.ShowExample,
		"apiKey":         cliConfig.ApiKey,
	}
}
