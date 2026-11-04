package main

import (
	"fmt"
	"github.com/jeppeklh/switchlet/internal/app"
	"github.com/jeppeklh/switchlet/internal/history"
	"io"
	"path/filepath"
	"time"
)

func runUndoCommand(workingDirectory string, args []string, output io.Writer, outputOptions commandOutputOptions, loadOptions projectLoadOptions) error {
	if wantsHelpFlag(args) {
		_, err := io.WriteString(output, undoHelpText())
		return err
	}
	jsonOutput := containsJSONFlag(args)
	var dryRun, enable, disable, discard, status, recoverOperation bool
	flags := map[string]*bool{"--json": &jsonOutput, "--dry-run": &dryRun, "--enable": &enable, "--disable": &disable, "--discard": &discard, "--status": &status, "--recover": &recoverOperation}
	positionals, err := parseProjectArguments(args, flags, &outputOptions, &loadOptions)
	if err != nil {
		return usageCommandError(outputOptions, jsonOutput, "undo: %v\n\n%s", err, undoHelpText())
	}
	modes := 0
	for _, active := range []bool{enable, disable, discard, status, recoverOperation} {
		if active {
			modes++
		}
	}
	if len(positionals) != 0 || modes > 1 || (dryRun && (enable || disable || discard || status)) {
		return usageCommandError(outputOptions, jsonOutput, "undo accepts one action and no positional arguments; --dry-run applies only to undo/recovery\n\n%s", undoHelpText())
	}
	configPath, err := selectedConfigPath(workingDirectory, loadOptions)
	if err != nil {
		return runtimeCommandError(outputOptions, jsonOutput, err)
	}
	store, err := history.New(configPath)
	if err != nil {
		return runtimeCommandError(outputOptions, jsonOutput, err)
	}
	workflow := app.NewUndoWorkflow(store)
	var result app.UndoResult
	switch {
	case enable:
		result, err = workflow.Configure("enable")
	case disable:
		result, err = workflow.Configure("disable")
	case discard:
		result, err = workflow.Configure("discard")
	case status:
		result, err = workflow.Status()
	case recoverOperation && dryRun:
		result, err = workflow.PreviewRecovery()
	case recoverOperation:
		result, err = workflow.Recover()
	case dryRun:
		result, err = workflow.Preview()
	default:
		result, err = workflow.Undo()
	}
	if err != nil {
		return runtimeCommandError(outputOptions, jsonOutput, err)
	}
	if jsonOutput {
		return writeJSON(output, struct {
			SchemaVersion int            `json:"schemaVersion"`
			Result        app.UndoResult `json:"result"`
		}{commandJSONSchemaVersion, result})
	}
	return writeUndoText(output, result, filepath.Dir(configPath))
}

func writeUndoText(output io.Writer, result app.UndoResult, projectRoot string) error {
	switch result.Action {
	case "enable":
		_, err := fmt.Fprintln(output, "Undo recording enabled for this project. Snapshots may contain secrets; they are stored privately without encryption.")
		return err
	case "disable":
		_, err := fmt.Fprintln(output, "Undo recording disabled; saved snapshot discarded.")
		return err
	case "discard":
		_, err := fmt.Fprintln(output, fmt.Sprintf("Saved undo snapshot discarded. Recording enabled: %t. Target files were not changed.", result.Enabled))
		return err
	case "status":
		if _, err := fmt.Fprintf(output, "Undo recording enabled: %t\nSnapshot: %s\nStorage: %s\n", result.Enabled, result.Status, result.StorageDirectory); err != nil {
			return err
		}
		if result.ProfileName != "" {
			if _, err := fmt.Fprintf(output, "Profile: %q\nRecorded: %s\n", result.ProfileName, result.CreatedAt.Format(time.RFC3339)); err != nil {
				return err
			}
		}
	default:
		action := "Restored"
		if result.DryRun {
			action = "Would restore"
		}
		if _, err := fmt.Fprintf(output, "%s %d file(s) from apply of profile %q.\n", action, len(result.Files), result.ProfileName); err != nil {
			return err
		}
	}
	for _, file := range result.Files {
		if _, err := fmt.Fprintf(output, "  %s\n", displayProjectPath(projectRoot, file)); err != nil {
			return err
		}
	}
	return nil
}

func undoHelpText() string {
	return `Usage:
  switchlet undo [--dry-run] [--json] [--config <path>]
  switchlet undo --enable|--disable|--discard|--status [--json] [--config <path>]
  switchlet undo --recover [--dry-run] [--json] [--config <path>]

Undo the last successful changed apply using exact previous file bytes.
Recording is opt-in per project. Enable it before applying a profile.
Undo refuses every restoration if any affected file changed afterward, including
unmanaged values or permissions. A successful undo consumes the snapshot.
Dry runs and unchanged applies preserve the existing snapshot.

Snapshots can contain secrets from entire files. They are private, local,
unencrypted state outside the repository, restricted to the current user
(and SYSTEM on Windows). --disable and --discard delete the saved snapshot.

Interrupted operations block new applies. Inspect --status, preview
--recover --dry-run, then use --recover to finish restoring the recorded state.
Recovery accepts only exact recorded before/after bytes; it cannot force away
external edits. --discard explicitly abandons recovery without changing targets.

Undo reads captured files independently of current profile definitions and
current environment values. --config also works when that config cannot parse.
Normal text and JSON output never include snapshot contents.
`
}
