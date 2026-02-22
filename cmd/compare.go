package cmd

import (
	"strings"

	"github.com/git-l10n/git-po-helper/repository"
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
	// --range, --commit, --since are mutually exclusive
	nSet := 0
	if strings.TrimSpace(v.O.Range) != "" {
		nSet++
	}
	if strings.TrimSpace(v.O.Commit) != "" {
		nSet++
	}
	if strings.TrimSpace(v.O.Since) != "" {
		nSet++
	}
	if nSet > 1 {
		return newUserError("only one of --range, --commit, or --since may be specified")
	}

	// Resolve range for both modes
	var revRange string
	if c := strings.TrimSpace(v.O.Commit); c != "" {
		revRange = c + "^.." + c
	} else if s := strings.TrimSpace(v.O.Since); s != "" {
		revRange = s + ".."
	} else {
		revRange = strings.TrimSpace(v.O.Range)
	}
	if revRange == "" {
		switch len(args) {
		case 0:
			revRange = "HEAD.."
		case 1:
			revRange = "HEAD.."
		case 2:
			// Compare two files in worktree
		}
	}

	if len(args) > 2 {
		return newUserErrorF("too many arguments (%d > 2)", len(args))
	}

	repository.ChdirProjectRoot()

	var (
		oldCommit, newCommit string
		oldFile, newFile     string
	)
	// Parse revision: "a..b", "a..", or "a"
	if strings.Contains(revRange, "..") {
		parts := strings.SplitN(revRange, "..", 2)
		oldCommit = strings.TrimSpace(parts[0])
		newCommit = strings.TrimSpace(parts[1])
	} else if revRange != "" {
		// a : first is a~, second is a
		oldCommit = revRange + "~"
		newCommit = revRange
	}

	// Set File
	switch len(args) {
	case 0:
		// Automatically or manually select PO file from changed files
	case 1:
		oldFile = args[0]
		newFile = args[0]
	case 2:
		oldFile = args[0]
		newFile = args[1]
		if oldCommit != "" || newCommit != "" {
			return newUserErrorF("cannot specify revision for multiple files: %s and %s",
				oldFile, newFile)
		}
	}

	// Resolve poFile when not specified
	if len(args) == 0 {
		changedPoFiles, err := util.GetChangedPoFilesRange(oldCommit, newCommit)
		if err != nil {
			return newUserErrorF("failed to get changed po files: %v", err)
		}

		oldFile, err = util.ResolvePoFile(oldFile, changedPoFiles)
		if err != nil {
			return newUserErrorF("failed to resolve default po file: %v", err)
		}
		newFile = oldFile
	}

	if v.O.Stat {
		return v.executeStat(oldCommit, oldFile, newCommit, newFile)
	}
	return v.executeNew(oldCommit, oldFile, newCommit, newFile)
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
