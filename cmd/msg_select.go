package cmd

import (
	"bytes"
	"io"
	"os"

	"github.com/git-l10n/git-po-helper/util"
	"github.com/spf13/cobra"
)

type msgSelectCommand struct {
	cmd *cobra.Command
	O   struct {
		Range    string
		NoHeader bool
		Output   string
		JSON     bool
	}
}

func (v *msgSelectCommand) Command() *cobra.Command {
	if v.cmd != nil {
		return v.cmd
	}

	v.cmd = &cobra.Command{
		Use:   "msg-select <po-file>",
		Short: "Extract entries from PO/POT file by index range",
		Long: `Extract entries from a PO/POT file or a gettext JSON file by entry number range.
Input can be either a PO/POT file or a JSON file (same schema as produced by --json).
Use -o <file> to write to a file (avoids stderr mixing when redirecting stdout).
Use --json to output a single JSON object (header_comment, header_meta, entries) instead of PO text.
See docs/design/msg-select-json-output.md for the gettext JSON schema (GettextJSON/GettextEntry in util/gettext_json.go).

Entry 0 is the header entry; it is included when content entries are selected
(use --no-header to omit; for JSON output the file header is always included).
Entry numbers 1, 2, 3, ... refer to the first, second, third content entries.
If no content entries match the range, PO output is empty; JSON output has entries: [].

Range format (--range): comma-separated numbers or ranges, e.g. "3,5,9-13".
Omit --range to select all entries.
  - Single numbers: 3, 5 (extract entries 3 and 5)
  - Ranges: 9-13 (extract entries 9 through 13 inclusive)
  - -N: entries 1 through N (e.g. -5 for first 5 entries)
  - N-: entries N through last (e.g. 50- for entries 50 to end)
  - Combined: 3,5,9-13 (extract entries 3, 5, 9, 10, 11, 12, 13)

Examples:
  git-po-helper msg-select --range "1-10" po/zh_CN.po
  git-po-helper msg-select --range "1" --json po/zh_CN.po
  git-po-helper msg-select --range "-5" -o po/review-batch.po po/review.po
  git-po-helper msg-select --range "50-" po/git.pot`,
		SilenceErrors: true,
		RunE: func(cmd *cobra.Command, args []string) error {
			return v.Execute(args)
		},
	}

	v.cmd.Flags().StringVar(&v.O.Range, "range", "", "entry range to extract (e.g. 3,5,9-13); omit to select all")
	v.cmd.Flags().BoolVar(&v.O.NoHeader, "no-header", false, "omit header entry from output")
	v.cmd.Flags().BoolVar(&v.O.JSON, "json", false, "output JSON instead of PO text")
	v.cmd.Flags().StringVarP(&v.O.Output, "output", "o", "",
		"write output to file (use - for stdout); empty output overwrites file")

	return v.cmd
}

func (v msgSelectCommand) Execute(args []string) error {
	if len(args) != 1 {
		return newUserError("msg-select requires exactly one argument: <po-file>")
	}

	poFile := args[0]
	var w io.Writer = os.Stdout
	if v.O.Output != "" && v.O.Output != "-" {
		f, err := os.Create(v.O.Output)
		if err != nil {
			return newUserErrorF("failed to create output file %s: %v", v.O.Output, err)
		}
		defer f.Close()
		w = f
	}
	// Detect input format: JSON if file starts with '{' after whitespace
	peek, err := os.ReadFile(poFile)
	if err != nil {
		return newUserErrorF("failed to read %s: %v", poFile, err)
	}
	if len(peek) > 512 {
		peek = peek[:512]
	}
	trimmed := bytes.TrimLeft(peek, " \t\r\n")
	if len(trimmed) > 0 && trimmed[0] == '{' {
		return util.SelectGettextJSONFromFile(poFile, v.O.Range, w, v.O.JSON)
	}
	if v.O.JSON {
		return util.WriteGettextJSONFromPOFile(poFile, v.O.Range, w)
	}
	return util.MsgSelect(poFile, v.O.Range, w, v.O.NoHeader)
}

var msgSelectCmd = msgSelectCommand{}

func init() {
	rootCmd.AddCommand(msgSelectCmd.Command())
}
