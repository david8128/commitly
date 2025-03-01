

# Commitly

A CLI written in Go that generates git commit messages for you using AI models from multiple providers (OpenAI, Claude, Gemini, and Deepseek) and follows the conventional commit format.

[![Go Report Card](https://goreportcard.com/badge/github.com/8128david/commitly)](https://goreportcard.com/report/github.com/8128david/commitly)

## Features

- **Multiple AI Provider Support**:
  - OpenAI (GPT-4o, GPT-3.5-turbo)
  - Claude (Claude 3.5 Sonnet)
  - Gemini (Gemini 1.5 Flash)
  - Deepseek (Deepseek Chat)
- **Conventional Commit Format**: Generates commit messages following the conventional commits specification
- **Flexible Configuration**: Configure API keys, models, and providers through environment variables or config file
- **Provider Redirection**: Use any provider as a fallback for another (e.g., use Claude when OpenAI is specified)
- **Bullet Point Format**: Organizes changes in easy-to-read bullet points
- **Git Integration**: Analyzes git diffs and commit history to generate contextual commit messages

## Installation

### From Source

```bash
go install github.com/8128david/commitly@latest
```

## Setup

You'll need API keys for the providers you want to use. You can set them up in two ways:

### Using Environment Variables

```bash
# For OpenAI
export OPENAI_API_KEY=sk-xxxxxxx

# For Claude
export ANTHROPIC_API_KEY=sk-ant-xxxxxxx

# For Deepseek
export DEEPSEEK_API_KEY=xxxxxxx

# For Gemini
export GEMINI_API_KEY=xxxxxxx
```

### Using Config File

```bash
# Set OpenAI API key
commitly config set openai.api_key sk-xxxxxxx

# Set Claude API key
commitly config set claude.api_key sk-ant-xxxxxxx

# Set Deepseek API key
commitly config set deepseek.api_key xxxxxxx

# Set Gemini API key
commitly config set gemini.api_key xxxxxxx

# Set default provider
commitly config set default.provider openai
```

## Provider Redirection

You can configure one provider to use another:

```bash
# Use Claude instead of OpenAI
commitly config set openai.provider claude
commitly config set openai.api_key sk-ant-xxxxxxx
commitly config set openai.model claude-3-5-sonnet-20241022

# Use Gemini instead of OpenAI
commitly config set openai.provider gemini
commitly config set openai.api_key xxxxxxx
commitly config set openai.model gemini-1.5-flash-latest
```

## Usage

### Generate a Commit Message

```bash
commitly
```

The tool will:
1. Analyze your git diff
2. Look at your recent commit history
3. Generate a conventional commit message with bullet points
4. Display the result

### View Configuration

```bash
commitly config show
```

This will display your current configuration including providers, models, and API keys (masked for security).

## Configuration Options

| Option | Description |
|--------|-------------|
| default.provider | Default AI provider to use (openai, claude, deepseek, gemini) |
| [provider].api_key | API key for the specified provider |
| [provider].model | Model to use for the specified provider |
| [provider].provider | Redirect to another provider |

## How It Works

Commitly analyzes:
1. The git diff of your staged changes
2. Your recent commit history
3. The Jira ticket name you provide

It then sends this information to the configured AI provider with a prompt that instructs it to generate a conventional commit message with bullet points explaining the changes.

## License

MIT License

## Acknowledgements

This project was inspired by tools like CodeGPT and uses libraries from:
- OpenAI Go Client
- Go Anthropic
- Deepseek Go
- Google Generative AI Go
