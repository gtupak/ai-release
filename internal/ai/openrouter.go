package ai

import (
	"airelease/internal/config"
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"strings"
	"time"
)

const (
	defaultModel = "qwen/qwen3.5-flash-02-23"
	endpointURL  = "https://openrouter.ai/api/v1/chat/completions"
)

type PRContext struct {
	Number int
	Title  string
	URL    string
	Body   string
}

type openRouterRequest struct {
	Model       string             `json:"model"`
	Temperature float64            `json:"temperature"`
	Messages    []openRouterPrompt `json:"messages"`
}

type openRouterPrompt struct {
	Role    string `json:"role"`
	Content string `json:"content"`
}

type openRouterResponse struct {
	Choices []struct {
		Message struct {
			Content string `json:"content"`
		} `json:"message"`
	} `json:"choices"`
	Error *struct {
		Message string `json:"message"`
	} `json:"error,omitempty"`
}

type ReleaseSuggestion struct {
	SuggestedVersion string   `json:"suggested_version"`
	Titles           []string `json:"titles"`
}

func DefaultModel() string {
	return defaultModel
}

func GetCustomModelURL() (string, error) {
	cfg, err := config.LoadGlobal()
	if err != nil {
		return "", err
	}
	return cfg.CustomModelURL, nil
}

func GenerateReleaseSuggestion(
	apiKey, model, repoSlug, baseBranch, latestReleaseTitle, latestReleaseTag string,
	prs []PRContext,
	customModelURL string,
	customHeaders map[string]string,
	openRouterMode bool,
) (ReleaseSuggestion, error) {
	apiKey = strings.TrimSpace(apiKey)
	if apiKey == "" {
		return ReleaseSuggestion{}, fmt.Errorf("OpenRouter API key is required to generate release suggestions")
	}
	model = strings.TrimSpace(model)
	if model == "" {
		model = defaultModel
	}

	endpoint := endpointURL
	if !openRouterMode && strings.TrimSpace(customModelURL) != "" {
		baseURL := strings.TrimSuffix(strings.TrimSpace(customModelURL), "/")
		if !strings.HasSuffix(baseURL, "/chat/completions") {
			baseURL = baseURL + "/chat/completions"
		}
		endpoint = baseURL
	}

	reqBody := openRouterRequest{
		Model:       model,
		Temperature: 0.6,
		Messages: []openRouterPrompt{
			{
				Role: "system",
				Content: "You help create semantic versioned releases. " +
					"Return ONLY valid JSON with this exact shape: " +
					"{\"suggested_version\":\"X.Y.Z\",\"titles\":[\"title 1\",\"title 2\",\"title 3\"]}. " +
					"Rules: suggested_version has no leading v; use semver; titles must be concise and ready for GitHub release names. " +
					"IMPORTANT: Each title must summarize ALL the PRs together into a single cohesive release title. " +
					"Do not create titles for individual PRs - create titles that capture the overall theme or purpose of all PRs combined.",
			},
			{
				Role:    "user",
				Content: buildPrompt(repoSlug, baseBranch, latestReleaseTitle, latestReleaseTag, prs),
			},
		},
	}

	payload, err := json.Marshal(reqBody)
	if err != nil {
		return ReleaseSuggestion{}, fmt.Errorf("marshal OpenRouter request: %w", err)
	}

	req, err := http.NewRequest(http.MethodPost, endpoint, bytes.NewReader(payload))
	if err != nil {
		return ReleaseSuggestion{}, fmt.Errorf("create OpenRouter request: %w", err)
	}

	req.Header.Set("Authorization", "Bearer "+apiKey)
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("HTTP-Referer", "https://github.com")
	req.Header.Set("X-Title", "airelease")
	if !openRouterMode && len(customHeaders) > 0 {
		for key, value := range customHeaders {
			req.Header.Set(key, value)
		}
	}

	client := &http.Client{Timeout: 45 * time.Second}
	resp, err := client.Do(req)
	if err != nil {
		return ReleaseSuggestion{}, fmt.Errorf("call OpenRouter: %w", err)
	}
	defer resp.Body.Close()

	body, _ := io.ReadAll(resp.Body)
	resp.Body = io.NopCloser(bytes.NewReader(body))

	var decoded openRouterResponse
	if err := json.NewDecoder(bytes.NewReader(body)).Decode(&decoded); err != nil {
		return ReleaseSuggestion{}, fmt.Errorf("decode OpenRouter response: %w", err)
	}

	if resp.StatusCode >= 300 {
		if decoded.Error != nil && strings.TrimSpace(decoded.Error.Message) != "" {
			return ReleaseSuggestion{}, fmt.Errorf("OpenRouter error (%d): %s", resp.StatusCode, decoded.Error.Message)
		}
		return ReleaseSuggestion{}, fmt.Errorf("OpenRouter error status: %d", resp.StatusCode)
	}

	if len(decoded.Choices) == 0 || strings.TrimSpace(decoded.Choices[0].Message.Content) == "" {
		return ReleaseSuggestion{}, fmt.Errorf("OpenRouter returned no message content")
	}

	return parseReleaseSuggestion(decoded.Choices[0].Message.Content)
}

func parseReleaseSuggestion(raw string) (ReleaseSuggestion, error) {
	text := stripFences(strings.TrimSpace(raw))

	var suggestion ReleaseSuggestion
	if err := json.Unmarshal([]byte(text), &suggestion); err != nil {
		return ReleaseSuggestion{}, fmt.Errorf("parse OpenRouter JSON suggestion: %w", err)
	}

	suggestion.SuggestedVersion = strings.TrimPrefix(strings.TrimSpace(suggestion.SuggestedVersion), "v")
	if !isValidSemver(suggestion.SuggestedVersion) {
		suggestion.SuggestedVersion = ""
	}

	titles := make([]string, 0, len(suggestion.Titles))
	for _, title := range suggestion.Titles {
		title = strings.TrimSpace(title)
		if title != "" {
			titles = append(titles, title)
		}
	}
	suggestion.Titles = titles
	return suggestion, nil
}

func buildPrompt(repoSlug, baseBranch, latestReleaseTitle, latestReleaseTag string, prs []PRContext) string {
	var b strings.Builder
	b.WriteString("Repository: " + repoSlug + "\n")
	b.WriteString("Base branch: " + baseBranch + "\n")
	if strings.TrimSpace(latestReleaseTitle) != "" || strings.TrimSpace(latestReleaseTag) != "" {
		b.WriteString("Previous release: " + strings.TrimSpace(latestReleaseTitle) + " (" + strings.TrimSpace(latestReleaseTag) + ")\n")
	}
	b.WriteString("\nAll PRs included in this release (create titles that summarize ALL of these together):\n")
	if len(prs) == 0 {
		b.WriteString("(No PR descriptions available.)\n")
		return b.String()
	}

	for _, pr := range prs {
		b.WriteString(fmt.Sprintf("PR #%d\n", pr.Number))
		b.WriteString("Title: " + sanitize(pr.Title) + "\n")
		b.WriteString("URL: " + sanitize(pr.URL) + "\n")
		body := strings.TrimSpace(pr.Body)
		if body == "" {
			body = "(no description)"
		}
		b.WriteString("Description:\n")
		b.WriteString(body + "\n\n")
	}
	b.WriteString("Generate 3 release title options that summarize ALL the PRs above into a single cohesive release title. " +
		"Each title should capture the overall theme or purpose of all changes combined, not individual PR titles.")
	return b.String()
}

func stripFences(s string) string {
	trim := strings.TrimSpace(s)
	if strings.HasPrefix(trim, "```") && strings.HasSuffix(trim, "```") {
		trim = strings.TrimPrefix(trim, "```")
		trim = strings.TrimSuffix(trim, "```")
		trim = strings.TrimSpace(trim)
		parts := strings.SplitN(trim, "\n", 2)
		if len(parts) == 2 {
			if !strings.Contains(parts[0], ":") && len(parts[0]) < 20 {
				return strings.TrimSpace(parts[1])
			}
		}
	}
	return trim
}

func sanitize(s string) string {
	return strings.TrimSpace(strings.ReplaceAll(s, "\n", " "))
}

func isValidSemver(version string) bool {
	if version == "" {
		return false
	}
	for _, part := range strings.SplitN(version, "-", 2) {
		_ = part
	}
	chunks := strings.SplitN(version, "-", 2)
	core := chunks[0]
	coreParts := strings.Split(core, ".")
	if len(coreParts) != 3 {
		return false
	}
	for _, p := range coreParts {
		if p == "" {
			return false
		}
		for _, r := range p {
			if r < '0' || r > '9' {
				return false
			}
		}
	}
	if len(chunks) == 2 {
		suffix := chunks[1]
		if suffix == "" {
			return false
		}
		for _, r := range suffix {
			if (r >= 'a' && r <= 'z') || (r >= 'A' && r <= 'Z') || (r >= '0' && r <= '9') || r == '.' {
				continue
			}
			return false
		}
	}
	return true
}
