package cmd

import (
	"os"

	"github.com/git-l10n/git-po-helper/util"
	"github.com/spf13/cobra"
)

type msgSelectCommand struct {
	cmd *cobra.Command
	O   struct {
		Range    string
		NoHeader bool
	}
}

func (v *msgSelectCommand) Command() *cobra.Command {
	if v.cmd != nil {
		return v.cmd
	}

	v.cmd = &cobra.Command{
		Use:   "msg-select <po-file>",
		Short: "Extract entries from PO/POT file by index range",
		Long: `Extract entries from a PO or POT file by entry number range and write to stdout.

Entry 0 is the header entry; it is included when content entries are selected
(use --no-header to omit). Entry numbers 1, 2, 3, ... refer to the first,
second, third content entries. If no content entries match the range,
output is empty.

Range format (--range): comma-separated numbers or ranges, e.g. "3,5,9-13"
  - Single numbers: 3, 5 (extract entries 3 and 5)
  - Ranges: 9-13 (extract entries 9 through 13 inclusive)
  - -N: entries 1 through N (e.g. -5 for first 5 entries)
  - N-: entries N through last (e.g. 50- for entries 50 to end)
  - Combined: 3,5,9-13 (extract entries 3, 5, 9, 10, 11, 12, 13)

Examples:
  git-po-helper msg-select --range "1-10" po/zh_CN.po
  git-po-helper msg-select --range "-5" po/zh_CN.po
  git-po-helper msg-select --range "50-" po/git.pot`,
		SilenceErrors: true,
		RunE: func(cmd *cobra.Command, args []string) error {
			return v.Execute(args)
		},
	}

	v.cmd.Flags().StringVar(&v.O.Range, "range", "", "entry range to extract (e.g. 3,5,9-13)")
	v.cmd.Flags().BoolVar(&v.O.NoHeader, "no-header", false, "omit header entry from output")
	_ = v.cmd.MarkFlagRequired("range")

	return v.cmd
}

func (v msgSelectCommand) Execute(args []string) error {
	if len(args) != 1 {
		return newUserError("msg-select requires exactly one argument: <po-file>")
	}

	poFile := args[0]
	return util.MsgSelect(poFile, v.O.Range, os.Stdout, v.O.NoHeader)
}

var msgSelectCmd = msgSelectCommand{}

func init() {
	rootCmd.AddCommand(msgSelectCmd.Command())
}
