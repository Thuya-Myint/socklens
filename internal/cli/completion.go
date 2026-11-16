package cli

import (
	"flag"
	"fmt"
	"io"
	"strings"
)

var completionCommands = []string{"list", "inspect", "audit", "check", "probe", "tls", "watch", "doctor", "wait", "terminate", "update", "update-check", "updates", "version", "help", "completion"}
var completionFlags = []string{"--help", "--color", "--json", "--port", "--pid", "--protocol", "--listening", "--host", "--ports", "--timeout", "--dry-run"}

func completionCommand(args []string, out, errOut io.Writer) int {
	fs := flag.NewFlagSet("completion", flag.ContinueOnError)
	fs.SetOutput(errOut)
	if len(args) != 1 {
		return fail(errOut, fmt.Errorf("usage: socklens completion bash|zsh|fish|powershell"))
	}
	shell := strings.ToLower(args[0])
	commands := strings.Join(completionCommands, " ")
	flags := strings.Join(completionFlags, " ")
	var script string
	switch shell {
	case "bash":
		script = fmt.Sprintf(`_socklens_complete() {
 local cur="${COMP_WORDS[COMP_CWORD]}"
 if (( COMP_CWORD == 1 )); then
  COMPREPLY=( $(compgen -W '%s' -- "$cur") )
 else
  COMPREPLY=( $(compgen -W '%s' -- "$cur") )
 fi
}
complete -F _socklens_complete socklens
`, commands, flags)
	case "zsh":
		script = fmt.Sprintf("#compdef socklens\n_arguments '1:command:(%s)' '*:option:(%s)'\n", commands, flags)
	case "fish":
		var lines []string
		for _, c := range completionCommands {
			lines = append(lines, "complete -c socklens -f -n '__fish_use_subcommand' -a '"+c+"'")
		}
		for _, f := range completionFlags {
			lines = append(lines, "complete -c socklens -l '"+strings.TrimPrefix(f, "--")+"'")
		}
		script = strings.Join(lines, "\n") + "\n"
	case "powershell":
		script = fmt.Sprintf(`Register-ArgumentCompleter -Native -CommandName socklens -ScriptBlock {
 param($wordToComplete, $commandAst, $cursorPosition)
 $choices = if ($commandAst.CommandElements.Count -le 2) { '%s'.Split(' ') } else { '%s'.Split(' ') }
 $choices | Where-Object { $_ -like "$wordToComplete*" } | ForEach-Object {
  [System.Management.Automation.CompletionResult]::new($_, $_, 'ParameterValue', $_)
 }
}
`, commands, flags)
	default:
		return fail(errOut, fmt.Errorf("unsupported shell %q; use bash, zsh, fish, or powershell", shell))
	}
	if _, err := io.WriteString(out, script); err != nil {
		return fail(errOut, err)
	}
	return 0
}
