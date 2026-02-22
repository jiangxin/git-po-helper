package cmd

import (
	"strings"

	"github.com/git-l10n/git-po-helper/repository"
	"github.com/git-l10n/git-po-helper/util"
	"github.com/spf13/cobra"
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
		Use:           "compare --stat [-r revision | --commit <commit> | --since <commit>] [[<src>] <target>]",
		Short:         "Show changes between two l10n files",
		SilenceErrors: true,
		RunE: func(cmd *cobra.Command, args []string) error {
			return v.Execute(args)
		},
	}
	v.cmd.Flags().BoolVar(&v.O.Stat, "stat", false, "show diff statistics")
	v.cmd.Flags().StringVarP(&v.O.Range, "range", "r", "",
		"revision range: a..b (a and b), a.. (a and working tree), or a (a~ and a)")
	v.cmd.Flags().StringVar(&v.O.Commit, "commit", "",
		"equivalent to -r <commit>^..<commit>")
	v.cmd.Flags().StringVar(&v.O.Since, "since", "",
		"equivalent to -r <commit>.. (compare commit with working tree)")

	return v.cmd
}

func (v compareCommand) Execute(args []string) error {
	var (
		src, dest util.FileRevision
	)

	if !v.O.Stat {
		return newUserError("compare command requires \"--stat\" parameter")
	}

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

	if len(args) > 2 {
		return newUserErrorF("too many arguments (%d > 2)", len(args))
	}

	repository.ChdirProjectRoot()

	// Resolve range: --commit X => X^..X, --since X => X.., else use --range
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
			revRange = "HEAD~..HEAD"
		case 1:
			// Compare HEAD version with worktree
			revRange = "HEAD~.."
		case 2:
			// Compare two files in worktree
			revRange = ""
		}
	}

	// Parse revision: "a..b", "a..", or "a"
	if strings.Contains(revRange, "..") {
		parts := strings.SplitN(revRange, "..", 2)
		src.Revision = strings.TrimSpace(parts[0])
		right := strings.TrimSpace(parts[1])
		if right == "" {
			dest.Revision = "" // working tree
		} else {
			dest.Revision = right
		}
	} else {
		// a : first is a~, second is a
		src.Revision = revRange + "^"
		dest.Revision = revRange
	}

	// Set File
	switch len(args) {
	case 0:
		// Automatically or manually select PO file from changed files
	case 1:
		src.File = args[0]
		dest.File = args[0]
	case 2:
		src.File = args[0]
		dest.File = args[1]
		if src.Revision != "" || dest.Revision != "" {
			return newUserErrorF("cannot specify revision for multiple files: %s and %s", src.File, dest.File)
		}
	}

	if !util.PoFileRevisionDiffStat(src, dest) {
		return errExecute
	}
	return nil
}

var compareCmd = compareCommand{}

func init() {
	rootCmd.AddCommand(compareCmd.Command())
}
