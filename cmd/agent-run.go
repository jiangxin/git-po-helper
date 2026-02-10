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

	// Add update-po subcommand
	updatePoCmd := &cobra.Command{
		Use:   "update-po [po/XX.po]",
		Short: "Update a po/XX.po file using an agent",
		Long: `Update a specific po/XX.po file using a configured agent.

This command uses an agent with a configured prompt to update the target
PO file according to po/README.md. The agent command and prompt are
specified in the git-po-helper.yaml configuration file.

If only one agent is configured, the --agent flag is optional. If multiple
agents are configured, you must specify which agent to use with --agent.

If no po/XX.po argument is given, the PO file is derived from
default_lang_code in configuration (e.g., po/zh_CN.po).

The command performs validation checks if configured:
- Pre-validation: checks entry count before update (if po_entries_before_update is set)
- Post-validation: checks entry count after update (if po_entries_after_update is set)
- Syntax validation: validates the PO file using msgfmt

Examples:
  # Use default_lang_code to locate PO file
  git-po-helper agent-run update-po

  # Explicitly specify the PO file
  git-po-helper agent-run update-po po/zh_CN.po

  # Use a specific agent
  git-po-helper agent-run update-po --agent claude po/zh_CN.po`,
		SilenceErrors: true,
		RunE: func(cmd *cobra.Command, args []string) error {
			// Execute in root of worktree.
			repository.ChdirProjectRoot()

			if len(args) > 1 {
				return newUserError("update-po command expects at most one argument: po/XX.po")
			}

			poFile := ""
			if len(args) == 1 {
				poFile = args[0]
			}

			return util.CmdAgentRunUpdatePo(v.O.Agent, poFile)
		},
	}

	updatePoCmd.Flags().StringVar(&v.O.Agent,
		"agent",
		"",
		"agent name to use (required if multiple agents are configured)")

	_ = viper.BindPFlag("agent-run--agent", updatePoCmd.Flags().Lookup("agent"))

	// Add translate subcommand
	translateCmd := &cobra.Command{
		Use:   "translate [po/XX.po]",
		Short: "Translate new and fuzzy entries in a po/XX.po file using an agent",
		Long: `Translate new strings and fix fuzzy translations in a PO file using a configured agent.

This command uses an agent with a configured prompt to translate all untranslated
entries (new strings) and resolve all fuzzy entries in the target PO file.
The agent command and prompt are specified in the git-po-helper.yaml configuration file.

If only one agent is configured, the --agent flag is optional. If multiple
agents are configured, you must specify which agent to use with --agent.

If no po/XX.po argument is given, the PO file is derived from
default_lang_code in configuration (e.g., po/zh_CN.po).

The command performs validation checks:
- Pre-validation: counts new (untranslated) and fuzzy entries before translation
- Post-validation: verifies all new and fuzzy entries are resolved (count must be 0)
- Syntax validation: validates the PO file using msgfmt

The operation is considered successful only if both new entry count and
fuzzy entry count are 0 after translation.

Examples:
  # Use default_lang_code to locate PO file
  git-po-helper agent-run translate

  # Explicitly specify the PO file
  git-po-helper agent-run translate po/zh_CN.po

  # Use a specific agent
  git-po-helper agent-run translate --agent claude po/zh_CN.po`,
		SilenceErrors: true,
		RunE: func(cmd *cobra.Command, args []string) error {
			// Execute in root of worktree.
			repository.ChdirProjectRoot()

			if len(args) > 1 {
				return newUserError("translate command expects at most one argument: po/XX.po")
			}

			poFile := ""
			if len(args) == 1 {
				poFile = args[0]
			}

			return util.CmdAgentRunTranslate(v.O.Agent, poFile)
		},
	}

	translateCmd.Flags().StringVar(&v.O.Agent,
		"agent",
		"",
		"agent name to use (required if multiple agents are configured)")

	_ = viper.BindPFlag("agent-run--agent", translateCmd.Flags().Lookup("agent"))

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

			return util.CmdAgentRunShowConfig()
		},
	}

	v.cmd.AddCommand(updatePotCmd)
	v.cmd.AddCommand(updatePoCmd)
	v.cmd.AddCommand(translateCmd)
	v.cmd.AddCommand(showConfigCmd)

	return v.cmd
}

var agentRunCmd = agentRunCommand{}

func init() {
	rootCmd.AddCommand(agentRunCmd.Command())
}
