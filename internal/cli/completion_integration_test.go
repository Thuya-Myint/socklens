package cli

import (
	"bytes"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strings"
	"testing"
)

// Exercise the generated adapters with a real executable, rather than checking
// only that their text includes flag names. Missing shells are explicit skips.
func TestGeneratedCompletionAdapters(t *testing.T) {
	dir := t.TempDir()
	name := "socklens"
	if runtime.GOOS == "windows" {
		name += ".exe"
	}
	binary := filepath.Join(dir, name)
	build := exec.Command("go", "build", "-o", binary, "../../cmd/socklens")
	if output, err := build.CombinedOutput(); err != nil {
		t.Fatalf("build: %v\n%s", err, output)
	}
	for _, shell := range []string{"bash", "zsh", "fish", "powershell"} {
		t.Run(shell, func(t *testing.T) {
			executable := shell
			if shell == "powershell" {
				executable = "pwsh"
			}
			shellPath, err := exec.LookPath(executable)
			if err != nil {
				t.Skipf("%s unavailable", executable)
			}
			if runtime.GOOS == "windows" && shell != "powershell" {
				t.Skip("Unix adapters are exercised on Unix hosts")
			}
			var script, errs bytes.Buffer
			if code := completionCommand([]string{shell}, &script, &errs); code != 0 {
				t.Fatal(&errs)
			}
			completionPath := filepath.Join(dir, "completion."+shell)
			if shell == "powershell" {
				completionPath = filepath.Join(dir, "completion.ps1")
			}
			if err := os.WriteFile(completionPath, script.Bytes(), 0600); err != nil {
				t.Fatal(err)
			}
			var command *exec.Cmd
			switch shell {
			case "bash":
				command = exec.Command(shellPath, "-c", `source "$1"; COMP_WORDS=(socklens snapshot d); COMP_CWORD=2; _socklens_complete; printf '%s\n' "${COMPREPLY[@]}"; COMP_WORDS=(socklens list --protocol t); COMP_CWORD=3; _socklens_complete; printf '%s\n' "${COMPREPLY[@]}"`, "test", completionPath)
			case "zsh":
				// Replace the terminal-only insertion operation with an argument
				// recorder while exercising the real adapter and executable.
				command = exec.Command(shellPath, "-c", `compdef() { :; }; compadd() { print -l -- "$@"; }; source "$1"; words=(socklens snapshot d); CURRENT=3; _socklens_complete; words=(socklens list --protocol t); CURRENT=4; _socklens_complete`, "test", completionPath)
			case "fish":
				command = exec.Command(shellPath, "-c", `source $argv[1]; complete -C 'socklens snapshot d'; complete -C 'socklens list --protocol t'`, completionPath)
			case "powershell":
				// Paths are passed as data, including spaces and apostrophes.
				command = exec.Command(shellPath, "-NoProfile", "-NonInteractive", "-Command", `. $env:SOCKLENS_COMPLETION_SCRIPT; foreach ($line in @('socklens snapshot d','socklens list --protocol t')) { (TabExpansion2 $line $line.Length).CompletionMatches.CompletionText }`)
			}
			command.Env = append(os.Environ(), "PATH="+dir+string(os.PathListSeparator)+os.Getenv("PATH"), "SOCKLENS_COMPLETION_SCRIPT="+completionPath)
			output, err := command.CombinedOutput()
			if err != nil {
				t.Fatalf("%v\n%s", err, output)
			}
			if !strings.Contains(string(output), "diff") || !strings.Contains(string(output), "tcp") {
				t.Fatalf("missing subcommand or enum suggestion: %s", output)
			}
		})
	}
}
