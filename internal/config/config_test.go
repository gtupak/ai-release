package config

import (
	"os"
	"path/filepath"
	"testing"
)

func TestLoadGlobalMissingFileReturnsEmptyConfig(t *testing.T) {
	home := t.TempDir()
	oldHome := os.Getenv("HOME")
	t.Cleanup(func() {
		_ = os.Setenv("HOME", oldHome)
	})
	if err := os.Setenv("HOME", home); err != nil {
		t.Fatalf("Setenv(HOME) error: %v", err)
	}

	cfg, err := LoadGlobal()
	if err != nil {
		t.Fatalf("LoadGlobal() unexpected error: %v", err)
	}
	if cfg.OpenRouterAPIKey != "" {
		t.Fatalf("expected empty OpenRouter key, got %q", cfg.OpenRouterAPIKey)
	}
	if len(cfg.RepoBaseBranches) != 0 {
		t.Fatalf("expected empty repo map, got %#v", cfg.RepoBaseBranches)
	}
}

func TestLoadGlobalMalformedConfig(t *testing.T) {
	home := t.TempDir()
	oldHome := os.Getenv("HOME")
	t.Cleanup(func() {
		_ = os.Setenv("HOME", oldHome)
	})
	if err := os.Setenv("HOME", home); err != nil {
		t.Fatalf("Setenv(HOME) error: %v", err)
	}

	cfgPath := filepath.Join(home, ".airelease", "config.json")
	if err := os.MkdirAll(filepath.Dir(cfgPath), 0o755); err != nil {
		t.Fatalf("MkdirAll() error: %v", err)
	}
	if err := os.WriteFile(cfgPath, []byte("{invalid"), 0o644); err != nil {
		t.Fatalf("WriteFile() error: %v", err)
	}

	_, err := LoadGlobal()
	if err == nil {
		t.Fatal("expected parse error, got nil")
	}
}

func TestSaveAndLoadGlobalRoundTrip(t *testing.T) {
	home := t.TempDir()
	oldHome := os.Getenv("HOME")
	t.Cleanup(func() {
		_ = os.Setenv("HOME", oldHome)
	})
	if err := os.Setenv("HOME", home); err != nil {
		t.Fatalf("Setenv(HOME) error: %v", err)
	}

	want := GlobalConfig{OpenRouterAPIKey: "test-key"}
	if err := SaveGlobal(want); err != nil {
		t.Fatalf("SaveGlobal() error: %v", err)
	}

	got, err := LoadGlobal()
	if err != nil {
		t.Fatalf("LoadGlobal() error: %v", err)
	}
	if got.OpenRouterAPIKey != want.OpenRouterAPIKey {
		t.Fatalf("expected api key %q, got %q", want.OpenRouterAPIKey, got.OpenRouterAPIKey)
	}
}

func TestSaveAndLoadGlobalModelRoundTrip(t *testing.T) {
	home := t.TempDir()
	oldHome := os.Getenv("HOME")
	t.Cleanup(func() {
		_ = os.Setenv("HOME", oldHome)
	})
	if err := os.Setenv("HOME", home); err != nil {
		t.Fatalf("Setenv(HOME) error: %v", err)
	}

	want := GlobalConfig{OpenRouterModel: "qwen/qwen3.5-flash-02-23"}
	if err := SaveGlobal(want); err != nil {
		t.Fatalf("SaveGlobal() error: %v", err)
	}

	got, err := LoadGlobal()
	if err != nil {
		t.Fatalf("LoadGlobal() error: %v", err)
	}
	if got.OpenRouterModel != want.OpenRouterModel {
		t.Fatalf("expected model %q, got %q", want.OpenRouterModel, got.OpenRouterModel)
	}
}

func TestSetAndGetRepoBaseBranch(t *testing.T) {
	home := t.TempDir()
	repoRoot := filepath.Join(home, "work", "repo-a")
	if err := os.MkdirAll(repoRoot, 0o755); err != nil {
		t.Fatalf("MkdirAll(repoRoot) error: %v", err)
	}

	oldHome := os.Getenv("HOME")
	t.Cleanup(func() {
		_ = os.Setenv("HOME", oldHome)
	})
	if err := os.Setenv("HOME", home); err != nil {
		t.Fatalf("Setenv(HOME) error: %v", err)
	}

	if err := SetRepoBaseBranch(repoRoot, "develop"); err != nil {
		t.Fatalf("SetRepoBaseBranch() error: %v", err)
	}

	got, err := GetRepoBaseBranch(repoRoot)
	if err != nil {
		t.Fatalf("GetRepoBaseBranch() error: %v", err)
	}
	if got != "develop" {
		t.Fatalf("expected base branch %q, got %q", "develop", got)
	}
}
