package cmd

import (
	"fmt"

	"github.com/git-l10n/git-po-helper/repository"
	"github.com/git-l10n/git-po-helper/util"
	"github.com/spf13/cobra"
)

func newAgentRunReportCmd() *cobra.Command {
	const defaultPath = "po/review.po"
	return &cobra.Command{
		Use:   "report [path]",
		Short: "Report aggregated review statistics from batch or single JSON",
		Long: `Report review statistics for agent-run review output.

If path is given (e.g. po/review.po), derives po/review.json and po/review.po.
If any files match po/review-batch-*.json, they are loaded and aggregated
into one result; otherwise po/review.json is used.

Default path is ` + defaultPath + ` when omitted.`,
		SilenceErrors: true,
		RunE: func(cmd *cobra.Command, args []string) error {
			repository.ChdirProjectRoot()

			path := defaultPath
			if len(args) > 0 {
				path = args[0]
			}
			result, err := util.ReportReviewFromPathWithBatches(path)
			if err != nil {
				return newUserErrorF("%v", err)
			}

			jsonFile, _ := util.DeriveReviewPaths(path)
			fmt.Printf("Review JSON: %s\n", jsonFile)
			fmt.Printf("  Total entries: %d\n", result.Review.TotalEntries)
			fmt.Printf("  Issues found: %d\n", len(result.Review.Issues))
			fmt.Printf("  Review score: %d/100\n", result.Score)
			fmt.Printf("  Critical (score 0): %d\n", result.CriticalCount)
			fmt.Printf("  Major (score 2):   %d\n", result.MajorCount)
			fmt.Printf("  Minor (score 1):   %d\n", result.MinorCount)
			fmt.Printf("  Perfect (no issue): %d\n", result.PerfectCount())
			return nil
		},
	}
}
