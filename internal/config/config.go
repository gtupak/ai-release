package config

import (
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strings"
)

const (
	globalDirName  = ".airelease"
	globalFileName = "config.json"
)

type GlobalConfig struct {
	OpenRouterAPIKey string            `json:"openrouter_api_key"`
	OpenRouterModel  string            `json:"openrouter_model,omitempty"`
	RepoBaseBranches map[string]string `json:"repo_base_branches,omitempty"`
}

func GlobalPath() (string, error) {
	home, err := os.UserHomeDir()
	if err != nil {
		return "", fmt.Errorf("resolve home directory: %w", err)
	}
	return filepath.Join(home, globalDirName, globalFileName), nil
}

func LoadGlobal() (GlobalConfig, error) {
	cfgPath, err := GlobalPath()
	if err != nil {
		return GlobalConfig{}, err
	}

	b, err := os.ReadFile(cfgPath)
	if errors.Is(err, os.ErrNotExist) {
		return GlobalConfig{}, nil
	}
	if err != nil {
		return GlobalConfig{}, fmt.Errorf("read %s: %w", cfgPath, err)
	}

	var cfg GlobalConfig
	if err := json.Unmarshal(b, &cfg); err != nil {
		return GlobalConfig{}, fmt.Errorf("parse %s: %w", cfgPath, err)
	}
	cfg.OpenRouterAPIKey = strings.TrimSpace(cfg.OpenRouterAPIKey)
	cfg.OpenRouterModel = strings.TrimSpace(cfg.OpenRouterModel)
	if cfg.RepoBaseBranches == nil {
		cfg.RepoBaseBranches = map[string]string{}
	}
	return cfg, nil
}

func SaveGlobal(cfg GlobalConfig) error {
	cfgPath, err := GlobalPath()
	if err != nil {
		return err
	}

	cfg.OpenRouterAPIKey = strings.TrimSpace(cfg.OpenRouterAPIKey)
	cfg.OpenRouterModel = strings.TrimSpace(cfg.OpenRouterModel)
	if cfg.RepoBaseBranches == nil {
		cfg.RepoBaseBranches = map[string]string{}
	}
	for repoPath, branch := range cfg.RepoBaseBranches {
		cleanRepoPath := strings.TrimSpace(repoPath)
		cleanBranch := strings.TrimSpace(branch)
		if cleanRepoPath == "" || cleanBranch == "" {
			delete(cfg.RepoBaseBranches, repoPath)
			continue
		}
		if cleanRepoPath != repoPath {
			delete(cfg.RepoBaseBranches, repoPath)
			cfg.RepoBaseBranches[cleanRepoPath] = cleanBranch
			continue
		}
		cfg.RepoBaseBranches[repoPath] = cleanBranch
	}

	b, err := json.MarshalIndent(cfg, "", "  ")
	if err != nil {
		return fmt.Errorf("marshal global config: %w", err)
	}
	b = append(b, '\n')

	if err := os.MkdirAll(filepath.Dir(cfgPath), 0o755); err != nil {
		return fmt.Errorf("create config directory %s: %w", filepath.Dir(cfgPath), err)
	}
	if err := os.WriteFile(cfgPath, b, 0o600); err != nil {
		return fmt.Errorf("write %s: %w", cfgPath, err)
	}
	return nil
}

func SetRepoBaseBranch(repoRoot, branch string) error {
	repoKey, err := normalizeRepoPath(repoRoot)
	if err != nil {
		return err
	}
	branch = strings.TrimSpace(branch)
	if branch == "" {
		return fmt.Errorf("branch cannot be empty")
	}

	cfg, err := LoadGlobal()
	if err != nil {
		return err
	}
	if cfg.RepoBaseBranches == nil {
		cfg.RepoBaseBranches = map[string]string{}
	}
	cfg.RepoBaseBranches[repoKey] = branch
	return SaveGlobal(cfg)
}

func GetRepoBaseBranch(repoRoot string) (string, error) {
	repoKey, err := normalizeRepoPath(repoRoot)
	if err != nil {
		return "", err
	}

	cfg, err := LoadGlobal()
	if err != nil {
		return "", err
	}
	return strings.TrimSpace(cfg.RepoBaseBranches[repoKey]), nil
}

func normalizeRepoPath(repoRoot string) (string, error) {
	repoRoot = strings.TrimSpace(repoRoot)
	if repoRoot == "" {
		return "", fmt.Errorf("repo path cannot be empty")
	}
	absPath, err := filepath.Abs(repoRoot)
	if err != nil {
		return "", fmt.Errorf("resolve absolute repo path: %w", err)
	}
	return filepath.Clean(absPath), nil
}
