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
	if len(args) == 0 {
		return release.Create(defaultBaseBranch)
	}

	if args[0] == "config" {
		return runConfig(args[1:])
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
	case "model":
		return runConfigModel(args[1:])
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

func usage() string {
	return `Usage:
  airelease
  airelease config base <branch>
  airelease config openrouter-api-key <api-key>
  airelease config model <openrouter-model>

Commands:
  (no args)           Create a GitHub release from merged PRs.
  config base <name>  Save default base branch for this repository.
  config openrouter-api-key <key>
                     Save a global OpenRouter API key.
  config model <name> Save a global OpenRouter model.`
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
