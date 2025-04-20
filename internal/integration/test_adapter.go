// internal/integration/test_adapter.go
package integration

import (
	"context"

	"github.com/phrazzld/architect/internal/gemini"
	"github.com/phrazzld/architect/internal/llm"
)

// LLMClientAdapter adapts a gemini.Client to implement the llm.LLMClient interface
// This provides consistency between the gemini.MockClient and llm.LLMClient interfaces
// in tests, ensuring both return the same responses configured in the MockClient
type LLMClientAdapter struct {
	geminiClient gemini.Client
	modelName    string
}

// NewLLMClientAdapter creates a new adapter wrapping a gemini.Client
func NewLLMClientAdapter(client gemini.Client, modelName string) *LLMClientAdapter {
	return &LLMClientAdapter{
		geminiClient: client,
		modelName:    modelName,
	}
}

// GenerateContent adapts gemini.Client.GenerateContent to llm.LLMClient.GenerateContent
func (a *LLMClientAdapter) GenerateContent(ctx context.Context, prompt string, params map[string]interface{}) (*llm.ProviderResult, error) {
	// Call the wrapped gemini client's GenerateContent method
	result, err := a.geminiClient.GenerateContent(ctx, prompt, params)
	if err != nil {
		return nil, err // Pass the error through directly
	}

	// Convert gemini.GenerationResult to llm.ProviderResult
	return &llm.ProviderResult{
		Content:      result.Content,
		FinishReason: result.FinishReason,
	}, nil
}
// GetModelName returns the model name stored in the adapter
func (a *LLMClientAdapter) GetModelName() string {
	return a.modelName
}

// Close adapts gemini.Client.Close to llm.LLMClient.Close
func (a *LLMClientAdapter) Close() error {
	return a.geminiClient.Close()
}
