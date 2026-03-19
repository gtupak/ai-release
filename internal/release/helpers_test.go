package release

import (
	"reflect"
	"testing"
)

func TestExtractPRNumbersFromCommitMessage(t *testing.T) {
	t.Parallel()

	msg := "feat: add pipeline (#12)\n\nMerge pull request #14 from foo/bar\nfix: more work (#12)"
	got := extractPRNumbersFromCommitMessage(msg)
	want := []int{12, 14}
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("expected %v, got %v", want, got)
	}
}

func TestNormalizeVersion(t *testing.T) {
	t.Parallel()

	v, err := normalizeVersion(" v1.2.3 ")
	if err != nil {
		t.Fatalf("normalizeVersion() error: %v", err)
	}
	if v != "1.2.3" {
		t.Fatalf("expected 1.2.3, got %q", v)
	}
}

func TestNormalizeVersionInvalid(t *testing.T) {
	t.Parallel()

	if _, err := normalizeVersion("oops"); err == nil {
		t.Fatal("expected error")
	}
}

func TestParseSemverCore(t *testing.T) {
	t.Parallel()

	maj, min, patch, ok := parseSemverCore("v2.3.9")
	if !ok {
		t.Fatal("expected semver core to parse")
	}
	if maj != 2 || min != 3 || patch != 9 {
		t.Fatalf("unexpected parsed semver: %d.%d.%d", maj, min, patch)
	}
}
