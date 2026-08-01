package main

import (
	"fmt"
	"io"
	"strings"
)

type completionCommandSpec struct {
	Name        string
	Description string
	Flags       []completionFlagSpec
}

type completionFlagSpec struct {
	Name        string
	Description string
}

var completionCommands = []completionCommandSpec{
	{Name: "help", Description: "Show help text"},
	{Name: "init", Description: "Guided setup for a .switchlet.yaml", Flags: []completionFlagSpec{{Name: "--overwrite", Description: "Replace an existing .switchlet.yaml without prompting"}}},
	{Name: "config", Description: "Edit .switchlet.yaml in the configuration editor"},
	{Name: "list", Description: "List configured profiles", Flags: []completionFlagSpec{{Name: "--json", Description: "Write machine-readable JSON output"}}},
	{Name: "inspect", Description: "Inspect one configured profile", Flags: []completionFlagSpec{{Name: "--json", Description: "Write machine-readable JSON output"}}},
	{Name: "apply", Description: "Apply one configured profile", Flags: []completionFlagSpec{
		{Name: "--json", Description: "Write machine-readable JSON output"},
		{Name: "--dry-run", Description: "Validate without writing target files"},
		{Name: "--allow-protected", Description: "Allow non-interactive apply for a protected profile"},
	}},
	{Name: "status", Description: "Compare current managed values", Flags: []completionFlagSpec{{Name: "--json", Description: "Write machine-readable JSON output"}}},
	{Name: "diff", Description: "Compare one profile with current managed values", Flags: []completionFlagSpec{
		{Name: "--json", Description: "Write machine-readable JSON output"},
		{Name: "--patch", Description: "Write read-only managed patch text"},
	}},
	{Name: "version", Description: "Show version information"},
	{Name: "completion", Description: "Generate a static shell completion script"},
}

var supportedCompletionShells = []string{"bash", "zsh", "fish"}

func writeCompletionScript(output io.Writer, shell string) error {
	switch shell {
	case "bash":
		_, err := io.WriteString(output, bashCompletionScript())
		return err
	case "zsh":
		_, err := io.WriteString(output, zshCompletionScript())
		return err
	case "fish":
		_, err := io.WriteString(output, fishCompletionScript())
		return err
	default:
		return usageCommandError(false, "unsupported completion shell %q\n\nSupported shells: %s\n\n%s", shell, strings.Join(supportedCompletionShells, ", "), completionHelpText())
	}
}

func bashCompletionScript() string {
	var builder strings.Builder
	builder.WriteString("# bash completion for switchlet\n")
	builder.WriteString("_switchlet_completion() {\n")
	builder.WriteString("    local cur command\n")
	builder.WriteString("    COMPREPLY=()\n")
	builder.WriteString("    cur=\"${COMP_WORDS[COMP_CWORD]}\"\n")
	builder.WriteString("\n")
	builder.WriteString("    if [[ ${COMP_CWORD} -eq 1 ]]; then\n")
	fmt.Fprintf(&builder, "        COMPREPLY=( $(compgen -W %q -- \"$cur\") )\n", strings.Join(append([]string{"--help", "--version"}, completionCommandNames()...), " "))
	builder.WriteString("        return 0\n")
	builder.WriteString("    fi\n")
	builder.WriteString("\n")
	builder.WriteString("    command=\"${COMP_WORDS[1]}\"\n")
	builder.WriteString("    case \"$command\" in\n")
	for _, command := range completionCommands {
		if len(command.Flags) == 0 {
			continue
		}

		fmt.Fprintf(&builder, "        %s)\n", command.Name)
		fmt.Fprintf(&builder, "            COMPREPLY=( $(compgen -W %q -- \"$cur\") )\n", strings.Join(completionFlagNames(command.Flags), " "))
		builder.WriteString("            ;;\n")
	}
	builder.WriteString("        completion)\n")
	fmt.Fprintf(&builder, "            COMPREPLY=( $(compgen -W %q -- \"$cur\") )\n", strings.Join(supportedCompletionShells, " "))
	builder.WriteString("            ;;\n")
	builder.WriteString("    esac\n")
	builder.WriteString("}\n")
	builder.WriteString("complete -F _switchlet_completion switchlet\n")

	return builder.String()
}

func zshCompletionScript() string {
	var builder strings.Builder
	builder.WriteString("#compdef switchlet\n\n")
	builder.WriteString("_switchlet() {\n")
	builder.WriteString("  local -a commands\n")
	builder.WriteString("  commands=(\n")
	for _, command := range completionCommands {
		fmt.Fprintf(&builder, "    %q\n", command.Name+":"+command.Description)
	}
	builder.WriteString("  )\n")
	builder.WriteString("\n")
	builder.WriteString("  local context state line\n")
	builder.WriteString("  typeset -A opt_args\n")
	builder.WriteString("  _arguments -C \\\n")
	builder.WriteString("    '(-h --help)'{-h,--help}'[Show help text]' \\\n")
	builder.WriteString("    '--version[Show version information]' \\\n")
	builder.WriteString("    '1:command:->command' \\\n")
	builder.WriteString("    '*::argument:->argument'\n")
	builder.WriteString("\n")
	builder.WriteString("  case $state in\n")
	builder.WriteString("    command)\n")
	builder.WriteString("      _describe 'command' commands\n")
	builder.WriteString("      ;;\n")
	builder.WriteString("    argument)\n")
	builder.WriteString("      case $words[2] in\n")
	for _, command := range completionCommands {
		if len(command.Flags) == 0 {
			continue
		}

		fmt.Fprintf(&builder, "        %s)\n", command.Name)
		builder.WriteString("          _values 'flag' \\\n")
		for flagIndex, flag := range command.Flags {
			lineSuffix := " \\\n"
			if flagIndex == len(command.Flags)-1 {
				lineSuffix = "\n"
			}
			fmt.Fprintf(&builder, "            %q%s", flag.Name+"["+flag.Description+"]", lineSuffix)
		}
		builder.WriteString("          ;;\n")
	}
	builder.WriteString("        completion)\n")
	builder.WriteString("          _values 'shell' bash zsh fish\n")
	builder.WriteString("          ;;\n")
	builder.WriteString("      esac\n")
	builder.WriteString("      ;;\n")
	builder.WriteString("  esac\n")
	builder.WriteString("}\n\n")
	builder.WriteString("_switchlet \"$@\"\n")

	return builder.String()
}

func fishCompletionScript() string {
	var builder strings.Builder
	builder.WriteString("# fish completion for switchlet\n")
	builder.WriteString("complete -c switchlet -f\n")
	builder.WriteString("complete -c switchlet -s h -l help -d 'Show help text'\n")
	builder.WriteString("complete -c switchlet -l version -d 'Show version information'\n")
	for _, command := range completionCommands {
		fmt.Fprintf(&builder, "complete -c switchlet -n '__fish_use_subcommand' -a %q -d %q\n", command.Name, command.Description)
	}
	for _, command := range completionCommands {
		for _, flag := range command.Flags {
			fmt.Fprintf(&builder, "complete -c switchlet -n '__fish_seen_subcommand_from %s' -l %s -d %q\n", command.Name, strings.TrimPrefix(flag.Name, "--"), flag.Description)
		}
	}
	fmt.Fprintf(&builder, "complete -c switchlet -n '__fish_seen_subcommand_from completion' -a %q -d 'Shell'\n", strings.Join(supportedCompletionShells, " "))

	return builder.String()
}

func completionCommandNames() []string {
	commandNames := make([]string, 0, len(completionCommands))
	for _, command := range completionCommands {
		commandNames = append(commandNames, command.Name)
	}

	return commandNames
}

func completionFlagNames(flags []completionFlagSpec) []string {
	flagNames := make([]string, 0, len(flags))
	for _, flag := range flags {
		flagNames = append(flagNames, flag.Name)
	}

	return flagNames
}
