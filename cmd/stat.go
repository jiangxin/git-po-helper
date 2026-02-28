package cmd

import (
	"fmt"
	"strings"

	"github.com/git-l10n/git-po-helper/flag"
	"github.com/git-l10n/git-po-helper/util"
	"github.com/spf13/cobra"
)

type statCommand struct {
	cmd *cobra.Command
}

func (v *statCommand) Command() *cobra.Command {
	if v.cmd != nil {
		return v.cmd
	}

	v.cmd = &cobra.Command{
		Use:   "stat <po-file> [po-file...]",
		Short: "Report statistics for PO file(s)",
		Long: `Report entry statistics for a PO file:
  translated   - entries with non-empty translation
  untranslated - entries with empty msgstr
  same         - entries where msgstr equals msgid (suspect untranslated)
  fuzzy        - entries with fuzzy flag
  obsolete     - obsolete entries (#~ format)

When run inside a git worktree, paths are relative to the project root (e.g. po/zh_CN.po).
When run outside a git repository, paths are relative to the current directory or absolute.

For review JSON report, use: git-po-helper agent-run report [path]`,
		SilenceErrors: true,
		RunE: func(cmd *cobra.Command, args []string) error {
			return v.Execute(args)
		},
	}

	return v.cmd
}

func (v statCommand) Execute(args []string) error {
	if len(args) < 1 {
		return newUserError("stat requires at least one argument: <po-file> [po-file...]")
	}

	for i, poFile := range args {
		if !util.Exist(poFile) {
			return newUserError("file does not exist:", poFile)
		}

		stats, err := util.CountPoReportStats(poFile)
		if err != nil {
			return newUserErrorF("%v", err)
		}

		if flag.Verbose() > 0 {
			if i > 0 {
				fmt.Println()
			}
			title := fmt.Sprintf("PO file: %s", poFile)
			fmt.Println(title)
			fmt.Println(strings.Repeat("-", len(title)))
			fmt.Printf("  translated:   %d\n", stats.Translated)
			fmt.Printf("  untranslated: %d\n", stats.Untranslated)
			fmt.Printf("  same:         %d\n", stats.Same)
			fmt.Printf("  fuzzy:        %d\n", stats.Fuzzy)
			fmt.Printf("  obsolete:     %d\n", stats.Obsolete)
		} else {
			fmt.Printf("%s: %s", poFile, util.FormatStatLine(stats))
		}
	}

	return nil
}

var statCmd = statCommand{}

func init() {
	rootCmd.AddCommand(statCmd.Command())
}
