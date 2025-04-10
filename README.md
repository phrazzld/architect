# Architect

A powerful code-base context analysis and planning tool that leverages Google's Gemini AI to generate detailed, actionable technical plans for software projects.

## Overview

Architect analyzes your codebase and uses Gemini AI to create comprehensive technical plans for new features, refactoring, bug fixes, or any software development task. By understanding your existing code structure and patterns, Architect provides contextually relevant guidance tailored to your specific project.

## Important Update: Task Input Method

**Please note:** The way you provide the task description to Architect has changed:

* The `--task-file` flag is now **required** for generating plans. You must provide the task description in a file. This allows for more complex and structured task inputs.
* The `--task` flag is now fully **deprecated** and is no longer functional. All task descriptions must be provided via files.
* For `--dry-run` operations, the `--task-file` flag is optional. You can run a simple dry run with just `architect --dry-run ./` to see which files would be included.

Please update your workflows accordingly. See the Usage examples and Configuration Options below for details.

## Features

- **Contextual Analysis**: Analyzes your codebase to understand its structure, patterns, and dependencies
- **Smart Filtering**: Include/exclude specific file types or directories from analysis
- **Gemini AI Integration**: Leverages Google's powerful Gemini models for intelligent planning
- **Customizable Output**: Configure the format of the generated plan
- **Git-Aware**: Respects .gitignore patterns when scanning your codebase
- **Token Management**: Smart token counting with limit checking to prevent API errors
- **Dry Run Mode**: Preview which files would be included and see token statistics before API calls
- **Task File Input**: Load task descriptions from external files
- **Custom Prompts**: Use your own prompt templates for specialized plan generation
- **Structured Logging**: Clear, color-coded logs with configurable verbosity levels
- **Interactive Progress**: Visual spinner indicates progress during API calls
- **User Confirmation**: Optional confirmation for large token counts
- **Task Clarification**: Interactive AI-powered process to refine task descriptions

## Installation

```bash
# Build from source
git clone https://github.com/yourusername/architect.git
cd architect
go build

# Install globally
go install
```

## Usage

```bash
# Basic usage (Task description in task.txt)
architect --task-file task.txt path/to/your/project

# Example: Create a plan using a task file (e.g., auth_task.txt)
# Contents of auth_task.txt: "Implement JWT-based user authentication and authorization"
architect --task-file auth_task.txt ./

# Specify output file (default is PLAN.md)
architect --task-file task.txt --output auth_plan.md ./

# Include only specific file extensions
architect --task-file task.txt --include .go,.md ./

# Use a different Gemini model
architect --task-file task.txt --model gemini-2.5-pro-exp-03-25 ./

# Dry run to see which files would be included (without generating a plan)
architect --dry-run ./

# Dry run with context from a task file
architect --dry-run --task-file task.txt ./

# Request confirmation before proceeding if token count exceeds threshold
architect --task-file task.txt --confirm-tokens 25000 ./

# Use a custom prompt template
architect --task-file task.txt --prompt-template custom_template.tmpl ./

# View available example templates
architect --list-examples

# View a specific example template
architect --show-example feature.tmpl

# Save an example template to a file (for customization)
architect --show-example feature.tmpl > my-feature-template.tmpl

# Enable interactive task clarification
architect --task-file task.txt --clarify ./
```

### Required Environment Variable

```bash
# Set your Gemini API key
export GEMINI_API_KEY="your-api-key-here"
```

## Configuration Options

| Flag | Description | Default |
|------|-------------|---------|
| `--task-file` | Path to a file containing the task description (Required unless --dry-run is used) | `(Required)` |
| `--output` | Output file path for the generated plan | `PLAN.md` |
| `--model` | Gemini model to use for generation | `gemini-2.5-pro-exp-03-25` |
| `--verbose` | Enable verbose logging output (shorthand for --log-level=debug) | `false` |
| `--log-level` | Set logging level (debug, info, warn, error) | `info` |
| `--color` | Enable/disable colored log output | `true` |
| `--include` | Comma-separated list of file extensions to include | (All files) |
| `--exclude` | Comma-separated list of file extensions to exclude | (Common binary and media files) |
| `--exclude-names` | Comma-separated list of file/dir names to exclude | (Common directories like .git, node_modules) |
| `--format` | Format string for each file. Use {path} and {content} | `<{path}>\n```\n{content}\n```\n</{path}>\n\n` |
| `--dry-run` | Show files that would be included and token count, but don't call the API | `false` |
| `--confirm-tokens` | Prompt for confirmation if token count exceeds this value (0 = never prompt) | `0` |
| `--prompt-template` | Path to a custom prompt template file (.tmpl) | uses default template |
| `--clarify` | Enable interactive task clarification to refine your task description | `false` |
| `--list-examples` | List available example prompt template files | `false` |
| `--show-example` | Display the content of a specific example template | `""` |
| `--task` | (Deprecated) Description of the task. This flag is no longer functional and will be removed in a future version. Use --task-file instead. | `""` |

## Configuration Files

Architect follows the XDG Base Directory Specification for configuration files, looking for settings in standardized locations across platforms.

### Configuration File Locations

- **User configuration**: `~/.config/architect/config.toml` (Linux/macOS) or `%APPDATA%\architect\config.toml` (Windows)
- **System configuration**: `/etc/xdg/architect/config.toml` (Linux) or system-wide locations on other platforms

### Automatic Configuration Initialization

Architect automatically creates necessary configuration directories and files the first time you run any command:

- No explicit initialization command is needed
- User configuration directory (`~/.config/architect/`) is created automatically
- Default configuration file is generated with sensible defaults
- Template directory (`~/.config/architect/templates/`) is created for custom templates

When automatic initialization occurs, you'll see a message like this:

```
✓ Architect configuration initialized automatically.
  Created default configuration file at: /home/user/.config/architect/config.toml
  Applying default settings:
    - Output File: PLAN.md
    - Model: gemini-2.5-pro-exp-03-25
    - Log Level: info
    - Default Template: default.tmpl
  You can customize these settings by editing the file.
```

This message only appears the first time you run the tool, or if your configuration file is deleted.

### Configuration Format

Architect uses TOML as its configuration format. Example configuration:

```toml
# General configuration
output_file = "PLAN.md"
model = "gemini-2.5-pro-exp-03-25"
log_level = "info"
use_colors = true

# Template settings
[templates]
default = "default.tmpl" 
clarify = "clarify.tmpl"
dir = "templates"  # Relative to config dir or absolute path

# File exclusion patterns
[excludes]
extensions = ".exe,.bin,.o,.jar"
names = ".git,node_modules,vendor"
```

For a complete example with all available options, see the [example configuration file](internal/config/example_config.toml).

### Configuration Precedence

Settings are loaded with the following precedence (highest to lowest):
1. Command-line flags
2. User configuration file
3. System configuration files
4. Default values

### Template Directories

Templates are searched in this order:
1. Absolute paths or paths relative to current directory (if specified directly)
2. Custom paths configured in the config file
3. User template directory: `~/.config/architect/templates/`
4. System template directories
5. Built-in embedded templates (included in the binary)

The template system is designed to work properly when the application is installed globally. No matter which directory you run Architect from, it will always find the appropriate templates using the XDG-compliant paths or embedded defaults.

## Token Management

Architect implements intelligent token management to prevent API errors and optimize context:

- **Accurate Token Counting**: Uses Gemini's API to get precise token counts for your content
- **Pre-API Validation**: Checks token count against model limits before making API calls
- **Fail-Fast Strategy**: Provides clear error messages when token limits would be exceeded
- **Token Statistics**: Shows token usage as a percentage of the model's limit
- **Optional Confirmation**: Can prompt for confirmation when token count exceeds a threshold

When token limits are exceeded, try:
1. Limiting file scope with `--include` or additional filtering flags
2. Using a model with a higher token limit
3. Splitting your task into smaller, more focused requests

## Generated Plan Structure

The generated PLAN.md includes:

1. **Overview**: Brief explanation of the task and changes involved
2. **Task Breakdown**: Detailed list of specific tasks with effort estimates
3. **Implementation Details**: Guidance for complex tasks with code snippets
4. **Potential Challenges**: Identified risks, edge cases, and dependencies
5. **Testing Strategy**: Approach for verifying the implementation
6. **Open Questions**: Ambiguities needing clarification


## Custom Prompt Templates

Architect supports custom prompt templates for specialized plan generation:

1. Create a .tmpl file using Go's text/template syntax
2. Reference the following variables in your template:
   - `{{.Task}}`: The task description
   - `{{.Context}}`: The codebase context
3. Pass your template file with `--prompt-template`

### Example Templates

Architect comes with several example prompt templates that you can use as starting points for your own custom templates:

```bash
# List available example templates
architect --list-examples

# View the content of a specific example template
architect --show-example basic.tmpl

# Save an example template to a file (for customization)
architect --show-example feature.tmpl > my-feature-template.tmpl

# Use your custom template
architect --task-file task.txt --prompt-template my-feature-template.tmpl ./
```

Available example templates include:
- `basic.tmpl`: A simple template with minimal structure
- `detailed.tmpl`: A comprehensive template with detailed sections
- `bugfix.tmpl`: A template specifically designed for bug fixes
- `feature.tmpl`: A template designed for new feature implementation

Templates are searched in this order:
1. Absolute paths or paths relative to current directory (if specified directly)
2. Custom paths configured in the config file
3. User template directory: `~/.config/architect/templates/`
4. System template directories
5. Built-in embedded templates (included in the binary)

## Troubleshooting

### API Key Issues
- Ensure `GEMINI_API_KEY` is set correctly in your environment
- Check that your API key has appropriate permissions for the model you're using

### Token Limit Errors
- Use `--dry-run` to see token statistics without making API calls
- Reduce the number of files analyzed with `--include` or other filtering flags
- Try a model with a higher token limit

### Performance Tips
- Start with small, focused parts of your codebase and gradually include more
- Use `--include` to target only relevant file types for your task
- Set `--log-level=debug` for detailed information about the analysis process

### Common Issues
- **"Template not found"**: Ensure templates exist in the expected directories, or provide absolute paths
- **Running from different directories**: Architect uses XDG config paths, not project-relative configs
- **No files processed**: Check paths and filters; use `--dry-run` to see what would be included
- **Configuration file not created**: If the automatic initialization fails to create the configuration file (e.g., due to permission issues), the tool will still run with default settings in memory
- **Custom configuration lost**: If you accidentally delete your configuration file, a new default one will be created on the next run; consider backing up your custom settings
- **Unexpected default values**: If some settings are using defaults unexpectedly, check for typos in your config file as TOML requires exact field names

## Contributing

Contributions are welcome! Please feel free to submit a Pull Request.

## License

[MIT License](LICENSE)