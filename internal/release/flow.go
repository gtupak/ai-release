package release

import (
	"bufio"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"time"

	"airelease/internal/ai"
	"airelease/internal/config"
	"airelease/internal/gh"
	"airelease/internal/git"
)

type releaseCandidate struct {
	tag   string
	title string
	notes string
}

func Create(defaultBaseBranch string) error {
	if err := gh.EnsureInstalled(); err != nil {
		return err
	}

	repoRoot, err := git.RepoRoot()
	if err != nil {
		return err
	}
	if !git.HasCommits(repoRoot) {
		return fmt.Errorf("repository has no commits yet")
	}
	if _, err := os.Stat(filepath.Join(repoRoot, ".git")); err != nil {
		return fmt.Errorf("this command must be run inside a git repository")
	}
	if _, err := git.CurrentBranch(repoRoot); err != nil {
		return err
	}
	if err := gh.EnsureAuthHealthy(); err != nil {
		return err
	}

	repo, err := gh.Repo(repoRoot)
	if err != nil {
		return err
	}
	repoSlug := strings.TrimSpace(repo.NameWithOwner)
	defaultBranch := strings.TrimSpace(repo.DefaultBranchRef.Name)
	if defaultBranch == "" {
		defaultBranch = defaultBaseBranch
	}

	baseBranch, err := resolveBaseBranch(repoRoot, defaultBranch, defaultBaseBranch)
	if err != nil {
		return err
	}

	printHeader("Repository context")
	fmt.Printf("Repository: %s\n", repoSlug)
	fmt.Printf("Default branch: %s\n", defaultBranch)
	fmt.Printf("Base branch: %s\n", baseBranch)

	printHeader("Latest release")
	latest, foundLatest, err := gh.LatestRelease(repoRoot, repoSlug)
	if err != nil {
		return err
	}

	latestTitle := ""
	latestTag := ""
	latestPublishedDate := ""
	if foundLatest {
		latestTitle = strings.TrimSpace(latest.Name)
		if latestTitle == "" {
			latestTitle = "(untitled release)"
		}
		latestTag = strings.TrimSpace(latest.TagName)
		latestPublishedDate = publishedDate(latest.PublishedAt)
		fmt.Printf("Title: %s\n", latestTitle)
		fmt.Printf("Tag: %s\n", latestTag)
	} else {
		fmt.Println("No existing release found.")
	}

	printHeader("Pull requests included since last release")
	prNumbers, err := discoverPRNumbers(repoRoot, repoSlug, baseBranch, latestTag, latestPublishedDate)
	if err != nil {
		return err
	}

	prs, prLines, err := loadPRContext(repoRoot, repoSlug, prNumbers)
	if err != nil {
		return err
	}
	if len(prs) == 0 {
		fmt.Println("No merged pull requests detected since the last release.")
	}

	printHeader("Create release")
	customModelURL, _ := config.GetCustomModelURL()
	customHeaders, _ := config.GetCustomHeaders()
	openRouterMode, _ := config.GetOpenRouterMode()

	apiKey, apiKeyErr := resolveOpenRouterAPIKey()
	hasCustomURL := strings.TrimSpace(customModelURL) != ""
	hasCustomHeaders := len(customHeaders) > 0

	suggestion := ai.ReleaseSuggestion{}
	releaseTitle := ""
	suggestedVersion := ""

	if apiKeyErr != nil && !hasCustomURL && !hasCustomHeaders {
		fmt.Println("OpenRouter API key is not configured. Skipping AI suggestions.")
	} else {
		if apiKeyErr != nil && hasCustomURL {
			if authHeader, exists := customHeaders["Authorization"]; exists {
				apiKey = strings.TrimPrefix(authHeader, "Bearer ")
			}
		}
		if apiKey == "" && hasCustomURL {
			if authHeader, exists := customHeaders["Authorization"]; exists {
				apiKey = authHeader
			}
		}

		printHeader("AI release suggestions")
		model, _ := resolveOpenRouterModel()
		stopSpinner := startSpinner()
		suggestion, err = ai.GenerateReleaseSuggestion(
			apiKey,
			model,
			repoSlug,
			baseBranch,
			latestTitle,
			latestTag,
			prs,
			customModelURL,
			customHeaders,
			openRouterMode,
		)
		stopSpinner()
		fmt.Println() // New line after spinner stops
		if err != nil {
			fmt.Println("Warning: OpenRouter suggestions failed:")
			fmt.Println(err)
		} else {
			suggestedVersion = suggestion.SuggestedVersion
			if suggestedVersion != "" {
				fmt.Printf("Suggested version: v%s\n", suggestedVersion)
			} else {
				fmt.Println("No valid version suggestion returned by AI.")
			}

			if len(suggestion.Titles) >= 3 {
				selected, err := chooseReleaseTitle(suggestion.Titles)
				if err != nil {
					return err
				}
				releaseTitle = selected
			} else if len(suggestion.Titles) > 0 {
				fmt.Println("Warning: OpenRouter did not return 3 usable titles. Please enter your own.")
			}
		}
	}

	version, err := chooseVersion(latestTag, suggestedVersion)
	if err != nil {
		return err
	}

	if strings.TrimSpace(releaseTitle) == "" {
		releaseTitle, err = promptRequired("Enter release title: ")
		if err != nil {
			return err
		}
	}

	tag := "v" + version
	exists, err := gh.ReleaseExists(repoRoot, repoSlug, tag)
	if err != nil {
		return err
	}
	if exists {
		return fmt.Errorf("release %q already exists", tag)
	}

	candidate := releaseCandidate{
		tag:   tag,
		title: releaseTitle,
		notes: buildNotes(prLines),
	}

	printReleaseSummary(candidate, latestTitle, latestTag, len(prLines))
	confirm, err := prompt("Proceed with creating this release? (yes/no): ")
	if err != nil {
		return err
	}
	if strings.TrimSpace(confirm) != "yes" {
		fmt.Println("Release creation cancelled.")
		return nil
	}

	notesFile, err := writeNotesTempFile(candidate.notes)
	if err != nil {
		return err
	}
	defer os.Remove(notesFile)

	if err := gh.CreateRelease(repoRoot, repoSlug, candidate.tag, candidate.title, baseBranch, notesFile); err != nil {
		return err
	}
	fmt.Printf("Release created successfully: %s\n", candidate.tag)
	return nil
}

func resolveBaseBranch(repoRoot, defaultBranch, fallbackBranch string) (string, error) {
	cfgBranch, err := config.GetRepoBaseBranch(repoRoot)
	if err != nil {
		return "", err
	}
	if strings.TrimSpace(cfgBranch) != "" {
		return strings.TrimSpace(cfgBranch), nil
	}
	if strings.TrimSpace(defaultBranch) != "" {
		return strings.TrimSpace(defaultBranch), nil
	}
	return strings.TrimSpace(fallbackBranch), nil
}

func discoverPRNumbers(repoRoot, repoSlug, baseBranch, latestTag, latestPublishedDate string) ([]int, error) {
	numbers := make([]int, 0)
	if strings.TrimSpace(latestTag) != "" {
		comparePayload, err := gh.Compare(repoRoot, repoSlug, latestTag, baseBranch)
		if err == nil {
			for _, c := range comparePayload.Commits {
				numbers = append(numbers, extractPRNumbersFromCommitMessage(c.Commit.Message)...)
			}
		}
	}
	numbers = sortedUnique(numbers)
	if len(numbers) > 0 {
		return numbers, nil
	}

	fmt.Println("Could not detect PRs from tag comparison. Falling back to recent merged PRs.")
	limit := 30
	mergedAtOrAfter := ""
	if latestPublishedDate != "" {
		limit = 200
		mergedAtOrAfter = latestPublishedDate
	}
	fallback, err := gh.ListMergedPRNumbers(repoRoot, repoSlug, baseBranch, mergedAtOrAfter, limit)
	if err != nil {
		return nil, err
	}
	return sortedUnique(fallback), nil
}

func loadPRContext(repoRoot, repoSlug string, prNumbers []int) ([]ai.PRContext, []string, error) {
	prs := make([]ai.PRContext, 0, len(prNumbers))
	prLines := make([]string, 0, len(prNumbers))
	for i, num := range prNumbers {
		pr, err := gh.ViewPR(repoRoot, repoSlug, num)
		if err != nil {
			return nil, nil, err
		}
		line := fmt.Sprintf("- #%d %s (%s)", pr.Number, strings.TrimSpace(pr.Title), strings.TrimSpace(pr.URL))
		prLines = append(prLines, line)
		prs = append(prs, ai.PRContext{
			Number: pr.Number,
			Title:  strings.TrimSpace(pr.Title),
			URL:    strings.TrimSpace(pr.URL),
			Body:   strings.TrimSpace(pr.Body),
		})

		fmt.Printf("%d. %s\n", i+1, line)
		if desc := firstNonEmptyPRBodyLine(pr.Body); desc != "" {
			fmt.Printf("   %s\n", desc)
		}
	}
	return prs, prLines, nil
}

func chooseReleaseTitle(titles []string) (string, error) {
	fmt.Println("")
	fmt.Println("Suggested titles:")
	for i := 0; i < len(titles) && i < 3; i++ {
		fmt.Printf("%d) %s\n", i+1, titles[i])
	}
	fmt.Println("4) Write my own")
	choice, err := prompt("Choose release title option (1-4): ")
	if err != nil {
		return "", err
	}
	switch strings.TrimSpace(choice) {
	case "1":
		return titles[0], nil
	case "2":
		return titles[1], nil
	case "3":
		return titles[2], nil
	case "4", "":
		return promptRequired("Enter release title: ")
	default:
		fmt.Println("Invalid choice. Please enter a custom title.")
		return promptRequired("Enter release title: ")
	}
}

func chooseVersion(latestTag, suggestedVersion string) (string, error) {
	versionInput := ""
	if major, minor, patch, ok := parseSemverCore(latestTag); ok {
		majorVersion := fmt.Sprintf("%d.0.0", major+1)
		minorVersion := fmt.Sprintf("%d.%d.0", major, minor+1)
		patchVersion := fmt.Sprintf("%d.%d.%d", major, minor, patch+1)
		fmt.Println("")
		fmt.Println("Version options:")
		fmt.Printf("1) major: v%s\n", majorVersion)
		fmt.Printf("2) minor: v%s\n", minorVersion)
		fmt.Printf("3) patch: v%s\n", patchVersion)
		fmt.Println("4) Enter my own")
		choice, err := prompt("Choose version option (1-4): ")
		if err != nil {
			return "", err
		}
		switch strings.TrimSpace(choice) {
		case "1":
			versionInput = majorVersion
		case "2":
			versionInput = minorVersion
		case "3", "":
			versionInput = patchVersion
		case "4":
		default:
			fmt.Println("Invalid choice. Please enter a custom version.")
		}
	} else if strings.TrimSpace(suggestedVersion) != "" {
		versionInput = suggestedVersion
	}

	if strings.TrimSpace(versionInput) == "" {
		userInput, err := promptRequired("Enter release version (e.g. 1.4.0): ")
		if err != nil {
			return "", err
		}
		versionInput = userInput
	}
	return normalizeVersion(versionInput)
}

func buildNotes(prLines []string) string {
	var b strings.Builder
	b.WriteString("## Changes\n\n")
	if len(prLines) == 0 {
		b.WriteString("- No merged PRs detected since the previous release.\n")
		return b.String()
	}
	for _, line := range prLines {
		b.WriteString(line + "\n")
	}
	return b.String()
}

func writeNotesTempFile(notes string) (string, error) {
	f, err := os.CreateTemp("", "airelease-notes-*.md")
	if err != nil {
		return "", err
	}
	defer f.Close()

	if _, err := f.WriteString(notes); err != nil {
		return "", err
	}
	return f.Name(), nil
}

func printReleaseSummary(candidate releaseCandidate, latestTitle, latestTag string, prCount int) {
	fmt.Println("")
	fmt.Println("Release summary")
	fmt.Printf("Tag: %s\n", candidate.tag)
	fmt.Printf("Title: %s\n", candidate.title)
	if strings.TrimSpace(latestTitle) != "" || strings.TrimSpace(latestTag) != "" {
		fmt.Printf("Previous release: %s (%s)\n", latestTitle, latestTag)
	}
	fmt.Printf("Included PRs: %d\n", prCount)
}

func printHeader(title string) {
	fmt.Println("")
	fmt.Println("========================================")
	fmt.Println(title)
	fmt.Println("========================================")
}

func prompt(question string) (string, error) {
	reader := bufio.NewReader(os.Stdin)
	fmt.Print(question)
	line, err := reader.ReadString('\n')
	if err != nil {
		return "", err
	}
	return strings.TrimSpace(line), nil
}

func promptRequired(question string) (string, error) {
	value, err := prompt(question)
	if err != nil {
		return "", err
	}
	if strings.TrimSpace(value) == "" {
		return "", fmt.Errorf("value is required")
	}
	return value, nil
}

func resolveOpenRouterAPIKey() (string, error) {
	cfg, err := config.LoadGlobal()
	if err != nil {
		return "", err
	}
	return strings.TrimSpace(cfg.OpenRouterAPIKey), nil
}

func resolveOpenRouterModel() (string, error) {
	cfg, err := config.LoadGlobal()
	if err != nil {
		return "", err
	}
	if model := strings.TrimSpace(cfg.OpenRouterModel); model != "" {
		return model, nil
	}
	return ai.DefaultModel(), nil
}

func startSpinner() func() {
	var wg sync.WaitGroup
	stop := make(chan bool)
	wg.Add(1)

	go func() {
		defer wg.Done()
		frames := []string{"⠋", "⠙", "⠹", "⠸", "⠼", "⠴", "⠦", "⠧", "⠇", "⠏"}
		i := 0
		ticker := time.NewTicker(100 * time.Millisecond)
		defer ticker.Stop()

		for {
			select {
			case <-stop:
				// Clear the spinner line
				fmt.Print("\r\033[K")
				return
			case <-ticker.C:
				fmt.Printf("\rGenerating suggestions %s", frames[i])
				i = (i + 1) % len(frames)
			}
		}
	}()

	return func() {
		close(stop)
		wg.Wait()
	}
}

func CreateWithDryRun(dryRun bool, defaultBaseBranch string) error {
	if dryRun {
		fmt.Println("=== DRY RUN MODE ===")
		fmt.Println("This will show what would happen without creating a release")
		fmt.Println()
	}

	repoRoot, err := git.RepoRoot()
	if err != nil {
		return err
	}

	repoSlug, baseBranch, err := createBaseContext(repoRoot, defaultBaseBranch)
	if err != nil {
		return err
	}

	printHeader("Repository context")
	fmt.Printf("Repository: %s\n", repoSlug)
	fmt.Printf("Base branch: %s\n", baseBranch)

	printHeader("Latest release")
	latest, foundLatest, err := gh.LatestRelease(repoRoot, repoSlug)
	if err != nil {
		return err
	}

	latestTitle := ""
	latestTag := ""
	latestPublishedDate := ""
	if foundLatest {
		latestTitle = strings.TrimSpace(latest.Name)
		if latestTitle == "" {
			latestTitle = "(untitled release)"
		}
		latestTag = strings.TrimSpace(latest.TagName)
		latestPublishedDate = publishedDate(latest.PublishedAt)
		fmt.Printf("Title: %s\n", latestTitle)
		fmt.Printf("Tag: %s\n", latestTag)
	} else {
		fmt.Println("No existing release found.")
	}

	printHeader("Pull requests included since last release")
	prNumbers, err := discoverPRNumbers(repoRoot, repoSlug, baseBranch, latestTag, latestPublishedDate)
	if err != nil {
		return err
	}

	prs, _, err := loadPRContext(repoRoot, repoSlug, prNumbers)
	if err != nil {
		return err
	}
	if len(prs) == 0 {
		fmt.Println("No merged pull requests detected since the last release.")
	}

	printHeader("Create release (DRY RUN)")

	if dryRun {
		printHeader("AI release suggestions")
		customModelURL, _ := config.GetCustomModelURL()
		customHeaders, _ := config.GetCustomHeaders()
		openRouterMode, _ := config.GetOpenRouterMode()

		apiKey, apiKeyErr := resolveOpenRouterAPIKey()
		hasCustomURL := strings.TrimSpace(customModelURL) != ""
		hasCustomHeaders := len(customHeaders) > 0

		if apiKeyErr != nil && !hasCustomURL && !hasCustomHeaders {
			printHeader("AI release suggestions")
			fmt.Println("OpenRouter API key is not configured. Skipping AI suggestions.")
		} else {
			if apiKeyErr != nil && hasCustomURL {
				if authHeader, exists := customHeaders["Authorization"]; exists {
					apiKey = strings.TrimPrefix(authHeader, "Bearer ")
				}
			}
			if apiKey == "" && hasCustomURL {
				if authHeader, exists := customHeaders["Authorization"]; exists {
					apiKey = authHeader
				}
			}

			model, _ := resolveOpenRouterModel()
			stopSpinner := startSpinner()
			suggestion, err := ai.GenerateReleaseSuggestion(
				apiKey,
				model,
				repoSlug,
				baseBranch,
				latestTitle,
				latestTag,
				prs,
				customModelURL,
				customHeaders,
				openRouterMode,
			)
			stopSpinner()
			fmt.Println()
			if err != nil {
				fmt.Println("Warning: AI suggestion failed:")
				fmt.Println(err)
			} else {
				if suggestion.SuggestedVersion != "" {
					fmt.Printf("Suggested version: v%s\n", suggestion.SuggestedVersion)
				}
				fmt.Println("Suggested titles:")
				for i, title := range suggestion.Titles {
					if i >= 3 {
						break
					}
					fmt.Printf("  %d) %s\n", i+1, title)
				}
			}
		}

		fmt.Println()
		fmt.Println("=== DRY RUN COMPLETE ===")
		fmt.Println("No release was created.")
		return nil
	}

	return Create(defaultBaseBranch)
}

func createBaseContext(repoRoot, defaultBaseBranch string) (string, string, error) {
	if err := gh.EnsureInstalled(); err != nil {
		return "", "", err
	}

	if !git.HasCommits(repoRoot) {
		return "", "", fmt.Errorf("repository has no commits yet")
	}
	if _, err := os.Stat(filepath.Join(repoRoot, ".git")); err != nil {
		return "", "", fmt.Errorf("this command must be run inside a git repository")
	}
	if _, err := git.CurrentBranch(repoRoot); err != nil {
		return "", "", err
	}
	if err := gh.EnsureAuthHealthy(); err != nil {
		return "", "", err
	}

	repo, err := gh.Repo(repoRoot)
	if err != nil {
		return "", "", err
	}
	repoSlug := strings.TrimSpace(repo.NameWithOwner)
	defaultBranch := strings.TrimSpace(repo.DefaultBranchRef.Name)
	if defaultBranch == "" {
		defaultBranch = defaultBaseBranch
	}

	baseBranch, err := resolveBaseBranch(repoRoot, defaultBranch, defaultBaseBranch)
	if err != nil {
		return "", "", err
	}

	return repoSlug, baseBranch, nil
}
