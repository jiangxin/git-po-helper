package cmd

import (
	"github.com/git-l10n/git-po-helper/repository"
	"github.com/git-l10n/git-po-helper/util"
	"github.com/spf13/cobra"
	"github.com/spf13/viper"
)

type agentRunCommand struct {
	cmd *cobra.Command
	O   struct {
		Agent string
	}
}

func (v *agentRunCommand) Command() *cobra.Command {
	if v.cmd != nil {
		return v.cmd
	}

	v.cmd = &cobra.Command{
		Use:           "agent-run",
		Short:         "Run agent commands for automation",
		SilenceErrors: true,
	}

	// Add update-pot subcommand
	updatePotCmd := &cobra.Command{
		Use:           "update-pot",
		Short:         "Update po/git.pot using an agent",
		SilenceErrors: true,
		RunE: func(cmd *cobra.Command, args []string) error {
			// Execute in root of worktree.
			repository.ChdirProjectRoot()

			if len(args) != 0 {
				return newUserError("update-pot command needs no arguments")
			}

			return util.CmdAgentRunUpdatePot(v.O.Agent)
		},
	}

	updatePotCmd.Flags().StringVar(&v.O.Agent,
		"agent",
		"",
		"agent name to use (required if multiple agents are configured)")

	_ = viper.BindPFlag("agent-run--agent", updatePotCmd.Flags().Lookup("agent"))

	v.cmd.AddCommand(updatePotCmd)

	return v.cmd
}

var agentRunCmd = agentRunCommand{}

func init() {
	rootCmd.AddCommand(agentRunCmd.Command())
}
