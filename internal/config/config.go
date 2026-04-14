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
	CustomModelURL   string            `json:"custom_model_url,omitempty"`
	CustomHeaders    map[string]string `json:"custom_headers,omitempty"`
	OpenRouterMode   bool              `json:"openrouter_mode"`
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
	cfg.CustomModelURL = strings.TrimSpace(cfg.CustomModelURL)
	if cfg.CustomHeaders == nil {
		cfg.CustomHeaders = map[string]string{}
	}
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
	cfg.CustomModelURL = strings.TrimSpace(cfg.CustomModelURL)
	if cfg.CustomHeaders == nil {
		cfg.CustomHeaders = map[string]string{}
	}
	for key, value := range cfg.CustomHeaders {
		cleanKey := strings.TrimSpace(key)
		cleanValue := strings.TrimSpace(value)
		if cleanKey == "" || cleanValue == "" {
			delete(cfg.CustomHeaders, key)
			continue
		}
		if cleanKey != key {
			delete(cfg.CustomHeaders, key)
			cfg.CustomHeaders[cleanKey] = cleanValue
			continue
		}
		cfg.CustomHeaders[key] = cleanValue
	}
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

func SetCustomModelURL(url string, disableOpenRouter bool) error {
	url = strings.TrimSpace(url)
	if url == "" {
		return fmt.Errorf("URL cannot be empty")
	}

	cfg, err := LoadGlobal()
	if err != nil {
		return err
	}
	cfg.CustomModelURL = url
	if disableOpenRouter {
		cfg.OpenRouterMode = false
	}
	return SaveGlobal(cfg)
}

func SetCustomHeader(key, value string, disableOpenRouter bool) error {
	key = strings.TrimSpace(key)
	value = strings.TrimSpace(value)
	if key == "" {
		return fmt.Errorf("header key cannot be empty")
	}
	if value == "" {
		return fmt.Errorf("header value cannot be empty")
	}

	cfg, err := LoadGlobal()
	if err != nil {
		return err
	}
	if cfg.CustomHeaders == nil {
		cfg.CustomHeaders = map[string]string{}
	}
	cfg.CustomHeaders[key] = value
	if disableOpenRouter {
		cfg.OpenRouterMode = false
	}
	return SaveGlobal(cfg)
}

func RemoveCustomHeader(key string) error {
	key = strings.TrimSpace(key)
	if key == "" {
		return fmt.Errorf("header key cannot be empty")
	}

	cfg, err := LoadGlobal()
	if err != nil {
		return err
	}
	if cfg.CustomHeaders == nil {
		cfg.CustomHeaders = map[string]string{}
	}
	if _, exists := cfg.CustomHeaders[key]; !exists {
		return fmt.Errorf("header %q does not exist", key)
	}
	delete(cfg.CustomHeaders, key)
	return SaveGlobal(cfg)
}

func GetCustomModelURL() (string, error) {
	cfg, err := LoadGlobal()
	if err != nil {
		return "", err
	}
	return cfg.CustomModelURL, nil
}

func GetCustomHeaders() (map[string]string, error) {
	cfg, err := LoadGlobal()
	if err != nil {
		return nil, err
	}
	if cfg.CustomHeaders == nil {
		return map[string]string{}, nil
	}
	return cfg.CustomHeaders, nil
}

func SetOpenRouterMode(mode bool) error {
	cfg, err := LoadGlobal()
	if err != nil {
		return err
	}
	cfg.OpenRouterMode = mode
	return SaveGlobal(cfg)
}

func GetOpenRouterMode() (bool, error) {
	cfg, err := LoadGlobal()
	if err != nil {
		return true, err
	}
	return cfg.OpenRouterMode, nil
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
