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
		Use:   "agent-run",
		Short: "Run agent commands for automation",
		Long: `Run agent commands for automating localization tasks.

This command uses configured code agents (like Claude, Gemini, etc.) to
automate various localization operations. The agent configuration is
read from git-po-helper.yaml in the repository root or user home directory.`,
		SilenceErrors: true,
	}

	// Add update-pot subcommand
	updatePotCmd := &cobra.Command{
		Use:   "update-pot",
		Short: "Update po/git.pot using an agent",
		Long: `Update the po/git.pot template file using a configured agent.

This command uses an agent with a configured prompt to update the po/git.pot
file according to po/README.md. The agent command is specified in the
git-po-helper.yaml configuration file.

If only one agent is configured, the --agent flag is optional. If multiple
agents are configured, you must specify which agent to use with --agent.

The command performs validation checks if configured:
- Pre-validation: checks entry count before update (if pot_entries_before_update is set)
- Post-validation: checks entry count after update (if pot_entries_after_update is set)
- Syntax validation: validates the POT file using msgfmt

Examples:
  # Use the default agent (if only one is configured)
  git-po-helper agent-run update-pot

  # Use a specific agent
  git-po-helper agent-run update-pot --agent claude`,
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
