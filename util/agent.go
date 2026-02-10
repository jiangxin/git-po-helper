// Package util provides utility functions for agent execution.
package util

import (
	"bufio"
	"bytes"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"

	"github.com/git-l10n/git-po-helper/config"
	"github.com/git-l10n/git-po-helper/repository"
	log "github.com/sirupsen/logrus"
)

// CountPotEntries counts msgid entries in a POT file.
// It excludes the header entry (which has an empty msgid) and counts
// only non-empty msgid entries.
//
// The function:
// - Opens the POT file
// - Scans for lines starting with "msgid " (excluding commented entries)
// - Parses msgid values to identify the header entry (empty msgid)
// - Returns the count of non-empty msgid entries
func CountPotEntries(potFile string) (int, error) {
	f, err := os.Open(potFile)
	if err != nil {
		return 0, fmt.Errorf("failed to open POT file %s: %w", potFile, err)
	}
	defer f.Close()

	scanner := bufio.NewScanner(f)
	count := 0
	inMsgid := false
	msgidValue := ""
	headerFound := false

	for scanner.Scan() {
		line := scanner.Text()
		trimmed := strings.TrimSpace(line)

		// Skip comment lines (obsolete entries, etc.)
		if strings.HasPrefix(trimmed, "#") {
			continue
		}

		// Check for msgid line
		if strings.HasPrefix(trimmed, "msgid ") {
			// If we were already in a msgid, finish the previous one
			if inMsgid {
				if !headerFound && strings.Trim(msgidValue, `"`) == "" {
					headerFound = true
				} else if strings.Trim(msgidValue, `"`) != "" {
					// Non-empty msgid entry
					count++
				}
			}
			// Start new msgid entry
			inMsgid = true
			// Extract the msgid value (may be on same line or continue on next lines)
			msgidValue = strings.TrimPrefix(trimmed, "msgid ")
			msgidValue = strings.TrimSpace(msgidValue)
			// Remove quotes if present
			msgidValue = strings.Trim(msgidValue, `"`)
			continue
		}

		// If we're in a msgid entry and this line continues it (starts with quote)
		if inMsgid && strings.HasPrefix(trimmed, `"`) {
			// Continuation line - append to msgidValue (remove quotes)
			contValue := strings.Trim(trimmed, `"`)
			msgidValue += contValue
			continue
		}

		// If we encounter msgstr, it means we've finished the msgid
		if inMsgid && strings.HasPrefix(trimmed, "msgstr") {
			// End of msgid entry
			if !headerFound && strings.Trim(msgidValue, `"`) == "" {
				headerFound = true
			} else if strings.Trim(msgidValue, `"`) != "" {
				// Non-empty msgid entry
				count++
			}
			inMsgid = false
			msgidValue = ""
			continue
		}

		// Empty line might indicate end of entry, but we'll rely on msgstr
		// to be more accurate
	}

	// Handle last entry if file doesn't end with newline or msgstr
	if inMsgid {
		if !headerFound && strings.Trim(msgidValue, `"`) == "" {
			headerFound = true
		} else if strings.Trim(msgidValue, `"`) != "" {
			count++
		}
	}

	if err := scanner.Err(); err != nil {
		return 0, fmt.Errorf("failed to read POT file %s: %w", potFile, err)
	}

	return count, nil
}

// CountPoEntries counts msgid entries in a PO file.
// It excludes the header entry (which has an empty msgid) and counts
// only non-empty msgid entries.
//
// The function:
// - Opens the PO file
// - Scans for lines starting with "msgid " (excluding commented entries)
// - Parses msgid values to identify the header entry (empty msgid)
// - Returns the count of non-empty msgid entries
func CountPoEntries(poFile string) (int, error) {
	f, err := os.Open(poFile)
	if err != nil {
		return 0, fmt.Errorf("failed to open PO file %s: %w", poFile, err)
	}
	defer f.Close()

	scanner := bufio.NewScanner(f)
	count := 0
	inMsgid := false
	msgidValue := ""
	headerFound := false

	for scanner.Scan() {
		line := scanner.Text()
		trimmed := strings.TrimSpace(line)

		// Skip comment lines (obsolete entries, etc.)
		if strings.HasPrefix(trimmed, "#") {
			continue
		}

		// Check for msgid line
		if strings.HasPrefix(trimmed, "msgid ") {
			// If we were already in a msgid, finish the previous one
			if inMsgid {
				if !headerFound && strings.Trim(msgidValue, `"`) == "" {
					headerFound = true
				} else if strings.Trim(msgidValue, `"`) != "" {
					// Non-empty msgid entry
					count++
				}
			}
			// Start new msgid entry
			inMsgid = true
			// Extract the msgid value (may be on same line or continue on next lines)
			msgidValue = strings.TrimPrefix(trimmed, "msgid ")
			msgidValue = strings.TrimSpace(msgidValue)
			// Remove quotes if present
			msgidValue = strings.Trim(msgidValue, `"`)
			continue
		}

		// If we're in a msgid entry and this line continues it (starts with quote)
		if inMsgid && strings.HasPrefix(trimmed, `"`) {
			// Continuation line - append to msgidValue (remove quotes)
			contValue := strings.Trim(trimmed, `"`)
			msgidValue += contValue
			continue
		}

		// If we encounter msgstr, it means we've finished the msgid
		if inMsgid && strings.HasPrefix(trimmed, "msgstr") {
			// End of msgid entry
			if !headerFound && strings.Trim(msgidValue, `"`) == "" {
				headerFound = true
			} else if strings.Trim(msgidValue, `"`) != "" {
				// Non-empty msgid entry
				count++
			}
			inMsgid = false
			msgidValue = ""
			continue
		}

		// Empty line might indicate end of entry, but we'll rely on msgstr
		// to be more accurate
	}

	// Handle last entry if file doesn't end with newline or msgstr
	if inMsgid {
		if !headerFound && strings.Trim(msgidValue, `"`) == "" {
			headerFound = true
		} else if strings.Trim(msgidValue, `"`) != "" {
			count++
		}
	}

	if err := scanner.Err(); err != nil {
		return 0, fmt.Errorf("failed to read PO file %s: %w", poFile, err)
	}

	return count, nil
}

// ReplacePlaceholders replaces placeholders in a template string with actual values.
// Supported placeholders:
//   - {prompt} - replaced with the prompt text
//   - {source} - replaced with the source file path (po file)
//   - {commit} - replaced with the commit ID (default: HEAD)
//
// If a placeholder value is empty, it will be replaced with an empty string.
func ReplacePlaceholders(template string, prompt, source, commit string) string {
	result := template
	result = strings.ReplaceAll(result, "{prompt}", prompt)
	result = strings.ReplaceAll(result, "{source}", source)
	result = strings.ReplaceAll(result, "{commit}", commit)
	return result
}

// ExecuteAgentCommand executes an agent command and captures both stdout and stderr.
// The command is executed in the specified working directory.
//
// Parameters:
//   - cmd: Command and arguments as a slice (e.g., []string{"claude", "-p", "{prompt}"})
//   - workDir: Working directory for command execution (empty string uses current working directory).
//     To use repository root, pass repository.WorkDir() explicitly.
//
// Returns:
//   - stdout: Standard output from the command
//   - stderr: Standard error from the command
//   - error: Error if command execution fails (includes non-zero exit codes)
//
// The function:
//   - Replaces placeholders in command arguments using ReplacePlaceholders
//   - Executes the command in the specified working directory
//   - Captures both stdout and stderr separately
//   - Returns an error if the command exits with a non-zero status code
func ExecuteAgentCommand(cmd []string, workDir string) ([]byte, []byte, error) {
	if len(cmd) == 0 {
		return nil, nil, fmt.Errorf("command cannot be empty")
	}

	// Determine working directory
	if workDir == "" {
		// Use current working directory as default
		// Callers should provide repository.WorkDir() explicitly if they want repository root
		var err error
		workDir, err = os.Getwd()
		if err != nil {
			return nil, nil, fmt.Errorf("failed to get working directory: %w", err)
		}
	}

	// Replace placeholders in command arguments
	// Note: Placeholders should be replaced before calling this function,
	// but we'll handle it here for safety
	execCmd := exec.Command(cmd[0], cmd[1:]...)
	execCmd.Dir = workDir

	log.Debugf("executing agent command: %s (workDir: %s)", strings.Join(cmd, " "), workDir)

	// Capture stdout and stderr separately
	var stdoutBuf, stderrBuf bytes.Buffer
	execCmd.Stdout = &stdoutBuf
	execCmd.Stderr = &stderrBuf

	// Execute the command
	err := execCmd.Run()
	stdout := stdoutBuf.Bytes()
	stderr := stderrBuf.Bytes()

	// Check for execution errors
	if err != nil {
		// If command exited with non-zero status, include stderr in error message
		if exitError, ok := err.(*exec.ExitError); ok {
			return stdout, stderr, fmt.Errorf("agent command failed with exit code %d: %w\nstderr: %s",
				exitError.ExitCode(), err, string(stderr))
		}
		return stdout, stderr, fmt.Errorf("failed to execute agent command: %w\nstderr: %s", err, string(stderr))
	}

	log.Debugf("agent command completed successfully (stdout: %d bytes, stderr: %d bytes)",
		len(stdout), len(stderr))

	return stdout, stderr, nil
}

// SelectAgent selects an agent from the configuration based on the provided agent name.
// If agentName is empty, it auto-selects an agent (only works if exactly one agent is configured).
// Returns the selected agent, its key, and an error if selection fails.
func SelectAgent(cfg *config.AgentConfig, agentName string) (config.Agent, string, error) {
	if agentName != "" {
		// Use specified agent
		log.Debugf("using specified agent: %s", agentName)
		agent, ok := cfg.Agents[agentName]
		if !ok {
			agentList := make([]string, 0, len(cfg.Agents))
			for k := range cfg.Agents {
				agentList = append(agentList, k)
			}
			log.Errorf("agent '%s' not found in configuration. Available agents: %v", agentName, agentList)
			return config.Agent{}, "", fmt.Errorf("agent '%s' not found in configuration\nAvailable agents: %s\nHint: Check git-po-helper.yaml for configured agents", agentName, strings.Join(agentList, ", "))
		}
		return agent, agentName, nil
	}

	// Auto-select agent
	log.Debugf("auto-selecting agent from configuration")
	if len(cfg.Agents) == 0 {
		log.Error("no agents configured")
		return config.Agent{}, "", fmt.Errorf("no agents configured\nHint: Add at least one agent to git-po-helper.yaml in the 'agents' section")
	}
	if len(cfg.Agents) > 1 {
		agentList := make([]string, 0, len(cfg.Agents))
		for k := range cfg.Agents {
			agentList = append(agentList, k)
		}
		log.Errorf("multiple agents configured (%s), --agent flag required", strings.Join(agentList, ", "))
		return config.Agent{}, "", fmt.Errorf("multiple agents configured (%s), please specify --agent\nHint: Use --agent flag to select one of the available agents", strings.Join(agentList, ", "))
	}

	// Only one agent, use it
	for k, v := range cfg.Agents {
		return v, k, nil
	}

	// This should never happen, but handle it gracefully
	return config.Agent{}, "", fmt.Errorf("unexpected error: no agent selected")
}

// BuildAgentCommand builds an agent command by replacing placeholders in the agent's command template.
// It replaces {prompt}, {source}, and {commit} placeholders with actual values.
func BuildAgentCommand(agent config.Agent, prompt, source, commit string) []string {
	cmd := make([]string, len(agent.Cmd))
	for i, arg := range agent.Cmd {
		cmd[i] = ReplacePlaceholders(arg, prompt, source, commit)
	}
	return cmd
}

// GetPotFilePath returns the full path to the POT file in the repository.
func GetPotFilePath() string {
	workDir := repository.WorkDir()
	return filepath.Join(workDir, PoDir, GitPot)
}

// GetPrompt returns the update_pot prompt from configuration, or an error if not configured.
func GetPrompt(cfg *config.AgentConfig) (string, error) {
	prompt := cfg.Prompt.UpdatePot
	if prompt == "" {
		log.Error("prompt.update_pot is not configured")
		return "", fmt.Errorf("prompt.update_pot is not configured\nHint: Add 'prompt.update_pot' to git-po-helper.yaml")
	}
	log.Debugf("using prompt: %s", prompt)
	return prompt, nil
}
