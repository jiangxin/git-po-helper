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
		Use:           "agent-test",
		Short:         "Test agent commands with multiple runs",
		SilenceErrors: true,
	}

	// Add update-pot subcommand
	updatePotCmd := &cobra.Command{
		Use:           "update-pot",
		Short:         "Test update-pot operation multiple times and calculate average score",
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

	v.cmd.AddCommand(updatePotCmd)

	return v.cmd
}

var agentTestCmd = agentTestCommand{}

func init() {
	rootCmd.AddCommand(agentTestCmd.Command())
}
