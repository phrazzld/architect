# Architect Configuration Files

This directory contains default configuration files for the Architect tool.

## Models Configuration

The `models.yaml` file defines the LLM providers and models available to the Architect tool. It includes:

- API key environment variable mappings
- Provider definitions (OpenAI, Gemini, OpenRouter)
- Model configurations with context sizes and output limits
- Default parameter settings for each model

## Installation

You can install the configuration files using the provided installation script:

```bash
./config/install.sh
```

This will create the necessary directories and copy the configuration files to `~/.config/architect/`.

## Manual Installation

If you prefer to install manually:

1. Create the configuration directory:
   ```bash
   mkdir -p ~/.config/architect
   ```

2. Copy the models.yaml file:
   ```bash
   cp config/models.yaml ~/.config/architect/models.yaml
   ```

3. Set your API keys as environment variables:
   ```bash
   export OPENAI_API_KEY="your-openai-api-key"
   export GEMINI_API_KEY="your-gemini-api-key"
   export OPENROUTER_API_KEY="your-openrouter-api-key"
   ```

## Customization

You can customize the `models.yaml` file to:

- Add new models as they become available
- Adjust token limits to match model updates
- Configure default parameters for each model
- Add custom API endpoints (for self-hosted models or proxies)

After modifying the configuration, restart Architect for the changes to take effect.

## Provider-Specific Configuration

### OpenAI

The OpenAI provider uses the `OPENAI_API_KEY` environment variable and supports models like `gpt-4-turbo`, `gpt-4o`, and `gpt-3.5-turbo`. Each model configuration includes:

- Context window size (e.g., 128000 tokens for `gpt-4-turbo`)
- Maximum output tokens
- Default parameters like temperature and top_p

### Gemini

The Gemini provider uses the `GEMINI_API_KEY` environment variable and supports models like `gemini-1.5-pro` and `gemini-1.5-flash`. These models have large context windows (up to 1M tokens) and support parameters like temperature, top_p, and top_k.

### OpenRouter

The OpenRouter provider uses the `OPENROUTER_API_KEY` environment variable and provides unified access to models from multiple AI companies through a single API.

#### Model ID Format

OpenRouter model IDs use the format: `provider/model-name`

For example:
- `openrouter/deepseek/deepseek-r1`
- `openrouter/x-ai/grok-3-beta`
- `openrouter/deepseek/deepseek-chat-v3-0324`

In the configuration file, these models are defined with:
```yaml
- name: openrouter/deepseek/deepseek-r1
  provider: openrouter
  api_model_id: deepseek/deepseek-r1
  context_window: 131072  # 128k tokens
  max_output_tokens: 33792
  parameters:
    temperature:
      type: float
      default: 0.7
```

The `api_model_id` is the identifier used when making API requests to OpenRouter, which follows the format `provider/model` without the `openrouter/` prefix that's used in the model's name in our configuration.

#### Configuration Notes

- **API Endpoint**: The default OpenRouter API endpoint is `https://openrouter.ai/api/v1`. You can specify a custom endpoint using the `base_url` field in the provider definition.
- **Context Window**: For each model, specify the appropriate context window and maximum output tokens as provided by OpenRouter.
- **Parameters**: OpenRouter supports standard parameters like temperature and top_p.

To use OpenRouter models with Architect, ensure the `OPENROUTER_API_KEY` environment variable is set and reference the models using their full names (e.g., `--model openrouter/deepseek/deepseek-r1`).
