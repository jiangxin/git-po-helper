// Package util provides business logic for agent-run command.
package util

import (
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"

	"github.com/git-l10n/git-po-helper/config"
	"github.com/git-l10n/git-po-helper/repository"
	log "github.com/sirupsen/logrus"
)

// ValidatePotEntryCount validates the entry count in a POT file.
// If expectedCount is nil or 0, validation is disabled and the function returns nil.
// Otherwise, it counts entries using CountPotEntries() and compares with expectedCount.
// Returns an error if counts don't match, nil if they match or validation is disabled.
// The stage parameter is used for error messages ("before update" or "after update").
func ValidatePotEntryCount(potFile string, expectedCount *int, stage string) error {
	// If expectedCount is nil or 0, validation is disabled
	if expectedCount == nil || *expectedCount == 0 {
		return nil
	}

	// Count entries in POT file
	actualCount, err := CountPotEntries(potFile)
	if err != nil {
		return fmt.Errorf("failed to count entries %s: %w", stage, err)
	}

	// Compare with expected count
	if actualCount != *expectedCount {
		return fmt.Errorf("entry count %s: expected %d, got %d", stage, *expectedCount, actualCount)
	}

	log.Debugf("entry count %s validation passed: %d entries", stage, actualCount)
	return nil
}

// ValidatePotFile validates POT file syntax using msgfmt.
// Returns an error if the file is invalid, nil if valid.
func ValidatePotFile(potFile string) error {
	if !Exist(potFile) {
		return fmt.Errorf("POT file does not exist: %s", potFile)
	}

	// Use msgfmt --check to validate POT file syntax
	// For POT files, we use a simpler validation than PO files
	cmd := exec.Command("msgfmt",
		"-o",
		os.DevNull,
		"--check",
		potFile)
	cmd.Dir = repository.WorkDir()

	// Capture stderr for error messages
	stderr, err := cmd.StderrPipe()
	if err != nil {
		return fmt.Errorf("failed to create stderr pipe: %w", err)
	}

	if err := cmd.Start(); err != nil {
		return fmt.Errorf("failed to start msgfmt: %w", err)
	}

	// Read stderr output
	var stderrOutput strings.Builder
	buf := make([]byte, 1024)
	for {
		n, err := stderr.Read(buf)
		if n > 0 {
			stderrOutput.Write(buf[:n])
		}
		if err != nil {
			break
		}
	}

	if err := cmd.Wait(); err != nil {
		errorMsg := stderrOutput.String()
		if errorMsg == "" {
			errorMsg = err.Error()
		}
		return fmt.Errorf("POT file validation failed: %s", errorMsg)
	}

	log.Debugf("POT file validation passed: %s", potFile)
	return nil
}

// CmdAgentRunUpdatePot implements the agent-run update-pot command logic.
// It loads configuration, selects an agent, performs pre-validation,
// executes the agent command, performs post-validation, and validates POT file syntax.
func CmdAgentRunUpdatePot(agentName string) error {
	// Load configuration
	cfg, err := config.LoadAgentConfig()
	if err != nil {
		return fmt.Errorf("failed to load agent configuration: %w", err)
	}

	// Determine agent to use
	var selectedAgent config.Agent
	var agentKey string

	if agentName != "" {
		// Use specified agent
		agent, ok := cfg.Agents[agentName]
		if !ok {
			return fmt.Errorf("agent '%s' not found in configuration", agentName)
		}
		selectedAgent = agent
		agentKey = agentName
	} else {
		// Auto-select agent
		if len(cfg.Agents) == 0 {
			return fmt.Errorf("no agents configured")
		}
		if len(cfg.Agents) > 1 {
			agentList := make([]string, 0, len(cfg.Agents))
			for k := range cfg.Agents {
				agentList = append(agentList, k)
			}
			return fmt.Errorf("multiple agents configured (%s), please specify --agent", strings.Join(agentList, ", "))
		}
		// Only one agent, use it
		for k, v := range cfg.Agents {
			selectedAgent = v
			agentKey = k
			break
		}
	}

	log.Debugf("using agent: %s", agentKey)

	// Get repository root and POT file path
	workDir := repository.WorkDir()
	potFile := filepath.Join(workDir, PoDir, GitPot)

	// Pre-validation: Check entry count before update
	if cfg.AgentTest.PotEntriesBeforeUpdate != nil && *cfg.AgentTest.PotEntriesBeforeUpdate != 0 {
		log.Debugf("performing pre-validation: checking entry count before update")
		if err := ValidatePotEntryCount(potFile, cfg.AgentTest.PotEntriesBeforeUpdate, "before update"); err != nil {
			return fmt.Errorf("pre-validation failed: %w", err)
		}
	}

	// Get prompt from configuration
	prompt := cfg.Prompt.UpdatePot
	if prompt == "" {
		return fmt.Errorf("prompt.update_pot is not configured")
	}

	// Replace placeholders in agent command
	// For update-pot, we only need to replace {prompt}
	agentCmd := make([]string, len(selectedAgent.Cmd))
	for i, arg := range selectedAgent.Cmd {
		agentCmd[i] = ReplacePlaceholders(arg, prompt, "", "")
	}

	log.Debugf("executing agent command: %s", strings.Join(agentCmd, " "))

	// Execute agent command
	stdout, stderr, err := ExecuteAgentCommand(agentCmd, workDir)
	if err != nil {
		// Log stderr if available
		if len(stderr) > 0 {
			log.Errorf("agent command stderr: %s", string(stderr))
		}
		// Log stdout if available (might contain useful info even on error)
		if len(stdout) > 0 {
			log.Debugf("agent command stdout: %s", string(stdout))
		}
		return fmt.Errorf("agent command failed: %w", err)
	}

	// Log output if verbose
	if len(stdout) > 0 {
		log.Debugf("agent command stdout: %s", string(stdout))
	}
	if len(stderr) > 0 {
		log.Debugf("agent command stderr: %s", string(stderr))
	}

	// Post-validation: Check entry count after update
	if cfg.AgentTest.PotEntriesAfterUpdate != nil && *cfg.AgentTest.PotEntriesAfterUpdate != 0 {
		log.Debugf("performing post-validation: checking entry count after update")
		if err := ValidatePotEntryCount(potFile, cfg.AgentTest.PotEntriesAfterUpdate, "after update"); err != nil {
			return fmt.Errorf("post-validation failed: %w", err)
		}
	}

	// Validate POT file syntax
	log.Debugf("validating POT file syntax: %s", potFile)
	if err := ValidatePotFile(potFile); err != nil {
		return fmt.Errorf("POT file validation failed: %w", err)
	}

	log.Infof("agent-run update-pot completed successfully")
	return nil
}
