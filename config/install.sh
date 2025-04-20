#!/bin/bash
#
# Installation script for Architect models configuration
#
# This script installs the default models.yaml file to the user's
# ~/.config/architect directory, creating the directory if it doesn't exist.

set -e

# Define colors for output
GREEN='\033[0;32m'
YELLOW='\033[1;33m'
RED='\033[0;31m'
NC='\033[0m' # No Color

# Determine the script's directory
SCRIPT_DIR="$( cd "$( dirname "${BASH_SOURCE[0]}" )" && pwd )"

# Define paths
CONFIG_DIR="$HOME/.config/architect"
MODELS_CONFIG="$CONFIG_DIR/models.yaml"
SOURCE_CONFIG="$SCRIPT_DIR/models.yaml"

echo -e "${GREEN}Installing Architect models configuration...${NC}"

# Check if source file exists
if [ ! -f "$SOURCE_CONFIG" ]; then
    echo -e "${RED}Error: Source configuration file not found at $SOURCE_CONFIG${NC}"
    exit 1
fi

# Create the config directory if it doesn't exist
if [ ! -d "$CONFIG_DIR" ]; then
    echo -e "Creating configuration directory: $CONFIG_DIR"
    mkdir -p "$CONFIG_DIR"
fi

# Check if config file already exists and prompt for overwrite
if [ -f "$MODELS_CONFIG" ]; then
    echo -e "${YELLOW}Warning: Configuration file already exists at $MODELS_CONFIG${NC}"
    read -p "Do you want to overwrite it? (y/N): " CONFIRM
    if [[ ! "$CONFIRM" =~ ^[Yy]$ ]]; then
        echo "Installation cancelled."
        exit 0
    fi
fi

# Copy the configuration file
cp "$SOURCE_CONFIG" "$MODELS_CONFIG"
echo -e "${GREEN}Successfully installed models configuration to:${NC} $MODELS_CONFIG"

# Check for API keys in environment
echo -e "\n${GREEN}API Key Check:${NC}"
if [ -z "$OPENAI_API_KEY" ]; then
    echo -e "${YELLOW}Warning: OPENAI_API_KEY environment variable is not set${NC}"
    echo "You'll need to set this to use OpenAI models:"
    echo "  export OPENAI_API_KEY=\"your-openai-api-key\""
else
    echo -e "- OpenAI API key: ${GREEN}Found${NC}"
fi

if [ -z "$GEMINI_API_KEY" ]; then
    echo -e "${YELLOW}Warning: GEMINI_API_KEY environment variable is not set${NC}"
    echo "You'll need to set this to use Gemini models:"
    echo "  export GEMINI_API_KEY=\"your-gemini-api-key\""
else
    echo -e "- Gemini API key: ${GREEN}Found${NC}"
fi

if [ -z "$OPENROUTER_API_KEY" ]; then
    echo -e "${YELLOW}Warning: OPENROUTER_API_KEY environment variable is not set${NC}"
    echo "You'll need to set this to use OpenRouter models:"
    echo "  export OPENROUTER_API_KEY=\"your-openrouter-api-key\""
else
    echo -e "- OpenRouter API key: ${GREEN}Found${NC}"
fi

echo -e "\n${GREEN}Installation complete!${NC}"
echo "You can now use Architect with the configured models."
