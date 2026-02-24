package main

import (
	"os"
	"strings"

	"github.com/git-l10n/git-po-helper/cmd"
)

const (
	// Program is name for this project
	Program = "git-po-helper"
)

func main() {
	resp := cmd.Execute()

	if resp.Err != nil {
		if resp.IsUserError() {
			if resp.Cmd.SilenceErrors {
				resp.Cmd.Printf("ERROR: %s\n", resp.Err)
				resp.Cmd.Println("")
			}
			resp.Cmd.Println(resp.Cmd.UsageString())
		} else if resp.Cmd.SilenceErrors {
			resp.Cmd.Println("")
			// Use CommandPath() to get full command path (e.g., "git-po-helper agent-run translate")
			// Remove Program prefix to get subcommand path (e.g., "agent-run translate")
			cmdPath := resp.Cmd.CommandPath()
			subCmdPath := strings.TrimPrefix(cmdPath, Program+" ")
			if subCmdPath == "" {
				// Fallback to Name() if CommandPath() only contains Program
				subCmdPath = resp.Cmd.Name()
			}
			resp.Cmd.Printf("ERROR: fail to execute \"%s %s\"\n", Program, subCmdPath)
		}
		os.Exit(-1)
	}
}
