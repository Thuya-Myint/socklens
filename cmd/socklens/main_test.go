package main

import "testing"

func TestVersionInfo(t *testing.T) {
	oldVersion, oldCommit, oldDate := version, commit, buildDate
	defer func() { version, commit, buildDate = oldVersion, oldCommit, oldDate }()
	version, commit, buildDate = "v1.2.3", "abcdef123456", "2026-10-26T00:00:00Z"
	if got := versionInfo(); got != "v1.2.3" {
		t.Fatalf("version output must contain only the version: %q", got)
	}
}
