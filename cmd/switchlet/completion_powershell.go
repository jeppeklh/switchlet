package main

import (
	"fmt"
	"strings"
)

func powerShellCompletionScript() string {
	var builder strings.Builder

	builder.WriteString("# PowerShell completion for switchlet\n")
	writePowerShellCommandSpecs(&builder)
	writePowerShellGlobalFlagSpecs(&builder)
	writePowerShellCommandFlagSpecs(&builder)
	fmt.Fprintf(&builder, "$__switchletSupportedShells = @(%s)\n\n", powerShellQuotedJoin(supportedCompletionShells))

	builder.WriteString("function __switchlet_CommandElementText {\n")
	builder.WriteString("    param([System.Management.Automation.Language.CommandElementAst]$Element)\n\n")
	builder.WriteString("    if ($Element -is [System.Management.Automation.Language.StringConstantExpressionAst]) {\n")
	builder.WriteString("        return $Element.Value\n")
	builder.WriteString("    }\n\n")
	builder.WriteString("    if ($Element -is [System.Management.Automation.Language.ExpandableStringExpressionAst]) {\n")
	builder.WriteString("        return $Element.Value\n")
	builder.WriteString("    }\n\n")
	builder.WriteString("    return $Element.Extent.Text\n")
	builder.WriteString("}\n\n")

	builder.WriteString("function __switchlet_ParseCompletionContext {\n")
	builder.WriteString("    param([System.Management.Automation.Language.CommandAst]$CommandAst)\n\n")
	builder.WriteString("    $tokens = New-Object 'System.Collections.Generic.List[string]'\n")
	builder.WriteString("    foreach ($element in $CommandAst.CommandElements) {\n")
	builder.WriteString("        $tokens.Add((__switchlet_CommandElementText $element))\n")
	builder.WriteString("    }\n\n")
	builder.WriteString("    $loadArgs = New-Object 'System.Collections.Generic.List[string]'\n")
	builder.WriteString("    $command = $null\n")
	builder.WriteString("    $commandIndex = -1\n")
	builder.WriteString("    $index = 1\n\n")
	builder.WriteString("    while ($index -lt $tokens.Count) {\n")
	builder.WriteString("        $token = $tokens[$index]\n\n")
	builder.WriteString("        if ($token -eq '--config') {\n")
	builder.WriteString("            if ($index + 1 -lt $tokens.Count) {\n")
	builder.WriteString("                $loadArgs.Add('--config')\n")
	builder.WriteString("                $loadArgs.Add($tokens[$index + 1])\n")
	builder.WriteString("            }\n")
	builder.WriteString("            $index += 2\n")
	builder.WriteString("            continue\n")
	builder.WriteString("        }\n\n")
	builder.WriteString("        if ($token.StartsWith('--config=')) {\n")
	builder.WriteString("            $loadArgs.Add($token)\n")
	builder.WriteString("            $index += 1\n")
	builder.WriteString("            continue\n")
	builder.WriteString("        }\n\n")
	builder.WriteString("        if ($token -eq '--no-color') {\n")
	builder.WriteString("            $index += 1\n")
	builder.WriteString("            continue\n")
	builder.WriteString("        }\n\n")
	builder.WriteString("        $command = $token\n")
	builder.WriteString("        $commandIndex = $index\n")
	builder.WriteString("        break\n")
	builder.WriteString("    }\n\n")
	builder.WriteString("    return [pscustomobject]@{\n")
	builder.WriteString("        Tokens = $tokens.ToArray()\n")
	builder.WriteString("        Command = $command\n")
	builder.WriteString("        CommandIndex = $commandIndex\n")
	builder.WriteString("        LoadArgs = $loadArgs.ToArray()\n")
	builder.WriteString("    }\n")
	builder.WriteString("}\n\n")

	builder.WriteString("function __switchlet_CurrentWordIncluded {\n")
	builder.WriteString("    param($Context, [string]$WordToComplete)\n\n")
	builder.WriteString("    return $WordToComplete -ne '' -and $Context.Tokens.Count -gt 0 -and $Context.Tokens[-1] -eq $WordToComplete\n")
	builder.WriteString("}\n\n")

	builder.WriteString("function __switchlet_PreviousToken {\n")
	builder.WriteString("    param($Context, [string]$WordToComplete)\n\n")
	builder.WriteString("    $previousIndex = $Context.Tokens.Count - 1\n")
	builder.WriteString("    if (__switchlet_CurrentWordIncluded $Context $WordToComplete) {\n")
	builder.WriteString("        $previousIndex -= 1\n")
	builder.WriteString("    }\n")
	builder.WriteString("    if ($previousIndex -lt 0) {\n")
	builder.WriteString("        return $null\n")
	builder.WriteString("    }\n\n")
	builder.WriteString("    return $Context.Tokens[$previousIndex]\n")
	builder.WriteString("}\n\n")

	builder.WriteString("function __switchlet_ArgumentScanLimit {\n")
	builder.WriteString("    param($Context, [string]$WordToComplete)\n\n")
	builder.WriteString("    if (__switchlet_CurrentWordIncluded $Context $WordToComplete) {\n")
	builder.WriteString("        return $Context.Tokens.Count - 1\n")
	builder.WriteString("    }\n\n")
	builder.WriteString("    return $Context.Tokens.Count\n")
	builder.WriteString("}\n\n")

	builder.WriteString("function __switchlet_ExpectedProfileCompletionNeeded {\n")
	builder.WriteString("    param($Context, [string]$WordToComplete)\n\n")
	builder.WriteString("    $previousToken = __switchlet_PreviousToken $Context $WordToComplete\n")
	builder.WriteString("    if ($Context.Command -eq 'status' -and $previousToken -eq '--expect') {\n")
	builder.WriteString("        return $true\n")
	builder.WriteString("    }\n")
	builder.WriteString("    if ($Context.Command -eq 'apply' -and $previousToken -eq '--expect-current') {\n")
	builder.WriteString("        return $true\n")
	builder.WriteString("    }\n\n")
	builder.WriteString("    return $false\n")
	builder.WriteString("}\n\n")

	builder.WriteString("function __switchlet_SelectedProfileCompletionState {\n")
	builder.WriteString("    param($Context, [string]$WordToComplete)\n\n")
	builder.WriteString("    $state = [pscustomobject]@{\n")
	builder.WriteString("        Needed = $false\n")
	builder.WriteString("        AfterDelimiter = $false\n")
	builder.WriteString("    }\n\n")
	builder.WriteString("    if (-not $Context.Command) {\n")
	builder.WriteString("        return $state\n")
	builder.WriteString("    }\n")
	builder.WriteString("    if ($Context.Command -notin @('inspect', 'apply', 'diff')) {\n")
	builder.WriteString("        return $state\n")
	builder.WriteString("    }\n\n")
	builder.WriteString("    $afterDelimiter = $false\n")
	builder.WriteString("    $scanLimit = __switchlet_ArgumentScanLimit $Context $WordToComplete\n\n")
	builder.WriteString("    for ($index = $Context.CommandIndex + 1; $index -lt $scanLimit; $index++) {\n")
	builder.WriteString("        $token = $Context.Tokens[$index]\n\n")
	builder.WriteString("        if ($token -eq '--') {\n")
	builder.WriteString("            $afterDelimiter = $true\n")
	builder.WriteString("            continue\n")
	builder.WriteString("        }\n\n")
	builder.WriteString("        if (-not $afterDelimiter) {\n")
	builder.WriteString("            if ($token -eq '--config' -or $token -eq '--expect' -or $token -eq '--expect-current') {\n")
	builder.WriteString("                $index += 1\n")
	builder.WriteString("                continue\n")
	builder.WriteString("            }\n")
	builder.WriteString("            if ($token.StartsWith('--config=') -or $token.StartsWith('--expect=') -or $token.StartsWith('--expect-current=')) {\n")
	builder.WriteString("                continue\n")
	builder.WriteString("            }\n")
	builder.WriteString("            if ($token.StartsWith('-')) {\n")
	builder.WriteString("                continue\n")
	builder.WriteString("            }\n")
	builder.WriteString("        }\n\n")
	builder.WriteString("        return $state\n")
	builder.WriteString("    }\n\n")
	builder.WriteString("    if (-not [string]::IsNullOrEmpty($WordToComplete) -and $WordToComplete.StartsWith('-') -and -not $afterDelimiter) {\n")
	builder.WriteString("        return $state\n")
	builder.WriteString("    }\n\n")
	builder.WriteString("    $state.Needed = $true\n")
	builder.WriteString("    $state.AfterDelimiter = $afterDelimiter\n")
	builder.WriteString("    return $state\n")
	builder.WriteString("}\n\n")

	builder.WriteString("function __switchlet_MatchesPrefix {\n")
	builder.WriteString("    param([string]$Candidate, [string]$WordToComplete)\n\n")
	builder.WriteString("    if ([string]::IsNullOrEmpty($WordToComplete)) {\n")
	builder.WriteString("        return $true\n")
	builder.WriteString("    }\n\n")
	builder.WriteString("    return $Candidate.StartsWith($WordToComplete, [System.StringComparison]::OrdinalIgnoreCase)\n")
	builder.WriteString("}\n\n")

	builder.WriteString("function __switchlet_QuoteCompletionText {\n")
	builder.WriteString("    param([string]$Value)\n\n")
	builder.WriteString("    if ($Value -match '^[A-Za-z0-9._/:\\-]+$') {\n")
	builder.WriteString("        return $Value\n")
	builder.WriteString("    }\n\n")
	builder.WriteString("    return \"'\" + $Value.Replace(\"'\", \"''\") + \"'\"\n")
	builder.WriteString("}\n\n")

	builder.WriteString("function __switchlet_NewCompletionResult {\n")
	builder.WriteString("    param([string]$CompletionText, [string]$ListItemText, [string]$ResultType, [string]$ToolTip)\n\n")
	builder.WriteString("    return [System.Management.Automation.CompletionResult]::new($CompletionText, $ListItemText, ([System.Management.Automation.CompletionResultType]$ResultType), $ToolTip)\n")
	builder.WriteString("}\n\n")

	builder.WriteString("function __switchlet_CompleteFromSpecs {\n")
	builder.WriteString("    param([string]$WordToComplete, $Specs, [string]$ResultType)\n\n")
	builder.WriteString("    foreach ($spec in $Specs) {\n")
	builder.WriteString("        if (__switchlet_MatchesPrefix $spec.Name $WordToComplete) {\n")
	builder.WriteString("            __switchlet_NewCompletionResult $spec.Name $spec.Name $ResultType $spec.Description\n")
	builder.WriteString("        }\n")
	builder.WriteString("    }\n")
	builder.WriteString("}\n\n")

	builder.WriteString("function __switchlet_CompleteProfiles {\n")
	builder.WriteString("    param($Context, [string]$WordToComplete, [bool]$NeedsDelimiter)\n\n")
	builder.WriteString("    try {\n")
	builder.WriteString("        $profileNames = @(& switchlet @($Context.LoadArgs) '__complete-profile-names' 2>$null)\n")
	builder.WriteString("    }\n")
	builder.WriteString("    catch {\n")
	builder.WriteString("        return @()\n")
	builder.WriteString("    }\n\n")
	builder.WriteString("    $results = New-Object 'System.Collections.Generic.List[System.Management.Automation.CompletionResult]'\n")
	builder.WriteString("    foreach ($profileName in $profileNames) {\n")
	builder.WriteString("        if (-not (__switchlet_MatchesPrefix $profileName $WordToComplete)) {\n")
	builder.WriteString("            continue\n")
	builder.WriteString("        }\n\n")
	builder.WriteString("        $completionText = __switchlet_QuoteCompletionText $profileName\n")
	builder.WriteString("        if ($NeedsDelimiter -and $profileName.StartsWith('-')) {\n")
	builder.WriteString("            $completionText = '-- ' + $completionText\n")
	builder.WriteString("        }\n\n")
	builder.WriteString("        $results.Add((__switchlet_NewCompletionResult $completionText $profileName 'ParameterValue' 'Profile'))\n")
	builder.WriteString("    }\n\n")
	builder.WriteString("    return $results.ToArray()\n")
	builder.WriteString("}\n\n")

	builder.WriteString("Register-ArgumentCompleter -Native -CommandName switchlet -ScriptBlock {\n")
	builder.WriteString("    param($wordToComplete, $commandAst, $cursorPosition)\n\n")
	builder.WriteString("    $Context = __switchlet_ParseCompletionContext $commandAst\n\n")
	builder.WriteString("    if (-not $Context.Command) {\n")
	builder.WriteString("        __switchlet_CompleteFromSpecs $wordToComplete $__switchletGlobalFlags 'ParameterName'\n")
	builder.WriteString("        __switchlet_CompleteFromSpecs $wordToComplete $__switchletCommands 'Command'\n")
	builder.WriteString("        return\n")
	builder.WriteString("    }\n\n")
	builder.WriteString("    if ($Context.Command -eq 'completion') {\n")
	builder.WriteString("        $scanLimit = __switchlet_ArgumentScanLimit $Context $wordToComplete\n")
	builder.WriteString("        $shellSelected = $false\n")
	builder.WriteString("        for ($index = $Context.CommandIndex + 1; $index -lt $scanLimit; $index++) {\n")
	builder.WriteString("            if ($Context.Tokens[$index] -eq '--') {\n")
	builder.WriteString("                continue\n")
	builder.WriteString("            }\n")
	builder.WriteString("            $shellSelected = $true\n")
	builder.WriteString("            break\n")
	builder.WriteString("        }\n")
	builder.WriteString("        if (-not $shellSelected) {\n")
	builder.WriteString("            foreach ($shell in $__switchletSupportedShells) {\n")
	builder.WriteString("                if (__switchlet_MatchesPrefix $shell $wordToComplete) {\n")
	builder.WriteString("                    __switchlet_NewCompletionResult $shell $shell 'ParameterValue' 'Shell'\n")
	builder.WriteString("                }\n")
	builder.WriteString("            }\n")
	builder.WriteString("            return\n")
	builder.WriteString("        }\n")
	builder.WriteString("    }\n\n")
	builder.WriteString("    if (__switchlet_ExpectedProfileCompletionNeeded $Context $wordToComplete) {\n")
	builder.WriteString("        $profileResults = @(__switchlet_CompleteProfiles $Context $wordToComplete $false)\n")
	builder.WriteString("        if ($profileResults.Count -gt 0 -or $wordToComplete -ne '') {\n")
	builder.WriteString("            $profileResults\n")
	builder.WriteString("            return\n")
	builder.WriteString("        }\n")
	builder.WriteString("    }\n\n")
	builder.WriteString("    $selectedProfileState = __switchlet_SelectedProfileCompletionState $Context $wordToComplete\n")
	builder.WriteString("    if ($selectedProfileState.Needed) {\n")
	builder.WriteString("        $profileResults = @(__switchlet_CompleteProfiles $Context $wordToComplete (-not $selectedProfileState.AfterDelimiter))\n")
	builder.WriteString("        if ($profileResults.Count -gt 0 -or $wordToComplete -ne '') {\n")
	builder.WriteString("            $profileResults\n")
	builder.WriteString("            return\n")
	builder.WriteString("        }\n")
	builder.WriteString("    }\n\n")
	builder.WriteString("    if ([string]::IsNullOrEmpty($wordToComplete) -or $wordToComplete.StartsWith('-')) {\n")
	builder.WriteString("        if ($__switchletCommandFlags.ContainsKey($Context.Command)) {\n")
	builder.WriteString("            __switchlet_CompleteFromSpecs $wordToComplete $__switchletCommandFlags[$Context.Command] 'ParameterName'\n")
	builder.WriteString("        }\n")
	builder.WriteString("    }\n")
	builder.WriteString("}\n")

	return builder.String()
}

func writePowerShellCommandSpecs(builder *strings.Builder) {
	builder.WriteString("$__switchletCommands = @(\n")
	for _, command := range completionCommands {
		fmt.Fprintf(builder, "    [pscustomobject]@{ Name = %s; Description = %s }\n", powerShellQuotedString(command.Name), powerShellQuotedString(command.Description))
	}
	builder.WriteString(")\n\n")
}

func writePowerShellGlobalFlagSpecs(builder *strings.Builder) {
	globalFlags := []completionFlagSpec{
		{Name: "--help", Description: "Show help text"},
		{Name: "--version", Description: "Show version information"},
		{Name: configFlag, Description: "Load a specific .switchlet.yaml"},
		{Name: noColorFlag, Description: "Disable styled command output"},
	}

	builder.WriteString("$__switchletGlobalFlags = @(\n")
	writePowerShellFlagSpecs(builder, globalFlags)
	builder.WriteString(")\n\n")
}

func writePowerShellCommandFlagSpecs(builder *strings.Builder) {
	builder.WriteString("$__switchletCommandFlags = @{\n")
	for _, command := range completionCommands {
		if len(command.Flags) == 0 {
			continue
		}

		fmt.Fprintf(builder, "    %s = @(\n", powerShellQuotedString(command.Name))
		writePowerShellFlagSpecs(builder, command.Flags)
		builder.WriteString("    )\n")
	}
	builder.WriteString("}\n\n")
}

func writePowerShellFlagSpecs(builder *strings.Builder, flags []completionFlagSpec) {
	for _, flag := range flags {
		fmt.Fprintf(builder, "        [pscustomobject]@{ Name = %s; Description = %s }\n", powerShellQuotedString(flag.Name), powerShellQuotedString(flag.Description))
	}
}

func powerShellQuotedJoin(values []string) string {
	quotedValues := make([]string, 0, len(values))
	for _, value := range values {
		quotedValues = append(quotedValues, powerShellQuotedString(value))
	}

	return strings.Join(quotedValues, ", ")
}

func powerShellQuotedString(value string) string {
	return "'" + strings.ReplaceAll(value, "'", "''") + "'"
}
