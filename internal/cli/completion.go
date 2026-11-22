package cli

import (
	"bytes"
	"fmt"
	"io"
	"slices"
	"strings"
)

var completionCommands = []string{"list", "inspect", "audit", "check", "probe", "tls", "watch", "snapshot", "doctor", "wait", "terminate", "update", "update-check", "updates", "version", "help", "completion"}

const completionFiles = "__socklens_files__"

// completionCandidates never runs the supplied command. Flag declarations are
// obtained from an allowlisted command's help, with no user arguments forwarded.
func completionCandidates(words []string) []string {
	if len(words) == 0 {
		words = []string{""}
	}
	cur := words[len(words)-1]
	prior := words[:len(words)-1]
	var command []string
	for len(prior) > 0 {
		if prior[0] == "--color" || prior[0] == "-color" {
			if len(prior) == 1 {
				return completionValues("color", cur, "")
			}
			prior = prior[2:]
			continue
		}
		if strings.HasPrefix(prior[0], "--color=") || strings.HasPrefix(prior[0], "-color=") {
			prior = prior[1:]
			continue
		}
		break
	}
	if len(prior) == 0 {
		if key, value, ok := strings.Cut(cur, "="); ok && strings.TrimLeft(key, "-") == "color" {
			return completionValues("color", value, key+"=")
		}
		return completionPrefix(append(slices.Clone(completionCommands), "--help", "--color", "--version"), cur)
	}
	if !slices.Contains(completionCommands, prior[0]) {
		return nil
	}
	command = []string{prior[0]}
	prior = prior[1:]
	if command[0] == "snapshot" {
		if len(prior) == 0 {
			return completionPrefix([]string{"save", "diff"}, cur)
		}
		if prior[0] != "save" && prior[0] != "diff" {
			return nil
		}
		command = append(command, prior[0])
		prior = prior[1:]
	}
	if command[0] == "completion" {
		return completionPrefix([]string{"bash", "zsh", "fish", "powershell"}, cur)
	}
	flags := completionFlagInfo(command)
	for i := 0; i < len(prior); i++ {
		arg := prior[i]
		if arg == "--" || !strings.HasPrefix(arg, "-") {
			return []string{completionFiles}
		}
		name, _, equals := strings.Cut(strings.TrimLeft(arg, "-"), "=")
		value, exists := flags[name]
		if !exists {
			return nil
		}
		if value && !equals {
			if i+1 == len(prior) {
				return completionValues(name, cur, "")
			}
			i++
		}
	}
	if key, value, ok := strings.Cut(cur, "="); ok && strings.HasPrefix(key, "-") {
		name := strings.TrimLeft(key, "-")
		if _, exists := flags[name]; exists {
			return completionValues(name, value, key+"=")
		}
		return nil
	}
	if (cur != "" && !strings.HasPrefix(cur, "-")) || (cur == "" && command[0] == "snapshot") {
		return []string{completionFiles}
	}
	candidates := []string{"--help"}
	for name := range flags {
		candidates = append(candidates, "--"+name)
	}
	slices.Sort(candidates)
	return completionPrefix(candidates, cur)
}

func completionFlagInfo(command []string) map[string]bool {
	var help bytes.Buffer
	Run(append(slices.Clone(command), "--help"), strings.NewReader(""), io.Discard, &help, "completion")
	flags := map[string]bool{}
	for _, line := range strings.Split(help.String(), "\n") {
		// Go's flag package starts declarations with two spaces; descriptions
		// start with a tab. Only declarations are part of this interface.
		if !strings.HasPrefix(line, "  -") {
			continue
		}
		fields := strings.Fields(strings.SplitN(line, "\t", 2)[0])
		if len(fields) > 0 {
			flags[strings.TrimLeft(fields[0], "-")] = len(fields) > 1
		}
	}
	// snapshot save accepts filters, not output-format switches.
	if len(command) == 2 && command[1] == "save" {
		delete(flags, "json")
		delete(flags, "format")
	}
	return flags
}
func completionPrefix(values []string, prefix string) []string {
	var result []string
	for _, value := range values {
		if strings.HasPrefix(value, prefix) {
			result = append(result, value)
		}
	}
	return result
}
func completionValues(name, prefix, leader string) []string {
	var values []string
	switch name {
	case "color":
		values = []string{"auto", "always", "never"}
	case "protocol":
		values = []string{"tcp", "udp"}
	case "format":
		values = []string{"table", "json-v1"}
	case "family":
		values = []string{"any", "ipv4", "ipv6"}
	case "until":
		values = []string{"present", "absent"}
	case "ca-file":
		return []string{completionFiles}
	}
	values = completionPrefix(values, prefix)
	for i := range values {
		values[i] = leader + values[i]
	}
	return values
}
func completeCommand(words []string, out, errOut io.Writer) int {
	for _, value := range completionCandidates(words) {
		if _, err := fmt.Fprintln(out, value); err != nil {
			return fail(errOut, err)
		}
	}
	return 0
}

func completionCommand(args []string, out, errOut io.Writer) int {
	if len(args) != 1 {
		return fail(errOut, fmt.Errorf("usage: socklens completion bash|zsh|fish|powershell"))
	}
	var script string
	switch strings.ToLower(args[0]) {
	case "bash":
		script = `# SockLens commands include list, inspect, watch and snapshot.
_socklens_complete() {
 local candidate
 COMPREPLY=()
 while IFS= read -r candidate; do
  if [[ "$candidate" == __socklens_files__ ]]; then
   compopt -o default
   return
  fi
  COMPREPLY+=("$candidate")
 done < <("${COMP_WORDS[0]}" __complete "${COMP_WORDS[@]:1:COMP_CWORD}")
}
complete -F _socklens_complete socklens
`
	case "zsh":
		script = `#compdef socklens
# SockLens commands include list, inspect, watch and snapshot.
_socklens_complete() {
 local -a candidates
 candidates=("${(@f)$("${words[1]}" __complete "${(@)words[2,CURRENT]}")}")
 if (( ${candidates[(Ie)__socklens_files__]} )); then
  _files
 else
  compadd -- "${candidates[@]}"
 fi
}
compdef _socklens_complete socklens
`
	case "fish":
		script = `# SockLens commands include list, inspect, watch and snapshot.
function __socklens_candidates
 set -l words (commandline -opc)
 set -e words[1]
 command socklens __complete $words "(commandline -ct)"
end
function __socklens_files
 contains -- __socklens_files__ (__socklens_candidates)
end
complete -c socklens -f -n 'not __socklens_files' -a '(__socklens_candidates)'
complete -c socklens -F -n '__socklens_files'
`
	case "powershell":
		script = `# SockLens commands include list, inspect, watch and snapshot.
Register-ArgumentCompleter -Native -CommandName socklens -ScriptBlock {
 param($wordToComplete, $commandAst, $cursorPosition)
 $elements = @($commandAst.CommandElements | Select-Object -Skip 1 | Where-Object { $_.Extent.StartOffset -lt $cursorPosition })
 $words = @($elements | ForEach-Object {
  if ($_ -is [System.Management.Automation.Language.StringConstantExpressionAst]) { $_.Value }
  else { $_.Extent.Text }
 })
 if ($elements.Count -eq 0 -or $elements[-1].Extent.EndOffset -lt $cursorPosition) { $words += '' }
 elseif ($words.Count -gt 0) { $words[-1] = $wordToComplete }
 $choices = @(& $commandAst.CommandElements[0].Value __complete @words)
 if ($choices -contains '__socklens_files__') {
  [System.Management.Automation.CompletionCompleters]::CompleteFilename($wordToComplete)
  return
 }
 $choices | ForEach-Object {
  [System.Management.Automation.CompletionResult]::new($_, $_, 'ParameterValue', $_)
 }
}
`
	default:
		return fail(errOut, fmt.Errorf("unsupported shell %q; use bash, zsh, fish, or powershell", args[0]))
	}
	if _, err := io.WriteString(out, script); err != nil {
		return fail(errOut, err)
	}
	return 0
}
