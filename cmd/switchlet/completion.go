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
	{Name: "undo", Description: "Undo the last recorded apply", Flags: []completionFlagSpec{{Name: "--enable", Description: "Enable private per-project snapshots"}, {Name: "--disable", Description: "Disable recording and discard snapshot"}, {Name: "--discard", Description: "Discard snapshot without changing target files"}, {Name: "--status", Description: "Show undo recording and snapshot status"}, {Name: "--recover", Description: "Recover an interrupted apply or undo"}, {Name: "--dry-run", Description: "Check restoration without writing target files"}, {Name: "--json", Description: "Write machine-readable metadata"}, {Name: "--config", Description: "Select the recorded project configuration"}, {Name: "--no-color", Description: "Disable styling"}}},
	{Name: "help", Description: "Show help text"},
	{Name: "init", Description: "Guided setup for a .switchlet.yaml", Flags: []completionFlagSpec{{Name: "--overwrite", Description: "Replace an existing .switchlet.yaml without prompting"}}},
	{Name: "config", Description: "Edit .switchlet.yaml in the configuration editor", Flags: []completionFlagSpec{{Name: "--config", Description: "Load a specific .switchlet.yaml"}}},
	{Name: "list", Description: "List configured profiles", Flags: []completionFlagSpec{{Name: "--json", Description: "Write machine-readable JSON output"}, {Name: "--config", Description: "Load a specific .switchlet.yaml"}, {Name: "--no-color", Description: "Disable styled command output"}}},
	{Name: "inspect", Description: "Inspect one configured profile", Flags: []completionFlagSpec{{Name: "--json", Description: "Write machine-readable JSON output"}, {Name: "--config", Description: "Load a specific .switchlet.yaml"}, {Name: "--no-color", Description: "Disable styled command output"}}},
	{Name: "apply", Description: "Apply one configured profile", Flags: []completionFlagSpec{
		{Name: "--json", Description: "Write machine-readable JSON output"},
		{Name: "--dry-run", Description: "Validate without writing target files"},
		{Name: "--allow-protected", Description: "Allow non-interactive apply for a protected profile"},
		{Name: "--expect-current", Description: "Require the current complete profile before apply or dry run"},
		{Name: "--config", Description: "Load a specific .switchlet.yaml"},
		{Name: "--no-color", Description: "Disable styled command output"},
	}},
	{Name: "status", Description: "Compare current managed values", Flags: []completionFlagSpec{{Name: "--json", Description: "Write machine-readable JSON output"}, {Name: "--short", Description: "Write a concise current-profile summary"}, {Name: "--name", Description: "Write only the current complete profile name"}, {Name: "--expect", Description: "Assert the current complete profile"}, {Name: "--config", Description: "Load a specific .switchlet.yaml"}, {Name: "--no-color", Description: "Disable styled command output"}}},
	{Name: "diff", Description: "Compare one profile with current managed values", Flags: []completionFlagSpec{{Name: "--json", Description: "Write machine-readable JSON output"}, {Name: "--patch", Description: "Write read-only managed patch text"}, {Name: "--exit-code", Description: "Return non-zero when the profile would change files"}, {Name: "--config", Description: "Load a specific .switchlet.yaml"}, {Name: "--no-color", Description: "Disable styled command output"}}},
	{Name: "doctor", Description: "Check project health", Flags: []completionFlagSpec{{Name: "--json", Description: "Write machine-readable JSON output"}, {Name: "--strict", Description: "Return non-zero when warnings are present"}, {Name: "--config", Description: "Load a specific .switchlet.yaml"}, {Name: "--no-color", Description: "Disable styled command output"}}},
	{Name: "targets", Description: "List configured targets", Flags: []completionFlagSpec{{Name: "--json", Description: "Write machine-readable JSON output"}, {Name: "--config", Description: "Load a specific .switchlet.yaml"}, {Name: "--no-color", Description: "Disable styled command output"}}},
	{Name: "version", Description: "Show version information"},
	{Name: "completion", Description: "Generate a shell completion script"},
}

var supportedCompletionShells = []string{"bash", "zsh", "fish", "powershell"}
var profileCompletionCommands = []string{"inspect", "apply", "diff"}

const profileCompletionCommandName = "__complete-profile-names"

func writeCompletionScript(output io.Writer, shell string, outputOptions commandOutputOptions) error {
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
	case "powershell":
		_, err := io.WriteString(output, powerShellCompletionScript())
		return err
	default:
		return usageCommandError(outputOptions, false, "unsupported completion shell %q\n\nSupported shells: %s\n\n%s", shell, strings.Join(supportedCompletionShells, ", "), completionHelpText())
	}
}

func bashCompletionScript() string {
	var builder strings.Builder
	builder.WriteString("# bash completion for switchlet\n")
	builder.WriteString("__switchlet_completion_context() {\n")
	builder.WriteString("    command=\"\"\n")
	builder.WriteString("    command_index=0\n")
	builder.WriteString("    completion_load_args=()\n")
	builder.WriteString("    local index word\n")
	builder.WriteString("    index=1\n")
	builder.WriteString("    while (( index < COMP_CWORD )); do\n")
	builder.WriteString("        word=\"${COMP_WORDS[index]}\"\n")
	builder.WriteString("        case \"$word\" in\n")
	builder.WriteString("            --config)\n")
	builder.WriteString("                if (( index + 1 < ${#COMP_WORDS[@]} )); then\n")
	builder.WriteString("                    completion_load_args+=(\"--config\" \"${COMP_WORDS[index+1]}\")\n")
	builder.WriteString("                fi\n")
	builder.WriteString("                index=$((index + 2))\n")
	builder.WriteString("                ;;\n")
	builder.WriteString("            --config=*)\n")
	builder.WriteString("                completion_load_args+=(\"$word\")\n")
	builder.WriteString("                index=$((index + 1))\n")
	builder.WriteString("                ;;\n")
	builder.WriteString("            --no-color)\n")
	builder.WriteString("                index=$((index + 1))\n")
	builder.WriteString("                ;;\n")
	builder.WriteString("            *)\n")
	builder.WriteString("                command=\"$word\"\n")
	builder.WriteString("                command_index=$index\n")
	builder.WriteString("                return 0\n")
	builder.WriteString("                ;;\n")
	builder.WriteString("        esac\n")
	builder.WriteString("    done\n")
	builder.WriteString("    return 1\n")
	builder.WriteString("}\n")
	builder.WriteString("\n")
	builder.WriteString("__switchlet_needs_profile_completion() {\n")
	builder.WriteString("    case \"$command\" in\n")
	builder.WriteString("        status)\n")
	builder.WriteString("            [[ ${COMP_CWORD} -gt $((command_index + 1)) && \"${COMP_WORDS[COMP_CWORD-1]}\" == \"--expect\" ]] && return 0\n")
	builder.WriteString("            return 1\n")
	builder.WriteString("            ;;\n")
	builder.WriteString("        apply)\n")
	builder.WriteString("            [[ ${COMP_CWORD} -gt $((command_index + 1)) && \"${COMP_WORDS[COMP_CWORD-1]}\" == \"--expect-current\" ]] && return 0\n")
	builder.WriteString("            ;;\n")
	builder.WriteString("        inspect|diff)\n")
	builder.WriteString("            ;;\n")
	builder.WriteString("        *) return 1 ;;\n")
	builder.WriteString("    esac\n")
	builder.WriteString("    [[ \"$cur\" == -* ]] && return 1\n")
	builder.WriteString("\n")
	builder.WriteString("    local index word\n")
	builder.WriteString("    for (( index = command_index + 1; index < COMP_CWORD; index++ )); do\n")
	builder.WriteString("        word=\"${COMP_WORDS[index]}\"\n")
	builder.WriteString("        case \"$word\" in\n")
	builder.WriteString("            --config|--expect|--expect-current)\n")
	builder.WriteString("                index=$((index + 1))\n")
	builder.WriteString("                continue\n")
	builder.WriteString("                ;;\n")
	builder.WriteString("            --config=*|--expect=*|--expect-current=*)\n")
	builder.WriteString("                continue\n")
	builder.WriteString("                ;;\n")
	builder.WriteString("        esac\n")
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
	fmt.Fprintf(&builder, "    done < <(switchlet \"${completion_load_args[@]}\" %s 2>/dev/null)\n", profileCompletionCommandName)
	builder.WriteString("}\n")
	builder.WriteString("\n")
	builder.WriteString("_switchlet_completion() {\n")
	builder.WriteString("    local cur command command_index\n")
	builder.WriteString("    local -a completion_load_args\n")
	builder.WriteString("    COMPREPLY=()\n")
	builder.WriteString("    cur=\"${COMP_WORDS[COMP_CWORD]}\"\n")
	builder.WriteString("    __switchlet_completion_context\n")
	builder.WriteString("\n")
	builder.WriteString("    if [[ -z \"$command\" ]]; then\n")
	fmt.Fprintf(&builder, "        COMPREPLY=( $(compgen -W %q -- \"$cur\") )\n", strings.Join(append([]string{"--help", "--version", configFlag, noColorFlag}, completionCommandNames()...), " "))
	builder.WriteString("        return 0\n")
	builder.WriteString("    fi\n")
	builder.WriteString("\n")
	builder.WriteString("    if __switchlet_needs_profile_completion; then\n")
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
	builder.WriteString("_switchlet_parse_completion_context() {\n")
	builder.WriteString("  typeset -g switchlet_completion_command_name\n")
	builder.WriteString("  typeset -g switchlet_completion_command_index\n")
	builder.WriteString("  typeset -ga switchlet_completion_load_args\n")
	builder.WriteString("  switchlet_completion_command_name=\"\"\n")
	builder.WriteString("  switchlet_completion_command_index=0\n")
	builder.WriteString("  switchlet_completion_load_args=()\n")
	builder.WriteString("  local index word\n")
	builder.WriteString("  index=2\n")
	builder.WriteString("  while (( index < CURRENT )); do\n")
	builder.WriteString("    word=\"${words[index]}\"\n")
	builder.WriteString("    case \"$word\" in\n")
	builder.WriteString("      --config)\n")
	builder.WriteString("        if (( index + 1 < CURRENT )); then\n")
	builder.WriteString("          switchlet_completion_load_args+=(--config \"${words[index+1]}\")\n")
	builder.WriteString("        fi\n")
	builder.WriteString("        (( index += 2 ))\n")
	builder.WriteString("        ;;\n")
	builder.WriteString("      --config=*)\n")
	builder.WriteString("        switchlet_completion_load_args+=(\"$word\")\n")
	builder.WriteString("        (( index += 1 ))\n")
	builder.WriteString("        ;;\n")
	builder.WriteString("      --no-color)\n")
	builder.WriteString("        (( index += 1 ))\n")
	builder.WriteString("        ;;\n")
	builder.WriteString("      *)\n")
	builder.WriteString("        switchlet_completion_command_name=\"$word\"\n")
	builder.WriteString("        switchlet_completion_command_index=$index\n")
	builder.WriteString("        return 0\n")
	builder.WriteString("        ;;\n")
	builder.WriteString("    esac\n")
	builder.WriteString("  done\n")
	builder.WriteString("  return 1\n")
	builder.WriteString("}\n\n")
	builder.WriteString("_switchlet_profile_names() {\n")
	builder.WriteString("  local -a profiles completion_command\n")
	builder.WriteString("  local output\n")
	builder.WriteString("  _switchlet_parse_completion_context\n")
	builder.WriteString("  completion_command=(switchlet)\n")
	builder.WriteString("  completion_command+=(\"${switchlet_completion_load_args[@]}\")\n")
	fmt.Fprintf(&builder, "  completion_command+=(%q)\n", profileCompletionCommandName)
	builder.WriteString("  output=\"$(${completion_command[@]} 2>/dev/null)\"\n")
	builder.WriteString("  [[ -n \"$output\" ]] || return 1\n")
	builder.WriteString("  profiles=(\"${(@f)output}\")\n")
	builder.WriteString("  compadd -a profiles\n")
	builder.WriteString("}\n\n")
	builder.WriteString("_switchlet_needs_profile_completion() {\n")
	builder.WriteString("  _switchlet_parse_completion_context || return 1\n")
	builder.WriteString("  if [[ \"$switchlet_completion_command_name\" == status ]]; then\n")
	builder.WriteString("    [[ $CURRENT -gt $((switchlet_completion_command_index + 1)) && \"${words[CURRENT-1]}\" == \"--expect\" ]] && return 0\n")
	builder.WriteString("    return 1\n")
	builder.WriteString("  fi\n")
	builder.WriteString("  if [[ \"$switchlet_completion_command_name\" == apply ]]; then\n")
	builder.WriteString("    [[ $CURRENT -gt $((switchlet_completion_command_index + 1)) && \"${words[CURRENT-1]}\" == \"--expect-current\" ]] && return 0\n")
	builder.WriteString("  fi\n")
	builder.WriteString("  case \"$switchlet_completion_command_name\" in\n")
	builder.WriteString("    inspect|apply|diff) ;;\n")
	builder.WriteString("    *) return 1 ;;\n")
	builder.WriteString("  esac\n")
	builder.WriteString("  [[ \"${words[CURRENT]}\" == -* ]] && return 1\n")
	builder.WriteString("  local index word\n")
	builder.WriteString("  for (( index = switchlet_completion_command_index + 1; index < CURRENT; index++ )); do\n")
	builder.WriteString("    word=\"${words[index]}\"\n")
	builder.WriteString("    case \"$word\" in\n")
	builder.WriteString("      --config|--expect|--expect-current)\n")
	builder.WriteString("        (( index += 1 ))\n")
	builder.WriteString("        continue\n")
	builder.WriteString("        ;;\n")
	builder.WriteString("      --config=*|--expect=*|--expect-current=*)\n")
	builder.WriteString("        continue\n")
	builder.WriteString("        ;;\n")
	builder.WriteString("    esac\n")
	builder.WriteString("    [[ \"$word\" == -* ]] && continue\n")
	builder.WriteString("    return 1\n")
	builder.WriteString("  done\n")
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
	fmt.Fprintf(&builder, "    '%s[Load a specific .switchlet.yaml]:config file:_files' \\\n", configFlag)
	fmt.Fprintf(&builder, "    '%s[Disable styled command output]' \\\n", noColorFlag)
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
	builder.WriteString("      _switchlet_parse_completion_context\n")
	builder.WriteString("      case $switchlet_completion_command_name in\n")
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
	fmt.Fprintf(&builder, "          _values 'shell' %s\n", strings.Join(supportedCompletionShells, " "))
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
	builder.WriteString("function __switchlet_completion_command_context\n")
	builder.WriteString("    set -g __switchlet_command_name ''\n")
	builder.WriteString("    set -g __switchlet_command_index 0\n")
	builder.WriteString("    set -e __switchlet_completion_load_args\n")
	builder.WriteString("    set -l tokens (commandline -opc)\n")
	builder.WriteString("    set -l index 2\n")
	builder.WriteString("    while test $index -le (count $tokens)\n")
	builder.WriteString("        set -l token $tokens[$index]\n")
	builder.WriteString("        switch $token\n")
	builder.WriteString("            case '--config'\n")
	builder.WriteString("                set -l next_index (math $index + 1)\n")
	builder.WriteString("                if test $next_index -le (count $tokens)\n")
	builder.WriteString("                    set -ga __switchlet_completion_load_args --config $tokens[$next_index]\n")
	builder.WriteString("                end\n")
	builder.WriteString("                set index (math $index + 2)\n")
	builder.WriteString("            case '--config=*'\n")
	builder.WriteString("                set -ga __switchlet_completion_load_args $token\n")
	builder.WriteString("                set index (math $index + 1)\n")
	builder.WriteString("            case '--no-color'\n")
	builder.WriteString("                set index (math $index + 1)\n")
	builder.WriteString("            case '*'\n")
	builder.WriteString("                set -g __switchlet_command_name $token\n")
	builder.WriteString("                set -g __switchlet_command_index $index\n")
	builder.WriteString("                return 0\n")
	builder.WriteString("        end\n")
	builder.WriteString("    end\n")
	builder.WriteString("    return 1\n")
	builder.WriteString("end\n\n")
	builder.WriteString("function __switchlet_previous_token\n")
	builder.WriteString("    set -l tokens (commandline -opc)\n")
	builder.WriteString("    set -l current (commandline -ct)\n")
	builder.WriteString("    if test (count $tokens) -eq 0\n")
	builder.WriteString("        return 1\n")
	builder.WriteString("    end\n")
	builder.WriteString("    set -l previous $tokens[-1]\n")
	builder.WriteString("    if test \"$previous\" = \"$current\"; and test (count $tokens) -ge 2\n")
	builder.WriteString("        set previous $tokens[-2]\n")
	builder.WriteString("    end\n")
	builder.WriteString("    printf '%s\\n' $previous\n")
	builder.WriteString("end\n\n")
	builder.WriteString("function __switchlet_complete_profiles\n")
	builder.WriteString("    __switchlet_completion_command_context >/dev/null 2>/dev/null\n")
	fmt.Fprintf(&builder, "    switchlet $__switchlet_completion_load_args %s 2>/dev/null\n", profileCompletionCommandName)
	builder.WriteString("end\n\n")
	builder.WriteString("function __switchlet_profile_completion_needed\n")
	builder.WriteString("    set -l current (commandline -ct)\n")
	builder.WriteString("    if not __switchlet_completion_command_context\n")
	builder.WriteString("        return 1\n")
	builder.WriteString("    end\n")
	builder.WriteString("\n")
	builder.WriteString("    set -l previous (__switchlet_previous_token)\n")
	builder.WriteString("    if test \"$__switchlet_command_name\" = status\n")
	builder.WriteString("        if test \"$previous\" = \"--expect\"\n")
	builder.WriteString("            return 0\n")
	builder.WriteString("        end\n")
	builder.WriteString("        return 1\n")
	builder.WriteString("    end\n")
	builder.WriteString("\n")
	builder.WriteString("    if test \"$__switchlet_command_name\" = apply; and test \"$previous\" = \"--expect-current\"\n")
	builder.WriteString("        return 0\n")
	builder.WriteString("    end\n")
	builder.WriteString("\n")
	builder.WriteString("    switch $__switchlet_command_name\n")
	fmt.Fprintf(&builder, "        case %s\n", strings.Join(profileCompletionCommands, " "))
	builder.WriteString("        case '*'\n")
	builder.WriteString("            return 1\n")
	builder.WriteString("    end\n")
	builder.WriteString("\n")
	builder.WriteString("    if string match -q -- '-*' \"$current\"\n")
	builder.WriteString("        return 1\n")
	builder.WriteString("    end\n")
	builder.WriteString("\n")
	builder.WriteString("    set -l tokens (commandline -opc)\n")
	builder.WriteString("    set -l index (math $__switchlet_command_index + 1)\n")
	builder.WriteString("    while test $index -le (count $tokens)\n")
	builder.WriteString("        set -l token $tokens[$index]\n")
	builder.WriteString("        if test \"$token\" = \"$current\"\n")
	builder.WriteString("            set index (math $index + 1)\n")
	builder.WriteString("            continue\n")
	builder.WriteString("        end\n")
	builder.WriteString("        switch $token\n")
	builder.WriteString("            case '--config' '--expect' '--expect-current'\n")
	builder.WriteString("                set index (math $index + 2)\n")
	builder.WriteString("                continue\n")
	builder.WriteString("            case '--config=*' '--expect=*' '--expect-current=*'\n")
	builder.WriteString("                set index (math $index + 1)\n")
	builder.WriteString("                continue\n")
	builder.WriteString("        end\n")
	builder.WriteString("        if not string match -q -- '-*' \"$token\"\n")
	builder.WriteString("            return 1\n")
	builder.WriteString("        end\n")
	builder.WriteString("        set index (math $index + 1)\n")
	builder.WriteString("    end\n")
	builder.WriteString("\n")
	builder.WriteString("    return 0\n")
	builder.WriteString("end\n\n")
	builder.WriteString("complete -c switchlet -f\n")
	builder.WriteString("complete -c switchlet -s h -l help -d 'Show help text'\n")
	builder.WriteString("complete -c switchlet -l version -d 'Show version information'\n")
	fmt.Fprintf(&builder, "complete -c switchlet -l %s -r -d 'Load a specific .switchlet.yaml'\n", strings.TrimPrefix(configFlag, "--"))
	fmt.Fprintf(&builder, "complete -c switchlet -l %s -d 'Disable styled command output'\n", strings.TrimPrefix(noColorFlag, "--"))
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

func writeProfileNameCompletions(output io.Writer, workingDirectory string, loadOptions projectLoadOptions) error {
	profileNames, err := loadProfileCompletionNames(workingDirectory, loadOptions)
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

func loadProfileCompletionNames(workingDirectory string, loadOptions projectLoadOptions) ([]string, error) {
	configPath, err := selectedConfigPath(workingDirectory, loadOptions)
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
