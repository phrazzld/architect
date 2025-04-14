package orchestrator

import (
	"context"
	"strings"
	"testing"
	"time"

	"github.com/phrazzld/architect/internal/architect/interfaces"
	"github.com/phrazzld/architect/internal/fileutil"
)

// TestRun_DryRun tests the Run method in dry run mode
func TestRun_DryRun(t *testing.T) {
	ctx := context.Background()
	deps := newTestDeps()
	deps.setupDryRunConfig()
	deps.setupBasicContext()

	// Run the orchestrator in dry run mode
	err := deps.runOrchestrator(ctx, deps.instructions)
	if err != nil {
		t.Fatalf("Expected no error in dry run mode, got: %v", err)
	}

	// Verify dry run workflow
	deps.verifyDryRunWorkflow(t)

	// Verify that DisplayDryRunInfo was called
	if len(deps.contextGatherer.DisplayDryRunInfoCalls) != 1 {
		t.Errorf("Expected 1 call to DisplayDryRunInfo, got %d", len(deps.contextGatherer.DisplayDryRunInfoCalls))
	}

	// Verify that token checks were still performed
	if len(deps.tokenManager.CheckTokenLimitCalls) == 0 {
		t.Error("Expected token checks to be performed in dry run mode")
	}
}

// TestRun_ModelProcessing tests the Run method with model processing
func TestRun_ModelProcessing(t *testing.T) {
	ctx := context.Background()
	deps := newTestDeps()
	modelNames := []string{"model1", "model2"}
	deps.setupMultiModelConfig(modelNames)
	deps.setupBasicContext()

	// Setup client for each model
	deps.setupGeminiClient()

	// Run the orchestrator
	err := deps.runOrchestrator(ctx, deps.instructions)
	if err != nil {
		t.Fatalf("Expected no error, got: %v", err)
	}

	// Verify the basic workflow executed correctly
	deps.verifyBasicWorkflow(t, modelNames)

	// Verify that the API client was initialized for each model
	if len(deps.apiService.InitClientCalls) != len(modelNames) {
		t.Errorf("Expected %d InitClient calls, got %d", len(modelNames), len(deps.apiService.InitClientCalls))
	}

	// Verify each model was processed with the correct data
	for i, modelName := range modelNames {
		// Check model name
		if deps.apiService.InitClientCalls[i].ModelName != modelName {
			t.Errorf("Expected model name %s, got %s", modelName, deps.apiService.InitClientCalls[i].ModelName)
		}

		// Check output file path contains model name
		if !strings.Contains(deps.fileWriter.SaveToFileCalls[i].OutputFile, modelName) {
			t.Errorf("Expected output file path to contain model name %s, got %s",
				modelName, deps.fileWriter.SaveToFileCalls[i].OutputFile)
		}
	}
}

// TestRun_ContextCancellation tests that the Run method respects context cancellation
func TestRun_ContextCancellation(t *testing.T) {
	// Create a context with cancellation
	ctx, cancel := context.WithCancel(context.Background())

	deps := newTestDeps()
	modelNames := []string{"model1", "model2", "model3"}
	deps.setupMultiModelConfig(modelNames)

	// Set up a slow context gathering operation that gives us time to cancel
	deps.contextGatherer.GatherContextFunc = func(ctx context.Context, config interfaces.GatherConfig) ([]fileutil.FileMeta, *interfaces.ContextStats, error) {
		// Wait a bit to simulate work
		select {
		case <-time.After(100 * time.Millisecond):
			// This simulates the normal completion path
			return []fileutil.FileMeta{}, &interfaces.ContextStats{}, nil
		case <-ctx.Done():
			// This is what should happen when we cancel
			return nil, nil, ctx.Err()
		}
	}

	// Cancel the context after a short delay
	go func() {
		time.Sleep(50 * time.Millisecond)
		cancel()
	}()

	// Run the orchestrator with the cancellable context
	err := deps.runOrchestrator(ctx, deps.instructions)

	// Verify that we got a cancellation error
	if err == nil {
		t.Fatal("Expected an error due to cancellation, got nil")
	}

	if !strings.Contains(err.Error(), "context canceled") && !strings.Contains(err.Error(), "deadline exceeded") {
		t.Errorf("Expected context cancellation error, got: %v", err)
	}
}

// TestBuildPrompt tests the buildPrompt method
func TestBuildPrompt(t *testing.T) {
	orch := &Orchestrator{
		logger: &mockLogger{},
	}

	// Test cases
	testCases := []struct {
		name          string
		instructions  string
		files         []fileutil.FileMeta
		expectedParts []string // Strings that should appear in the prompt
	}{
		{
			name:         "basic prompt",
			instructions: "Test instructions",
			files: []fileutil.FileMeta{
				{Path: "file1.go", Content: "package main"},
				{Path: "file2.go", Content: "func test() {}"},
			},
			expectedParts: []string{
				"Test instructions",
				"file1.go",
				"package main",
				"file2.go",
				"func test() {}",
			},
		},
		{
			name:         "empty files",
			instructions: "Empty test",
			files:        []fileutil.FileMeta{},
			expectedParts: []string{
				"Empty test",
				"No files were provided",
			},
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			result := orch.buildPrompt(tc.instructions, tc.files)

			// Check that the result contains expected parts
			for _, part := range tc.expectedParts {
				if !strings.Contains(result, part) {
					t.Errorf("Expected prompt to contain '%s', but it doesn't. Prompt: %s", part, result)
				}
			}
		})
	}
}
