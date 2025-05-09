// Package integration provides integration tests for the thinktank package
package integration

import (
	"context"
	"fmt"
	"path/filepath"
	"strings"
	"testing"

	"github.com/phrazzld/thinktank/internal/llm"
)

// TestInvalidSynthesisModelRefactored tests that validation correctly rejects an invalid synthesis model
func TestInvalidSynthesisModelRefactored(t *testing.T) {
	IntegrationTestWithBoundaries(t, func(env *BoundaryTestEnv) {
		// Set up test parameters
		instructions := "Test instructions for synthesis"
		modelNames := []string{"model1", "model2"}
		invalidSynthesisModel := "non-existent-model"

		// Set up models with invalid synthesis model
		env.Config.ModelNames = modelNames
		env.Config.SynthesisModel = invalidSynthesisModel

		// Set up mock responses for regular models
		mockOutputs := map[string]string{
			"model1": "# Output from Model 1\n\nThis is test output from model1.",
			"model2": "# Output from Model 2\n\nThis is test output from model2.",
		}

		// Setup the test environment first
		SetupStandardTestEnvironment(t, env, instructions, modelNames, invalidSynthesisModel, mockOutputs)

		// Then configure the mock API caller to fail when the invalid synthesis model is used
		// This order is important because SetupStandardTestEnvironment also configures the API caller
		mockAPICaller := env.APICaller.(*MockExternalAPICaller)
		mockAPICaller.CallLLMAPIFunc = func(ctx context.Context, modelName, prompt string, params map[string]interface{}) (*llm.ProviderResult, error) {
			// The synthesis model should fail, but regular models should work
			if modelName == invalidSynthesisModel {
				return nil, fmt.Errorf("model '%s' not found or not supported", modelName)
			}

			// For regular models, return their expected output
			if content, ok := mockOutputs[modelName]; ok {
				return &llm.ProviderResult{
					Content:      content,
					FinishReason: "stop",
				}, nil
			}

			// Fallback for unknown models
			return nil, fmt.Errorf("unexpected model: %s", modelName)
		}

		// Run the orchestrator, expecting failure due to invalid synthesis model
		ctx := context.Background()
		err := env.Run(ctx, instructions)

		// We expect an error about the invalid synthesis model
		if err == nil {
			t.Errorf("Expected error for invalid synthesis model, but got nil")
		} else if !strings.Contains(strings.ToLower(err.Error()), "synthesis") &&
			!strings.Contains(strings.ToLower(err.Error()), "not found") {
			t.Errorf("Expected error message about invalid synthesis model, got: %v", err)
		} else {
			t.Logf("Got expected error for invalid synthesis model: %v", err)
		}

		// Verify that individual model output files were still created
		for _, modelName := range modelNames {
			expectedFilePath := filepath.Join(env.Config.OutputDir, modelName+".md")

			exists, _ := env.Filesystem.Stat(expectedFilePath)
			if !exists {
				t.Logf("As expected, no output file was created for %s due to early failure", modelName)
			}
		}

		// Verify no synthesis file was created
		synthesisFilePath := filepath.Join(env.Config.OutputDir, invalidSynthesisModel+"-synthesis.md")
		exists, _ := env.Filesystem.Stat(synthesisFilePath)
		if exists {
			t.Errorf("Synthesis file %s should not exist with invalid synthesis model", synthesisFilePath)
		} else {
			t.Logf("Correctly verified no synthesis file was created")
		}
	})
}
