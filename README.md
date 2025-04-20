# architect

A powerful code-base context analysis and planning tool that leverages Google's Gemini, OpenAI, and OpenRouter models to generate detailed, actionable technical plans for software projects.

## Overview

architect analyzes your codebase and uses Gemini, OpenAI, or OpenRouter models to create comprehensive technical plans for new features, refactoring, bug fixes, or any software development task. By understanding your existing code structure and patterns, architect provides contextually relevant guidance tailored to your specific project.

## Important Updates

### Token Handling Removal (v0.8.0)

**Important:** Token counting and validation functionality has been removed from architect. Key changes include:

* Token counting is no longer performed by architect - provider APIs handle their own token limits natively
* The `--confirm-tokens` flag has been removed
* Token statistics are no longer shown in dry run mode
* Error handling for token limits now relies on provider API error responses

This simplifies the application architecture and removes potential inaccuracies in token counting.

### Instruction Input Method

**Please note:** The way you provide instructions to architect has changed:

* The `--instructions` flag is now **required** for generating plans. You must provide the instructions in a file. This allows for more complex and structured inputs.
* The `--task-file` flag has been **removed** and replaced with the `--instructions` flag.
* For `--dry-run` operations, the `--instructions` flag is optional. You can run a simple dry run with just `architect --dry-run ./` to see which files would be included.

Please update your workflows accordingly. See the Usage examples and Configuration Options below for details.

### Upcoming API Changes (v0.8.0)

**Important:** The following internal APIs are deprecated and will be removed in v0.8.0 (Q4 2024):

* `InitClient` method - Use `InitLLMClient` instead
* `ProcessResponse` method - Use `ProcessLLMResponse` instead
* `llmToGeminiClientAdapter` - Use the provider-agnostic `llm.LLMClient` interface directly
* The entire `internal/architect/compat` package

If you're developing with or extending Architect, please migrate to the provider-agnostic methods. See the [Migration Guide](MIGRATION-GUIDE.md) and [Deprecation Plan](DEPRECATED-API-REMOVAL-PLAN.md) for details.

## Features

- **Contextual Analysis**: Analyzes your codebase to understand its structure, patterns, and dependencies
- **Smart Filtering**: Include/exclude specific file types or directories from analysis
- **Multiple AI Providers**: Support for Gemini, OpenAI, and OpenRouter models
- **Customizable Output**: Configure the format of the generated plan
- **Git-Aware**: Respects .gitignore patterns when scanning your codebase
- **Dry Run Mode**: Preview which files would be included before API calls
- **XML-Structured Approach**: Uses a simple XML structure for instructions and context
- **Instructions File Input**: Load instructions from external files
- **Structured Logging**: Clear, structured logs with configurable verbosity levels and optional audit logging

## Installation

```bash
# Build from source
git clone https://github.com/yourusername/architect.git
cd architect

# Recommended: Run the setup script to check dependencies and install pre-commit hooks
./scripts/setup.sh

# Or build manually
go build

# Install globally
go install
```

The setup script checks for required dependencies (like Go and pre-commit), offers to install them if missing, and configures the development environment automatically.

## Usage

```bash
# Basic usage (Instructions in instructions.txt)
architect --instructions instructions.txt path/to/your/project

# Example: Create a plan using an instructions file
# Contents of auth_instructions.txt: "Implement JWT-based user authentication and authorization"
architect --instructions auth_instructions.txt ./

# Specify output directory (default generates a unique run name directory)
architect --instructions instructions.txt --output-dir custom-dir ./

# Generate plans with multiple models (repeatable flag)
architect --instructions instructions.txt --model gemini-1.5-pro --model gemini-2.5-pro-exp-03-25 ./

# Include only specific file extensions
architect --instructions instructions.txt --include .go,.md ./

# Use a different Gemini model
architect --instructions instructions.txt --model gemini-2.5-pro-exp-03-25 ./

# Use an OpenAI model
architect --instructions instructions.txt --model gpt-4-turbo ./

# Use both Gemini and OpenAI models
architect --instructions instructions.txt --model gemini-2.5-pro-exp-03-25 --model gpt-4-turbo ./

# Use an OpenRouter model
architect --instructions instructions.txt --model openrouter/deepseek/deepseek-r1 ./

# Use models from multiple providers
architect --instructions instructions.txt --model gemini-1.5-pro --model gpt-4-turbo --model openrouter/x-ai/grok-3-beta ./

# Enable structured audit logging (JSON Lines format)
architect --instructions instructions.txt --audit-log-file audit.jsonl ./

# Dry run to see which files would be included (without generating a plan)
architect --dry-run ./

# Dry run with instructions file
architect --dry-run --instructions instructions.txt ./

# Control concurrency and rate limiting for multiple models
architect --instructions task.txt --model gemini-1.5-pro --model gpt-4-turbo --max-concurrent 3 --rate-limit 30 ./
```

### Required Environment Variables

```bash
# Set your Gemini API key (required for Gemini models)
export GEMINI_API_KEY="your-gemini-api-key-here"

# Set your OpenAI API key (required for OpenAI models)
export OPENAI_API_KEY="your-openai-api-key-here"

# Set your OpenRouter API key (required for OpenRouter models)
export OPENROUTER_API_KEY="your-openrouter-api-key-here"
```

> **Note**: You only need to set the API key for the provider(s) you're using. If you're exclusively using Gemini models, only `GEMINI_API_KEY` is required. Similarly, if you're only using OpenAI models, only `OPENAI_API_KEY` is required. If you're using OpenRouter models, `OPENROUTER_API_KEY` is required. Set all relevant environment variables for the providers you intend to use.

The required API key environment variables are defined in the `api_key_sources` section of your `~/.config/architect/models.yaml` file. If you add new providers, you can specify custom environment variable names for their API keys in this section.

## Configuration Options

| Flag | Description | Default |
|------|-------------|---------|
| `--instructions` | Path to a file containing the instructions (Required unless --dry-run is used) | `(Required)` |
| `--output-dir` | Directory path to store generated plans (one per model) | `(Auto-generated run name)` |
| `--model` | Model to use for generation (repeatable for multiple models, e.g., `gemini-2.5-pro-exp-03-25`, `gpt-4-turbo`) | `gemini-2.5-pro-exp-03-25` |
| `--verbose` | Enable verbose logging output (shorthand for --log-level=debug) | `false` |
| `--log-level` | Set logging level (debug, info, warn, error) | `info` |
| `--include` | Comma-separated list of file extensions to include | (All files) |
| `--exclude` | Comma-separated list of file extensions to exclude | (Common binary and media files) |
| `--exclude-names` | Comma-separated list of file/dir names to exclude | (Common directories like .git, node_modules) |
| `--format` | Format string for each file. Use {path} and {content} | `<{path}>\n```\n{content}\n```\n</{path}>\n\n` |
| `--dry-run` | Show files that would be included, but don't call the API | `false` |
| `--audit-log-file` | Path to write structured audit logs (JSON Lines format) | `(Disabled)` |
| `--max-concurrent` | Maximum number of concurrent API requests (0 = no limit) | `5` |
| `--rate-limit` | Maximum requests per minute per model (0 = no limit) | `60` |

## Configuration

architect is configured through command-line flags, environment variables (`GEMINI_API_KEY`, `OPENAI_API_KEY`, and/or `OPENROUTER_API_KEY`), and a models.yaml configuration file for provider and model settings.

### Models Configuration File

architect uses a `models.yaml` configuration file to define LLM providers and models:

- **Location**: `~/.config/architect/models.yaml`
- **Purpose**: Centralizes all model configuration including:
  - Available providers (OpenAI, Gemini, OpenRouter)
  - Model definitions with their API identifiers
  - Default parameter values (temperature, etc.)
  - API key sources (environment variables)
  - Custom API endpoints (optional)

#### Installing the Configuration File

```bash
# Create the configuration directory
mkdir -p ~/.config/architect

# Copy the default configuration file
cp config/models.yaml ~/.config/architect/models.yaml
```

#### Customizing the Configuration

You can customize `~/.config/architect/models.yaml` to:
- Add new models as they become available
- Configure default parameters for each model
- Add custom API endpoints (for self-hosted models or proxies)

### Default Values

architect comes with sensible defaults for most options:

- Output directory: Auto-generated run name directory (e.g., `eloquent-rabbit`)
- Output files: One file per model with name format `modelname.md`
- Model: `gemini-2.5-pro-exp-03-25` (can specify multiple models with repeatable `--model` flag)
- Supported models: All models defined in your models.yaml file
- Log level: `info`
- File formatting: XML-style format for context building
- Default exclusions for common binary, media files, and directories

You can override any of these defaults using the appropriate command-line flags.

## Error Handling and Provider Limits

Each LLM provider has its own context length limits and error handling mechanisms:

- **Provider-Native Limit Handling**: Each provider's API will return appropriate errors when limits are exceeded
- **Error Categorization**: architect categorizes provider errors into common types (authentication, rate limiting, context length)
- **Clear Error Messages**: Descriptive error messages help identify the cause of failures
- **Provider-Specific Guidance**: When limits are exceeded, architect provides guidance specific to the provider

When you receive context length errors from a provider:
1. Reduce the scope of files included in your analysis
2. Try a model with a larger context window
3. Split your task into smaller subtasks



## Multi-Provider and Multi-Model Support

architect supports generating plans with multiple AI models from multiple providers simultaneously:

- **Multiple Models**: Specify multiple models with the repeatable `--model` flag
- **Multiple Providers**: Seamlessly use Gemini, OpenAI, and OpenRouter models in the same run
- **Registry-Based Configuration**: Uses models.yaml to define providers, models, and their capabilities
- **Organized Output**: Each model's plan is saved as a separate file in the output directory
- **Run Name Directories**: Automatically creates a uniquely named directory for each run
- **Concurrent Processing**: Processes multiple models in parallel using Go's concurrency primitives
- **Rate Limiting**: Prevents overwhelming API endpoints with configurable concurrency and rate limits
- **Error Isolation**: If one model fails, others will still complete successfully
- **Output Files**: Each model's output is saved in its own file within the output directory
- **Provider-Specific Error Handling**: Each provider's errors are properly handled and reported

Example:
```bash
# Generate plans with models from all supported providers
architect --instructions task.md --model gemini-1.5-pro --model gpt-4-turbo --model openrouter/deepseek/deepseek-r1 ./src
```

This will generate:
```
/current-dir/random-runname/
  ├─ gemini-1.5-pro.md
  └─ gpt-4-turbo.md
```

### Concurrency Control

architect processes multiple models concurrently for improved performance, with built-in safeguards to prevent overwhelming API endpoints:

- **Concurrent Execution**: Multiple model requests run in parallel using goroutines
- **Per-Model Rate Limiting**: Each model has its own rate limit bucket to prevent API throttling
- **Provider-Aware Processing**: Handles different API requirements for Gemini and OpenAI automatically
- **Configurable Limits**: Adjust concurrency and rate limits to match your API quota

## Generated Plan Structure

Each generated plan file includes:

1. **Overview**: Brief explanation of the task and changes involved
2. **Task Breakdown**: Detailed list of specific tasks with effort estimates
3. **Implementation Details**: Guidance for complex tasks with code snippets
4. **Potential Challenges**: Identified risks, edge cases, and dependencies
5. **Testing Strategy**: Approach for verifying the implementation
6. **Open Questions**: Ambiguities needing clarification


## XML-Structured Approach

architect uses a simple XML structure to organize instructions and context:

1. Instructions are wrapped in `<instructions>...</instructions>` tags
2. Context files are wrapped in `<context>...</context>` tags
3. Each file in the context is formatted with its path and content
4. All XML special characters in file content are properly escaped

This approach provides a clear separation between the instructions and the codebase context, making it easier for the LLM to understand and process the input.

## Audit Logging

The structured audit logging feature records detailed information about each operation:

- **Format**: JSON Lines format for easy machine parsing
- **Content**: Records timestamps, operations, status, duration, token counts, and errors
- **Usage**: Enable with `--audit-log-file audit.jsonl` (replace with your desired path)
- **Purpose**: Provides auditability and data for programmatic analysis

Each log entry contains structured information that can be processed by tools, with fields including:
- Operation name
- Status (success/failure)
- Execution duration
- Input/output details
- Token counts
- Error information (if applicable)

## Troubleshooting

### Configuration Issues
- Ensure the `~/.config/architect/models.yaml` file exists and is properly formatted
- Check that the models you're using are defined in your models.yaml file
- Make sure the provider definitions in models.yaml match your needs

### API Key Issues
- For Gemini models, ensure `GEMINI_API_KEY` is set correctly in your environment
- For OpenAI models, ensure `OPENAI_API_KEY` is set correctly in your environment
- For OpenRouter models, ensure `OPENROUTER_API_KEY` is set correctly in your environment
- Check that your API keys have appropriate permissions for the models you're using
- Verify the environment variable names match those in the `api_key_sources` section of models.yaml

### Provider Token Limit Errors
- Reduce the number of files analyzed with `--include` or other filtering flags
- Try a model with a higher token limit
- If you receive API errors about context length or token limits, the provider's limits have been exceeded
- Remember that each provider handles their own token limits; architect no longer tracks this

### Performance Tips
- Start with small, focused parts of your codebase and gradually include more
- Use `--include` to target only relevant file types for your task
- Set `--log-level=debug` for detailed information about the analysis process
- Use multiple models with the `--model` flag (processed concurrently)
- Adjust `--max-concurrent` based on your system's resources and API limits

### Rate Limiting Issues
- If you encounter API rate limit errors, reduce the `--max-concurrent` value (e.g., from 5 to 3)
- Adjust the `--rate-limit` value to match your API quota (requests per minute per model)
- For large projects with multiple models, consider more conservative limits (e.g., `--max-concurrent 2 --rate-limit 30`)
- Remember that rate limits are applied per model, so using multiple models with high request rates may still encounter API throttling

### Common Issues
- **No files processed**: Check paths and filters; use `--dry-run` to see what would be included
- **Missing API key**: Ensure the appropriate API key environment variable is set correctly for the models you're using
- **Missing or invalid models.yaml**: Make sure the configuration file exists at `~/.config/architect/models.yaml`
- **Path issues**: When running commands, use absolute or correct relative paths to your project files
- **Flag precedence**: Remember that CLI flags always take precedence over default values
- **Model name errors**: Ensure you're using valid model names that are defined in your models.yaml file
- **Provider prefix**: For OpenRouter models, make sure to include the provider prefix (e.g., `openrouter/deepseek/deepseek-r1`)
- **Output directory permissions**: Check you have write access to the output directory when using `--output-dir`
- **Context length exceeded**: If you receive API errors about context length, reduce the number of files or try a model with a larger context window
- **Provider API errors**: Provider-specific errors (like rate limits or model availability) will be reported clearly in the error messages

## Contributing

Contributions are welcome! Please feel free to submit a Pull Request.

## License

[MIT License](LICENSE)
