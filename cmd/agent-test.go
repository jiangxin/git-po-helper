package cmd

import (
	"github.com/git-l10n/git-po-helper/repository"
	"github.com/git-l10n/git-po-helper/util"
	"github.com/spf13/cobra"
	"github.com/spf13/viper"
)

type agentTestCommand struct {
	cmd *cobra.Command
	O   struct {
		Agent string
		Runs  int
	}
}

func (v *agentTestCommand) Command() *cobra.Command {
	if v.cmd != nil {
		return v.cmd
	}

	v.cmd = &cobra.Command{
		Use:   "agent-test",
		Short: "Test agent commands with multiple runs",
		Long: `Test agent commands with multiple runs and calculate average scores.

This command runs agent operations multiple times to test reliability and
performance. It calculates an average score where success = 100 points and
failure = 0 points.

The number of runs can be specified via --runs flag or configured in
git-po-helper.yaml. If not specified, the default is 5 runs.

Entry count validation can be configured to verify that the agent correctly
updates files with the expected number of entries.`,
		SilenceErrors: true,
	}

	// Add update-pot subcommand
	updatePotCmd := &cobra.Command{
		Use:   "update-pot",
		Short: "Test update-pot operation multiple times and calculate average score",
		Long: `Test the update-pot operation multiple times and calculate an average score.

This command runs agent-run update-pot multiple times (default: 5, configurable
via --runs or config file) and provides detailed results including:
- Individual run results with validation status
- Success/failure counts
- Average score across all runs
- Entry count validation results (if configured)

Validation can be configured in git-po-helper.yaml:
- pot_entries_before_update: Expected entry count before update
- pot_entries_after_update: Expected entry count after update

If validation is configured:
- Pre-validation failure: Run is marked as failed (score = 0), agent is not executed
- Post-validation failure: Run is marked as failed (score = 0) even if agent succeeded
- Both validations pass: Run is marked as successful (score = 100)

If validation is not configured (null or 0), scoring is based on agent exit code:
- Agent succeeds (exit code 0): score = 100
- Agent fails (non-zero exit code): score = 0

Examples:
  # Run 5 tests with default agent
  git-po-helper agent-test update-pot

  # Run 10 tests with a specific agent
  git-po-helper agent-test update-pot --agent claude --runs 10`,
		SilenceErrors: true,
		RunE: func(cmd *cobra.Command, args []string) error {
			// Execute in root of worktree.
			repository.ChdirProjectRoot()

			if len(args) != 0 {
				return newUserError("update-pot command needs no arguments")
			}

			return util.CmdAgentTestUpdatePot(v.O.Agent, v.O.Runs)
		},
	}

	updatePotCmd.Flags().StringVar(&v.O.Agent,
		"agent",
		"",
		"agent name to use (required if multiple agents are configured)")
	updatePotCmd.Flags().IntVar(&v.O.Runs,
		"runs",
		0,
		"number of test runs (0 means use config file value or default to 5)")

	_ = viper.BindPFlag("agent-test--agent", updatePotCmd.Flags().Lookup("agent"))
	_ = viper.BindPFlag("agent-test--runs", updatePotCmd.Flags().Lookup("runs"))

	// Add show-config subcommand
	showConfigCmd := &cobra.Command{
		Use:   "show-config",
		Short: "Show the current agent configuration in YAML format",
		Long: `Display the complete agent configuration in YAML format.

This command loads the configuration from git-po-helper.yaml files
(user home directory and repository root) and displays the merged
configuration in YAML format.

The configuration is read from:
- User home directory: ~/.git-po-helper.yaml (lower priority)
- Repository root: <repo-root>/git-po-helper.yaml (higher priority, overrides user config)

If no configuration files are found, an empty configuration structure
will be displayed.`,
		SilenceErrors: true,
		RunE: func(cmd *cobra.Command, args []string) error {
			// Execute in root of worktree.
			repository.ChdirProjectRoot()

			if len(args) != 0 {
				return newUserError("show-config command needs no arguments")
			}

			// Require user confirmation before proceeding
			if err := util.ConfirmAgentTestExecution(); err != nil {
				return err
			}

			return util.CmdAgentRunShowConfig()
		},
	}

	v.cmd.AddCommand(updatePotCmd)
	v.cmd.AddCommand(showConfigCmd)

	return v.cmd
}

var agentTestCmd = agentTestCommand{}

func init() {
	rootCmd.AddCommand(agentTestCmd.Command())
}
