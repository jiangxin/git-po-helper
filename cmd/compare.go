package cmd

import (
	"github.com/git-l10n/git-po-helper/util"
	log "github.com/sirupsen/logrus"
	"github.com/spf13/cobra"
	"github.com/spf13/viper"
)

type compareCommand struct {
	cmd *cobra.Command
	O   struct {
		Range  string
		Commit string
		Since  string
		Stat   bool
	}
}

func (v *compareCommand) Command() *cobra.Command {
	if v.cmd != nil {
		return v.cmd
	}

	v.cmd = &cobra.Command{
		Use:   "compare [-r revision | --commit <commit> | --since <commit>] [[<src>] <target>]",
		Short: "Show changes between two l10n files",
		Long: `By default: output new or changed entries to stdout.
With --stat: show diff statistics between two l10n file versions.

If no po/XX.po argument is given, the PO file is selected from changed files
(interactive when multiple, auto when single).

Modes:
- --commit <commit>: compare parent of commit with the specified commit
- --since <commit>: compare since commit with current working tree
- no --commit/--since: compare HEAD with current working tree (local changes)

Exactly one of --range, --commit and --since may be specified.
Output is empty when there are no new or changed entries.`,
		SilenceErrors: true,
		RunE: func(cmd *cobra.Command, args []string) error {
			return v.Execute(args)
		},
	}
	v.cmd.Flags().BoolVar(&v.O.Stat, "stat", false, "show diff statistics (default: output new or changed entries)")
	v.cmd.Flags().StringVarP(&v.O.Range, "range", "r", "",
		"revision range: a..b (a and b), a.. (a and working tree), or a (a~ and a)")
	v.cmd.Flags().StringVar(&v.O.Commit, "commit", "",
		"equivalent to -r <commit>^..<commit>")
	v.cmd.Flags().StringVar(&v.O.Since, "since", "",
		"equivalent to -r <commit>.. (compare commit with working tree)")

	_ = viper.BindPFlag("compare--range", v.cmd.Flags().Lookup("range"))
	_ = viper.BindPFlag("compare--commit", v.cmd.Flags().Lookup("commit"))
	_ = viper.BindPFlag("compare--since", v.cmd.Flags().Lookup("since"))

	return v.cmd
}

func (v compareCommand) Execute(args []string) error {
	target, err := util.ResolveRevisionsAndFiles(v.O.Range, v.O.Commit, v.O.Since, args)
	if err != nil {
		return newUserErrorF("%v", err)
	}

	if v.O.Stat {
		return v.executeStat(target.OldCommit, target.OldFile, target.NewCommit, target.NewFile)
	}
	return v.executeNew(target.OldCommit, target.OldFile, target.NewCommit, target.NewFile)
}

func (v compareCommand) executeNew(oldCommit, oldFile, newCommit, newFile string) error {
	log.Debugf("outputting new entries from '%s:%s' to '%s:%s'",
		oldCommit, oldFile, newCommit, newFile)
	err := util.PrepareReviewData(oldCommit, oldFile, newCommit, newFile, "-")
	if err != nil {
		return newUserErrorF("failed to prepare review data: %v", err)
	}
	return nil
}

func (v compareCommand) executeStat(oldCommit, oldFile, newCommit, newFile string) error {
	var (
		oldFileRevision, newFileRevision util.FileRevision
	)

	oldFileRevision.Revision = oldCommit
	oldFileRevision.File = oldFile
	newFileRevision.Revision = newCommit
	newFileRevision.File = newFile

	if !util.PoFileRevisionDiffStat(oldFileRevision, newFileRevision) {
		return errExecute
	}
	return nil
}

var compareCmd = compareCommand{}

func init() {
	rootCmd.AddCommand(compareCmd.Command())
}
