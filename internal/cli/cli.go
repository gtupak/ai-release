package cli

import (
	"errors"
	"fmt"
	"strings"

	"airelease/internal/ai"
	"airelease/internal/config"
	"airelease/internal/git"
	"airelease/internal/release"
)

const defaultBaseBranch = "master"

func Run(args []string) error {
	var dryRun bool
	var baseBranch = defaultBaseBranch

	if len(args) == 0 {
		dryRun = false
	} else if args[0] == "--dry-run" {
		dryRun = true
		if len(args) > 1 && args[1] != "--dry-run" {
			return fmt.Errorf("unknown flag %q", args[0])
		}
	} else {
		baseBranch = args[0]
		dryRun = false
	}

	if len(args) > 0 && args[0] == "config" {
		return runConfig(args[1:])
	}

	if dryRun {
		return release.CreateWithDryRun(true, baseBranch)
	}

	if len(args) == 0 {
		return release.Create(baseBranch)
	}

	return fmt.Errorf("unknown command %q\n\n%s", args[0], usage())
}

func runConfig(args []string) error {
	if len(args) < 1 {
		return errors.New("missing config subcommand\n\n" + usage())
	}

	switch args[0] {
	case "base":
		return runConfigBase(args[1:])
	case "openrouter-api-key":
		return runConfigOpenRouterAPIKey(args[1:])
	case "openrouter":
		return runConfigOpenRouter(args[1:])
	case "model":
		return runConfigModel(args[1:])
	case "model-url":
		return runConfigModelURL(args[1:])
	case "model-header":
		return runConfigModelHeader(args[1:])
	default:
		return fmt.Errorf("unknown config subcommand %q\n\n%s", args[0], usage())
	}
}

func runConfigBase(args []string) error {
	if len(args) != 1 {
		return errors.New("usage: airelease config base <branch>")
	}

	branch := strings.TrimSpace(args[0])
	if branch == "" {
		return errors.New("branch name cannot be empty")
	}

	repoRoot, err := git.RepoRoot()
	if err != nil {
		return err
	}

	if err := config.SetRepoBaseBranch(repoRoot, branch); err != nil {
		return err
	}
	cfgPath, err := config.GlobalPath()
	if err != nil {
		return err
	}

	fmt.Printf("saved base branch %q for repo %q to %s\n", branch, repoRoot, cfgPath)
	return nil
}

func runConfigOpenRouterAPIKey(args []string) error {
	if len(args) != 1 {
		return errors.New("usage: airelease config openrouter-api-key <api-key>")
	}

	apiKey := strings.TrimSpace(args[0])
	if apiKey == "" {
		return errors.New("api key cannot be empty")
	}

	cfg, err := config.LoadGlobal()
	if err != nil {
		return err
	}
	cfg.OpenRouterAPIKey = apiKey
	if err := config.SaveGlobal(cfg); err != nil {
		return err
	}

	cfgPath, err := config.GlobalPath()
	if err != nil {
		return err
	}
	fmt.Printf("saved OpenRouter API key to %s\n", cfgPath)
	return nil
}

func runConfigOpenRouter(args []string) error {
	if len(args) != 1 {
		return errors.New("usage: airelease config openrouter <true|false>")
	}

	modeStr := strings.TrimSpace(strings.ToLower(args[0]))
	var mode bool

	switch modeStr {
	case "true":
		mode = true
	case "false":
		mode = false
	default:
		return errors.New("invalid value: expected 'true' or 'false'")
	}

	if err := config.SetOpenRouterMode(mode); err != nil {
		return err
	}

	cfgPath, err := config.GlobalPath()
	if err != nil {
		return err
	}
	fmt.Printf("saved OpenRouter mode %v to %s\n", mode, cfgPath)
	return nil
}

func runConfigModel(args []string) error {
	if len(args) != 1 {
		return errors.New("usage: airelease config model <openrouter-model>")
	}

	model := strings.TrimSpace(args[0])
	if model == "" {
		return errors.New("model cannot be empty")
	}

	cfg, err := config.LoadGlobal()
	if err != nil {
		return err
	}
	cfg.OpenRouterModel = model
	if err := config.SaveGlobal(cfg); err != nil {
		return err
	}

	cfgPath, err := config.GlobalPath()
	if err != nil {
		return err
	}
	fmt.Printf("saved OpenRouter model %q to %s\n", model, cfgPath)
	return nil
}

func runConfigModelURL(args []string) error {
	if len(args) != 1 {
		return errors.New("usage: airelease config model-url <url>")
	}

	url := strings.TrimSpace(args[0])
	if url == "" {
		return errors.New("URL cannot be empty")
	}

	if err := config.SetCustomModelURL(url, true); err != nil {
		return err
	}

	cfgPath, err := config.GlobalPath()
	if err != nil {
		return err
	}
	fmt.Printf("saved custom model URL %q to %s\n", url, cfgPath)
	fmt.Println("OpenRouter mode disabled for custom URL usage")
	return nil
}

func runConfigModelHeader(args []string) error {
	if len(args) < 1 {
		return errors.New("usage: airelease config model-header <key:value>\n       airelease config model-header remove <key>\n       airelease config model-header list\n\nExamples:\n  airelease config model-header \"X-Custom: value\"\n  airelease config model-header \"Authorization: Bearer token123\"")
	}

	switch args[0] {
	case "remove":
		if len(args) != 2 {
			return errors.New("usage: airelease config model-header remove <key>")
		}
		return runConfigModelHeaderRemove(args[1:])
	case "list":
		return runConfigModelHeaderList()
	default:
		return runConfigModelHeaderAdd(args)
	}
}

func runConfigModelHeaderAdd(args []string) error {
	if len(args) != 1 {
		return errors.New("usage: airelease config model-header <key:value>")
	}

	header := args[0]
	key, value, err := parseHeader(header)
	if err != nil {
		return err
	}

	if err := config.SetCustomHeader(key, value, true); err != nil {
		return err
	}

	cfgPath, err := config.GlobalPath()
	if err != nil {
		return err
	}
	fmt.Printf("added header %q: %q to %s\n", key, value, cfgPath)
	fmt.Println("OpenRouter mode disabled for custom headers usage")
	return nil
}

func runConfigModelHeaderRemove(args []string) error {
	if len(args) != 1 {
		return errors.New("usage: airelease config model-header remove <key>")
	}

	key := strings.TrimSpace(args[0])
	if key == "" {
		return errors.New("header key cannot be empty")
	}

	if err := config.RemoveCustomHeader(key); err != nil {
		return err
	}

	cfgPath, err := config.GlobalPath()
	if err != nil {
		return err
	}
	fmt.Printf("removed header %q from %s\n", key, cfgPath)
	return nil
}

func runConfigModelHeaderList() error {
	headers, err := config.GetCustomHeaders()
	if err != nil {
		return err
	}

	if len(headers) == 0 {
		fmt.Println("no custom headers configured")
		return nil
	}

	fmt.Println("custom headers:")
	for key, value := range headers {
		fmt.Printf("  %s: %s\n", key, value)
	}
	return nil
}

func parseHeader(header string) (key, value string, err error) {
	colonIndex := strings.Index(header, ":")
	if colonIndex == -1 {
		return "", "", errors.New("invalid header format: expected key:value")
	}

	key = strings.TrimSpace(header[:colonIndex])
	value = strings.TrimSpace(header[colonIndex+1:])

	if key == "" {
		return "", "", errors.New("header key cannot be empty")
	}
	if value == "" {
		return "", "", errors.New("header value cannot be empty")
	}

	return key, value, nil
}

func usage() string {
	return `Usage:
  airelease
  airelease config base <branch>
  airelease config openrouter-api-key <api-key>
  airelease config openrouter <true|false>
  airelease config model <openrouter-model>
  airelease config model-url <url>
  airelease config model-header <key:value>
  airelease config model-header remove <key>
  airelease config model-header list

Commands:
  (no args)                       Create a GitHub release from merged PRs.
  config base <name>              Save default base branch for this repository.
  config openrouter-api-key <key> Save a global OpenRouter API key.
  config openrouter <true|false>  Enable/disable OpenRouter mode (default: true)
  config model <name>             Save a global OpenRouter model name.
	config model-url <url>          Set a custom model endpoint (full path, e.g., https://example.com/v1/chat/completions)
  config model-header <k:v>       Add a custom header (format: key:value).
  config model-header remove <k>  Remove a custom header by key.
  config model-header list        List all custom headers.`
}

func ResolveOpenRouterModel() (string, error) {
	cfg, err := config.LoadGlobal()
	if err != nil {
		return "", err
	}
	if model := strings.TrimSpace(cfg.OpenRouterModel); model != "" {
		return model, nil
	}
	return ai.DefaultModel(), nil
}

func ResolveOpenRouterAPIKey() (string, error) {
	cfg, err := config.LoadGlobal()
	if err != nil {
		return "", err
	}
	if key := strings.TrimSpace(cfg.OpenRouterAPIKey); key != "" {
		return key, nil
	}
	return "", fmt.Errorf("missing OpenRouter API key; set it with `airelease config openrouter-api-key <api-key>`")
}
