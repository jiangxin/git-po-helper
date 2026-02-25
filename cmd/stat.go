package cmd

import (
	"encoding/json"
	"fmt"
	"os"
	"strings"

	"github.com/git-l10n/git-po-helper/flag"
	"github.com/git-l10n/git-po-helper/repository"
	"github.com/git-l10n/git-po-helper/util"
	"github.com/spf13/cobra"
)

type statCommand struct {
	cmd *cobra.Command
	O   struct {
		Review string
	}
}

func (v *statCommand) Command() *cobra.Command {
	if v.cmd != nil {
		return v.cmd
	}

	v.cmd = &cobra.Command{
		Use:   "stat [po-file]",
		Short: "Report statistics for a PO file or review JSON",
		Long: `Report entry statistics for a PO file:
  translated   - entries with non-empty translation
  untranslated - entries with empty msgstr
  same         - entries where msgstr equals msgid (suspect untranslated)
  fuzzy        - entries with fuzzy flag
  obsolete     - obsolete entries (#~ format)

With --review <json-file>: report review results from agent-run review JSON.
If the JSON has no total_entries, po-file is required to count entries.`,
		SilenceErrors: true,
		RunE: func(cmd *cobra.Command, args []string) error {
			return v.Execute(args)
		},
	}

	v.cmd.Flags().StringVar(&v.O.Review, "review", "", "report from review JSON file (agent-run review output)")

	return v.cmd
}

func (v statCommand) Execute(args []string) error {
	repository.ChdirProjectRoot()

	if v.O.Review != "" {
		return v.executeReviewReport(args)
	}

	if len(args) != 1 {
		return newUserError("stat requires exactly one argument: <po-file>")
	}

	poFile := args[0]
	if !util.Exist(poFile) {
		return newUserError("file does not exist:", poFile)
	}

	stats, err := util.CountPoReportStats(poFile)
	if err != nil {
		return err
	}

	if flag.Verbose() > 0 {
		title := fmt.Sprintf("PO file: %s", poFile)
		fmt.Println(title)
		fmt.Println(strings.Repeat("-", len(title)))
		fmt.Printf("  translated:   %d\n", stats.Translated)
		fmt.Printf("  untranslated: %d\n", stats.Untranslated)
		fmt.Printf("  same:         %d\n", stats.Same)
		fmt.Printf("  fuzzy:        %d\n", stats.Fuzzy)
		fmt.Printf("  obsolete:     %d\n", stats.Obsolete)
	} else {
		fmt.Print(util.FormatStatLine(stats))
	}

	return nil
}

func (v statCommand) executeReviewReport(args []string) error {
	data, err := os.ReadFile(v.O.Review)
	if err != nil {
		return fmt.Errorf("failed to read review JSON %s: %w", v.O.Review, err)
	}

	var review util.ReviewJSONResult
	if err := json.Unmarshal(data, &review); err != nil {
		return fmt.Errorf("failed to parse review JSON: %w", err)
	}

	if review.TotalEntries <= 0 {
		if len(args) != 1 {
			return newUserError("review JSON has no total_entries; provide <po-file> to count entries")
		}
		poFile := args[0]
		if !util.Exist(poFile) {
			return newUserError("file does not exist:", poFile)
		}
		count, err := util.CountPoEntries(poFile)
		if err != nil {
			return fmt.Errorf("failed to count entries in %s: %w", poFile, err)
		}
		review.TotalEntries = count
	}

	score, err := util.CalculateReviewScore(&review)
	if err != nil {
		return fmt.Errorf("failed to calculate review score: %w", err)
	}

	criticalCount := 0
	minorCount := 0
	for _, issue := range review.Issues {
		switch issue.Score {
		case 0:
			criticalCount++
		case 2:
			minorCount++
		}
	}

	fmt.Printf("Review JSON: %s\n", v.O.Review)
	fmt.Printf("  Total entries: %d\n", review.TotalEntries)
	fmt.Printf("  Issues found: %d\n", len(review.Issues))
	fmt.Printf("  Review score: %d/100\n", score)
	if len(review.Issues) > 0 {
		fmt.Printf("  Critical (score 0): %d\n", criticalCount)
		fmt.Printf("  Minor (score 2):   %d\n", minorCount)
	}

	return nil
}

var statCmd = statCommand{}

func init() {
	rootCmd.AddCommand(statCmd.Command())
}
