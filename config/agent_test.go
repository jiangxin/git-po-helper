package config

import (
	"os"
	"path/filepath"
	"testing"
)

func TestLoadConfigFromFile_MissingFile(t *testing.T) {
	// Create a temporary directory
	tmpDir, err := os.MkdirTemp("", "git-po-helper-test-*")
	if err != nil {
		t.Fatalf("failed to create temp dir: %v", err)
	}
	defer os.RemoveAll(tmpDir)

	configPath := filepath.Join(tmpDir, "git-po-helper.yaml")

	// Test missing file - should return error (loadConfigFromFile doesn't handle missing files)
	config, err := loadConfigFromFile(configPath)
	if err == nil {
		t.Fatal("loadConfigFromFile should return error for missing file")
	}
	if config != nil {
		t.Fatal("loadConfigFromFile should return nil config for missing file")
	}
}

func TestLoadAgentConfig_ValidFile(t *testing.T) {
	// Create a temporary directory
	tmpDir, err := os.MkdirTemp("", "git-po-helper-test-*")
	if err != nil {
		t.Fatalf("failed to create temp dir: %v", err)
	}
	defer os.RemoveAll(tmpDir)

	configPath := filepath.Join(tmpDir, "git-po-helper.yaml")

	// Create a valid YAML config file
	validYAML := `default_lang_code: "zh_CN"
prompt:
  update_pot: "update po/git.pot according to po/README.md"
  update_po: "update {source} according to po/README.md"
agents:
  claude:
    cmd: ["claude", "-p", "{prompt}"]
  gemini:
    cmd: ["gemini", "--prompt", "{prompt}"]
`

	if err := os.WriteFile(configPath, []byte(validYAML), 0644); err != nil {
		t.Fatalf("failed to write config file: %v", err)
	}

	config, err := loadConfigFromFile(configPath)
	if err != nil {
		t.Fatalf("loadConfigFromFile should succeed for valid file, got error: %v", err)
	}
	if config == nil {
		t.Fatal("loadConfigFromFile should return config, got nil")
	}
	if config.DefaultLangCode != "zh_CN" {
		t.Fatalf("expected DefaultLangCode 'zh_CN', got '%s'", config.DefaultLangCode)
	}
	if config.Prompt.UpdatePot != "update po/git.pot according to po/README.md" {
		t.Fatalf("expected UpdatePot prompt, got '%s'", config.Prompt.UpdatePot)
	}
	if len(config.Agents) != 2 {
		t.Fatalf("expected 2 agents, got %d", len(config.Agents))
	}
	if config.Agents["claude"].Cmd[0] != "claude" {
		t.Fatalf("expected claude agent command, got %v", config.Agents["claude"].Cmd)
	}
}

func TestLoadAgentConfig_InvalidYAML(t *testing.T) {
	// Create a temporary directory
	tmpDir, err := os.MkdirTemp("", "git-po-helper-test-*")
	if err != nil {
		t.Fatalf("failed to create temp dir: %v", err)
	}
	defer os.RemoveAll(tmpDir)

	configPath := filepath.Join(tmpDir, "git-po-helper.yaml")

	// Create an invalid YAML file
	invalidYAML := `default_lang_code: "zh_CN"
prompt:
  update_pot: "update po/git.pot according to po/README.md"
agents:
  claude:
    cmd: ["claude", "-p", "{prompt}"]
    invalid: [unclosed bracket
`

	if err := os.WriteFile(configPath, []byte(invalidYAML), 0644); err != nil {
		t.Fatalf("failed to write config file: %v", err)
	}

	config, err := loadConfigFromFile(configPath)
	if err == nil {
		t.Fatal("loadConfigFromFile should return error for invalid YAML")
	}
	if config != nil {
		t.Fatal("loadConfigFromFile should return nil config for invalid YAML")
	}
}

func TestAgentConfig_Validate(t *testing.T) {
	tests := []struct {
		name    string
		config  *AgentConfig
		wantErr bool
		errMsg  string
	}{
		{
			name: "valid config",
			config: &AgentConfig{
				Prompt: PromptConfig{
					UpdatePot: "update pot",
				},
				Agents: map[string]Agent{
					"claude": {
						Cmd: []string{"claude", "-p", "{prompt}"},
					},
				},
			},
			wantErr: false,
		},
		{
			name: "no agents",
			config: &AgentConfig{
				Prompt: PromptConfig{
					UpdatePot: "update pot",
				},
				Agents: map[string]Agent{},
			},
			wantErr: true,
			errMsg:  "at least one agent must be configured",
		},
		{
			name: "empty agent command",
			config: &AgentConfig{
				Prompt: PromptConfig{
					UpdatePot: "update pot",
				},
				Agents: map[string]Agent{
					"claude": {
						Cmd: []string{},
					},
				},
			},
			wantErr: true,
			errMsg:  "agent 'claude' has empty command",
		},
		{
			name: "missing update_pot prompt",
			config: &AgentConfig{
				Prompt: PromptConfig{
					UpdatePot: "",
				},
				Agents: map[string]Agent{
					"claude": {
						Cmd: []string{"claude", "-p", "{prompt}"},
					},
				},
			},
			wantErr: true,
			errMsg:  "prompt.update_pot is required",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := tt.config.Validate()
			if tt.wantErr {
				if err == nil {
					t.Fatalf("Validate() expected error, got nil")
				}
				if tt.errMsg != "" && err.Error() != tt.errMsg {
					t.Fatalf("Validate() expected error message '%s', got '%s'", tt.errMsg, err.Error())
				}
			} else {
				if err != nil {
					t.Fatalf("Validate() unexpected error: %v", err)
				}
			}
		})
	}
}

func TestMergeConfigs(t *testing.T) {
	baseConfig := &AgentConfig{
		DefaultLangCode: "en_US",
		Prompt: PromptConfig{
			UpdatePot: "base update pot",
			UpdatePo:  "base update po",
		},
		Agents: map[string]Agent{
			"claude": {
				Cmd: []string{"claude", "-p", "{prompt}"},
			},
			"gemini": {
				Cmd: []string{"gemini", "--prompt", "{prompt}"},
			},
		},
	}

	repoConfig := &AgentConfig{
		DefaultLangCode: "zh_CN",
		Prompt: PromptConfig{
			UpdatePot: "repo update pot",
		},
		Agents: map[string]Agent{
			"claude": {
				Cmd: []string{"claude", "--new-flag", "{prompt}"},
			},
		},
	}

	merged := mergeConfigs(baseConfig, repoConfig)

	// Check that repo config overrides base config
	if merged.DefaultLangCode != "zh_CN" {
		t.Fatalf("expected DefaultLangCode 'zh_CN', got '%s'", merged.DefaultLangCode)
	}

	// Check that repo config overrides base prompt
	if merged.Prompt.UpdatePot != "repo update pot" {
		t.Fatalf("expected UpdatePot 'repo update pot', got '%s'", merged.Prompt.UpdatePot)
	}

	// Check that base prompt fields are preserved if not overridden
	if merged.Prompt.UpdatePo != "base update po" {
		t.Fatalf("expected UpdatePo 'base update po', got '%s'", merged.Prompt.UpdatePo)
	}

	// Check that repo agents override base agents
	if len(merged.Agents) != 2 {
		t.Fatalf("expected 2 agents, got %d", len(merged.Agents))
	}
	if merged.Agents["claude"].Cmd[1] != "--new-flag" {
		t.Fatalf("expected claude agent to have --new-flag, got %v", merged.Agents["claude"].Cmd)
	}

	// Check that base agents are preserved if not overridden
	if merged.Agents["gemini"].Cmd[0] != "gemini" {
		t.Fatalf("expected gemini agent to be preserved, got %v", merged.Agents["gemini"].Cmd)
	}
}
