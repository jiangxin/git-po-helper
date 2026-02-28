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
		Output       string
		JSON         bool
		Translated   bool
		Untranslated bool
		Fuzzy        bool
		WithObsolete bool
		NoObsolete   bool
		OnlySame     bool
		OnlyObsolete bool
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

By default, all entries are selected (translated, same, untranslated, fuzzy, obsolete).
Use --translated, --untranslated, --fuzzy to filter by state (OR relationship).
Use --no-obsolete to exclude obsolete; --only-same or --only-obsolete for a single state.

Write result to the file given by -o; use -o - or omit -o to write to stdout.
Use --json to output gettext JSON; otherwise output is PO format.`,
		SilenceErrors: true,
		RunE: func(cmd *cobra.Command, args []string) error {
			return v.Execute(args)
		},
	}

	fs := v.cmd.Flags()
	fs.SortFlags = false

	// General options
	fs.StringVarP(&v.O.Output, "output", "o", "",
		"write output to file (use - for stdout); default is stdout")
	fs.BoolVar(&v.O.JSON, "json", false, "output JSON instead of PO text")
	fs.SetAnnotation("output", "group", []string{"General options"})
	fs.SetAnnotation("json", "group", []string{"General options"})

	// State filter: translated, untranslated, fuzzy (OR when combined)
	fs.BoolVar(&v.O.Translated, "translated", false, "select translated entries")
	fs.BoolVar(&v.O.Untranslated, "untranslated", false, "select untranslated entries")
	fs.BoolVar(&v.O.Fuzzy, "fuzzy", false, "select fuzzy entries")
	fs.SetAnnotation("translated", "group", []string{"State filter"})
	fs.SetAnnotation("untranslated", "group", []string{"State filter"})
	fs.SetAnnotation("fuzzy", "group", []string{"State filter"})

	// Obsolete handling: include or exclude
	fs.BoolVar(&v.O.WithObsolete, "with-obsolete", false, "include obsolete entries (default)")
	fs.BoolVar(&v.O.NoObsolete, "no-obsolete", false, "exclude obsolete entries")
	fs.SetAnnotation("with-obsolete", "group", []string{"Obsolete handling"})
	fs.SetAnnotation("no-obsolete", "group", []string{"Obsolete handling"})

	// Single-state filter: mutually exclusive with state filter above
	fs.BoolVar(&v.O.OnlySame, "only-same", false, "only entries where msgstr equals msgid")
	fs.BoolVar(&v.O.OnlyObsolete, "only-obsolete", false, "only obsolete entries")
	fs.SetAnnotation("only-same", "group", []string{"Single-state filter"})
	fs.SetAnnotation("only-obsolete", "group", []string{"Single-state filter"})

	// Custom usage template with grouped flags
	v.cmd.SetUsageTemplate(`Usage:{{if .Runnable}}
  {{.UseLine}}{{end}}{{if .HasAvailableSubCommands}}
  {{.CommandPath}} [command]{{end}}{{if gt (len .Aliases) 0}}

Aliases:
  {{.NameAndAliases}}{{end}}{{if .HasExample}}

Examples:
{{.Example}}{{end}}{{if .HasAvailableSubCommands}}

Available Commands:{{range .Commands}}{{if (or .IsAvailableCommand (eq .Name "help"))}}
  {{rpad .Name .NamePadding }} {{.Short}}{{end}}{{end}}{{end}}{{if .HasAvailableLocalFlags}}

Flags:
{{flagUsagesByGroup . | trimTrailingWhitespaces}}{{end}}{{if .HasAvailableInheritedFlags}}

Global Flags:
{{.InheritedFlags.FlagUsages | trimTrailingWhitespaces}}{{end}}{{if .HasHelpSubCommands}}

Additional help topics:{{range .Commands}}{{if .IsAdditionalHelpTopicCommand}}
  {{rpad .CommandPath .CommandPathPadding}} {{.Short}}{{end}}{{end}}{{end}}{{if .HasAvailableSubCommands}}

Use "{{.CommandPath}} [command] --help" for more information about a command.{{end}}
`)

	return v.cmd
}

func (v msgCatCommand) Execute(args []string) error {
	if len(args) == 0 {
		return newUserError("msg-cat requires at least one input file")
	}
	filter, err := v.buildFilter()
	if err != nil {
		return err
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

	// Apply state filter
	if filter != nil {
		merged.Entries = util.FilterGettextEntries(merged.Entries, *filter)
	}

	if v.O.JSON {
		return util.WriteGettextJSONToJSON(merged, w)
	}
	return util.WriteGettextJSONToPO(merged, w)
}

func (v msgCatCommand) buildFilter() (*util.EntryStateFilter, error) {
	if v.O.OnlySame && v.O.OnlyObsolete {
		return nil, newUserError("--only-same and --only-obsolete are mutually exclusive")
	}
	if v.O.OnlySame && (v.O.Translated || v.O.Untranslated || v.O.Fuzzy) {
		return nil, newUserError("--only-same is mutually exclusive with --translated, --untranslated, --fuzzy")
	}
	if v.O.OnlyObsolete && (v.O.Translated || v.O.Untranslated || v.O.Fuzzy) {
		return nil, newUserError("--only-obsolete is mutually exclusive with --translated, --untranslated, --fuzzy")
	}
	f := util.EntryStateFilter{
		Translated:   v.O.Translated,
		Untranslated: v.O.Untranslated,
		Fuzzy:        v.O.Fuzzy,
		WithObsolete: !v.O.NoObsolete,
		NoObsolete:   v.O.NoObsolete,
		OnlySame:     v.O.OnlySame,
		OnlyObsolete: v.O.OnlyObsolete,
	}
	return &f, nil
}

var msgCatCmd = msgCatCommand{}

func init() {
	rootCmd.AddCommand(msgCatCmd.Command())
}
