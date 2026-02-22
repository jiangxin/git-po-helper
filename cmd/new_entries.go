package cmd

import (
	"github.com/git-l10n/git-po-helper/repository"
	"github.com/git-l10n/git-po-helper/util"
	"github.com/spf13/cobra"
	"github.com/spf13/viper"
)

type newEntriesCommand struct {
	cmd *cobra.Command
	O   struct {
		Commit string
		Since  string
	}
}

func (v *newEntriesCommand) Command() *cobra.Command {
	if v.cmd != nil {
		return v.cmd
	}

	v.cmd = &cobra.Command{
		Use:   "new-entries [po/XX.po]",
		Short: "Output new or changed entries between two PO file versions",
		Long: `Compare two PO file versions and output new or changed entries to stdout.

Similar to agent-run review but without agent execution. Reuses the same
comparison logic (PrepareReviewData) and po file selection (ResolvePoFile).

If no po/XX.po argument is given, the PO file is selected from changed files
(interactive when multiple, auto when single).

Modes:
- --commit <commit>: compare parent of commit with the specified commit
- --since <commit>: compare since commit with current working tree
- no --commit/--since: compare HEAD with current working tree (local changes)

Exactly one of --commit and --since may be specified.
Output is empty when there are no new or changed entries.

Examples:
  git-po-helper new-entries
  git-po-helper new-entries po/zh_CN.po
  git-po-helper new-entries --since HEAD~5
  git-po-helper new-entries --commit abc123`,
		SilenceErrors: true,
		RunE: func(cmd *cobra.Command, args []string) error {
			return v.Execute(args)
		},
	}

	v.cmd.Flags().StringVar(&v.O.Commit,
		"commit",
		"",
		"compare parent of commit with the specified commit")
	v.cmd.Flags().StringVar(&v.O.Since,
		"since",
		"",
		"compare since commit with current working tree")

	_ = viper.BindPFlag("new-entries--commit", v.cmd.Flags().Lookup("commit"))
	_ = viper.BindPFlag("new-entries--since", v.cmd.Flags().Lookup("since"))

	return v.cmd
}

func (v *newEntriesCommand) Execute(args []string) error {
	repository.ChdirProjectRoot()

	if len(args) > 1 {
		return newUserError("new-entries expects at most one argument: po/XX.po")
	}

	if v.O.Commit != "" && v.O.Since != "" {
		return newUserError("new-entries expects only one of --commit or --since")
	}

	poFile := ""
	if len(args) == 1 {
		poFile = args[0]
	}

	return util.CmdNewEntries(poFile, v.O.Commit, v.O.Since)
}

var newEntriesCmd = newEntriesCommand{}

func init() {
	rootCmd.AddCommand(newEntriesCmd.Command())
}
