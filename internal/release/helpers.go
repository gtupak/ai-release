package release

import (
	"fmt"
	"regexp"
	"sort"
	"strconv"
	"strings"
	"time"
)

var (
	prRefPattern       = regexp.MustCompile(`\(#([0-9]+)\)`)
	mergePRRefPattern  = regexp.MustCompile(`Merge pull request #([0-9]+)`)
	semverCorePattern  = regexp.MustCompile(`^([0-9]+)\.([0-9]+)\.([0-9]+)$`)
	semverLoosePattern = regexp.MustCompile(`^[0-9]+\.[0-9]+\.[0-9]+([-.][0-9A-Za-z.-]+)?$`)
)

func extractPRNumbersFromCommitMessage(message string) []int {
	numbers := map[int]struct{}{}
	for _, match := range prRefPattern.FindAllStringSubmatch(message, -1) {
		if len(match) < 2 {
			continue
		}
		n, err := strconv.Atoi(match[1])
		if err == nil && n > 0 {
			numbers[n] = struct{}{}
		}
	}
	for _, match := range mergePRRefPattern.FindAllStringSubmatch(message, -1) {
		if len(match) < 2 {
			continue
		}
		n, err := strconv.Atoi(match[1])
		if err == nil && n > 0 {
			numbers[n] = struct{}{}
		}
	}
	return sortedKeys(numbers)
}

func sortedUnique(nums []int) []int {
	set := map[int]struct{}{}
	for _, n := range nums {
		if n > 0 {
			set[n] = struct{}{}
		}
	}
	return sortedKeys(set)
}

func sortedKeys(set map[int]struct{}) []int {
	keys := make([]int, 0, len(set))
	for n := range set {
		keys = append(keys, n)
	}
	sort.Ints(keys)
	return keys
}

func firstNonEmptyPRBodyLine(body string) string {
	body = strings.ReplaceAll(body, "\r", "")
	for _, line := range strings.Split(body, "\n") {
		line = strings.TrimSpace(line)
		if line != "" {
			if len(line) > 240 {
				return line[:240] + "..."
			}
			return line
		}
	}
	return ""
}

func publishedDate(iso8601 string) string {
	if strings.TrimSpace(iso8601) == "" {
		return ""
	}
	t, err := time.Parse(time.RFC3339, iso8601)
	if err != nil {
		return ""
	}
	return t.Format("2006-01-02")
}

func parseSemverCore(tag string) (major, minor, patch int, ok bool) {
	tag = strings.TrimSpace(strings.TrimPrefix(tag, "v"))
	m := semverCorePattern.FindStringSubmatch(tag)
	if len(m) != 4 {
		return 0, 0, 0, false
	}
	maj, err1 := strconv.Atoi(m[1])
	min, err2 := strconv.Atoi(m[2])
	pat, err3 := strconv.Atoi(m[3])
	if err1 != nil || err2 != nil || err3 != nil {
		return 0, 0, 0, false
	}
	return maj, min, pat, true
}

func normalizeVersion(version string) (string, error) {
	clean := strings.TrimSpace(version)
	clean = strings.TrimPrefix(clean, "v")
	clean = strings.TrimSpace(clean)
	if !semverLoosePattern.MatchString(clean) {
		return "", fmt.Errorf("version must match semver (e.g. 1.4.0)")
	}
	return clean, nil
}
