package architect

import (
	"context"
	"errors"
	"os"
	"testing"

	"github.com/phrazzld/architect/internal/gemini"
	"github.com/phrazzld/architect/internal/llm"
	"github.com/phrazzld/architect/internal/logutil"
)

type mockLogger struct {
	logutil.LoggerInterface
	debugCalled bool
}

func (m *mockLogger) Debug(format string, args ...interface{}) {
	m.debugCalled = true
}

func TestGetTokenInfo(t *testing.T) {
	logger := &mockLogger{}
	ctx := context.Background()

	// Test successful token count
	t.Run("Success", func(t *testing.T) {
		mockClient := &gemini.MockClient{
			GetModelInfoFunc: func(ctx context.Context) (*gemini.ModelInfo, error) {
				return &gemini.ModelInfo{
					Name:             "test-model",
					InputTokenLimit:  1000,
					OutputTokenLimit: 500,
				}, nil
			},
			CountTokensFunc: func(ctx context.Context, prompt string) (*gemini.TokenCount, error) {
				return &gemini.TokenCount{
					Total: 42,
				}, nil
			},
		}

		tokenManager, err := NewTokenManager(logger, mockClient)
		if err != nil {
			t.Fatalf("Failed to create token manager: %v", err)
		}

		result, err := tokenManager.GetTokenInfo(ctx, "test prompt")
		if err != nil {
			t.Fatalf("Expected no error, got %v", err)
		}

		if result.TokenCount != 42 {
			t.Errorf("Expected token count 42, got %d", result.TokenCount)
		}

		if result.InputLimit != 1000 {
			t.Errorf("Expected input limit 1000, got %d", result.InputLimit)
		}

		if result.Percentage != 4.2 {
			t.Errorf("Expected percentage 4.2, got %.1f", result.Percentage)
		}

		if result.ExceedsLimit {
			t.Error("Expected no token limit exceeded, got exceeded")
		}

		if !logger.debugCalled {
			t.Error("Expected logger.Debug to be called, but it wasn't")
		}
	})

	// Test token limit exceeded
	t.Run("ExceedsLimit", func(t *testing.T) {
		mockClient := &gemini.MockClient{
			GetModelInfoFunc: func(ctx context.Context) (*gemini.ModelInfo, error) {
				return &gemini.ModelInfo{
					Name:             "test-model",
					InputTokenLimit:  100,
					OutputTokenLimit: 50,
				}, nil
			},
			CountTokensFunc: func(ctx context.Context, prompt string) (*gemini.TokenCount, error) {
				return &gemini.TokenCount{
					Total: 150,
				}, nil
			},
		}

		tokenManager, err := NewTokenManager(logger, mockClient)
		if err != nil {
			t.Fatalf("Failed to create token manager: %v", err)
		}

		result, err := tokenManager.GetTokenInfo(ctx, "test prompt")
		if err != nil {
			t.Fatalf("Expected no error, got %v", err)
		}

		if !result.ExceedsLimit {
			t.Error("Expected token limit exceeded, got not exceeded")
		}

		if result.Percentage != 150.0 {
			t.Errorf("Expected percentage 150.0, got %.1f", result.Percentage)
		}

		if result.LimitError == "" {
			t.Error("Expected non-empty limit error, got empty string")
		}
	})

	// Test model info error
	t.Run("ModelInfoError", func(t *testing.T) {
		mockClient := &gemini.MockClient{
			GetModelInfoFunc: func(ctx context.Context) (*gemini.ModelInfo, error) {
				return nil, errors.New("model info error")
			},
		}

		tokenManager, err := NewTokenManager(logger, mockClient)
		if err != nil {
			t.Fatalf("Failed to create token manager: %v", err)
		}

		_, err = tokenManager.GetTokenInfo(ctx, "test prompt")
		if err == nil {
			t.Error("Expected error when GetModelInfo fails, got nil")
		}
	})

	// Test count tokens error
	t.Run("CountTokensError", func(t *testing.T) {
		mockClient := &gemini.MockClient{
			GetModelInfoFunc: func(ctx context.Context) (*gemini.ModelInfo, error) {
				return &gemini.ModelInfo{
					Name:             "test-model",
					InputTokenLimit:  1000,
					OutputTokenLimit: 500,
				}, nil
			},
			CountTokensFunc: func(ctx context.Context, prompt string) (*gemini.TokenCount, error) {
				return nil, errors.New("count tokens error")
			},
		}

		tokenManager, err := NewTokenManager(logger, mockClient)
		if err != nil {
			t.Fatalf("Failed to create token manager: %v", err)
		}

		_, err = tokenManager.GetTokenInfo(ctx, "test prompt")
		if err == nil {
			t.Error("Expected error when CountTokens fails, got nil")
		}
	})

	// Test API error in GetModelInfo
	t.Run("APIErrorInGetModelInfo", func(t *testing.T) {
		apiErr := gemini.CreateAPIError(
			llm.CategoryInvalidRequest,
			"API error",
			errors.New("model info request failed"),
			"",
		)
		mockClient := &gemini.MockClient{
			GetModelInfoFunc: func(ctx context.Context) (*gemini.ModelInfo, error) {
				return nil, apiErr
			},
		}

		tokenManager, err := NewTokenManager(logger, mockClient)
		if err != nil {
			t.Fatalf("Failed to create token manager: %v", err)
		}

		_, err = tokenManager.GetTokenInfo(ctx, "test prompt")
		if err != apiErr {
			t.Errorf("Expected API error to be passed through, got: %v", err)
		}
	})

	// Test nil client
	t.Run("NilClient", func(t *testing.T) {
		_, err := NewTokenManager(logger, nil)
		if err == nil {
			t.Error("Expected error when client is nil, got nil")
		}
	})
}

func TestCheckTokenLimit(t *testing.T) {
	logger := &mockLogger{}
	ctx := context.Background()

	// Test no limit exceeded
	t.Run("NotExceeded", func(t *testing.T) {
		mockClient := &gemini.MockClient{
			GetModelInfoFunc: func(ctx context.Context) (*gemini.ModelInfo, error) {
				return &gemini.ModelInfo{
					Name:             "test-model",
					InputTokenLimit:  1000,
					OutputTokenLimit: 500,
				}, nil
			},
			CountTokensFunc: func(ctx context.Context, prompt string) (*gemini.TokenCount, error) {
				return &gemini.TokenCount{
					Total: 42,
				}, nil
			},
		}

		tokenManager, err := NewTokenManager(logger, mockClient)
		if err != nil {
			t.Fatalf("Failed to create token manager: %v", err)
		}

		err = tokenManager.CheckTokenLimit(ctx, "test prompt")
		if err != nil {
			t.Errorf("Expected no error, got %v", err)
		}
	})

	// Test limit exceeded
	t.Run("Exceeded", func(t *testing.T) {
		mockClient := &gemini.MockClient{
			GetModelInfoFunc: func(ctx context.Context) (*gemini.ModelInfo, error) {
				return &gemini.ModelInfo{
					Name:             "test-model",
					InputTokenLimit:  100,
					OutputTokenLimit: 50,
				}, nil
			},
			CountTokensFunc: func(ctx context.Context, prompt string) (*gemini.TokenCount, error) {
				return &gemini.TokenCount{
					Total: 150,
				}, nil
			},
		}

		tokenManager, err := NewTokenManager(logger, mockClient)
		if err != nil {
			t.Fatalf("Failed to create token manager: %v", err)
		}

		err = tokenManager.CheckTokenLimit(ctx, "test prompt")
		if err == nil {
			t.Error("Expected error when token limit exceeded, got nil")
		}
	})
}

func TestPromptForConfirmation(t *testing.T) {
	if os.Getenv("TEST_INTERACTIVE") != "1" {
		t.Skip("Skipping interactive test")
	}

	logger := &mockLogger{}
	mockClient := &gemini.MockClient{}
	tokenManager, _ := NewTokenManager(logger, mockClient)

	// Below threshold
	result := tokenManager.PromptForConfirmation(100, 150)
	if !result {
		t.Error("Expected confirmation when token count is below threshold")
	}

	// Threshold disabled
	result = tokenManager.PromptForConfirmation(100, 0)
	if !result {
		t.Error("Expected confirmation when threshold is disabled")
	}

	// Note: We can't easily test the interactive confirmation without mocking os.Stdin
	// A proper test would use a custom reader, but that's beyond the scope of this example
}
