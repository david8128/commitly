package main

import (
	"bufio"
	"context"
	"encoding/json"
	"errors"
	"flag"
	"fmt"
	"io/ioutil"
	"log"
	"os"
	"os/exec"
	"path/filepath"
	"strings"

	"github.com/cohesion-org/deepseek-go"
	"github.com/google/generative-ai-go/genai"
	"github.com/liushuangls/go-anthropic/v2"
	"github.com/openai/openai-go"
	openaiOption "github.com/openai/openai-go/option"
	googleOption "google.golang.org/api/option"
)

type Provider string

const (
	ProviderOpenAI   Provider = "openai"
	ProviderClaude   Provider = "claude"
	ProviderDeepseek Provider = "deepseek"
	ProviderGemini   Provider = "gemini"
)

// ProviderConfig holds configuration for a specific provider
type ProviderConfig struct {
	Provider string `json:"provider"`
	APIKey   string `json:"api_key"`
	Model    string `json:"model"`
}

// Config holds application configuration
type Config struct {
	OpenAI          ProviderConfig `json:"openai"`
	Claude          ProviderConfig `json:"claude"`
	Deepseek        ProviderConfig `json:"deepseek"`
	Gemini          ProviderConfig `json:"gemini"`
	DefaultProvider string         `json:"default_provider"`
}

func main() {
	// Define command-line flags
	configCmd := flag.NewFlagSet("config", flag.ExitOnError)
	configSetCmd := flag.NewFlagSet("set", flag.ExitOnError)
	configGetCmd := flag.NewFlagSet("get", flag.ExitOnError)
	configShowCmd := flag.NewFlagSet("show", flag.ExitOnError)

	// Parse command-line arguments
	if len(os.Args) > 1 {
		switch os.Args[1] {
		case "config":
			if len(os.Args) > 2 {
				switch os.Args[2] {
				case "set":
					if len(os.Args) < 5 {
						fmt.Println("Usage: commitly config set <key> <value>")
						fmt.Println("Example: commitly config set openai.api_key sk-xxxxxxx")
						fmt.Println("Example: commitly config set openai.provider claude")
						fmt.Println("Example: commitly config set openai.model gpt-4-turbo")
						os.Exit(1)
					}
					configSetCmd.Parse(os.Args[3:])
					key := os.Args[3]
					value := os.Args[4]
					if err := setConfig(key, value); err != nil {
						log.Fatalf("Error setting config: %v", err)
					}
					fmt.Printf("Config %s set successfully\n", key)
					return
				case "get":
					if len(os.Args) < 4 {
						fmt.Println("Usage: commitly config get <key>")
						fmt.Println("Example: commitly config get openai.api_key")
						os.Exit(1)
					}
					configGetCmd.Parse(os.Args[3:])
					key := os.Args[3]
					value, err := getConfigValue(key)
					if err != nil {
						log.Fatalf("Error getting config: %v", err)
					}
					fmt.Printf("%s = %s\n", key, value)
					return
				case "show":
					configShowCmd.Parse(os.Args[3:])
					cfg, err := loadConfig()
					if err != nil {
						log.Fatalf("Error loading config: %v", err)
					}
					printConfig(cfg)
					return
				default:
					fmt.Println("Available commands: set, get, show")
					os.Exit(1)
				}
			}
			configCmd.Parse(os.Args[2:])
			fmt.Println("Usage: commitly config <command>")
			fmt.Println("Available commands: set, get, show")
			return
		}
	}

	// Normal execution flow for generating commit message
	// Ask user for the Jira ticket name
	reader := bufio.NewReader(os.Stdin)
	fmt.Print("Enter the Jira ticket name: ")
	ticket, err := reader.ReadString('\n')
	if err != nil {
		log.Fatalf("Error reading ticket: %v", err)
	}
	ticket = strings.TrimSpace(ticket)

	// Get git diff of changes
	gitDiff, err := getGitDiff()
	if err != nil {
		log.Fatalf("Error getting git diff: %v", err)
	}

	// Get history of last 10 commits
	commitHistory, err := getCommitHistory()
	if err != nil {
		log.Fatalf("Error getting commit history: %v", err)
	}

	// Create the prompt
	prompt := fmt.Sprintf(
		"Generate a commit message for Jira ticket '%s' following this exact format:\n"+
			"<type>(%s): <title>\n\n"+
			"Changes:\n"+
			"- <first change>\n"+
			"- <second change>\n"+
			"- <additional changes if needed>\n\n"+
			"Where:\n"+
			"- <type> should be one of: feat, fix, docs, style, refactor, test, chore\n"+
			"- (%s) is the Jira ticket number\n"+
			"- <title> is a concise description\n"+
			"- Changes section should list the main modifications as bullet points\n\n"+
			"The diff of changes is:\n%s\n\n"+
			"The history of previous commit messages is:\n%s\n\n"+
			"Provide a commit message that follows this format strictly, with bullet points for changes.",
		ticket, ticket, ticket, gitDiff, commitHistory,
	)

	// Get provider from environment variable or config
	provider, err := getProvider()
	if err != nil {
		log.Fatalf("Error determining provider: %v", err)
	}

	// Generate commit message using selected provider
	commitMessage, err := generateCommitMessage(prompt, provider)
	if err != nil {
		log.Fatalf("Error generating commit message: %v", err)
	}

	fmt.Println("\nGenerated commit message:")
	fmt.Println(commitMessage)
}

func getProvider() (Provider, error) {
	// First check environment variable
	envProvider := os.Getenv("AI_PROVIDER")
	if envProvider != "" {
		return Provider(strings.ToLower(envProvider)), nil
	}

	// Then check config file
	cfg, err := loadConfig()
	if err == nil && cfg.DefaultProvider != "" {
		return Provider(strings.ToLower(cfg.DefaultProvider)), nil
	}

	// Default to OpenAI
	return ProviderOpenAI, nil
}

func getConfigPath() string {
	homeDir, err := os.UserHomeDir()
	if err != nil {
		return ".commitly.json"
	}
	return filepath.Join(homeDir, ".commitly.json")
}

func loadConfig() (*Config, error) {
	configPath := getConfigPath()
	
	// Check if config file exists
	if _, err := os.Stat(configPath); os.IsNotExist(err) {
		// Create default config
		cfg := &Config{
			DefaultProvider: string(ProviderOpenAI),
			OpenAI: ProviderConfig{
				Provider: string(ProviderOpenAI),
				Model:    "gpt-4o",
			},
			Claude: ProviderConfig{
				Provider: string(ProviderClaude),
				Model:    "claude-3-5-sonnet-20241022",
			},
			Deepseek: ProviderConfig{
				Provider: string(ProviderDeepseek),
				Model:    "deepseek-chat",
			},
			Gemini: ProviderConfig{
				Provider: string(ProviderGemini),
				Model:    "gemini-1.5-flash-latest",
			},
		}
		return cfg, nil
	}

	// Read config file
	data, err := ioutil.ReadFile(configPath)
	if err != nil {
		return nil, fmt.Errorf("error reading config file: %v", err)
	}

	// Parse config
	var cfg Config
	if err := json.Unmarshal(data, &cfg); err != nil {
		return nil, fmt.Errorf("error parsing config file: %v", err)
	}

	return &cfg, nil
}

func saveConfig(cfg *Config) error {
	configPath := getConfigPath()
	
	// Convert config to JSON
	data, err := json.MarshalIndent(cfg, "", "  ")
	if err != nil {
		return fmt.Errorf("error serializing config: %v", err)
	}

	// Write config file
	if err := ioutil.WriteFile(configPath, data, 0600); err != nil {
		return fmt.Errorf("error writing config file: %v", err)
	}

	return nil
}

func setConfig(key, value string) error {
	cfg, err := loadConfig()
	if err != nil {
		cfg = &Config{
			DefaultProvider: string(ProviderOpenAI),
			OpenAI: ProviderConfig{
				Provider: string(ProviderOpenAI),
				Model:    "gpt-4o",
			},
			Claude: ProviderConfig{
				Provider: string(ProviderClaude),
				Model:    "claude-3-5-sonnet-20241022",
			},
			Deepseek: ProviderConfig{
				Provider: string(ProviderDeepseek),
				Model:    "deepseek-chat",
			},
			Gemini: ProviderConfig{
				Provider: string(ProviderGemini),
				Model:    "gemini-1.5-flash-latest",
			},
		}
	}

	// Update config based on key
	parts := strings.Split(key, ".")
	if len(parts) != 2 {
		return fmt.Errorf("invalid config key format, expected 'section.key'")
	}

	section, key := parts[0], parts[1]
	switch section {
	case "openai":
		switch key {
		case "api_key":
			cfg.OpenAI.APIKey = value
		case "provider":
			cfg.OpenAI.Provider = value
		case "model":
			cfg.OpenAI.Model = value
		default:
			return fmt.Errorf("unknown key for openai: %s", key)
		}
	case "claude":
		switch key {
		case "api_key":
			cfg.Claude.APIKey = value
		case "provider":
			cfg.Claude.Provider = value
		case "model":
			cfg.Claude.Model = value
		default:
			return fmt.Errorf("unknown key for claude: %s", key)
		}
	case "deepseek":
		switch key {
		case "api_key":
			cfg.Deepseek.APIKey = value
		case "provider":
			cfg.Deepseek.Provider = value
		case "model":
			cfg.Deepseek.Model = value
		default:
			return fmt.Errorf("unknown key for deepseek: %s", key)
		}
	case "gemini":
		switch key {
		case "api_key":
			cfg.Gemini.APIKey = value
		case "provider":
			cfg.Gemini.Provider = value
		case "model":
			cfg.Gemini.Model = value
		default:
			return fmt.Errorf("unknown key for gemini: %s", key)
		}
	case "default":
		if key == "provider" {
			cfg.DefaultProvider = value
		} else {
			return fmt.Errorf("unknown key for default: %s", key)
		}
	default:
		return fmt.Errorf("unknown config section: %s", section)
	}

	// Save updated config
	return saveConfig(cfg)
}

func getConfigValue(key string) (string, error) {
	cfg, err := loadConfig()
	if err != nil {
		return "", err
	}

	// Get config value based on key
	parts := strings.Split(key, ".")
	if len(parts) != 2 {
		return "", fmt.Errorf("invalid config key format, expected 'section.key'")
	}

	section, key := parts[0], parts[1]
	switch section {
	case "openai":
		switch key {
		case "api_key":
			return cfg.OpenAI.APIKey, nil
		case "provider":
			return cfg.OpenAI.Provider, nil
		case "model":
			return cfg.OpenAI.Model, nil
		}
	case "claude":
		switch key {
		case "api_key":
			return cfg.Claude.APIKey, nil
		case "provider":
			return cfg.Claude.Provider, nil
		case "model":
			return cfg.Claude.Model, nil
		}
	case "deepseek":
		switch key {
		case "api_key":
			return cfg.Deepseek.APIKey, nil
		case "provider":
			return cfg.Deepseek.Provider, nil
		case "model":
			return cfg.Deepseek.Model, nil
		}
	case "gemini":
		switch key {
		case "api_key":
			return cfg.Gemini.APIKey, nil
		case "provider":
			return cfg.Gemini.Provider, nil
		case "model":
			return cfg.Gemini.Model, nil
		}
	case "default":
		if key == "provider" {
			return cfg.DefaultProvider, nil
		}
	}

	return "", fmt.Errorf("unknown config key: %s.%s", section, key)
}

func printConfig(cfg *Config) {
	fmt.Println("Current configuration:")
	fmt.Println("---------------------")
	fmt.Printf("Default Provider: %s\n\n", cfg.DefaultProvider)
	
	fmt.Println("OpenAI Configuration:")
	fmt.Printf("  Provider: %s\n", cfg.OpenAI.Provider)
	fmt.Printf("  Model: %s\n", cfg.OpenAI.Model)
	fmt.Printf("  API Key: %s\n\n", maskAPIKey(cfg.OpenAI.APIKey))
	
	fmt.Println("Claude Configuration:")
	fmt.Printf("  Provider: %s\n", cfg.Claude.Provider)
	fmt.Printf("  Model: %s\n", cfg.Claude.Model)
	fmt.Printf("  API Key: %s\n\n", maskAPIKey(cfg.Claude.APIKey))
	
	fmt.Println("Deepseek Configuration:")
	fmt.Printf("  Provider: %s\n", cfg.Deepseek.Provider)
	fmt.Printf("  Model: %s\n", cfg.Deepseek.Model)
	fmt.Printf("  API Key: %s\n\n", maskAPIKey(cfg.Deepseek.APIKey))
	
	fmt.Println("Gemini Configuration:")
	fmt.Printf("  Provider: %s\n", cfg.Gemini.Provider)
	fmt.Printf("  Model: %s\n", cfg.Gemini.Model)
	fmt.Printf("  API Key: %s\n", maskAPIKey(cfg.Gemini.APIKey))
}

func maskAPIKey(key string) string {
	if key == "" {
		return "[not set]"
	}
	if len(key) <= 8 {
		return "****"
	}
	return key[:4] + "..." + key[len(key)-4:]
}

func generateCommitMessage(prompt string, provider Provider) (string, error) {
	ctx := context.Background()

	// Get the configuration
	cfg, err := loadConfig()
	if err != nil {
		return "", fmt.Errorf("error loading configuration: %v", err)
	}

	// Check if the provider has a custom provider set
	var actualProvider Provider
	var model string

	switch provider {
	case ProviderOpenAI:
		if cfg.OpenAI.Provider != "" && cfg.OpenAI.Provider != string(ProviderOpenAI) {
			actualProvider = Provider(cfg.OpenAI.Provider)
		} else {
			actualProvider = ProviderOpenAI
		}
		model = cfg.OpenAI.Model
	case ProviderClaude:
		if cfg.Claude.Provider != "" && cfg.Claude.Provider != string(ProviderClaude) {
			actualProvider = Provider(cfg.Claude.Provider)
		} else {
			actualProvider = ProviderClaude
		}
		model = cfg.Claude.Model
	case ProviderDeepseek:
		if cfg.Deepseek.Provider != "" && cfg.Deepseek.Provider != string(ProviderDeepseek) {
			actualProvider = Provider(cfg.Deepseek.Provider)
		} else {
			actualProvider = ProviderDeepseek
		}
		model = cfg.Deepseek.Model
	case ProviderGemini:
		if cfg.Gemini.Provider != "" && cfg.Gemini.Provider != string(ProviderGemini) {
			actualProvider = Provider(cfg.Gemini.Provider)
		} else {
			actualProvider = ProviderGemini
		}
		model = cfg.Gemini.Model
	default:
		actualProvider = provider
	}

	// Generate message using the actual provider
	switch actualProvider {
	case ProviderOpenAI:
		return generateOpenAICommitMessage(ctx, prompt, model)
	case ProviderClaude:
		return generateClaudeCommitMessage(ctx, prompt, model)
	case ProviderDeepseek:
		return generateDeepseekCommitMessage(ctx, prompt, model)
	case ProviderGemini:
		return generateGeminiCommitMessage(ctx, prompt, model)
	default:
		return "", fmt.Errorf("unsupported provider: %s", actualProvider)
	}
}

func generateOpenAICommitMessage(ctx context.Context, prompt, model string) (string, error) {
	apiKey := getAPIKey(ProviderOpenAI)
	if apiKey == "" {
		return "", fmt.Errorf("OpenAI API key not found. Set it with:\n" +
			"export OPENAI_API_KEY=sk-xxxxxxx\n" +
			"or\n" +
			"commitly config set openai.api_key sk-xxxxxxx")
	}

	// Use default model if not specified
	if model == "" {
		model = "gpt-4o"
	}

	client := openai.NewClient(openaiOption.WithAPIKey(apiKey))
	
	response, err := client.Chat.Completions.New(ctx, openai.ChatCompletionNewParams{
		Model: openai.F(model),
		Messages: openai.F([]openai.ChatCompletionMessageParamUnion{
			openai.SystemMessage("You are a commit message generator that creates messages in the conventional commit format. " +
				"You always follow the format: <type>(<ticket>): <title>\n<optional body>. " +
				"Types are limited to: feat, fix, docs, style, refactor, test, chore. " +
				"Keep the title concise and descriptive. Add body only if additional context is needed."),
			openai.UserMessage(prompt),
		}),
		Temperature: openai.F(0.7),
	})
	if err != nil {
		return "", fmt.Errorf("OpenAI API error: %v", err)
	}

	return response.Choices[0].Message.Content, nil
}

func generateClaudeCommitMessage(ctx context.Context, prompt, model string) (string, error) {
	apiKey := getAPIKey(ProviderClaude)
	if apiKey == "" {
		return "", fmt.Errorf("Claude API key not found. Set it with:\n" +
			"export ANTHROPIC_API_KEY=sk-ant-xxxxxxx\n" +
			"or\n" +
			"commitly config set claude.api_key sk-ant-xxxxxxx")
	}

	// Use default model if not specified
	if model == "" {
		model = string(anthropic.ModelClaude3Dot5SonnetLatest)
	}

	client := anthropic.NewClient(apiKey)
	
	response, err := client.CreateMessages(ctx, anthropic.MessagesRequest{
		Model: anthropic.Model(model),
		MultiSystem: []anthropic.MessageSystemPart{
			{
				Type: "text",
				Text: "You are a commit message generator that creates messages in the conventional commit format. " +
					"You always follow the format: <type>(<ticket>): <title>\n<optional body>. " +
					"Types are limited to: feat, fix, docs, style, refactor, test, chore. " +
					"Keep the title concise and descriptive. Add body only if additional context is needed.",
			},
		},
		Messages: []anthropic.Message{
			anthropic.NewUserTextMessage(prompt),
		},
		MaxTokens: 1000,
	})
	if err != nil {
		var apiErr *anthropic.APIError
		if errors.As(err, &apiErr) {
			return "", fmt.Errorf("Claude API error - Type: %s, Message: %s", apiErr.Type, apiErr.Message)
		}
		return "", fmt.Errorf("Claude API error: %v", err)
	}

	if len(response.Content) == 0 {
		return "", fmt.Errorf("empty response from Claude API")
	}

	return response.Content[0].GetText(), nil
}

func generateDeepseekCommitMessage(ctx context.Context, prompt, model string) (string, error) {
	apiKey := getAPIKey(ProviderDeepseek)
	if apiKey == "" {
		return "", fmt.Errorf("Deepseek API key not found. Set it with:\n" +
			"export DEEPSEEK_API_KEY=xxxxxxx\n" +
			"or\n" +
			"commitly config set deepseek.api_key xxxxxxx")
	}

	// Use default model if not specified
	if model == "" {
		model = deepseek.DeepSeekChat
	}

	client := deepseek.NewClient(apiKey)
	
	response, err := client.CreateChatCompletion(ctx, &deepseek.ChatCompletionRequest{
		Model: model,
		Messages: []deepseek.ChatCompletionMessage{
			{Role: "system", Content: "You are a commit message generator that creates messages in the conventional commit format. " +
				"You always follow the format: <type>(<ticket>): <title>\n<optional body>. " +
				"Types are limited to: feat, fix, docs, style, refactor, test, chore. " +
				"Keep the title concise and descriptive. Add body only if additional context is needed."},
			{Role: "user", Content: prompt},
		},
	})
	if err != nil {
		return "", fmt.Errorf("Deepseek API error: %v", err)
	}

	return response.Choices[0].Message.Content, nil
}

func generateGeminiCommitMessage(ctx context.Context, prompt, model string) (string, error) {
	apiKey := getAPIKey(ProviderGemini)
	if apiKey == "" {
		return "", fmt.Errorf("Gemini API key not found. Set it with:\n" +
			"export GEMINI_API_KEY=xxxxxxx\n" +
			"or\n" +
			"commitly config set gemini.api_key xxxxxxx")
	}

	// Use default model if not specified
	if model == "" {
		model = "gemini-1.5-flash-latest"
	}

	client, err := genai.NewClient(ctx, googleOption.WithAPIKey(apiKey))
	if err != nil {
		return "", fmt.Errorf("error creating Gemini client: %v", err)
	}
	defer client.Close()

	geminiModel := client.GenerativeModel(model)
	
	// Create safety settings and generation config if needed
	safetySettings := []*genai.SafetySetting{
		{
			Category:  genai.HarmCategoryHarassment,
			Threshold: genai.HarmBlockNone,
		},
	}
	geminiModel.SafetySettings = safetySettings
	
	// Add system instruction as part of the conversation
	systemPrompt := "You are a commit message generator that creates messages in the conventional commit format. " +
		"You always follow the format: <type>(<ticket>): <title>\n<optional body>. " +
		"Types are limited to: feat, fix, docs, style, refactor, test, chore. " +
		"Keep the title concise and descriptive. Add body only if additional context is needed."
	
	// Create chat session with system prompt
	chat := geminiModel.StartChat()
	_, err = chat.SendMessage(ctx, genai.Text(systemPrompt))
	if err != nil {
		return "", fmt.Errorf("error sending system prompt to Gemini: %v", err)
	}
	
	// Send the actual prompt
	resp, err := chat.SendMessage(ctx, genai.Text(prompt))
	if err != nil {
		return "", fmt.Errorf("Gemini API error: %v", err)
	}

	if len(resp.Candidates) == 0 || len(resp.Candidates[0].Content.Parts) == 0 {
		return "", fmt.Errorf("empty response from Gemini API")
	}

	return fmt.Sprintf("%v", resp.Candidates[0].Content.Parts[0]), nil
}

func getAPIKey(provider Provider) string {
	var envKey, configKey string

	// Check environment variables first
	switch provider {
	case ProviderOpenAI:
		envKey = os.Getenv("OPENAI_API_KEY")
	case ProviderClaude:
		envKey = os.Getenv("ANTHROPIC_API_KEY")
	case ProviderDeepseek:
		envKey = os.Getenv("DEEPSEEK_API_KEY")
	case ProviderGemini:
		envKey = os.Getenv("GEMINI_API_KEY")
	}

	if envKey != "" {
		return envKey
	}

	// Then check config file
	cfg, err := loadConfig()
	if err != nil {
		return ""
	}

	switch provider {
	case ProviderOpenAI:
		configKey = cfg.OpenAI.APIKey
	case ProviderClaude:
		configKey = cfg.Claude.APIKey
	case ProviderDeepseek:
		configKey = cfg.Deepseek.APIKey
	case ProviderGemini:
		configKey = cfg.Gemini.APIKey
	}

	return configKey
}

// getGitDiff tries to get the diff from the last stash.
// If no stash is available, it uses "git diff".
func getGitDiff() (string, error) {
	// Try to get diff from last stash
	cmd := exec.Command("git", "stash", "show", "-p")
	output, err := cmd.CombinedOutput()
	if err != nil || len(output) == 0 {
		// If no stash or error occurs, use "git diff"
		cmd2 := exec.Command("git", "diff")
		output2, err2 := cmd2.CombinedOutput()
		if err2 != nil {
			return "", fmt.Errorf("error executing 'git diff': %v, output: %s", err2, string(output2))
		}
		return string(output2), nil
	}
	return string(output), nil
}

// getCommitHistory gets the messages from the last 10 commits.
func getCommitHistory() (string, error) {
	cmd := exec.Command("git", "log", "--pretty=format:%s", "-n", "10")
	output, err := cmd.CombinedOutput()
	if err != nil {
		return "", fmt.Errorf("error executing 'git log': %v, output: %s", err, string(output))
	}
	return string(output), nil
}
