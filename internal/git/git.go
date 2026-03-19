package git

import (
	"fmt"
	"os/exec"
	"strings"
)

func RepoRoot() (string, error) {
	out, err := runGit("", "rev-parse", "--show-toplevel")
	if err != nil {
		return "", fmt.Errorf("unable to find git repo root: %w", err)
	}
	return strings.TrimSpace(out), nil
}

func CurrentBranch(repoRoot string) (string, error) {
	out, err := runGit(repoRoot, "symbolic-ref", "--short", "HEAD")
	if err != nil {
		return "", fmt.Errorf("resolve current branch: %w", err)
	}
	branch := strings.TrimSpace(out)
	if branch == "" || branch == "HEAD" {
		return "", fmt.Errorf("detached HEAD is not supported")
	}
	return branch, nil
}

func HasCommits(repoRoot string) bool {
	_, err := runGit(repoRoot, "rev-parse", "--verify", "--quiet", "HEAD")
	return err == nil
}

func runGit(repoRoot string, args ...string) (string, error) {
	cmd := exec.Command("git", args...)
	if repoRoot != "" {
		cmd.Dir = repoRoot
	}
	out, err := cmd.CombinedOutput()
	if err != nil {
		return "", fmt.Errorf("git %s failed: %s", strings.Join(args, " "), strings.TrimSpace(string(out)))
	}
	return string(out), nil
}
