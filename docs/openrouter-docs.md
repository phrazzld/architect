# OpenRouter API Integration Guide for Go Developers

This guide details how to integrate the OpenRouter API into a Go application, focusing on direct HTTP interactions using standard Go libraries (`net/http`, `encoding/json`, etc.) or compatible OpenAI Go SDKs.

## Core Concepts

* **Unified Endpoint**: OpenRouter provides a single base URL for accessing various LLMs: `https://openrouter.ai/api/v1`
* **OpenAI Compatibility**: The API is designed to be largely compatible with the OpenAI API specification, particularly for chat completions. This allows using Go libraries built for OpenAI by configuring the base URL.
* **Model Identifiers**: Models are referenced using a `provider/model-name` format (e.g., `openai/gpt-4o`, `google/gemini-1.5-pro`, `anthropic/claude-3.5-sonnet`). A list of available models can be fetched from the `/models` endpoint [cite: 288-290].

## Authentication

* **Method**: Bearer Token Authentication.
* **Implementation**: Include an `Authorization` header in your HTTP requests with your OpenRouter API key.
* **Go Example (Conceptual Header)**:

    ```go
    req.Header.Set("Authorization", "Bearer <YOUR_OPENROUTER_API_KEY>")
    ```

* **API Key Management**: Keys are created and managed via the OpenRouter dashboard ([https://openrouter.ai/keys](https://openrouter.ai/keys)). You can also programmatically manage keys using Provisioning API Keys if needed [cite: 311-313].

## Making API Requests (Chat Completions)

The primary endpoint for generation is `/chat/completions`.

* **Endpoint**: `https://openrouter.ai/api/v1/chat/completions`
* **Method**: `POST`
* **Headers**:
    * `Authorization: Bearer <YOUR_OPENROUTER_API_KEY>` (Required)
    * `Content-Type: application/json` (Required)
    * `HTTP-Referer: <YOUR_APPLICATION_URL>` (Optional: For dashboard visibility/ranking)
    * `X-Title: <YOUR_APPLICATION_NAME>` (Optional: For dashboard visibility/ranking)
* **Request Body (JSON)**: The body should be a JSON object. You can define Go structs and marshal them using `encoding/json`.

    **Go Struct Example (Simplified)**:

    ```go
    package main

    import "encoding/json"

    type OpenRouterRequest struct {
        Model       string           `json:"model"`
        Messages    []Message        `json:"messages"`
        Temperature *float64         `json:"temperature,omitempty"` // Use pointers for optional fields
        MaxTokens   *int             `json:"max_tokens,omitempty"`
        Stream      bool             `json:"stream,omitempty"`
        Tools       []Tool           `json:"tools,omitempty"`
        ToolChoice  any              `json:"tool_choice,omitempty"` // Can be string or object
        Provider    *ProviderOptions `json:"provider,omitempty"`
        // Add other parameters like top_p, frequency_penalty etc. as needed [cite: 558-610]
    }

    type Message struct {
        Role    string `json:"role"` // "system", "user", "assistant", "tool"
        Content any    `json:"content"` // Can be string or []ContentPart for multimodal
        Name    string `json:"name,omitempty"`
        ToolCallID string `json:"tool_call_id,omitempty"` // Only for role "tool"
        ToolCalls  []ToolCall `json:"tool_calls,omitempty"` // Only for role "assistant" with tool calls
    }

    // For Multi-modal messages (role: "user")
    type ContentPart struct {
        Type     string    `json:"type"` // "text" or "image_url"
        Text     string    `json:"text,omitempty"`
        ImageURL *ImageURL `json:"image_url,omitempty"`
    }

    type ImageURL struct {
        URL    string `json:"url"` // "https://..." or "data:image/jpeg;base64,..."
        Detail string `json:"detail,omitempty"` // "low", "high", "auto"
    }

    // For Tool Use
    type Tool struct {
        Type     string    `json:"type"` // Currently only "function"
        Function Function  `json:"function"`
    }

    type Function struct {
        Name        string `json:"name"`
        Description string `json:"description,omitempty"`
        Parameters  any    `json:"parameters"` // Typically map[string]any representing JSON Schema
    }

     type ToolCall struct {
        ID       string       `json:"id"`
        Type     string       `json:"type"` // "function"
        Function FunctionCall `json:"function"`
    }

    type FunctionCall struct {
        Name      string `json:"name"`
        Arguments string `json:"arguments"` // JSON string
    }

    // For Provider Routing
    type ProviderOptions struct {
        Order           []string `json:"order,omitempty"`
        AllowFallbacks  *bool    `json:"allow_fallbacks,omitempty"`
        // Add other provider options... [cite: 188-198]
    }

    // Usage:
    // reqBody := OpenRouterRequest{
    //     Model: "google/gemini-1.5-pro",
    //     Messages: []Message{
    //         {Role: "user", Content: "Explain Go interfaces."},
    //     },
    //     // ... set other fields
    // }
    // jsonData, err := json.Marshal(reqBody)
    // // ... make HTTP POST request with jsonData
    ```

* **Using Go OpenAI SDKs**: Libraries like `github.com/sashabaranov/go-openai` can be used by configuring the `BaseURL` in the client config to `https://openrouter.ai/api/v1`. Pass the OpenRouter API key as the OpenAI key. Custom headers (`HTTP-Referer`, `X-Title`) might need specific handling depending on the library.

## Handling Responses

The API returns JSON responses. Define corresponding Go structs to unmarshal the response using `encoding/json`.

**Go Struct Example (Simplified Response)**:

```go
package main

import "encoding/json"

type OpenRouterResponse struct {
	ID      string   `json:"id"`
	Object  string   `json:"object"` // "chat.completion" or "chat.completion.chunk"
	Created int64    `json:"created"`
	Model   string   `json:"model"`
	Choices []Choice `json:"choices"`
	Usage   *Usage   `json:"usage,omitempty"` // Present in non-streaming, last chunk in streaming
	Error   *APIError `json:"error,omitempty"` // Check this for API-level errors
}

type Choice struct {
	Index        int          `json:"index"`
	Message      *Message     `json:"message,omitempty"` // For non-streaming response
	Delta        *Delta       `json:"delta,omitempty"`   // For streaming response
	FinishReason *string      `json:"finish_reason"`     // "stop", "length", "tool_calls", "content_filter", "error", null
	Logprobs     *LogProbs    `json:"logprobs,omitempty"`
	Error        *APIError    `json:"error,omitempty"` // Check this for per-choice errors during generation
}

// Delta is used for streaming responses
type Delta struct {
	Role      string     `json:"role,omitempty"`
	Content   *string    `json:"content"` // Use pointer to differentiate null/empty
	ToolCalls []ToolCall `json:"tool_calls,omitempty"`
}

type Usage struct {
	PromptTokens     int `json:"prompt_tokens"`
	CompletionTokens int `json:"completion_tokens"`
	TotalTokens      int `json:"total_tokens"`
}

type LogProbs struct {
    Content []TokenLogProb `json:"content"`
}

type TokenLogProb struct {
    Token   string  `json:"token"`
    LogProb float64 `json:"logprob"`
    Bytes   []byte  `json:"bytes"` // Can be null
    TopLogProbs []TopLogProb `json:"top_logprobs"`
}

type TopLogProb struct {
    Token   string  `json:"token"`
    LogProb float64 `json:"logprob"`
    Bytes   []byte  `json:"bytes"` // Can be null
}


type APIError struct {
	Code    any    `json:"code"` // Can be string or number depending on source
	Message string `json:"message"`
	Param   string `json:"param,omitempty"`
	Type    string `json:"type,omitempty"`
	// OpenRouter specific metadata might be here
	Metadata map[string]any `json:"metadata,omitempty"`
}

// Usage:
// var respData OpenRouterResponse
// err := json.NewDecoder(httpResponse.Body).Decode(&respData)
// // Check for errors, process respData.Choices...
Streaming ResponsesEnable: Set "stream": true in the request body.Format: Server-Sent Events (SSE). The server sends data chunks prefixed with data: . The stream ends with data: [DONE].Go Implementation:Make the HTTP request as usual.Read the response body iteratively using a bufio.Scanner or similar.For each line:Check if it starts with data: .If yes, strip the prefix.Check if the data is [DONE]. If yes, stop processing.Otherwise, unmarshal the JSON data (which will be an OpenRouterResponse chunk with a Delta field in Choices) into your Go struct.Append the Delta.Content to your result buffer. Handle Delta.Role on the first chunk and ToolCalls as they arrive.Handle potential SSE comments (lines starting with :) by ignoring them [cite: 472-474].Conceptual Go Streaming Snippet:package main

import (
	"bufio"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"strings"
)

func processStream(body io.ReadCloser) error {
	defer body.Close()
	scanner := bufio.NewScanner(body)
	fullResponse := ""

	for scanner.Scan() {
		line := scanner.Text()
		if strings.HasPrefix(line, "data: ") {
			data := strings.TrimPrefix(line, "data: ")
			if data == "[DONE]" {
				break // End of stream
			}

			var chunk OpenRouterResponse // Use the response struct defined earlier
			err := json.Unmarshal([]byte(data), &chunk)
			if err != nil {
				fmt.Printf("Error unmarshalling chunk: %v\n", err)
				continue // Or handle error more robustly
			}

			if len(chunk.Choices) > 0 && chunk.Choices[0].Delta != nil && chunk.Choices[0].Delta.Content != nil {
				fullResponse += *chunk.Choices[0].Delta.Content
				fmt.Print(*chunk.Choices[0].Delta.Content) // Print delta content as it arrives
			}
			// TODO: Handle ToolCalls, Role, Usage from last chunk, Errors etc.

		} else if strings.HasPrefix(line, ":") {
			// Ignore SSE comments
			fmt.Println("Received comment:", line)
		} else if line != "" {
			// Handle potential unexpected lines or errors in stream format
			fmt.Println("Received unexpected line:", line)
		}
	}
	fmt.Println("\n--- End of Stream ---")
	fmt.Println("Full Response:", fullResponse)

	if err := scanner.Err(); err != nil {
		return fmt.Errorf("error reading stream: %w", err)
	}
	return nil
}

// Usage:
// resp, err := httpClient.Do(req) // req should have stream:true
// // ... error check ...
// if resp.StatusCode == http.StatusOK {
//     err = processStream(resp.Body)
//     // ... error check ...
// } else {
//     // Handle non-200 status code (see Error Handling)
// }
Error HandlingHTTP Status Codes: OpenRouter uses standard HTTP status codes. Check http.Response.StatusCode.200 OK: Success (but check the JSON body for potential errors during generation).400 Bad Request: Invalid request JSON or parameters [cite: 540].401 Unauthorized: Invalid or missing API key [cite: 541].402 Payment Required: Insufficient credits [cite: 542].403 Forbidden: Input flagged by moderation [cite: 543].429 Too Many Requests: Rate limit exceeded [cite: 545].5xx Server Error: Issues on OpenRouter's or the provider's side [cite: 546-547].JSON Error Body: For non-200 responses, or for errors within a 200 OK streaming response, the JSON body (or a chunk) may contain an error object (see APIError struct above). Parse this for details.Key Parameters & Featuresmodel: (Required) Specify the model ID (e.g., openai/gpt-4o).messages: (Required) Array of message objects (role, content).temperature, max_tokens, top_p, stop, etc.: Standard LLM sampling parameters [cite: 558-610]. Use pointers in Go structs for optional float/int parameters to distinguish between zero value and not being set.stream: (Optional bool) Set to true for SSE streaming.tools / tool_choice: For function calling/tool use. Requires careful implementation of the request/response flow in Go [cite: 230-286].provider: (Optional object) Customize provider routing (e.g., specify order, allow/disallow fallbacks) [cite: 188-198].response_format: (Optional object) e.g., { "type": "json_object" } to enforce JSON output for supported models [cite: 411-419]. Requires checking model compatibility.Structured Outputs (response_format: { type: "json_schema", json_schema: {...} }): Enforce a specific JSON schema for the output on supported models [cite: 210-228].Prompt Caching: Enabled automatically by some providers (like OpenAI) or controllable via cache_control for Anthropic. Check docs for details if optimizing cost is critical [cite: 199-209].Rate Limits & Key InfoRate limits are primarily tied to your account's credit balance (roughly 1 req/sec per dollar of credit, up to a max) [cite: 554-557].Free models have separate, lower limits [cite: 551-553].Check current key limits and usage: GET https://openrouter.ai/api/v1/auth/key (requires Authorization header) [cite:
