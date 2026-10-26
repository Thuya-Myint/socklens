package main

import (
	"strings"
	"testing"
)

func TestVersionInfo(t *testing.T) {
	oldVersion, oldCommit, oldDate := version, commit, buildDate
	defer func() { version, commit, buildDate = oldVersion, oldCommit, oldDate }()
	version, commit, buildDate = "v1.2.3", "abcdef123456", "2026-10-26T00:00:00Z"
	got := versionInfo()
	for _, want := range []string{version, commit, buildDate} {
		if !strings.Contains(got, want) {
			t.Fatalf("missing %s: %s", want, got)
		}
	}
}
