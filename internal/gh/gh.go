package gh

import (
	"bytes"
	"encoding/json"
	"fmt"
	"os"
	"os/exec"
	"strings"
)

type RepoView struct {
	NameWithOwner    string `json:"nameWithOwner"`
	DefaultBranchRef struct {
		Name string `json:"name"`
	} `json:"defaultBranchRef"`
}

type CompareResponse struct {
	Commits []struct {
		Commit struct {
			Message string `json:"message"`
		} `json:"commit"`
	} `json:"commits"`
}

type PullRequest struct {
	Number int    `json:"number"`
	Title  string `json:"title"`
	URL    string `json:"url"`
	Body   string `json:"body"`
}

type ReleaseView struct {
	TagName     string `json:"tag_name"`
	Name        string `json:"name"`
	PublishedAt string `json:"published_at"`
}

func EnsureInstalled() error {
	if _, err := exec.LookPath("gh"); err != nil {
		return fmt.Errorf("GitHub CLI `gh` is required but was not found in PATH")
	}
	return nil
}

func EnsureAuthHealthy() error {
	_, err := run("", "auth", "status")
	if err != nil {
		return fmt.Errorf("GitHub CLI is not authenticated. Run `gh auth login` first")
	}
	return nil
}

func Repo(repoRoot string) (RepoView, error) {
	out, err := run(repoRoot, "repo", "view", "--json", "nameWithOwner,defaultBranchRef")
	if err != nil {
		return RepoView{}, err
	}
	var repo RepoView
	if err := json.Unmarshal([]byte(out), &repo); err != nil {
		return RepoView{}, fmt.Errorf("parse gh repo view output: %w", err)
	}
	return repo, nil
}

func LatestRelease(repoRoot, slug string) (ReleaseView, bool, error) {
	out, err := run(repoRoot, "api", "/repos/"+slug+"/releases/latest")
	if err != nil {
		errStr := err.Error()
		if strings.Contains(errStr, "404") || strings.Contains(errStr, "Not Found") {
			return ReleaseView{}, false, nil
		}
		return ReleaseView{}, false, err
	}

	// If no JSON output was returned, it's likely a 404
	if strings.TrimSpace(out) == "" {
		fmt.Fprintf(os.Stderr, "DEBUG: GitHub API returned empty output for /repos/%s/releases/latest\n", slug)
		return ReleaseView{}, false, nil
	}

	var release ReleaseView
	if err := json.Unmarshal([]byte(out), &release); err != nil {
		return ReleaseView{}, false, fmt.Errorf("parse latest release payload: %w", err)
	}
	return release, true, nil
}

func Compare(repoRoot, slug, baseTag, branch string) (CompareResponse, error) {
	out, err := run(repoRoot, "api", "/repos/"+slug+"/compare/"+baseTag+"..."+branch)
	if err != nil {
		return CompareResponse{}, err
	}
	var payload CompareResponse
	if err := json.Unmarshal([]byte(out), &payload); err != nil {
		return CompareResponse{}, fmt.Errorf("parse compare payload: %w", err)
	}
	return payload, nil
}

func ListMergedPRNumbers(repoRoot, slug, baseBranch, mergedAtOrAfter string, limit int) ([]int, error) {
	args := []string{
		"pr", "list",
		"--repo", slug,
		"--state", "merged",
		"--base", baseBranch,
		"--limit", fmt.Sprintf("%d", limit),
		"--json", "number",
	}
	if strings.TrimSpace(mergedAtOrAfter) != "" {
		args = append(args, "--search", "merged:>="+mergedAtOrAfter)
	}

	out, err := run(repoRoot, args...)
	if err != nil {
		return nil, err
	}
	var items []struct {
		Number int `json:"number"`
	}
	if err := json.Unmarshal([]byte(out), &items); err != nil {
		return nil, fmt.Errorf("parse merged PR list: %w", err)
	}

	numbers := make([]int, 0, len(items))
	for _, item := range items {
		if item.Number > 0 {
			numbers = append(numbers, item.Number)
		}
	}
	return numbers, nil
}

func ViewPR(repoRoot, slug string, number int) (PullRequest, error) {
	out, err := run(
		repoRoot,
		"pr", "view", fmt.Sprintf("%d", number),
		"--repo", slug,
		"--json", "number,title,url,body",
	)
	if err != nil {
		return PullRequest{}, err
	}
	var pr PullRequest
	if err := json.Unmarshal([]byte(out), &pr); err != nil {
		return PullRequest{}, fmt.Errorf("parse PR payload for #%d: %w", number, err)
	}
	return pr, nil
}

func ReleaseExists(repoRoot, slug, tag string) (bool, error) {
	if err := EnsureInstalled(); err != nil {
		return false, err
	}

	cmd := exec.Command("gh", "release", "view", tag, "--repo", slug)
	if repoRoot != "" {
		cmd.Dir = repoRoot
	}
	var stderr bytes.Buffer
	cmd.Stderr = &stderr
	err := cmd.Run()
	if err != nil {
		stderrStr := stderr.String()
		if strings.Contains(stderrStr, "release not found") ||
			strings.Contains(stderrStr, "404") ||
			strings.Contains(err.Error(), "release not found") ||
			strings.Contains(err.Error(), "404") {
			return false, nil
		}
		return false, fmt.Errorf("gh release view %s --repo %s failed: %s", tag, slug, strings.TrimSpace(stderrStr))
	}
	return true, nil
}

func CreateRelease(repoRoot, slug, tag, title, target, notesFile string) error {
	_, err := run(
		repoRoot,
		"release", "create", tag,
		"--repo", slug,
		"--title", title,
		"--target", target,
		"--notes-file", notesFile,
	)
	if err != nil {
		return fmt.Errorf("gh release create failed: %w", err)
	}
	return nil
}

func run(repoRoot string, args ...string) (string, error) {
	if err := EnsureInstalled(); err != nil {
		return "", err
	}

	cmd := exec.Command("gh", args...)
	if repoRoot != "" {
		cmd.Dir = repoRoot
	}
	var stdout, stderr bytes.Buffer
	cmd.Stdout = &stdout
	cmd.Stderr = &stderr

	err := cmd.Run()
	if err != nil {
		stderrStr := stderr.String()
		stdoutStr := stdout.String()

		// Check if stderr contains JSON error response from GitHub API
		// even when exit code is 0 (gh sometimes outputs errors to stderr regardless of status)
		if strings.Contains(stderrStr, "status:") {
			jsonStart := strings.Index(stderrStr, "{")
			if jsonStart >= 0 {
				jsonEnd := strings.LastIndex(stderrStr, "}")
				if jsonEnd > jsonStart {
					jsonPart := stderrStr[jsonStart : jsonEnd+1]
					if strings.Contains(jsonPart, "404") {
						return "", fmt.Errorf("gh %s failed: %s", strings.Join(args, " "), strings.TrimSpace(stderrStr))
					}
					// Return the JSON even with error
					return jsonPart, fmt.Errorf("gh %s failed: %s", strings.Join(args, " "), strings.TrimSpace(stderrStr))
				}
			}
		}

		// Fall back to normal error handling
		output := strings.TrimSpace(stdoutStr)
		if output != "" {
			return output, fmt.Errorf("gh %s failed: %s", strings.Join(args, " "), strings.TrimSpace(stderrStr))
		}
		return "", fmt.Errorf("gh %s failed: %s", strings.Join(args, " "), strings.TrimSpace(stderrStr))
	}

	output := strings.TrimSpace(stdout.String())
	return output, nil
}
