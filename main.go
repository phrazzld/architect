// main.go
package main

import (
	"bufio"
	"context"
	"encoding/json"
	"flag"
	"fmt"
	"log"
	"os"
	"path/filepath"
	"strings"

	"github.com/phrazzld/architect/internal/config"
	"github.com/phrazzld/architect/internal/fileutil"
	"github.com/phrazzld/architect/internal/gemini"
	"github.com/phrazzld/architect/internal/logutil"
	"github.com/phrazzld/architect/internal/prompt"
	"github.com/phrazzld/architect/internal/spinner"
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

// Configuration holds the parsed command-line options
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
	NoSpinner       bool
	ClarifyTask     bool
	Paths           []string
	ApiKey          string
}

func main() {
	// Parse command line flags
	cliConfig := parseFlags()

	// Setup logging early for error reporting
	logger := setupLogging(cliConfig)

	// Initialize XDG-compliant configuration system
	configManager := initConfigSystem(logger)

	// Load configuration from files
	err := configManager.LoadFromFiles()
	if err != nil {
		logger.Warn("Failed to load configuration: %v", err)
		logger.Info("Using default configuration")
	}

	// Ensure configuration directories exist
	if err := configManager.EnsureConfigDirs(); err != nil {
		logger.Warn("Failed to create configuration directories: %v", err)
	}

	// Convert CLI flags to the format needed for merging
	cliFlags := convertConfigToMap(cliConfig)

	// Merge CLI flags with loaded configuration
	if err := configManager.MergeWithFlags(cliFlags); err != nil {
		logger.Warn("Failed to merge CLI flags with configuration: %v", err)
	}

	// Get the final configuration
	appConfig := configManager.GetConfig()

	// Create backfilled CLI config for backward compatibility
	config := backfillConfigFromAppConfig(cliConfig, appConfig)

	// Validate inputs
	validateInputs(config, logger)

	// Initialize API client
	ctx := context.Background()
	geminiClient := initGeminiClient(ctx, config, logger)
	defer geminiClient.Close()

	// If task clarification is enabled, let the user refine their task
	if config.ClarifyTask && !config.DryRun {
		config.TaskDescription = clarifyTaskDescriptionWithConfig(ctx, config, geminiClient, configManager, logger)
	}

	// Gather context from files
	projectContext := gatherContext(ctx, config, geminiClient, logger)

	// Generate content if not in dry run mode
	if !config.DryRun {
		generateAndSavePlanWithConfig(ctx, config, geminiClient, projectContext, configManager, logger)
	}
}

// clarifyTaskDescription is a backward-compatible wrapper for clarification process
func clarifyTaskDescription(ctx context.Context, config *Configuration, geminiClient gemini.Client, logger logutil.LoggerInterface) string {
	// Use legacy version with default prompt manager
	promptManager := prompt.NewManager(logger)
	return clarifyTaskDescriptionWithPromptManager(ctx, config, geminiClient, promptManager, logger)
}

// clarifyTaskDescriptionWithConfig performs task clarification using the config system
func clarifyTaskDescriptionWithConfig(ctx context.Context, config *Configuration, geminiClient gemini.Client, configManager config.ManagerInterface, logger logutil.LoggerInterface) string {
	// Set up prompt manager with config support
	promptManager, err := prompt.SetupPromptManagerWithConfig(logger, configManager)
	if err != nil {
		logger.Error("Failed to set up prompt manager: %v", err)
		// Fall back to the non-config version
		return clarifyTaskDescription(ctx, config, geminiClient, logger)
	}

	return clarifyTaskDescriptionWithPromptManager(ctx, config, geminiClient, promptManager, logger)
}

// clarifyTaskDescriptionWithPromptManager is the core implementation of the task clarification process
func clarifyTaskDescriptionWithPromptManager(ctx context.Context, config *Configuration, geminiClient gemini.Client, promptManager prompt.ManagerInterface, logger logutil.LoggerInterface) string {
	// Initialize spinner
	spinnerInstance := initSpinner(config, logger)

	// Original task description
	originalTask := config.TaskDescription

	// Build prompt for clarification (template loading is handled internally)
	spinnerInstance.Start("Analyzing task description...")
	data := &prompt.TemplateData{
		Task: originalTask,
	}

	clarifyPrompt, err := promptManager.BuildPrompt("clarify.tmpl", data)
	if err != nil {
		spinnerInstance.StopFail(fmt.Sprintf("Failed to build clarification prompt: %v", err))
		logger.Error("Failed to build clarification prompt: %v", err)
		return originalTask
	}

	// Call Gemini to generate clarification questions
	spinnerInstance.UpdateMessage("Generating clarification questions...")
	result, err := geminiClient.GenerateContent(ctx, clarifyPrompt)
	if err != nil {
		spinnerInstance.StopFail(fmt.Sprintf("Error generating clarification questions: %v", err))
		logger.Error("Error generating clarification questions: %v", err)
		return originalTask
	}

	// Process the JSON response
	var clarificationData struct {
		Analysis  string   `json:"analysis"`
		Questions []string `json:"questions"`
	}

	// Parse JSON response - must be valid JSON as requested in the prompt
	err = json.Unmarshal([]byte(result.Content), &clarificationData)
	if err != nil {
		spinnerInstance.StopFail(fmt.Sprintf("Failed to parse clarification response: %v", err))
		logger.Error("Failed to parse clarification response: %v", err)
		logger.Debug("Response content: %s", result.Content)
		return originalTask
	}

	// Stop spinner and start the interactive clarification process
	spinnerInstance.Stop("Task analysis complete")

	// Show the analysis to the user
	logger.Info("Task Analysis: %s", clarificationData.Analysis)

	// Present each question and collect answers
	var questionAnswers strings.Builder
	fmt.Println("\n🔍 Task Clarification:")

	reader := bufio.NewReader(os.Stdin)
	for i, question := range clarificationData.Questions {
		fmt.Printf("\n%d. %s\n", i+1, question)
		fmt.Print("   Your answer: ")

		answer, err := reader.ReadString('\n')
		if err != nil {
			logger.Error("Error reading input: %v", err)
			return originalTask
		}

		// Add the Q&A to our collection
		questionAnswers.WriteString(fmt.Sprintf("Question %d: %s\n", i+1, question))
		questionAnswers.WriteString(fmt.Sprintf("Answer %d: %s\n", i+1, strings.TrimSpace(answer)))
	}

	// Now refine the task with the answers
	spinnerInstance.Start("Refining task description...")

	// Build prompt for refinement (template loading is handled internally)
	refineData := &prompt.TemplateData{
		Task:    originalTask,
		Context: questionAnswers.String(),
	}

	refinePrompt, err := promptManager.BuildPrompt("refine.tmpl", refineData)
	if err != nil {
		spinnerInstance.StopFail(fmt.Sprintf("Failed to build refinement prompt: %v", err))
		logger.Error("Failed to build refinement prompt: %v", err)
		return originalTask
	}

	// Call Gemini to generate refined task
	result, err = geminiClient.GenerateContent(ctx, refinePrompt)
	if err != nil {
		spinnerInstance.StopFail(fmt.Sprintf("Error generating refined task: %v", err))
		logger.Error("Error generating refined task: %v", err)
		return originalTask
	}

	// Process the JSON response
	var refinementData struct {
		RefinedTask string   `json:"refined_task"`
		KeyPoints   []string `json:"key_points"`
	}

	// Parse JSON response
	err = json.Unmarshal([]byte(result.Content), &refinementData)
	if err != nil {
		spinnerInstance.StopFail(fmt.Sprintf("Failed to parse refinement response: %v", err))
		logger.Error("Failed to parse refinement response: %v", err)
		logger.Debug("Response content: %s", result.Content)
		return originalTask
	}

	// Stop spinner and show the refined task
	spinnerInstance.Stop("Task refinement complete")

	// Show the refinement results
	fmt.Println("\n✨ Refined Task Description:")
	fmt.Println(refinementData.RefinedTask)

	if len(refinementData.KeyPoints) > 0 {
		fmt.Println("\n🔑 Key Technical Points:")
		for i, point := range refinementData.KeyPoints {
			fmt.Printf("%d. %s\n", i+1, point)
		}
	}

	fmt.Println("\nProceeding with the refined task description...")

	return refinementData.RefinedTask
}

// parseFlags handles command line argument parsing
func parseFlags() *Configuration {
	config := &Configuration{}

	// Define flags
	taskFlag := flag.String("task", "", "Description of the task or goal for the plan.")
	taskFileFlag := flag.String("task-file", "", "Path to a file containing the task description (alternative to --task).")
	outputFileFlag := flag.String("output", defaultOutputFile, "Output file path for the generated plan.")
	modelNameFlag := flag.String("model", defaultModel, "Gemini model to use for generation.")
	verboseFlag := flag.Bool("verbose", false, "Enable verbose logging output (shorthand for --log-level=debug).")
	flag.String("log-level", "info", "Set logging level (debug, info, warn, error).")
	useColorsFlag := flag.Bool("color", true, "Enable/disable colored log output.")
	includeFlag := flag.String("include", "", "Comma-separated list of file extensions to include (e.g., .go,.md)")
	excludeFlag := flag.String("exclude", defaultExcludes, "Comma-separated list of file extensions to exclude.")
	excludeNamesFlag := flag.String("exclude-names", defaultExcludeNames, "Comma-separated list of file/dir names to exclude.")
	formatFlag := flag.String("format", defaultFormat, "Format string for each file. Use {path} and {content}.")
	dryRunFlag := flag.Bool("dry-run", false, "Show files that would be included and token count, but don't call the API.")
	confirmTokensFlag := flag.Int("confirm-tokens", 0, "Prompt for confirmation if token count exceeds this value (0 = never prompt)")
	promptTemplateFlag := flag.String("prompt-template", "", "Path to a custom prompt template file (.tmpl)")
	noSpinnerFlag := flag.Bool("no-spinner", false, "Disable spinner animation during API calls")
	clarifyTaskFlag := flag.Bool("clarify", false, "Enable interactive task clarification to refine your task description")

	// Set custom usage message
	flag.Usage = func() {
		fmt.Fprintf(os.Stderr, "Usage: %s (--task \"<description>\" | --task-file <path>) [options] <path1> [path2...]\n\n", os.Args[0])
		fmt.Fprintf(os.Stderr, "Arguments:\n")
		fmt.Fprintf(os.Stderr, "  <path1> [path2...]   One or more file or directory paths for project context.\n\n")
		fmt.Fprintf(os.Stderr, "Options:\n")
		flag.PrintDefaults()
		fmt.Fprintf(os.Stderr, "\nEnvironment Variables:\n")
		fmt.Fprintf(os.Stderr, "  %s: Required. Your Google AI Gemini API key.\n", apiKeyEnvVar)
	}

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
	config.NoSpinner = *noSpinnerFlag
	config.ClarifyTask = *clarifyTaskFlag
	config.Paths = flag.Args()
	config.ApiKey = os.Getenv(apiKeyEnvVar)

	return config
}

// setupLogging initializes the logger based on configuration
func setupLogging(config *Configuration) logutil.LoggerInterface {
	// Determine log level
	logLevel := logutil.InfoLevel
	if config.Verbose {
		logLevel = logutil.DebugLevel
	} else {
		// Get the log level from the configuration
		logLevelValue := flag.Lookup("log-level").Value.String()
		var err error
		logLevel, err = logutil.ParseLogLevel(logLevelValue)
		if err != nil {
			fmt.Fprintf(os.Stderr, "Warning: %v. Defaulting to 'info' level.\n", err)
			logLevel = logutil.InfoLevel
		}
	}

	// Store the log level in the config
	config.LogLevel = logLevel

	// Create structured logger
	logger := logutil.NewLogger(logLevel, os.Stderr, "[architect] ", config.UseColors)

	// Configure global logger
	log.SetOutput(os.Stderr) // Ensure global logger is configured

	return logger
}

// readTaskFromFile reads task description from a file
func readTaskFromFile(taskFilePath string, logger logutil.LoggerInterface) (string, error) {
	// Check if path is absolute, if not make it absolute
	if !filepath.IsAbs(taskFilePath) {
		cwd, err := os.Getwd()
		if err != nil {
			return "", fmt.Errorf("error getting current working directory: %w", err)
		}
		taskFilePath = filepath.Join(cwd, taskFilePath)
	}

	// Check if file exists
	if _, err := os.Stat(taskFilePath); os.IsNotExist(err) {
		return "", fmt.Errorf("task file not found: %s", taskFilePath)
	}

	// Read file content
	content, err := os.ReadFile(taskFilePath)
	if err != nil {
		return "", fmt.Errorf("error reading task file: %w", err)
	}

	// Return content as string
	return string(content), nil
}

// validateInputs verifies required inputs are provided
func validateInputs(config *Configuration, logger logutil.LoggerInterface) {
	// Check for task description (not required in dry run mode)
	if config.TaskFile != "" {
		// Load task from file
		taskContent, err := readTaskFromFile(config.TaskFile, logger)
		if err != nil {
			logger.Error("Failed to read task file: %v", err)
			flag.Usage()
			os.Exit(1)
		}

		// Set task description from file content
		config.TaskDescription = taskContent
		logger.Debug("Loaded task description from file: %s", config.TaskFile)
	}

	// Check if task description is still empty after potentially loading from file
	if config.TaskDescription == "" && !config.DryRun {
		logger.Error("Either --task or --task-file must be provided (except in dry-run mode).")
		flag.Usage()
		os.Exit(1)
	}

	// Check if both --task and --task-file are provided
	if config.TaskDescription != "" && config.TaskFile != "" && flag.Lookup("task").Value.String() != "" {
		logger.Warn("Both --task and --task-file flags were provided. Using task from --task-file.")
	}

	// Check for input paths
	if len(config.Paths) == 0 {
		logger.Error("At least one file or directory path must be provided as an argument.")
		flag.Usage()
		os.Exit(1)
	}

	// Check for API key
	if config.ApiKey == "" {
		logger.Error("%s environment variable not set.", apiKeyEnvVar)
		flag.Usage()
		os.Exit(1)
	}
}

// initGeminiClient creates and initializes the Gemini API client
func initGeminiClient(ctx context.Context, config *Configuration, logger logutil.LoggerInterface) gemini.Client {
	client, err := gemini.NewClient(ctx, config.ApiKey, config.ModelName)
	if err != nil {
		logger.Fatal("Error creating Gemini client: %v", err)
	}
	return client
}

// gatherContext collects and processes files based on configuration
func gatherContext(ctx context.Context, config *Configuration, geminiClient gemini.Client, logger logutil.LoggerInterface) string {
	// Initialize spinner
	spinnerInstance := initSpinner(config, logger)

	// Log appropriate message based on mode and start spinner
	if config.DryRun {
		spinnerInstance.Start("Gathering files that would be included in context...")
		logger.Info("Dry run mode: gathering files that would be included in context...")
	} else {
		spinnerInstance.Start("Gathering project context...")
		logger.Info("Gathering project context...")
	}

	// Setup file processing configuration
	fileConfig := fileutil.NewConfig(config.Verbose, config.Include, config.Exclude, config.ExcludeNames, config.Format, logger)

	// Track processed files for dry run mode
	var processedFiles []string
	if config.DryRun {
		processedFiles = make([]string, 0)
		collector := func(path string) {
			processedFiles = append(processedFiles, path)
		}
		fileConfig.SetFileCollector(collector)
	}

	// Gather project context
	projectContext, processedFilesCount, err := fileutil.GatherProjectContext(config.Paths, fileConfig)
	if err != nil {
		spinnerInstance.StopFail(fmt.Sprintf("Failed during project context gathering: %v", err))
		logger.Fatal("Failed during project context gathering: %v", err)
	}

	// Log warning if no files were processed
	if processedFilesCount == 0 {
		spinnerInstance.Stop("No files were processed for context. Check paths and filters.")
		logger.Warn("No files were processed for context. Check paths and filters.")
		return projectContext
	}

	// Update spinner message and calculate statistics
	spinnerInstance.UpdateMessage("Calculating token statistics...")
	charCount, lineCount, tokenCount := fileutil.CalculateStatisticsWithTokenCounting(ctx, geminiClient, projectContext, logger)

	// Handle dry run mode specific output
	if config.DryRun {
		spinnerInstance.Stop(fmt.Sprintf("Context gathered: %d files, %d lines, %d chars, %d tokens",
			processedFilesCount, lineCount, charCount, tokenCount))
		displayDryRunInfo(charCount, lineCount, tokenCount, processedFilesCount, processedFiles, ctx, geminiClient, logger)
	} else if config.LogLevel == logutil.DebugLevel || processedFilesCount > 0 {
		// Normal run mode
		spinnerInstance.Stop(fmt.Sprintf("Context gathered: %d files, %d lines, %d chars, %d tokens",
			processedFilesCount, lineCount, charCount, tokenCount))
		logger.Info("Context gathered: %d files processed, %d lines, %d chars, %d tokens.",
			processedFilesCount, lineCount, charCount, tokenCount)
	}

	return projectContext
}

// displayDryRunInfo shows detailed information for dry run mode
func displayDryRunInfo(charCount int, lineCount int, tokenCount int, processedFilesCount int,
	processedFiles []string, ctx context.Context, geminiClient gemini.Client, logger logutil.LoggerInterface) {

	logger.Info("Files that would be included in context:")
	if processedFilesCount == 0 {
		logger.Info("  No files matched the current filters.")
	} else {
		for i, file := range processedFiles {
			logger.Info("  %d. %s", i+1, file)
		}
	}

	logger.Info("Context statistics:")
	logger.Info("  Files: %d", processedFilesCount)
	logger.Info("  Lines: %d", lineCount)
	logger.Info("  Characters: %d", charCount)
	logger.Info("  Tokens: %d", tokenCount)

	// Get model info for token limit comparison
	modelInfo, modelInfoErr := geminiClient.GetModelInfo(ctx)
	if modelInfoErr != nil {
		// Check if it's an API error with enhanced details
		if apiErr, ok := gemini.IsAPIError(modelInfoErr); ok {
			logger.Warn("Could not get model information: %s", apiErr.Message)
			// Only show detailed info in debug logs
			logger.Debug("Model info error details: %s", apiErr.DebugInfo())
		} else {
			logger.Warn("Could not get model information: %v", modelInfoErr)
		}
	} else {
		// Convert to int32 for comparison with model limits
		tokenCountInt32 := int32(tokenCount)
		percentOfLimit := float64(tokenCountInt32) / float64(modelInfo.InputTokenLimit) * 100
		logger.Info("Token usage: %d / %d (%.1f%% of model's limit)",
			tokenCountInt32, modelInfo.InputTokenLimit, percentOfLimit)

		// Check if token count exceeds limit
		if tokenCountInt32 > modelInfo.InputTokenLimit {
			logger.Error("WARNING: Token count exceeds model's limit by %d tokens",
				tokenCountInt32-modelInfo.InputTokenLimit)
			logger.Error("Try reducing context by using --include, --exclude, or --exclude-names flags")
		} else {
			logger.Info("Context size is within the model's token limit")
		}
	}

	logger.Info("Dry run completed successfully.")
	logger.Info("To generate content, run without the --dry-run flag.")
}

// generateAndSavePlan is a backward-compatible wrapper for plan generation
func generateAndSavePlan(ctx context.Context, config *Configuration, geminiClient gemini.Client,
	projectContext string, logger logutil.LoggerInterface) {

	// Use the legacy version without config system support
	promptManager := prompt.NewManager(logger)
	generateAndSavePlanWithPromptManager(ctx, config, geminiClient, projectContext, promptManager, logger)
}

// generateAndSavePlanWithConfig creates and saves the plan to a file using the config system
func generateAndSavePlanWithConfig(ctx context.Context, config *Configuration, geminiClient gemini.Client,
	projectContext string, configManager config.ManagerInterface, logger logutil.LoggerInterface) {

	// Set up a prompt manager with config support
	promptManager, err := prompt.SetupPromptManagerWithConfig(logger, configManager)
	if err != nil {
		logger.Error("Failed to set up prompt manager: %v", err)
		// Fall back to non-config version
		generateAndSavePlan(ctx, config, geminiClient, projectContext, logger)
		return
	}

	generateAndSavePlanWithPromptManager(ctx, config, geminiClient, projectContext, promptManager, logger)
}

// generateAndSavePlanWithPromptManager is the core implementation of plan generation
func generateAndSavePlanWithPromptManager(ctx context.Context, config *Configuration, geminiClient gemini.Client,
	projectContext string, promptManager prompt.ManagerInterface, logger logutil.LoggerInterface) {

	// Initialize spinner
	spinnerInstance := initSpinner(config, logger)

	// Construct prompt using the provided prompt manager
	spinnerInstance.Start("Building prompt template...")
	generatedPrompt, err := buildPromptWithManager(config, config.TaskDescription, projectContext, promptManager, logger)
	if err != nil {
		spinnerInstance.StopFail(fmt.Sprintf("Failed to build prompt: %v", err))
		logger.Fatal("Failed to build prompt: %v", err)
	}
	spinnerInstance.Stop("Prompt template built successfully")

	// Debug logging of prompt details
	if config.LogLevel == logutil.DebugLevel {
		logger.Debug("Prompt length: %d characters", len(generatedPrompt))
		logger.Debug("Sending task to Gemini: %s", config.TaskDescription)
	}

	// Get token count for confirmation and limit checking
	spinnerInstance.Start("Checking token limits...")
	tokenInfo, err := getTokenInfo(ctx, geminiClient, generatedPrompt, logger)
	if err != nil {
		spinnerInstance.StopFail("Token count check failed")

		// Check if it's an API error with enhanced details
		if apiErr, ok := gemini.IsAPIError(err); ok {
			logger.Error("Token count check failed: %s", apiErr.Message)
			if apiErr.Suggestion != "" {
				logger.Error("Suggestion: %s", apiErr.Suggestion)
			}
			// Log more details in debug mode
			if config.LogLevel == logutil.DebugLevel {
				logger.Debug("Error details: %s", apiErr.DebugInfo())
			}
		} else {
			logger.Error("Token count check failed: %v", err)
			logger.Error("Try reducing context by using --include, --exclude, or --exclude-names flags")
		}

		logger.Fatal("Aborting generation to prevent API errors")
	}

	// If token limit is exceeded, abort
	if tokenInfo.exceedsLimit {
		spinnerInstance.StopFail("Token limit exceeded")
		logger.Error("Token limit exceeded: %s", tokenInfo.limitError)
		logger.Error("Try reducing context by using --include, --exclude, or --exclude-names flags")
		logger.Fatal("Aborting generation to prevent API errors")
	}
	spinnerInstance.Stop(fmt.Sprintf("Token check passed: %d / %d (%.1f%%)",
		tokenInfo.tokenCount, tokenInfo.inputLimit, tokenInfo.percentage))

	// Log token usage for regular (non-debug) mode
	if config.LogLevel != logutil.DebugLevel {
		logger.Info("Token usage: %d / %d (%.1f%%)",
			tokenInfo.tokenCount,
			tokenInfo.inputLimit,
			tokenInfo.percentage)
	}

	// Prompt for confirmation if threshold is set and exceeded
	if !promptForConfirmation(tokenInfo.tokenCount, config.ConfirmTokens, logger) {
		logger.Info("Operation cancelled by user.")
		return
	}

	// Call Gemini API
	spinnerInstance.Start(fmt.Sprintf("Generating plan using model %s...", config.ModelName))
	result, err := geminiClient.GenerateContent(ctx, generatedPrompt)
	if err != nil {
		spinnerInstance.StopFail("Generation failed")

		// Check if it's an API error with enhanced details
		if apiErr, ok := gemini.IsAPIError(err); ok {
			logger.Error("Error generating content: %s", apiErr.Message)
			if apiErr.Suggestion != "" {
				logger.Error("Suggestion: %s", apiErr.Suggestion)
			}
			// Log more details in debug mode
			if config.LogLevel == logutil.DebugLevel {
				logger.Debug("Error details: %s", apiErr.DebugInfo())
			}
		} else {
			logger.Error("Error generating content: %v", err)
		}

		logger.Fatal("Plan generation failed")
	}

	// Process API response
	generatedPlan := processApiResponse(result, logger)
	spinnerInstance.Stop("Plan generated successfully")

	// Debug logging of results
	if config.LogLevel == logutil.DebugLevel {
		logger.Debug("Plan received from Gemini.")
		if result.TokenCount > 0 {
			logger.Debug("Token usage: %d tokens", result.TokenCount)
		}
	}

	// Write the plan to file
	spinnerInstance.Start(fmt.Sprintf("Writing plan to %s...", config.OutputFile))
	saveToFile(generatedPlan, config.OutputFile, logger)
	spinnerInstance.Stop(fmt.Sprintf("Plan saved to %s", config.OutputFile))
}

// processApiResponse extracts content from the API response and handles errors
func processApiResponse(result *gemini.GenerationResult, logger logutil.LoggerInterface) string {
	// Check for empty content
	if result.Content == "" {
		// Build an informative error message
		finishReason := ""
		if result.FinishReason != "" {
			finishReason = fmt.Sprintf(" (Finish Reason: %s)", result.FinishReason)
		}

		// Check for safety blocks
		safetyInfo := ""
		if len(result.SafetyRatings) > 0 {
			blocked := false
			for _, rating := range result.SafetyRatings {
				if rating.Blocked {
					blocked = true
					safetyInfo += fmt.Sprintf(" Blocked by Safety Category: %s;", rating.Category)
				}
			}
			if blocked {
				safetyInfo = " Safety Blocking:" + safetyInfo
			}
		}

		logger.Fatal("Received empty response from Gemini.%s%s", finishReason, safetyInfo)
	}

	// Check for whitespace-only content
	if strings.TrimSpace(result.Content) == "" {
		logger.Fatal("Gemini returned an empty plan text.")
	}

	return result.Content
}

// saveToFile writes the generated plan to the specified file
func saveToFile(content string, outputFile string, logger logutil.LoggerInterface) {
	// Ensure output path is absolute
	outputPath := outputFile
	if !filepath.IsAbs(outputPath) {
		cwd, err := os.Getwd()
		if err != nil {
			logger.Fatal("Error getting current working directory: %v", err)
		}
		outputPath = filepath.Join(cwd, outputPath)
	}

	// Write to file
	logger.Info("Writing plan to %s...", outputPath)
	err := os.WriteFile(outputPath, []byte(content), 0644)
	if err != nil {
		logger.Fatal("Error writing plan to file %s: %v", outputPath, err)
	}

	logger.Info("Successfully generated plan and saved to %s", outputPath)
}

// initSpinner creates and configures a spinner instance based on config
func initSpinner(config *Configuration, logger logutil.LoggerInterface) *spinner.Spinner {
	// Configure spinner options based on user preferences
	spinnerOptions := &spinner.Options{
		Enabled:     !config.NoSpinner,
		CharSet:     14,  // Use circle dots by default
		RefreshRate: 100, // ms
		Output:      os.Stdout,
		Prefix:      " ",
		Suffix:      " ",
	}

	// If debug mode, disable spinner to avoid cluttering logs
	if config.LogLevel == logutil.DebugLevel {
		spinnerOptions.Enabled = false
	}

	// Create spinner instance
	return spinner.New(logger, spinnerOptions)
}

// promptForConfirmation asks for user confirmation to proceed
func promptForConfirmation(tokenCount int32, threshold int, logger logutil.LoggerInterface) bool {
	if threshold <= 0 || int32(threshold) > tokenCount {
		// No confirmation needed if threshold is disabled (0) or token count is below threshold
		return true
	}

	logger.Info("Token count (%d) exceeds confirmation threshold (%d).", tokenCount, threshold)
	logger.Info("Do you want to proceed with the API call? [y/N]: ")

	reader := bufio.NewReader(os.Stdin)
	response, err := reader.ReadString('\n')
	if err != nil {
		logger.Error("Error reading input: %v", err)
		return false
	}

	// Trim whitespace and convert to lowercase
	response = strings.ToLower(strings.TrimSpace(response))

	// Only proceed if the user explicitly confirms with 'y' or 'yes'
	return response == "y" || response == "yes"
}

// tokenInfoResult holds information about token counts and limits
type tokenInfoResult struct {
	tokenCount   int32
	inputLimit   int32
	exceedsLimit bool
	limitError   string
	percentage   float64
}

// getTokenInfo gets token count information and checks limits
func getTokenInfo(ctx context.Context, geminiClient gemini.Client, prompt string, logger logutil.LoggerInterface) (*tokenInfoResult, error) {
	// Create result structure
	result := &tokenInfoResult{
		exceedsLimit: false,
	}

	// Get model information (limits)
	modelInfo, err := geminiClient.GetModelInfo(ctx)
	if err != nil {
		// Pass through API errors directly for better error messages
		if _, ok := gemini.IsAPIError(err); ok {
			return nil, err
		}

		// Wrap other errors
		return nil, fmt.Errorf("failed to get model info for token limit check: %w", err)
	}

	// Store input limit
	result.inputLimit = modelInfo.InputTokenLimit

	// Count tokens in the prompt
	tokenResult, err := geminiClient.CountTokens(ctx, prompt)
	if err != nil {
		// Pass through API errors directly for better error messages
		if _, ok := gemini.IsAPIError(err); ok {
			return nil, err
		}

		// Wrap other errors
		return nil, fmt.Errorf("failed to count tokens for token limit check: %w", err)
	}

	// Store token count
	result.tokenCount = tokenResult.Total

	// Calculate percentage of limit
	result.percentage = float64(result.tokenCount) / float64(result.inputLimit) * 100

	// Log token usage information
	logger.Debug("Token usage: %d / %d (%.1f%%)",
		result.tokenCount,
		result.inputLimit,
		result.percentage)

	// Check if the prompt exceeds the token limit
	if result.tokenCount > result.inputLimit {
		result.exceedsLimit = true
		result.limitError = fmt.Sprintf("prompt exceeds token limit (%d tokens > %d token limit)",
			result.tokenCount, result.inputLimit)
	}

	return result, nil
}

// checkTokenLimit verifies that the prompt doesn't exceed the model's token limit
// Deprecated: Use getTokenInfo instead
func checkTokenLimit(ctx context.Context, geminiClient gemini.Client, prompt string, logger logutil.LoggerInterface) error {
	tokenInfo, err := getTokenInfo(ctx, geminiClient, prompt, logger)
	if err != nil {
		return err
	}

	if tokenInfo.exceedsLimit {
		return fmt.Errorf(tokenInfo.limitError)
	}

	return nil
}

// initConfigSystem initializes the configuration system
func initConfigSystem(logger logutil.LoggerInterface) config.ManagerInterface {
	return config.NewManager(logger)
}

// convertConfigToMap converts the CLI Configuration struct to a map for merging with loaded config
func convertConfigToMap(cliConfig *Configuration) map[string]interface{} {
	// Create a map of CLI flags suitable for merging
	return map[string]interface{}{
		"task_description":    cliConfig.TaskDescription,
		"task_file":           cliConfig.TaskFile,
		"output_file":         cliConfig.OutputFile,
		"model":               cliConfig.ModelName,
		"verbose":             cliConfig.Verbose,
		"log_level":           cliConfig.LogLevel,
		"use_colors":          cliConfig.UseColors,
		"include":             cliConfig.Include,
		"format":              cliConfig.Format,
		"clarify_task":        cliConfig.ClarifyTask,
		"dry_run":             cliConfig.DryRun,
		"confirm_tokens":      cliConfig.ConfirmTokens,
		"no_spinner":          cliConfig.NoSpinner,
		"paths":               cliConfig.Paths,
		"api_key":             cliConfig.ApiKey,
		"templates.default":   cliConfig.PromptTemplate, // Map prompt template to config format
		"excludes.extensions": cliConfig.Exclude,
		"excludes.names":      cliConfig.ExcludeNames,
	}
}

// backfillConfigFromAppConfig creates a Configuration object from AppConfig for backward compatibility
func backfillConfigFromAppConfig(cliConfig *Configuration, appConfig *config.AppConfig) *Configuration {
	// Start with CLI config to preserve fields not in AppConfig
	config := *cliConfig

	// Override with values from AppConfig only if they weren't explicitly set via CLI
	if flag.Lookup("output").Value.String() == defaultOutputFile {
		config.OutputFile = appConfig.OutputFile
	}
	if flag.Lookup("model").Value.String() == defaultModel {
		config.ModelName = appConfig.ModelName
	}
	if !isFlagSet("verbose") {
		config.Verbose = appConfig.Verbose
	}
	if !isFlagSet("color") {
		config.UseColors = appConfig.UseColors
	}
	if !isFlagSet("include") {
		config.Include = appConfig.Include
	}
	if !isFlagSet("exclude") {
		config.Exclude = appConfig.Excludes.Extensions
	}
	if !isFlagSet("exclude-names") {
		config.ExcludeNames = appConfig.Excludes.Names
	}
	if !isFlagSet("format") {
		config.Format = appConfig.Format
	}
	if !isFlagSet("confirm-tokens") {
		config.ConfirmTokens = appConfig.ConfirmTokens
	}
	if !isFlagSet("no-spinner") {
		config.NoSpinner = appConfig.NoSpinner
	}
	if !isFlagSet("clarify") {
		config.ClarifyTask = appConfig.ClarifyTask
	}
	if !isFlagSet("prompt-template") && appConfig.Templates.Default != "" {
		config.PromptTemplate = appConfig.Templates.Default
	}

	return &config
}

// isFlagSet checks if a flag was explicitly set on the command line
func isFlagSet(name string) bool {
	found := false
	flag.Visit(func(f *flag.Flag) {
		if f.Name == name {
			found = true
		}
	})
	return found
}

// buildPrompt constructs the prompt string for the Gemini API.
func buildPrompt(config *Configuration, task string, context string, logger logutil.LoggerInterface) (string, error) {
	// Use config-less version for backward compatibility
	return buildPromptWithManager(config, task, context, prompt.NewManager(logger), logger)
}

// buildPromptWithConfig constructs the prompt string using the configuration system
func buildPromptWithConfig(config *Configuration, task string, context string, configManager config.ManagerInterface, logger logutil.LoggerInterface) (string, error) {
	// Create a prompt manager with config support
	promptManager, err := prompt.SetupPromptManagerWithConfig(logger, configManager)
	if err != nil {
		return "", fmt.Errorf("failed to set up prompt manager: %w", err)
	}

	return buildPromptWithManager(config, task, context, promptManager, logger)
}

// buildPromptWithManager constructs the prompt string using the provided prompt manager.
// This function is exported for testing purposes.
func buildPromptWithManager(config *Configuration, task string, context string, promptManager prompt.ManagerInterface, logger logutil.LoggerInterface) (string, error) {
	// Create template data
	data := &prompt.TemplateData{
		Task:    task,
		Context: context, // context already has the <context> tags from fileutil
	}

	// Determine which template to use
	templateName := "default.tmpl"
	if config.PromptTemplate != "" {
		templateName = config.PromptTemplate
		logger.Debug("Using custom prompt template: %s", templateName)
	}

	// Build the prompt (template loading is now handled by the manager)
	generatedPrompt, err := promptManager.BuildPrompt(templateName, data)
	if err != nil {
		return "", fmt.Errorf("failed to build prompt: %w", err)
	}

	return generatedPrompt, nil
}
