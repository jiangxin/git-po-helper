// Package config provides configuration structures and loading for agent commands.
package config

// AgentConfig holds the complete agent configuration.
type AgentConfig struct {
	DefaultLangCode string            `yaml:"default_lang_code"`
	Prompt          PromptConfig      `yaml:"prompt"`
	AgentTest       AgentTestConfig   `yaml:"agent-test"`
	Agents          map[string]Agent  `yaml:"agents"`
}

// PromptConfig holds prompt templates for different operations.
type PromptConfig struct {
	UpdatePot    string `yaml:"update_pot"`
	UpdatePo     string `yaml:"update_po"`
	Translate    string `yaml:"translate"`
	ReviewSince  string `yaml:"review_since"`
	ReviewCommit string `yaml:"review_commit"`
}

// AgentTestConfig holds configuration for agent-test command.
type AgentTestConfig struct {
	Runs                       *int `yaml:"runs"`
	PotEntriesBeforeUpdate     *int `yaml:"pot_entries_before_update"`
	PotEntriesAfterUpdate      *int `yaml:"pot_entries_after_update"`
	PoEntriesBeforeUpdate      *int `yaml:"po_entries_before_update"`
	PoEntriesAfterUpdate       *int `yaml:"po_entries_after_update"`
	PoNewEntriesAfterUpdate    *int `yaml:"po_new_entries_after_update"`
	PoFuzzyEntriesAfterUpdate  *int `yaml:"po_fuzzy_entries_after_update"`
}

// Agent holds configuration for a single agent.
type Agent struct {
	Cmd []string `yaml:"cmd"`
}
