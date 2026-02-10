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
		return fmt.Errorf("failed to count entries %s in %s: %w", stage, potFile, err)
	}

	// Compare with expected count
	if actualCount != *expectedCount {
		return fmt.Errorf("entry count %s: expected %d, got %d (file: %s)", stage, *expectedCount, actualCount, potFile)
	}

	log.Debugf("entry count %s validation passed: %d entries", stage, actualCount)
	return nil
}

// ValidatePotFile validates POT file syntax using msgfmt.
// Returns an error if the file is invalid, nil if valid.
func ValidatePotFile(potFile string) error {
	if !Exist(potFile) {
		return fmt.Errorf("POT file does not exist: %s\nHint: Ensure the file exists or run the agent to create it", potFile)
	}

	// Use msgfmt --check to validate POT file syntax
	// For POT files, we use a simpler validation than PO files
	log.Debugf("running msgfmt --check on %s", potFile)
	cmd := exec.Command("msgfmt",
		"-o",
		os.DevNull,
		"--check",
		potFile)
	cmd.Dir = repository.WorkDir()

	// Capture stderr for error messages
	stderr, err := cmd.StderrPipe()
	if err != nil {
		return fmt.Errorf("failed to create stderr pipe for msgfmt: %w", err)
	}

	if err := cmd.Start(); err != nil {
		return fmt.Errorf("failed to start msgfmt command: %w\nHint: Ensure gettext tools (msgfmt) are installed", err)
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
		return fmt.Errorf("POT file syntax validation failed: %s\nHint: Check the POT file syntax and fix any errors reported by msgfmt", errorMsg)
	}

	log.Debugf("POT file validation passed: %s", potFile)
	return nil
}

// CmdAgentRunUpdatePot implements the agent-run update-pot command logic.
// It loads configuration, selects an agent, performs pre-validation,
// executes the agent command, performs post-validation, and validates POT file syntax.
func CmdAgentRunUpdatePot(agentName string) error {
	// Load configuration
	log.Debugf("loading agent configuration")
	cfg, err := config.LoadAgentConfig()
	if err != nil {
		log.Errorf("failed to load agent configuration: %v", err)
		return fmt.Errorf("failed to load agent configuration: %w\nHint: Ensure git-po-helper.yaml exists in repository root or user home directory", err)
	}

	// Determine agent to use
	selectedAgent, agentKey, err := SelectAgent(cfg, agentName)
	if err != nil {
		return err
	}

	log.Debugf("using agent: %s", agentKey)

	// Get repository root and POT file path
	workDir := repository.WorkDir()
	potFile := filepath.Join(workDir, PoDir, GitPot)
	log.Debugf("POT file path: %s", potFile)

	// Pre-validation: Check entry count before update
	if cfg.AgentTest.PotEntriesBeforeUpdate != nil && *cfg.AgentTest.PotEntriesBeforeUpdate != 0 {
		log.Infof("performing pre-validation: checking entry count before update (expected: %d)", *cfg.AgentTest.PotEntriesBeforeUpdate)
		if err := ValidatePotEntryCount(potFile, cfg.AgentTest.PotEntriesBeforeUpdate, "before update"); err != nil {
			log.Errorf("pre-validation failed: %v", err)
			return fmt.Errorf("pre-validation failed: %w\nHint: Ensure po/git.pot exists and has the expected number of entries", err)
		}
		log.Infof("pre-validation passed")
	}

	// Get prompt from configuration
	prompt := cfg.Prompt.UpdatePot
	if prompt == "" {
		log.Error("prompt.update_pot is not configured")
		return fmt.Errorf("prompt.update_pot is not configured\nHint: Add 'prompt.update_pot' to git-po-helper.yaml")
	}
	log.Debugf("using prompt: %s", prompt)

	// Replace placeholders in agent command
	// For update-pot, we only need to replace {prompt}
	agentCmd := make([]string, len(selectedAgent.Cmd))
	for i, arg := range selectedAgent.Cmd {
		agentCmd[i] = ReplacePlaceholders(arg, prompt, "", "")
	}

	// Execute agent command
	log.Infof("executing agent command: %s", strings.Join(agentCmd, " "))
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
		log.Errorf("agent command execution failed: %v", err)
		return fmt.Errorf("agent command failed: %w\nHint: Check that the agent command is correct and executable", err)
	}
	log.Infof("agent command completed successfully")

	// Log output if verbose
	if len(stdout) > 0 {
		log.Debugf("agent command stdout: %s", string(stdout))
	}
	if len(stderr) > 0 {
		log.Debugf("agent command stderr: %s", string(stderr))
	}

	// Post-validation: Check entry count after update
	if cfg.AgentTest.PotEntriesAfterUpdate != nil && *cfg.AgentTest.PotEntriesAfterUpdate != 0 {
		log.Infof("performing post-validation: checking entry count after update (expected: %d)", *cfg.AgentTest.PotEntriesAfterUpdate)
		if err := ValidatePotEntryCount(potFile, cfg.AgentTest.PotEntriesAfterUpdate, "after update"); err != nil {
			log.Errorf("post-validation failed: %v", err)
			return fmt.Errorf("post-validation failed: %w\nHint: The agent may not have updated the POT file correctly", err)
		}
		log.Infof("post-validation passed")
	}

	// Validate POT file syntax
	log.Infof("validating POT file syntax: %s", potFile)
	if err := ValidatePotFile(potFile); err != nil {
		log.Errorf("POT file syntax validation failed: %v", err)
		return fmt.Errorf("POT file validation failed: %w\nHint: Check the POT file syntax using 'msgfmt --check-format'", err)
	}
	log.Infof("POT file syntax validation passed")

	log.Infof("agent-run update-pot completed successfully")
	return nil
}
