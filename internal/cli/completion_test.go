package cli

import (
	"bytes"
	"strings"
	"testing"
)

func TestCompletionShells(t *testing.T) {
	for _, shell := range []string{"bash", "zsh", "fish", "powershell"} {
		t.Run(shell, func(t *testing.T) {
			var out, errs bytes.Buffer
			if code := Run([]string{"completion", shell}, strings.NewReader(""), &out, &errs, "test"); code != 0 {
				t.Fatalf("exit %d: %s", code, errs.String())
			}
			if !strings.Contains(out.String(), "socklens") || !strings.Contains(out.String(), "inspect") {
				t.Fatalf("missing completion data: %s", out.String())
			}
		})
	}
}
func TestCompletionRejectsUnknownShell(t *testing.T) {
	var out, errs bytes.Buffer
	if code := Run([]string{"completion", "unknown"}, strings.NewReader(""), &out, &errs, "test"); code != 1 {
		t.Fatalf("exit %d", code)
	}
}

func TestCompletionContext(t *testing.T) {
	tests := []struct {
		words  []string
		want   string
		absent string
	}{
		{[]string{"snap"}, "snapshot", "inspect"},
		{[]string{"snapshot", ""}, "save", "--port"},
		{[]string{"snapshot", "d"}, "diff", "save"},
		{[]string{"snapshot", "save", "--p"}, "--process", "--format"},
		{[]string{"snapshot", "diff", "--"}, "--json", "--port"},
		{[]string{"watch", "--"}, "--event-interval", "--ca-file"},
		{[]string{"list", "--"}, "--local-address", "--force"},
		{[]string{"inspect", "--"}, "--protocol", "--port"},
		{[]string{"updates", "--"}, "--enable", "--port"},
		{[]string{"list", "--protocol", "t"}, "tcp", "udp"},
		{[]string{"list", "--protocol=u"}, "--protocol=udp", "tcp"},
		{[]string{"--color", "a"}, "auto", "list"},
		{[]string{"--color", "never", "list", "--format", "j"}, "json-v1", "table"},
		{[]string{"list", "--json", "--p"}, "--pid", "--ca-file"},
		{[]string{"snapshot", "save", ""}, completionFiles, "--json"},
		{[]string{"snapshot", "diff", "before file.json", ""}, completionFiles, "--json"},
		{[]string{"tls", "--ca-file", ""}, completionFiles, "--host"},
		{[]string{"list", "--process", "--"}, "", "--pid"},
	}
	for _, tt := range tests {
		t.Run(strings.Join(tt.words, " "), func(t *testing.T) {
			got := completionCandidates(tt.words)
			contains := func(value string) bool {
				for _, candidate := range got {
					if candidate == value {
						return true
					}
				}
				return false
			}
			if tt.want != "" && !contains(tt.want) {
				t.Fatalf("got %q, want %q", got, tt.want)
			}
			if tt.absent != "" && contains(tt.absent) {
				t.Fatalf("unexpected %q in %q", tt.absent, got)
			}
		})
	}
}

func TestCompletionDoesNotExecuteCommands(t *testing.T) {
	// Even destructive/update words are treated as data. Only their help is run.
	for _, words := range [][]string{{"terminate", "--pid", "1", ""}, {"update", ""}, {"check", ""}, {"__complete", ""}, {"snapshot", "bogus", ""}} {
		var out, errs bytes.Buffer
		if code := Run(append([]string{"__complete"}, words...), strings.NewReader(""), &out, &errs, "test"); code != 0 || errs.Len() != 0 {
			t.Fatalf("%q: code=%d error=%s", words, code, &errs)
		}
	}
}
