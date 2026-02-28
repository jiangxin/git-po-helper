package cmd

import (
	"io"
	"os"

	"github.com/git-l10n/git-po-helper/util"
	"github.com/spf13/cobra"
)

type msgCatCommand struct {
	cmd *cobra.Command
	O   struct {
		Output string
		JSON   bool
	}
}

func (v *msgCatCommand) Command() *cobra.Command {
	if v.cmd != nil {
		return v.cmd
	}

	v.cmd = &cobra.Command{
		Use:   "msg-cat -o <output> [--json] [inputfile]...",
		Short: "Concatenate and merge PO/POT/JSON files",
		Long: `Merge one or more input files (PO, POT, or gettext JSON) into a single output.
Input files can have extension .po, .pot, or .json; format is auto-detected by content
(starts with '{') or by extension. For duplicate msgid (and msgid_plural for plurals),
the first occurrence by file order is kept.

Write result to the file given by -o; use -o - or omit -o to write to stdout.
Use --json to output gettext JSON; otherwise output is PO format.`,
		SilenceErrors: true,
		RunE: func(cmd *cobra.Command, args []string) error {
			return v.Execute(args)
		},
	}

	v.cmd.Flags().StringVarP(&v.O.Output, "output", "o", "",
		"write output to file (use - for stdout); default is stdout")
	v.cmd.Flags().BoolVar(&v.O.JSON, "json", false, "output JSON instead of PO text")

	return v.cmd
}

func (v msgCatCommand) Execute(args []string) error {
	if len(args) == 0 {
		return newUserError("msg-cat requires at least one input file")
	}

	var w io.Writer = os.Stdout
	if v.O.Output != "" && v.O.Output != "-" {
		f, err := os.Create(v.O.Output)
		if err != nil {
			return newUserErrorF("failed to create output file %s: %v", v.O.Output, err)
		}
		defer f.Close()
		w = f
	}

	sources := make([]*util.GettextJSON, 0, len(args))
	for _, path := range args {
		j, err := util.ReadFileToGettextJSON(path)
		if err != nil {
			return newUserErrorF("%v", err)
		}
		sources = append(sources, j)
	}
	merged := util.MergeGettextJSON(sources)

	if v.O.JSON {
		return util.WriteGettextJSONToJSON(merged, w)
	}
	return util.WriteGettextJSONToPO(merged, w)
}

var msgCatCmd = msgCatCommand{}

func init() {
	rootCmd.AddCommand(msgCatCmd.Command())
}
