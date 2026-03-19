package ai

import "testing"

func TestParseReleaseSuggestion(t *testing.T) {
	t.Parallel()

	raw := `{"suggested_version":"1.4.0","titles":["Router reliability improvements","Faster request routing","Operational polish"]}`
	got, err := parseReleaseSuggestion(raw)
	if err != nil {
		t.Fatalf("parseReleaseSuggestion() error: %v", err)
	}
	if got.SuggestedVersion != "1.4.0" {
		t.Fatalf("unexpected suggested version: %q", got.SuggestedVersion)
	}
	if len(got.Titles) != 3 {
		t.Fatalf("expected 3 titles, got %d", len(got.Titles))
	}
}

func TestParseReleaseSuggestionFenceAndLeadingV(t *testing.T) {
	t.Parallel()

	raw := "```json\n{\"suggested_version\":\"v2.0.1\",\"titles\":[\"One\",\"Two\",\"Three\"]}\n```"
	got, err := parseReleaseSuggestion(raw)
	if err != nil {
		t.Fatalf("parseReleaseSuggestion() error: %v", err)
	}
	if got.SuggestedVersion != "2.0.1" {
		t.Fatalf("expected normalized version, got %q", got.SuggestedVersion)
	}
}

func TestParseReleaseSuggestionInvalidJSON(t *testing.T) {
	t.Parallel()

	_, err := parseReleaseSuggestion("not-json")
	if err == nil {
		t.Fatal("expected parse error")
	}
}

func TestParseReleaseSuggestionInvalidSemverGetsCleared(t *testing.T) {
	t.Parallel()

	raw := `{"suggested_version":"banana","titles":["One","Two","Three"]}`
	got, err := parseReleaseSuggestion(raw)
	if err != nil {
		t.Fatalf("parseReleaseSuggestion() error: %v", err)
	}
	if got.SuggestedVersion != "" {
		t.Fatalf("expected invalid version to be cleared, got %q", got.SuggestedVersion)
	}
}
