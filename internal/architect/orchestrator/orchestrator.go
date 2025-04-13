// Package orchestrator is responsible for coordinating the core application workflow.
// It brings together various components like context gathering, API interaction,
// token management, and output writing to execute the main task defined
// by user instructions and configuration.
package orchestrator

import (
	"context"
	"errors"
	"fmt"
	"strings"
	"sync"
	"time"

	"github.com/phrazzld/architect/internal/architect/interfaces"
	"github.com/phrazzld/architect/internal/architect/modelproc"
	"github.com/phrazzld/architect/internal/architect/prompt"
	"github.com/phrazzld/architect/internal/auditlog"
	"github.com/phrazzld/architect/internal/config"
	"github.com/phrazzld/architect/internal/fileutil"
	"github.com/phrazzld/architect/internal/gemini"
	"github.com/phrazzld/architect/internal/logutil"
	"github.com/phrazzld/architect/internal/ratelimit"
)

// Orchestrator coordinates the main application logic.
// It depends on various services to perform tasks like interacting with the API,
// gathering context, managing tokens, writing files, logging audits, and handling rate limits.
type Orchestrator struct {
	apiService      interfaces.APIService
	contextGatherer interfaces.ContextGatherer
	tokenManager    interfaces.TokenManager
	fileWriter      interfaces.FileWriter
	auditLogger     auditlog.AuditLogger
	rateLimiter     *ratelimit.RateLimiter
	config          *config.CliConfig
	logger          logutil.LoggerInterface
}

// NewOrchestrator creates a new instance of the Orchestrator.
// It requires all necessary dependencies to be provided during construction,
// ensuring that the orchestrator is properly configured to execute its tasks.
func NewOrchestrator(
	apiService interfaces.APIService,
	contextGatherer interfaces.ContextGatherer,
	tokenManager interfaces.TokenManager,
	fileWriter interfaces.FileWriter,
	auditLogger auditlog.AuditLogger,
	rateLimiter *ratelimit.RateLimiter,
	config *config.CliConfig,
	logger logutil.LoggerInterface,
) *Orchestrator {
	return &Orchestrator{
		apiService:      apiService,
		contextGatherer: contextGatherer,
		tokenManager:    tokenManager,
		fileWriter:      fileWriter,
		auditLogger:     auditLogger,
		rateLimiter:     rateLimiter,
		config:          config,
		logger:          logger,
	}
}

// Run executes the main application workflow.
// It manages context gathering, prompt creation, and model processing.
func (o *Orchestrator) Run(ctx context.Context, instructions string) error {
	// STEP 1: Gather context from files
	contextFiles, contextStats, err := o.gatherProjectContext(ctx)
	if err != nil {
		return err
	}

	// STEP 2: Handle dry run mode (short-circuit if dry run)
	if o.config.DryRun {
		return o.handleDryRun(ctx, contextStats)
	}

	// STEP 3: Build prompt by combining instructions and context
	stitchedPrompt := o.buildPrompt(instructions, contextFiles)

	// STEP 4: Process models concurrently
	o.logRateLimitingConfiguration()
	modelErrors := o.processModels(ctx, stitchedPrompt)

	// STEP 5: Handle any errors from model processing
	if len(modelErrors) > 0 {
		return o.aggregateAndFormatErrors(modelErrors)
	}

	return nil
}

// gatherProjectContext collects relevant files from the project based on configuration.
func (o *Orchestrator) gatherProjectContext(ctx context.Context) ([]fileutil.FileMeta, *interfaces.ContextStats, error) {
	gatherConfig := interfaces.GatherConfig{
		Paths:        o.config.Paths,
		Include:      o.config.Include,
		Exclude:      o.config.Exclude,
		ExcludeNames: o.config.ExcludeNames,
		Format:       o.config.Format,
		Verbose:      o.config.Verbose,
		LogLevel:     o.config.LogLevel,
	}

	contextFiles, contextStats, err := o.contextGatherer.GatherContext(ctx, gatherConfig)
	if err != nil {
		return nil, nil, fmt.Errorf("failed during project context gathering: %w", err)
	}

	return contextFiles, contextStats, nil
}

// handleDryRun displays context statistics without performing API calls.
func (o *Orchestrator) handleDryRun(ctx context.Context, stats *interfaces.ContextStats) error {
	err := o.contextGatherer.DisplayDryRunInfo(ctx, stats)
	if err != nil {
		o.logger.Error("Error displaying dry run information: %v", err)
		return fmt.Errorf("error displaying dry run information: %w", err)
	}
	return nil
}

// buildPrompt creates the complete prompt by combining instructions with context files.
func (o *Orchestrator) buildPrompt(instructions string, contextFiles []fileutil.FileMeta) string {
	stitchedPrompt := prompt.StitchPrompt(instructions, contextFiles)
	o.logger.Info("Prompt constructed successfully")
	o.logger.Debug("Stitched prompt length: %d characters", len(stitchedPrompt))
	return stitchedPrompt
}

// logRateLimitingConfiguration logs information about concurrency and rate limits.
func (o *Orchestrator) logRateLimitingConfiguration() {
	if o.config.MaxConcurrentRequests > 0 {
		o.logger.Info("Concurrency limited to %d simultaneous requests", o.config.MaxConcurrentRequests)
	} else {
		o.logger.Info("No concurrency limit applied")
	}

	if o.config.RateLimitRequestsPerMinute > 0 {
		o.logger.Info("Rate limited to %d requests per minute per model", o.config.RateLimitRequestsPerMinute)
	} else {
		o.logger.Info("No rate limit applied")
	}

	o.logger.Info("Processing %d models concurrently...", len(o.config.ModelNames))
}

// processModels processes each model concurrently with rate limiting.
// Returns a slice of errors encountered during processing.
func (o *Orchestrator) processModels(ctx context.Context, stitchedPrompt string) []error {
	var wg sync.WaitGroup
	errChan := make(chan error, len(o.config.ModelNames))

	// Launch a goroutine for each model
	for _, modelName := range o.config.ModelNames {
		wg.Add(1)
		go o.processModelWithRateLimit(ctx, modelName, stitchedPrompt, &wg, errChan)
	}

	// Wait for all goroutines to complete
	wg.Wait()
	close(errChan)

	// Collect errors from the channel
	var modelErrors []error
	for err := range errChan {
		modelErrors = append(modelErrors, err)
	}

	return modelErrors
}

// processModelWithRateLimit processes a single model with rate limiting.
func (o *Orchestrator) processModelWithRateLimit(
	ctx context.Context,
	modelName string,
	stitchedPrompt string,
	wg *sync.WaitGroup,
	errChan chan<- error,
) {
	defer wg.Done()

	// Acquire rate limiting permission
	o.logger.Debug("Attempting to acquire rate limiter for model %s...", modelName)
	acquireStart := time.Now()
	if err := o.rateLimiter.Acquire(ctx, modelName); err != nil {
		o.logger.Error("Rate limiting error for model %s: %v", modelName, err)
		errChan <- fmt.Errorf("model %s rate limit: %w", modelName, err)
		return
	}
	acquireDuration := time.Since(acquireStart)
	o.logger.Debug("Rate limiter acquired for model %s (waited %v)", modelName, acquireDuration)

	// Release rate limiter when done
	defer func() {
		o.logger.Debug("Releasing rate limiter for model %s", modelName)
		o.rateLimiter.Release()
	}()

	// Create API service adapter and model processor
	apiServiceAdapter := &APIServiceAdapter{APIService: o.apiService}
	processor := modelproc.NewProcessor(
		apiServiceAdapter,
		nil, // tokenManager is created inside the Process method
		o.fileWriter,
		o.auditLogger,
		o.logger,
		o.config,
	)

	// Process the model
	err := processor.Process(ctx, modelName, stitchedPrompt)
	if err != nil {
		o.logger.Error("Processing model %s failed: %v", modelName, err)
		errChan <- fmt.Errorf("model %s: %w", modelName, err)
	}
}

// aggregateAndFormatErrors combines multiple errors into a single, user-friendly error message.
func (o *Orchestrator) aggregateAndFormatErrors(modelErrors []error) error {
	// Count rate limit errors
	var rateLimitErrors []error
	for _, err := range modelErrors {
		if strings.Contains(err.Error(), "rate limit") {
			rateLimitErrors = append(rateLimitErrors, err)
		}
	}

	// Build the error message
	errMsg := "errors occurred during model processing:"
	for _, e := range modelErrors {
		errMsg += "\n  - " + e.Error()
	}

	// Add rate limit guidance if applicable
	if len(rateLimitErrors) > 0 {
		errMsg += "\n\nTip: If you're encountering rate limit errors, consider adjusting the --max-concurrent and --rate-limit flags to prevent overwhelming the API."
	}

	return errors.New(errMsg)
}

// APIServiceAdapter adapts interfaces.APIService to modelproc.APIService
type APIServiceAdapter struct {
	APIService interfaces.APIService
}

func (a *APIServiceAdapter) InitClient(ctx context.Context, apiKey, modelName string) (gemini.Client, error) {
	return a.APIService.InitClient(ctx, apiKey, modelName)
}

func (a *APIServiceAdapter) ProcessResponse(result *gemini.GenerationResult) (string, error) {
	return a.APIService.ProcessResponse(result)
}

func (a *APIServiceAdapter) IsEmptyResponseError(err error) bool {
	return a.APIService.IsEmptyResponseError(err)
}

func (a *APIServiceAdapter) IsSafetyBlockedError(err error) bool {
	return a.APIService.IsSafetyBlockedError(err)
}

func (a *APIServiceAdapter) GetErrorDetails(err error) string {
	return a.APIService.GetErrorDetails(err)
}

// TokenManagerAdapter adapts interfaces.TokenManager to modelproc.TokenManager
// TokenManagerAdapter is no longer needed since we create TokenManagers in the Process method
// The ModelProcessor now creates its own TokenManager directly with the specific client
// This adapter remains as a placeholder but is not used, and will be removed in future refactoring
