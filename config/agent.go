// Package config provides configuration structures and loading for agent commands.
package config

import (
	"fmt"
	"os"
	"path/filepath"

	"gopkg.in/yaml.v3"

	"github.com/git-l10n/git-po-helper/repository"
	log "github.com/sirupsen/logrus"
)

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

// LoadAgentConfig loads agent configuration from multiple locations with priority:
// 1. User home directory: ~/.git-po-helper.yaml (lower priority)
// 2. Repository root: <repo-root>/git-po-helper.yaml (higher priority, overrides user config)
// Returns the configuration and an error. If both config files are missing, it returns
// a default empty config with a warning (not an error).
func LoadAgentConfig() (*AgentConfig, error) {
	var baseConfig, repoConfig AgentConfig
	configsLoaded := false

	// Load user home directory config first (lower priority)
	homeDir, err := os.UserHomeDir()
	if err == nil {
		userConfigPath := filepath.Join(homeDir, ".git-po-helper.yaml")
		if _, err := os.Stat(userConfigPath); err == nil {
			config, err := loadConfigFromFile(userConfigPath)
			if err != nil {
				return nil, fmt.Errorf("failed to load user config from %s: %w", userConfigPath, err)
			}
			baseConfig = *config
			configsLoaded = true
			log.Debugf("loaded user config from %s", userConfigPath)
		}
	}

	// Load repository root config (higher priority, overrides user config)
	workDir := repository.WorkDir()
	repoConfigPath := filepath.Join(workDir, "git-po-helper.yaml")
	if _, err := os.Stat(repoConfigPath); err == nil {
		config, err := loadConfigFromFile(repoConfigPath)
		if err != nil {
			return nil, fmt.Errorf("failed to load repo config from %s: %w", repoConfigPath, err)
		}
		repoConfig = *config
		configsLoaded = true
		log.Debugf("loaded repo config from %s", repoConfigPath)
	}

	// If no config files were found, return default config
	if !configsLoaded {
		userConfigPath := ""
		if homeDir != "" {
			userConfigPath = filepath.Join(homeDir, ".git-po-helper.yaml")
		} else {
			userConfigPath = "~/.git-po-helper.yaml"
		}
		log.Warnf("no configuration files found (checked %s and %s), using defaults", 
			userConfigPath, repoConfigPath)
		return &AgentConfig{
			Agents: make(map[string]Agent),
		}, nil
	}

	// Merge configurations: repo config overrides user config
	mergedConfig := mergeConfigs(&baseConfig, &repoConfig)

	// Initialize Agents map if nil
	if mergedConfig.Agents == nil {
		mergedConfig.Agents = make(map[string]Agent)
	}

	// Validate configuration
	if err := mergedConfig.Validate(); err != nil {
		return nil, fmt.Errorf("invalid configuration: %w", err)
	}

	return mergedConfig, nil
}

// loadConfigFromFile loads and parses a YAML config file without validation.
// This is used internally to load configs that will be merged.
func loadConfigFromFile(configPath string) (*AgentConfig, error) {
	data, err := os.ReadFile(configPath)
	if err != nil {
		return nil, fmt.Errorf("failed to read config file: %w", err)
	}

	var config AgentConfig
	if err := yaml.Unmarshal(data, &config); err != nil {
		return nil, fmt.Errorf("failed to parse YAML config file: %w", err)
	}

	return &config, nil
}

// mergeConfigs merges two AgentConfig structs, with repoConfig taking priority over baseConfig.
func mergeConfigs(baseConfig, repoConfig *AgentConfig) *AgentConfig {
	result := &AgentConfig{
		Agents: make(map[string]Agent),
	}

	// Start with base config
	if baseConfig != nil {
		result.DefaultLangCode = baseConfig.DefaultLangCode
		result.Prompt = baseConfig.Prompt
		result.AgentTest = baseConfig.AgentTest
		if baseConfig.Agents != nil {
			for k, v := range baseConfig.Agents {
				result.Agents[k] = v
			}
		}
	}

	// Override with repo config (higher priority)
	if repoConfig != nil {
		if repoConfig.DefaultLangCode != "" {
			result.DefaultLangCode = repoConfig.DefaultLangCode
		}
		// Merge Prompt config
		if repoConfig.Prompt.UpdatePot != "" {
			result.Prompt.UpdatePot = repoConfig.Prompt.UpdatePot
		}
		if repoConfig.Prompt.UpdatePo != "" {
			result.Prompt.UpdatePo = repoConfig.Prompt.UpdatePo
		}
		if repoConfig.Prompt.Translate != "" {
			result.Prompt.Translate = repoConfig.Prompt.Translate
		}
		if repoConfig.Prompt.ReviewSince != "" {
			result.Prompt.ReviewSince = repoConfig.Prompt.ReviewSince
		}
		if repoConfig.Prompt.ReviewCommit != "" {
			result.Prompt.ReviewCommit = repoConfig.Prompt.ReviewCommit
		}
		// Merge AgentTest config (pointer fields need special handling)
		if repoConfig.AgentTest.Runs != nil {
			result.AgentTest.Runs = repoConfig.AgentTest.Runs
		}
		if repoConfig.AgentTest.PotEntriesBeforeUpdate != nil {
			result.AgentTest.PotEntriesBeforeUpdate = repoConfig.AgentTest.PotEntriesBeforeUpdate
		}
		if repoConfig.AgentTest.PotEntriesAfterUpdate != nil {
			result.AgentTest.PotEntriesAfterUpdate = repoConfig.AgentTest.PotEntriesAfterUpdate
		}
		if repoConfig.AgentTest.PoEntriesBeforeUpdate != nil {
			result.AgentTest.PoEntriesBeforeUpdate = repoConfig.AgentTest.PoEntriesBeforeUpdate
		}
		if repoConfig.AgentTest.PoEntriesAfterUpdate != nil {
			result.AgentTest.PoEntriesAfterUpdate = repoConfig.AgentTest.PoEntriesAfterUpdate
		}
		if repoConfig.AgentTest.PoNewEntriesAfterUpdate != nil {
			result.AgentTest.PoNewEntriesAfterUpdate = repoConfig.AgentTest.PoNewEntriesAfterUpdate
		}
		if repoConfig.AgentTest.PoFuzzyEntriesAfterUpdate != nil {
			result.AgentTest.PoFuzzyEntriesAfterUpdate = repoConfig.AgentTest.PoFuzzyEntriesAfterUpdate
		}
		// Merge Agents (repo config agents override base config agents)
		if repoConfig.Agents != nil {
			for k, v := range repoConfig.Agents {
				result.Agents[k] = v
			}
		}
	}

	return result
}

// Validate validates the agent configuration and returns an error if invalid.
func (c *AgentConfig) Validate() error {
	// Check if at least one agent is configured
	if len(c.Agents) == 0 {
		return fmt.Errorf("at least one agent must be configured")
	}

	// Validate each agent
	for name, agent := range c.Agents {
		if len(agent.Cmd) == 0 {
			return fmt.Errorf("agent '%s' has empty command", name)
		}
	}

	// Validate that update_pot prompt is set (required for update-pot command)
	if c.Prompt.UpdatePot == "" {
		return fmt.Errorf("prompt.update_pot is required")
	}

	return nil
}
