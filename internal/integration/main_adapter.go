// internal/integration/main_adapter.go
package integration

import (
	"context"
	"flag"
	"os"
	"strings"

	"github.com/phrazzld/architect/internal/gemini"
	"github.com/phrazzld/architect/internal/logutil"
	"github.com/phrazzld/architect/internal/prompt"
)

// MainAdapter provides a testable interface to the main package's functionality
// without having to modify the main package directly
type MainAdapter struct {
	// Dependencies that we want to mock
	GeminiClientFactory  func(ctx context.Context, apiKey, modelName string) (gemini.Client, error)
	PromptManagerFactory func(logger logutil.LoggerInterface) prompt.ManagerInterface

	// Original flag set for restoring after test
	OrigFlagCommandLine *flag.FlagSet
}

// Configuration mirrors the main package Configuration struct
// This needs to be kept in sync with the main package
type Configuration struct {
	TaskDescription string
	TaskFile        string
	OutputFile      string
	ModelName       string
	Verbose         bool
	LogLevel        logutil.LogLevel
	UseColors       bool
	Include         string
	Exclude         string
	ExcludeNames    string
	Format          string
	DryRun          bool
	ConfirmTokens   int
	PromptTemplate  string
	Paths           []string
	ApiKey          string
}

// NewMainAdapter creates a new adapter for testing the main package
func NewMainAdapter() *MainAdapter {
	// Save the original flag.CommandLine
	origFlagCommandLine := flag.CommandLine

	// Create the adapter
	adapter := &MainAdapter{
		OrigFlagCommandLine: origFlagCommandLine,

		// Default to using the real client factory
		GeminiClientFactory: func(ctx context.Context, apiKey, modelName string) (gemini.Client, error) {
			return gemini.NewClient(ctx, apiKey, modelName)
		},

		// Default to using the real prompt manager factory
		PromptManagerFactory: func(logger logutil.LoggerInterface) prompt.ManagerInterface {
			return prompt.NewManager(logger)
		},
	}

	return adapter
}

// ResetFlags resets the flag.CommandLine for testing
func (a *MainAdapter) ResetFlags() {
	// Create a new FlagSet for this test
	flag.CommandLine = flag.NewFlagSet(os.Args[0], flag.ExitOnError)
}

// RestoreFlags restores the original flag.CommandLine
func (a *MainAdapter) RestoreFlags() {
	flag.CommandLine = a.OrigFlagCommandLine
}

// RunWithArgs simulates running the application with the given arguments
func (a *MainAdapter) RunWithArgs(args []string, env *TestEnv) error {
	// Save original args and restore at the end
	origArgs := os.Args
	defer func() { os.Args = origArgs }()

	// Set the args for this run
	os.Args = args

	// Reset flags for this test
	a.ResetFlags()
	defer a.RestoreFlags()

	// Set environment variables as needed
	if os.Getenv("GEMINI_API_KEY") == "" {
		os.Setenv("GEMINI_API_KEY", "test-api-key")
		defer os.Unsetenv("GEMINI_API_KEY")
	}

	// Parse flags and create configuration
	config := a.parseFlags()

	// Set up logging
	logger := env.Logger

	// Skip validation for testing
	// a.validateInputs(config, logger)

	// Initialize API client using our mock
	ctx := context.Background()
	geminiClient := env.MockClient

	// Task clarification code has been removed

	// Gather context from files
	projectContext := a.gatherContext(ctx, config, geminiClient, logger)

	// Generate content if not in dry run mode
	if !config.DryRun {
		a.generateAndSavePlan(ctx, config, geminiClient, projectContext, logger)
	}

	return nil
}

// parseFlags mirrors the main package's parseFlags function but for testing
func (a *MainAdapter) parseFlags() *Configuration {
	config := &Configuration{}

	// Define flags - this needs to match the main package's flags
	taskFlag := flag.String("task", "", "Description of the task or goal for the plan.")
	taskFileFlag := flag.String("task-file", "", "Path to a file containing the task description (alternative to --task).")
	outputFileFlag := flag.String("output", "PLAN.md", "Output file path for the generated plan.")
	modelNameFlag := flag.String("model", "gemini-2.5-pro-exp-03-25", "Gemini model to use for generation.")
	verboseFlag := flag.Bool("verbose", false, "Enable verbose logging output (shorthand for --log-level=debug).")
	logLevelFlag := flag.String("log-level", "info", "Set logging level (debug, info, warn, error).")
	useColorsFlag := flag.Bool("color", true, "Enable/disable colored log output.")
	includeFlag := flag.String("include", "", "Comma-separated list of file extensions to include (e.g., .go,.md)")
	excludeFlag := flag.String("exclude", "", "Comma-separated list of file extensions to exclude.")
	excludeNamesFlag := flag.String("exclude-names", "", "Comma-separated list of file/dir names to exclude.")
	formatFlag := flag.String("format", "<{path}>\n```\n{content}\n```\n</{path}>\n\n", "Format string for each file.")
	dryRunFlag := flag.Bool("dry-run", false, "Show files that would be included and token count, but don't call the API.")
	confirmTokensFlag := flag.Int("confirm-tokens", 0, "Prompt for confirmation if token count exceeds this value (0 = never prompt)")
	promptTemplateFlag := flag.String("prompt-template", "", "Path to a custom prompt template file (.tmpl)")

	// Parse flags
	flag.Parse()

	// Store flag values in configuration
	config.TaskDescription = *taskFlag
	config.TaskFile = *taskFileFlag
	config.OutputFile = *outputFileFlag
	config.ModelName = *modelNameFlag
	config.Verbose = *verboseFlag
	config.UseColors = *useColorsFlag
	config.Include = *includeFlag
	config.Exclude = *excludeFlag
	config.ExcludeNames = *excludeNamesFlag
	config.Format = *formatFlag
	config.DryRun = *dryRunFlag
	config.ConfirmTokens = *confirmTokensFlag
	config.PromptTemplate = *promptTemplateFlag
	config.Paths = flag.Args()
	config.ApiKey = os.Getenv("GEMINI_API_KEY")

	// Determine log level based on flags
	if config.Verbose {
		config.LogLevel = logutil.DebugLevel
	} else {
		var err error
		config.LogLevel, err = logutil.ParseLogLevel(*logLevelFlag)
		if err != nil {
			config.LogLevel = logutil.InfoLevel
		}
	}

	return config
}

// gatherContext is a simplified version of the main package's gatherContext
func (a *MainAdapter) gatherContext(ctx context.Context, config *Configuration, geminiClient gemini.Client, logger logutil.LoggerInterface) string {
	// Just return a simplified context for testing
	return "This is a simulated project context for testing."
}


// generateAndSavePlan is a simplified version of the main package's generateAndSavePlan
func (a *MainAdapter) generateAndSavePlan(ctx context.Context, config *Configuration, geminiClient gemini.Client, projectContext string, logger logutil.LoggerInterface) {
	// Check token limits first
	promptText := "Task: " + config.TaskDescription + "\n\nContext: " + projectContext

	// Get token count
	tokenCount, err := geminiClient.CountTokens(ctx, promptText)
	if err != nil {
		logger.Error("Error counting tokens: %v", err)
		return
	}

	// Get model info
	modelInfo, err := geminiClient.GetModelInfo(ctx)
	if err != nil {
		logger.Error("Error getting model info: %v", err)
		return
	}

	// Check if token count exceeds limit
	if tokenCount.Total > modelInfo.InputTokenLimit {
		logger.Error("Token count exceeds limit: %d > %d", tokenCount.Total, modelInfo.InputTokenLimit)
		return
	}

	// Generate content - For the test, we'll include the task description in the output
	// so we can verify that the refined task was used
	result, err := geminiClient.GenerateContent(ctx, promptText)
	if err != nil {
		logger.Error("Error generating content: %v", err)
		return
	}

	// Simple check for empty content
	if strings.TrimSpace(result.Content) == "" {
		logger.Error("Received empty content from Gemini")
		return
	}

	// For testing the clarification feature, we'll modify the output to include the task
	// This helps us verify that the refined task was used
	outputContent := "Task used: " + config.TaskDescription + "\n\n" + result.Content

	// Write to the output file
	err = os.WriteFile(config.OutputFile, []byte(outputContent), 0644)
	if err != nil {
		logger.Error("Error writing plan to file: %v", err)
		return
	}

	logger.Info("Plan saved to %s", config.OutputFile)
}
