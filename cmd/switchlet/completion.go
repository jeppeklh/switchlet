package main

import (
	"fmt"
	"io"
	"strings"

	"github.com/jeppeklh/switchlet/internal/config"
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
	{Name: "completion", Description: "Generate a shell completion script"},
}

var supportedCompletionShells = []string{"bash", "zsh", "fish"}
var profileCompletionCommands = []string{"inspect", "apply", "diff"}

const profileCompletionCommandName = "__complete-profile-names"

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
	builder.WriteString("__switchlet_needs_profile_completion() {\n")
	builder.WriteString("    case \"$command\" in\n")
	fmt.Fprintf(&builder, "        %s) ;;\n", strings.Join(profileCompletionCommands, "|"))
	builder.WriteString("        *) return 1 ;;\n")
	builder.WriteString("    esac\n")
	builder.WriteString("\n")
	builder.WriteString("    local index word\n")
	builder.WriteString("    for (( index = 2; index < COMP_CWORD; index++ )); do\n")
	builder.WriteString("        word=\"${COMP_WORDS[index]}\"\n")
	builder.WriteString("        if [[ \"$word\" != -* ]]; then\n")
	builder.WriteString("            return 1\n")
	builder.WriteString("        fi\n")
	builder.WriteString("    done\n")
	builder.WriteString("\n")
	builder.WriteString("    return 0\n")
	builder.WriteString("}\n")
	builder.WriteString("\n")
	builder.WriteString("__switchlet_complete_profiles() {\n")
	builder.WriteString("    local prefix candidate\n")
	builder.WriteString("    prefix=\"$1\"\n")
	builder.WriteString("    compopt -o filenames 2>/dev/null || true\n")
	builder.WriteString("    while IFS= read -r candidate; do\n")
	builder.WriteString("        [[ \"$candidate\" == \"$prefix\"* ]] || continue\n")
	builder.WriteString("        COMPREPLY+=(\"$candidate\")\n")
	fmt.Fprintf(&builder, "    done < <(switchlet %s 2>/dev/null)\n", profileCompletionCommandName)
	builder.WriteString("}\n")
	builder.WriteString("\n")
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
	builder.WriteString("    if [[ \"$cur\" != -* ]] && __switchlet_needs_profile_completion; then\n")
	builder.WriteString("        __switchlet_complete_profiles \"$cur\"\n")
	builder.WriteString("        if [[ ${#COMPREPLY[@]} -gt 0 || -n \"$cur\" ]]; then\n")
	builder.WriteString("            return 0\n")
	builder.WriteString("        fi\n")
	builder.WriteString("\n")
	builder.WriteString("        # No configured project was found; keep static flag completion available.\n")
	builder.WriteString("    fi\n")
	builder.WriteString("\n")
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
	builder.WriteString("_switchlet_profile_names() {\n")
	builder.WriteString("  local -a profiles\n")
	builder.WriteString("  local output\n")
	fmt.Fprintf(&builder, "  output=\"$(switchlet %s 2>/dev/null)\"\n", profileCompletionCommandName)
	builder.WriteString("  [[ -n \"$output\" ]] || return 1\n")
	builder.WriteString("  profiles=(\"${(@f)output}\")\n")
	builder.WriteString("  compadd -a profiles\n")
	builder.WriteString("}\n\n")
	builder.WriteString("_switchlet_has_profile_argument() {\n")
	builder.WriteString("  local index word\n")
	builder.WriteString("  for (( index = 3; index < CURRENT; index++ )); do\n")
	builder.WriteString("    word=\"${words[index]}\"\n")
	builder.WriteString("    [[ \"$word\" == -* ]] && continue\n")
	builder.WriteString("    return 0\n")
	builder.WriteString("  done\n")
	builder.WriteString("  return 1\n")
	builder.WriteString("}\n\n")
	builder.WriteString("_switchlet_needs_profile_completion() {\n")
	builder.WriteString("  case \"${words[2]}\" in\n")
	fmt.Fprintf(&builder, "    %s) ;;\n", strings.Join(profileCompletionCommands, "|"))
	builder.WriteString("    *) return 1 ;;\n")
	builder.WriteString("  esac\n")
	builder.WriteString("  [[ \"${words[CURRENT]}\" == -* ]] && return 1\n")
	builder.WriteString("  _switchlet_has_profile_argument && return 1\n")
	builder.WriteString("  return 0\n")
	builder.WriteString("}\n\n")
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
	builder.WriteString("      if _switchlet_needs_profile_completion; then\n")
	builder.WriteString("        _switchlet_profile_names && return\n")
	builder.WriteString("        [[ -n \"${words[CURRENT]}\" ]] && return\n")
	builder.WriteString("        # No configured project was found; keep static flag completion available.\n")
	builder.WriteString("      fi\n")
	builder.WriteString("\n")
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
	builder.WriteString("function __switchlet_complete_profiles\n")
	fmt.Fprintf(&builder, "    switchlet %s 2>/dev/null\n", profileCompletionCommandName)
	builder.WriteString("end\n\n")
	builder.WriteString("function __switchlet_profile_completion_needed\n")
	builder.WriteString("    set -l tokens (commandline -opc)\n")
	builder.WriteString("    set -l current (commandline -ct)\n")
	builder.WriteString("    if test (count $tokens) -lt 2\n")
	builder.WriteString("        return 1\n")
	builder.WriteString("    end\n")
	builder.WriteString("\n")
	builder.WriteString("    switch $tokens[2]\n")
	fmt.Fprintf(&builder, "        case %s\n", strings.Join(profileCompletionCommands, " "))
	builder.WriteString("        case '*'\n")
	builder.WriteString("            return 1\n")
	builder.WriteString("    end\n")
	builder.WriteString("\n")
	builder.WriteString("    if string match -q -- '-*' \"$current\"\n")
	builder.WriteString("        return 1\n")
	builder.WriteString("    end\n")
	builder.WriteString("\n")
	builder.WriteString("    for token in $tokens[3..-1]\n")
	builder.WriteString("        if test \"$token\" = \"$current\"\n")
	builder.WriteString("            continue\n")
	builder.WriteString("        end\n")
	builder.WriteString("        if not string match -q -- '-*' \"$token\"\n")
	builder.WriteString("            return 1\n")
	builder.WriteString("        end\n")
	builder.WriteString("    end\n")
	builder.WriteString("\n")
	builder.WriteString("    return 0\n")
	builder.WriteString("end\n\n")
	builder.WriteString("complete -c switchlet -f\n")
	builder.WriteString("complete -c switchlet -s h -l help -d 'Show help text'\n")
	builder.WriteString("complete -c switchlet -l version -d 'Show version information'\n")
	builder.WriteString("complete -c switchlet -n '__switchlet_profile_completion_needed' -a '(__switchlet_complete_profiles)' -d 'Profile'\n")
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

func writeProfileNameCompletions(output io.Writer, workingDirectory string) error {
	profileNames, err := loadProfileCompletionNames(workingDirectory)
	if err != nil {
		return nil
	}

	for _, profileName := range profileNames {
		if _, err := fmt.Fprintln(output, profileName); err != nil {
			return err
		}
	}

	return nil
}

func loadProfileCompletionNames(workingDirectory string) ([]string, error) {
	configPath, err := config.Discover(workingDirectory)
	if err != nil {
		return nil, err
	}

	loadedConfig, err := config.Load(configPath)
	if err != nil {
		return nil, err
	}

	profileNames := make([]string, 0, len(loadedConfig.Profiles))
	for _, profile := range loadedConfig.Profiles {
		profileNames = append(profileNames, profile.Name)
	}

	return profileNames, nil
}
