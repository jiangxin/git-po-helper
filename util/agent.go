// Package util provides utility functions for agent execution.
package util

import (
	"bufio"
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"time"

	"github.com/git-l10n/git-po-helper/config"
	"github.com/git-l10n/git-po-helper/repository"
	log "github.com/sirupsen/logrus"
)

const (
	// maxDisplayBytes is the maximum number of bytes to display for agent messages (4KB).
	maxDisplayBytes = 4096
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

// ExecuteAgentCommandStream executes an agent command and returns a reader for real-time stdout streaming.
// The command is executed in the specified working directory.
// This function is used for json format (stream-json internally) to process output in real-time.
//
// Parameters:
//   - cmd: Command and arguments as a slice
//   - workDir: Working directory for command execution
//
// Returns:
//   - stdoutReader: io.ReadCloser for reading stdout in real-time
//   - stderr: Standard error from the command (captured after execution)
//   - cmdProcess: *exec.Cmd for waiting on command completion
//   - error: Error if command setup fails
func ExecuteAgentCommandStream(cmd []string, workDir string) (stdoutReader io.ReadCloser, stderrBuf *bytes.Buffer, cmdProcess *exec.Cmd, err error) {
	if len(cmd) == 0 {
		return nil, nil, nil, fmt.Errorf("command cannot be empty")
	}

	// Determine working directory
	if workDir == "" {
		var err error
		workDir, err = os.Getwd()
		if err != nil {
			return nil, nil, nil, fmt.Errorf("failed to get working directory: %w", err)
		}
	}

	// Create command
	execCmd := exec.Command(cmd[0], cmd[1:]...)
	execCmd.Dir = workDir

	log.Debugf("executing agent command (streaming): %s (workDir: %s)", strings.Join(cmd, " "), workDir)

	// Get stdout pipe for real-time reading
	stdoutPipe, err := execCmd.StdoutPipe()
	if err != nil {
		return nil, nil, nil, fmt.Errorf("failed to create stdout pipe: %w", err)
	}

	// Capture stderr separately
	var stderrBuffer bytes.Buffer
	execCmd.Stderr = &stderrBuffer

	// Start command execution
	if err := execCmd.Start(); err != nil {
		stdoutPipe.Close()
		return nil, nil, nil, fmt.Errorf("failed to start agent command: %w", err)
	}

	return stdoutPipe, &stderrBuffer, execCmd, nil
}

// normalizeOutputFormat normalizes output format by converting underscores to hyphens
// and unifying stream-json/stream_json to json.
// This allows both "stream_json" and "stream-json" to be treated as "json".
func normalizeOutputFormat(format string) string {
	normalized := strings.ReplaceAll(format, "_", "-")
	// Unify stream-json to json (claude uses stream-json internally, but we simplify it to json)
	if normalized == "stream-json" {
		return "json"
	}
	return normalized
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
// For claude command, it also adds --output-format parameter based on agent.Output configuration.
func BuildAgentCommand(agent config.Agent, prompt, source, commit string) []string {
	cmd := make([]string, len(agent.Cmd))
	for i, arg := range agent.Cmd {
		cmd[i] = ReplacePlaceholders(arg, prompt, source, commit)
	}

	// For claude command, add --output-format parameter if output format is specified
	if len(cmd) > 0 && cmd[0] == "claude" {
		// Check if --output-format parameter already exists in the command
		hasOutputFormat := false
		for i, arg := range cmd {
			if arg == "--output-format" {
				hasOutputFormat = true
				// Skip the next argument (the format value)
				if i+1 < len(cmd) {
					_ = cmd[i+1]
				}
				break
			}
		}

		// Only add --output-format if it doesn't already exist
		if !hasOutputFormat {
			outputFormat := normalizeOutputFormat(agent.Output)
			if outputFormat == "" {
				outputFormat = "default"
			}

			// Add --output-format parameter for json format (claude uses stream-json internally)
			if outputFormat == "json" {
				cmd = append(cmd, "--verbose", "--output-format", "stream-json")
			}
			// For "default" format, no additional parameter is needed
		}
	}

	// For codex command, add --json parameter if output format is json
	if len(cmd) > 0 && cmd[0] == "codex" {
		// Check if --json parameter already exists in the command
		hasJSON := false
		for _, arg := range cmd {
			if arg == "--json" {
				hasJSON = true
				break
			}
		}

		// Only add --json if it doesn't already exist
		if !hasJSON {
			outputFormat := normalizeOutputFormat(agent.Output)
			if outputFormat == "" {
				outputFormat = "default"
			}

			// Add --json parameter for json format (codex uses JSONL format)
			if outputFormat == "json" {
				cmd = append(cmd, "--json")
			}
			// For "default" format, no additional parameter is needed
		}
	}

	// For opencode command, add --format json parameter if output format is json
	if len(cmd) > 0 && cmd[0] == "opencode" {
		// Check if --format parameter already exists in the command
		hasFormat := false
		for i, arg := range cmd {
			if arg == "--format" {
				hasFormat = true
				// Skip the next argument (the format value)
				if i+1 < len(cmd) {
					_ = cmd[i+1]
				}
				break
			}
		}

		// Only add --format if it doesn't already exist
		if !hasFormat {
			outputFormat := normalizeOutputFormat(agent.Output)
			if outputFormat == "" {
				outputFormat = "default"
			}

			// Add --format json parameter for json format (opencode uses JSONL format)
			if outputFormat == "json" {
				cmd = append(cmd, "--format", "json")
			}
			// For "default" format, no additional parameter is needed
		}
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

// CountNewEntries counts untranslated entries in a PO file.
// It uses `msgattrib --untranslated` to extract untranslated entries,
// then counts the msgid entries excluding the header entry (empty msgid).
//
// The function:
// - Executes `msgattrib --untranslated poFile`
// - Scans output for lines starting with "msgid "
// - Excludes the header entry (msgid "")
// - Returns the count of untranslated msgid entries
func CountNewEntries(poFile string) (int, error) {
	cmd := exec.Command("msgattrib", "--untranslated", poFile)
	output, err := cmd.Output()
	if err != nil {
		if exitError, ok := err.(*exec.ExitError); ok {
			return 0, fmt.Errorf("msgattrib failed for %s: %w\nstderr: %s",
				poFile, err, string(exitError.Stderr))
		}
		return 0, fmt.Errorf("failed to execute msgattrib for %s: %w", poFile, err)
	}

	scanner := bufio.NewScanner(bytes.NewReader(output))
	count := 0
	inMsgid := false
	msgidValue := ""

	for scanner.Scan() {
		line := scanner.Text()
		trimmed := strings.TrimSpace(line)

		// Check for msgid line
		if strings.HasPrefix(trimmed, "msgid ") {
			// Extract msgid value
			msgidValue = strings.TrimPrefix(trimmed, "msgid ")
			msgidValue = strings.TrimSpace(msgidValue)
			inMsgid = true
			continue
		}

		// If we're in a msgid and encounter a continuation line
		if inMsgid && strings.HasPrefix(trimmed, `"`) {
			// This is a multi-line msgid, just mark it as non-empty
			msgidValue += "continuation"
			continue
		}

		// If we encounter msgstr, finish the msgid
		if inMsgid && strings.HasPrefix(trimmed, "msgstr") {
			// Check if msgid is non-empty (not the header)
			if strings.Trim(msgidValue, `"`) != "" {
				count++
			}
			inMsgid = false
			msgidValue = ""
			continue
		}
	}

	if err := scanner.Err(); err != nil {
		return 0, fmt.Errorf("failed to scan msgattrib output: %w", err)
	}

	return count, nil
}

// CountFuzzyEntries counts fuzzy entries in a PO file.
// It uses `msgattrib --only-fuzzy` to extract fuzzy entries,
// then counts the msgid entries excluding the header entry (empty msgid).
//
// The function:
// - Executes `msgattrib --only-fuzzy poFile`
// - Scans output for lines starting with "msgid "
// - Excludes the header entry (msgid "")
// - Returns the count of fuzzy msgid entries
func CountFuzzyEntries(poFile string) (int, error) {
	cmd := exec.Command("msgattrib", "--only-fuzzy", poFile)
	output, err := cmd.Output()
	if err != nil {
		if exitError, ok := err.(*exec.ExitError); ok {
			return 0, fmt.Errorf("msgattrib failed for %s: %w\nstderr: %s",
				poFile, err, string(exitError.Stderr))
		}
		return 0, fmt.Errorf("failed to execute msgattrib for %s: %w", poFile, err)
	}

	scanner := bufio.NewScanner(bytes.NewReader(output))
	count := 0
	inMsgid := false
	msgidValue := ""

	for scanner.Scan() {
		line := scanner.Text()
		trimmed := strings.TrimSpace(line)

		// Check for msgid line
		if strings.HasPrefix(trimmed, "msgid ") {
			// Extract msgid value
			msgidValue = strings.TrimPrefix(trimmed, "msgid ")
			msgidValue = strings.TrimSpace(msgidValue)
			inMsgid = true
			continue
		}

		// If we're in a msgid and encounter a continuation line
		if inMsgid && strings.HasPrefix(trimmed, `"`) {
			// This is a multi-line msgid, just mark it as non-empty
			msgidValue += "continuation"
			continue
		}

		// If we encounter msgstr, finish the msgid
		if inMsgid && strings.HasPrefix(trimmed, "msgstr") {
			// Check if msgid is non-empty (not the header)
			if strings.Trim(msgidValue, `"`) != "" {
				count++
			}
			inMsgid = false
			msgidValue = ""
			continue
		}
	}

	if err := scanner.Err(); err != nil {
		return 0, fmt.Errorf("failed to scan msgattrib output: %w", err)
	}

	return count, nil
}

// ClaudeJSONOutput represents the JSON output format from Claude API.
type ClaudeJSONOutput struct {
	Type          string       `json:"type"`
	Subtype       string       `json:"subtype"`
	NumTurns      int          `json:"num_turns"`
	Result        string       `json:"result"`
	DurationAPIMS int          `json:"duration_api_ms"`
	Usage         *ClaudeUsage `json:"usage,omitempty"`
}

// ClaudeUsage represents usage information in Claude JSON output.
type ClaudeUsage struct {
	InputTokens  int `json:"input_tokens"`
	OutputTokens int `json:"output_tokens"`
}

// ClaudeSystemMessage represents a system initialization message in json format (stream-json internally).
type ClaudeSystemMessage struct {
	Type              string   `json:"type"`
	Subtype           string   `json:"subtype"`
	CWD               string   `json:"cwd"`
	SessionID         string   `json:"session_id"`
	Model             string   `json:"model"`
	Tools             []string `json:"tools,omitempty"`
	Agents            []string `json:"agents,omitempty"`
	ClaudeCodeVersion string   `json:"claude_code_version,omitempty"`
	UUID              string   `json:"uuid"`
}

// ClaudeMessageContent represents a content item in assistant message.
type ClaudeMessageContent struct {
	Type string `json:"type"`
	Text string `json:"text"`
}

// ClaudeMessage represents the message structure in assistant messages.
type ClaudeMessage struct {
	ID      string                 `json:"id"`
	Type    string                 `json:"type"`
	Role    string                 `json:"role"`
	Model   string                 `json:"model"`
	Content []ClaudeMessageContent `json:"content"`
	Usage   *ClaudeUsage           `json:"usage,omitempty"`
}

// ClaudeAssistantMessage represents an assistant message in json format (stream-json internally).
type ClaudeAssistantMessage struct {
	Type            string        `json:"type"`
	Message         ClaudeMessage `json:"message"`
	ParentToolUseID *string       `json:"parent_tool_use_id"`
	SessionID       string        `json:"session_id"`
	UUID            string        `json:"uuid"`
}

// CodexUsage represents token usage information in Codex JSON output.
type CodexUsage struct {
	InputTokens  int `json:"input_tokens"`
	OutputTokens int `json:"output_tokens"`
}

// CodexThreadStarted represents a thread.started message in Codex JSONL format.
type CodexThreadStarted struct {
	Type     string `json:"type"`
	ThreadID string `json:"thread_id"`
}

// CodexItem represents an item in Codex item.completed messages.
type CodexItem struct {
	ID   string `json:"id"`
	Type string `json:"type"`
	Text string `json:"text"`
}

// CodexItemCompleted represents an item.completed message in Codex JSONL format.
type CodexItemCompleted struct {
	Type string    `json:"type"`
	Item CodexItem `json:"item"`
}

// CodexTurnCompleted represents a turn.completed message in Codex JSONL format.
type CodexTurnCompleted struct {
	Type       string      `json:"type"`
	Usage      *CodexUsage `json:"usage,omitempty"`
	DurationMS int         `json:"duration_ms"`
}

// CodexJSONOutput represents the unified parsed information from Codex JSONL output.
type CodexJSONOutput struct {
	NumTurns      int         `json:"num_turns"`
	Usage         *CodexUsage `json:"usage,omitempty"`
	DurationAPIMS int         `json:"duration_api_ms"`
	Result        string      `json:"result"`
	ThreadID      string      `json:"thread_id"`
}

// OpenCodeUsage represents token usage information in OpenCode JSON output.
type OpenCodeUsage struct {
	InputTokens  int `json:"input_tokens"`
	OutputTokens int `json:"output_tokens"`
}

// OpenCodePart represents the part structure in OpenCode messages.
type OpenCodePart struct {
	ID        string             `json:"id"`
	SessionID string             `json:"sessionID"`
	MessageID string             `json:"messageID"`
	Type      string             `json:"type"`
	Text      string             `json:"text,omitempty"`
	Tool      string             `json:"tool,omitempty"`
	State     *OpenCodeToolState `json:"state,omitempty"`
	Tokens    *OpenCodeTokens    `json:"tokens,omitempty"`
}

// OpenCodeTokens represents token usage information in OpenCode step_finish messages.
type OpenCodeTokens struct {
	Total     int `json:"total"`
	Input     int `json:"input"`
	Output    int `json:"output"`
	Reasoning int `json:"reasoning"`
	Cache     struct {
		Read  int `json:"read"`
		Write int `json:"write"`
	} `json:"cache"`
}

// OpenCodeToolState represents the state information for tool_use messages.
type OpenCodeToolState struct {
	Status string `json:"status"`
	Input  struct {
		Command     string `json:"command,omitempty"`
		FilePath    string `json:"filePath,omitempty"`
		Description string `json:"description,omitempty"`
	} `json:"input"`
	Output string `json:"output"`
}

// OpenCodeStepStart represents a step_start message in OpenCode JSONL format.
type OpenCodeStepStart struct {
	Type      string       `json:"type"`
	SessionID string       `json:"sessionID"`
	Part      OpenCodePart `json:"part"`
}

// OpenCodeStepFinish represents a step_finish message in OpenCode JSONL format.
type OpenCodeStepFinish struct {
	Type      string       `json:"type"`
	SessionID string       `json:"sessionID"`
	Part      OpenCodePart `json:"part"`
}

// OpenCodeText represents a text message in OpenCode JSONL format.
type OpenCodeText struct {
	Type      string       `json:"type"`
	SessionID string       `json:"sessionID"`
	Part      OpenCodePart `json:"part"`
}

// OpenCodeToolUse represents a tool_use message in OpenCode JSONL format.
type OpenCodeToolUse struct {
	Type      string       `json:"type"`
	SessionID string       `json:"sessionID"`
	Part      OpenCodePart `json:"part"`
}

// OpenCodeJSONOutput represents the unified parsed information from OpenCode JSONL output.
type OpenCodeJSONOutput struct {
	NumTurns      int            `json:"num_turns"`
	Usage         *OpenCodeUsage `json:"usage,omitempty"`
	DurationAPIMS int            `json:"duration_api_ms"`
	Result        string         `json:"result"`
	SessionID     string         `json:"session_id"`
}

// ParseAgentOutput parses agent output based on the output format.
// Returns the actual content (result text) and the parsed JSON result.
// For claude, json format is treated as stream-json (JSONL format).
func ParseAgentOutput(output []byte, outputFormat string) (content []byte, result *ClaudeJSONOutput, err error) {
	// Normalize output format (convert underscores to hyphens and unify stream-json to json)
	outputFormat = normalizeOutputFormat(outputFormat)

	// Default format: return output as-is
	if outputFormat == "" || outputFormat == "default" {
		return output, nil, nil
	}

	// JSON format: parse as stream JSON (JSONL format, one JSON object per line)
	if outputFormat == "json" {
		return parseStreamJSON(output)
	}

	// Unknown format: return as-is
	log.Warnf("unknown output format: %s, treating as default", outputFormat)
	return output, nil, nil
}

// parseStreamJSON parses stream JSON format where each line is a JSON object.
func parseStreamJSON(output []byte) (content []byte, result *ClaudeJSONOutput, err error) {
	var resultBuilder strings.Builder
	var lastResult *ClaudeJSONOutput

	scanner := bufio.NewScanner(bytes.NewReader(output))
	// Increase buffer size to handle long lines (1MB initial, 10MB max)
	buf := make([]byte, 0, 1024*1024)
	scanner.Buffer(buf, 10*1024*1024) // Max token size: 10MB
	for scanner.Scan() {
		line := strings.TrimSpace(scanner.Text())
		if line == "" {
			continue
		}

		var jsonOutput ClaudeJSONOutput
		if err := json.Unmarshal([]byte(line), &jsonOutput); err != nil {
			// If line is not valid JSON, treat it as plain text
			resultBuilder.WriteString(line)
			resultBuilder.WriteString("\n")
			continue
		}

		// Accumulate result text
		if jsonOutput.Result != "" {
			resultBuilder.WriteString(jsonOutput.Result)
		}

		// Keep the latest JSON output (contains all fields including usage and duration_api_ms)
		lastResult = &jsonOutput
	}

	if err := scanner.Err(); err != nil {
		return output, nil, fmt.Errorf("failed to parse stream JSON: %w", err)
	}

	return []byte(resultBuilder.String()), lastResult, nil
}

// ParseStreamJSONRealtime parses stream JSON format in real-time, displaying messages as they arrive.
// It reads from the provided reader line by line, parses each JSON object, and displays
// system, assistant, and result messages in real-time.
// Returns the final result message and accumulated result text.
func ParseStreamJSONRealtime(reader io.Reader) (content []byte, result *ClaudeJSONOutput, err error) {
	var resultBuilder strings.Builder
	var lastResult *ClaudeJSONOutput

	scanner := bufio.NewScanner(reader)
	// Increase buffer size to handle long lines (1MB initial, 10MB max)
	buf := make([]byte, 0, 1024*1024)
	scanner.Buffer(buf, 10*1024*1024) // Max token size: 10MB
	for scanner.Scan() {
		line := strings.TrimSpace(scanner.Text())
		if line == "" {
			continue
		}

		// Try to parse as JSON to determine message type
		var baseMsg struct {
			Type string `json:"type"`
		}
		if err := json.Unmarshal([]byte(line), &baseMsg); err != nil {
			// If line is not valid JSON, treat it as plain text
			log.Debugf("stream-json: non-JSON line: %s", line)
			resultBuilder.WriteString(line)
			resultBuilder.WriteString("\n")
			fmt.Println(line)
			continue
		}

		// Parse based on message type
		switch baseMsg.Type {
		case "system":
			var sysMsg ClaudeSystemMessage
			if err := json.Unmarshal([]byte(line), &sysMsg); err == nil {
				printSystemMessage(&sysMsg)
			} else {
				log.Debugf("stream-json: failed to parse system message: %v", err)
			}
		case "assistant":
			var asstMsg ClaudeAssistantMessage
			if err := json.Unmarshal([]byte(line), &asstMsg); err == nil {
				printAssistantMessage(&asstMsg, &resultBuilder)
			} else {
				log.Debugf("stream-json: failed to parse assistant message: %v", err)
			}
		case "result":
			var resultMsg ClaudeJSONOutput
			if err := json.Unmarshal([]byte(line), &resultMsg); err == nil {
				// Print result parsing process
				resultSize := len(resultMsg.Result)
				printResultParsing(&resultMsg, resultSize)
				// Merge usage information: prefer the result with more complete usage info
				if lastResult == nil {
					lastResult = &resultMsg
				} else {
					// Merge usage information if the new result has it
					if resultMsg.Usage != nil && (resultMsg.Usage.InputTokens > 0 || resultMsg.Usage.OutputTokens > 0) {
						if lastResult.Usage == nil {
							lastResult.Usage = resultMsg.Usage
						} else {
							// Use the values from the new result if they are non-zero
							if resultMsg.Usage.InputTokens > 0 {
								lastResult.Usage.InputTokens = resultMsg.Usage.InputTokens
							}
							if resultMsg.Usage.OutputTokens > 0 {
								lastResult.Usage.OutputTokens = resultMsg.Usage.OutputTokens
							}
						}
					}
					// Always update duration_api_ms with the latest value
					if resultMsg.DurationAPIMS > 0 {
						lastResult.DurationAPIMS = resultMsg.DurationAPIMS
					}
					// Update result text if present
					if resultMsg.Result != "" {
						lastResult.Result = resultMsg.Result
					}
					// Merge NumTurns: use the maximum value
					if resultMsg.NumTurns > lastResult.NumTurns {
						lastResult.NumTurns = resultMsg.NumTurns
					}
				}
				printResultMessage(&resultMsg, &resultBuilder)
			} else {
				log.Debugf("stream-json: failed to parse result message: %v", err)
			}
		case "user":
			// User messages are typically tool results or intermediate messages
			// Log at debug level but don't display to avoid cluttering output
			log.Debugf("stream-json: received user message (suppressed from output)")
		default:
			// Unknown type, log at debug level and output as-is
			log.Debugf("stream-json: unknown message type: %s", baseMsg.Type)
			resultBuilder.WriteString(line)
			resultBuilder.WriteString("\n")
			fmt.Println(line)
		}
	}

	if err := scanner.Err(); err != nil {
		return []byte(resultBuilder.String()), lastResult, fmt.Errorf("failed to parse stream JSON: %w", err)
	}

	return []byte(resultBuilder.String()), lastResult, nil
}

// printSystemMessage displays system initialization information.
func printSystemMessage(msg *ClaudeSystemMessage) {
	fmt.Println()
	fmt.Println("🤖 System Initialization")
	fmt.Println("==========================================")
	if msg.SessionID != "" {
		fmt.Printf("**Session ID:** %s\n", msg.SessionID)
	}
	if msg.Model != "" {
		fmt.Printf("**Model:** %s\n", msg.Model)
	}
	if msg.CWD != "" {
		fmt.Printf("**Working Dir:** %s\n", msg.CWD)
	}
	if msg.ClaudeCodeVersion != "" {
		fmt.Printf("**Version:** %s\n", msg.ClaudeCodeVersion)
	}
	if len(msg.Tools) > 0 {
		fmt.Printf("**Tools:** %d\n", len(msg.Tools))
	}
	if len(msg.Agents) > 0 {
		fmt.Printf("**Agents:** %d\n", len(msg.Agents))
	}
	fmt.Println("==========================================")
	fmt.Println()
}

// truncateText truncates text to maxBytes bytes, appending "..." if truncated.
func truncateText(text string, maxBytes int) string {
	if len(text) <= maxBytes {
		return text
	}
	// Truncate to maxBytes - 3 to leave room for "..."
	truncated := text[:maxBytes-3]
	return truncated + "..."
}

// printAssistantMessage displays assistant message content, printing each text block on a separate line.
func printAssistantMessage(msg *ClaudeAssistantMessage, resultBuilder *strings.Builder) {
	if msg.Message.Content == nil {
		return
	}

	for _, content := range msg.Message.Content {
		if content.Type == "text" && content.Text != "" {
			// Truncate text to 4KB for display
			displayText := truncateText(content.Text, maxDisplayBytes)
			// Print agent marker with robot emoji at the beginning of agent output
			fmt.Print("🤖 ")
			fmt.Println(displayText)
			resultBuilder.WriteString(content.Text)
		}
	}
}

// printResultParsing displays the parsing process of a result message.
func printResultParsing(msg *ClaudeJSONOutput, resultSize int) {
	fmt.Printf("🤖 return result (%d bytes)\n", resultSize)
}

// printResultMessage displays the final result message.
func printResultMessage(msg *ClaudeJSONOutput, resultBuilder *strings.Builder) {
	if msg.Result != "" {
		fmt.Println()
		fmt.Println("✅ Final Result")
		fmt.Println("==========================================")
		// Print result text (may be multi-line)
		lines := strings.Split(msg.Result, "\n")
		for _, line := range lines {
			if line != "" {
				fmt.Println(line)
			}
		}
		fmt.Println("==========================================")
		resultBuilder.WriteString(msg.Result)
	}
}

// PrintAgentDiagnostics prints diagnostic information in a beautiful format.
// It accepts either ClaudeJSONOutput or CodexJSONOutput.
func PrintAgentDiagnostics(result interface{}) {
	var numTurns int
	var inputTokens, outputTokens int
	var durationAPIMS int
	hasInfo := false

	// Extract information based on type
	switch r := result.(type) {
	case *ClaudeJSONOutput:
		if r == nil {
			return
		}
		if r.NumTurns > 0 {
			numTurns = r.NumTurns
			hasInfo = true
		}
		if r.Usage != nil {
			if r.Usage.InputTokens > 0 {
				inputTokens = r.Usage.InputTokens
				hasInfo = true
			}
			if r.Usage.OutputTokens > 0 {
				outputTokens = r.Usage.OutputTokens
				hasInfo = true
			}
		}
		if r.DurationAPIMS > 0 {
			durationAPIMS = r.DurationAPIMS
			hasInfo = true
		}
	case *CodexJSONOutput:
		if r == nil {
			return
		}
		if r.NumTurns > 0 {
			numTurns = r.NumTurns
			hasInfo = true
		}
		if r.Usage != nil {
			if r.Usage.InputTokens > 0 {
				inputTokens = r.Usage.InputTokens
				hasInfo = true
			}
			if r.Usage.OutputTokens > 0 {
				outputTokens = r.Usage.OutputTokens
				hasInfo = true
			}
		}
		if r.DurationAPIMS > 0 {
			durationAPIMS = r.DurationAPIMS
			hasInfo = true
		}
	case *OpenCodeJSONOutput:
		if r == nil {
			return
		}
		if r.NumTurns > 0 {
			numTurns = r.NumTurns
			hasInfo = true
		}
		if r.Usage != nil {
			if r.Usage.InputTokens > 0 {
				inputTokens = r.Usage.InputTokens
				hasInfo = true
			}
			if r.Usage.OutputTokens > 0 {
				outputTokens = r.Usage.OutputTokens
				hasInfo = true
			}
		}
		if r.DurationAPIMS > 0 {
			durationAPIMS = r.DurationAPIMS
			hasInfo = true
		}
	default:
		return
	}

	if !hasInfo {
		return
	}

	fmt.Println()
	fmt.Println("📊 Agent Diagnostics")
	fmt.Println("==========================================")
	if numTurns > 0 {
		fmt.Printf("**Num turns:** %d\n", numTurns)
	}
	if inputTokens > 0 {
		fmt.Printf("**Input tokens:** %d\n", inputTokens)
	}
	if outputTokens > 0 {
		fmt.Printf("**Output tokens:** %d\n", outputTokens)
	}
	if durationAPIMS > 0 {
		durationSec := float64(durationAPIMS) / 1000.0
		fmt.Printf("**API duration:** %.2f s\n", durationSec)
	}
	fmt.Println("==========================================")
}

// ParseCodexJSONLRealtime parses Codex JSONL format in real-time, displaying messages as they arrive.
// It reads from the provided reader line by line, parses each JSON object, and displays
// thread.started, item.completed (agent_message), and turn.completed messages in real-time.
// Returns the final result and accumulated result text.
func ParseCodexJSONLRealtime(reader io.Reader) (content []byte, result *CodexJSONOutput, err error) {
	var lastResult *CodexJSONOutput
	var lastAgentMessage string
	startTime := time.Now()

	scanner := bufio.NewScanner(reader)
	// Increase buffer size to handle long lines (1MB initial, 10MB max)
	buf := make([]byte, 0, 1024*1024)
	scanner.Buffer(buf, 10*1024*1024) // Max token size: 10MB
	for scanner.Scan() {
		line := strings.TrimSpace(scanner.Text())
		if line == "" {
			continue
		}

		// Try to parse as JSON to determine message type
		var baseMsg struct {
			Type string `json:"type"`
		}
		if err := json.Unmarshal([]byte(line), &baseMsg); err != nil {
			// If line is not valid JSON, log at debug level only
			log.Debugf("codex-json: non-JSON lines, error: %s", err)
			fmt.Print("❓ ")
			fmt.Println(line)
			continue
		}

		// Parse based on message type
		switch baseMsg.Type {
		case "thread.started":
			var threadMsg CodexThreadStarted
			if err := json.Unmarshal([]byte(line), &threadMsg); err == nil {
				printCodexThreadStarted(&threadMsg)
				// Initialize result if needed
				if lastResult == nil {
					lastResult = &CodexJSONOutput{}
				}
				lastResult.ThreadID = threadMsg.ThreadID
			} else {
				log.Debugf("codex-json: failed to parse thread.started message: %v", err)
			}
		case "item.completed":
			var itemMsg CodexItemCompleted
			if err := json.Unmarshal([]byte(line), &itemMsg); err == nil {
				// Only display and count agent_message items
				if itemMsg.Item.Type == "agent_message" {
					// Increment NumTurns
					if lastResult == nil {
						lastResult = &CodexJSONOutput{}
					}
					lastResult.NumTurns++
					// Display message text
					printCodexAgentMessage(&itemMsg.Item, nil)
					// Store the last agent message (replace, don't accumulate)
					lastAgentMessage = itemMsg.Item.Text
				} else {
					// Log other item types at debug level
					log.Debugf("codex-json: received item.completed with type=%s (suppressed from output)", itemMsg.Item.Type)
				}
			} else {
				log.Debugf("codex-json: failed to parse item.completed message: %v", err)
			}
		case "turn.completed":
			var turnMsg CodexTurnCompleted
			if err := json.Unmarshal([]byte(line), &turnMsg); err == nil {
				// Merge usage information: prefer the result with more complete usage info
				if lastResult == nil {
					lastResult = &CodexJSONOutput{}
				}
				// Merge usage information if the new turn has it
				if turnMsg.Usage != nil && (turnMsg.Usage.InputTokens > 0 || turnMsg.Usage.OutputTokens > 0) {
					if lastResult.Usage == nil {
						lastResult.Usage = turnMsg.Usage
					} else {
						// Use the values from the new turn if they are non-zero
						if turnMsg.Usage.InputTokens > 0 {
							lastResult.Usage.InputTokens = turnMsg.Usage.InputTokens
						}
						if turnMsg.Usage.OutputTokens > 0 {
							lastResult.Usage.OutputTokens = turnMsg.Usage.OutputTokens
						}
					}
				}
				// Always update duration_api_ms with the latest value
				if turnMsg.DurationMS > 0 {
					lastResult.DurationAPIMS = turnMsg.DurationMS
				} else {
					// If DurationMS is 0, calculate elapsed time from function start
					elapsed := time.Since(startTime)
					lastResult.DurationAPIMS = int(elapsed.Milliseconds())
				}
				// Log turn completion at debug level
				log.Debugf("codex-json: received turn.completed (suppressed from output)")
			} else {
				log.Debugf("codex-json: failed to parse turn.completed message: %v", err)
			}
		default:
			// Unknown type, log at debug level only
			log.Debugf("codex-json: unknown message type: %s", baseMsg.Type)
		}
	}

	if err := scanner.Err(); err != nil {
		return []byte(lastAgentMessage), lastResult, fmt.Errorf("failed to parse codex JSONL: %w", err)
	}

	// Only return the last agent message
	return []byte(lastAgentMessage), lastResult, nil
}

// printCodexThreadStarted displays thread initialization information.
func printCodexThreadStarted(msg *CodexThreadStarted) {
	fmt.Println()
	fmt.Println("🤖 Session Started")
	fmt.Println("==========================================")
	if msg.ThreadID != "" {
		fmt.Printf("**Thread ID:** %s\n", msg.ThreadID)
	}
	fmt.Println("==========================================")
	fmt.Println()
}

// printCodexAgentMessage displays agent message content.
func printCodexAgentMessage(item *CodexItem, resultBuilder *strings.Builder) {
	if item.Text != "" {
		// Truncate text to 4KB for display
		displayText := truncateText(item.Text, maxDisplayBytes)
		// Print agent marker with robot emoji at the beginning of agent output
		fmt.Print("🤖 ")
		fmt.Println(displayText)
		// Note: resultBuilder is not used here, we only store the last message
	}
}

// ParseOpenCodeJSONLRealtime parses OpenCode JSONL format in real-time, displaying messages as they arrive.
// It reads from the provided reader line by line, parses each JSON object, and displays
// step_start, text, tool_use, and step_finish messages in real-time.
// Returns the final result and accumulated result text.
func ParseOpenCodeJSONLRealtime(reader io.Reader) (content []byte, result *OpenCodeJSONOutput, err error) {
	var resultBuilder strings.Builder
	var lastResult *OpenCodeJSONOutput
	var inStep bool
	startTime := time.Now()
	const maxToolOutputBytes = 2048 // 2KB limit for tool output

	scanner := bufio.NewScanner(reader)
	// Increase buffer size to handle long lines (1MB initial, 10MB max)
	buf := make([]byte, 0, 1024*1024)
	scanner.Buffer(buf, 10*1024*1024) // Max token size: 10MB
	for scanner.Scan() {
		line := strings.TrimSpace(scanner.Text())
		if line == "" {
			continue
		}

		// Try to parse as JSON to determine message type
		var baseMsg struct {
			Type string `json:"type"`
		}
		if err := json.Unmarshal([]byte(line), &baseMsg); err != nil {
			// If line is not valid JSON, log at debug level only
			log.Debugf("opencode-json: non-JSON lines, error: %s", err)
			fmt.Print("❓ ")
			fmt.Println(line)
			continue
		}

		// Parse based on message type
		switch baseMsg.Type {
		case "step_start":
			var stepMsg OpenCodeStepStart
			if err := json.Unmarshal([]byte(line), &stepMsg); err == nil {
				// Initialize result if needed
				if lastResult == nil {
					lastResult = &OpenCodeJSONOutput{}
				}
				lastResult.SessionID = stepMsg.SessionID
				// Increment NumTurns when step starts
				lastResult.NumTurns++
				inStep = true
				log.Debugf("opencode-json: received step_start (turn %d)", lastResult.NumTurns)
			} else {
				log.Debugf("opencode-json: failed to parse step_start message: %v", err)
			}
		case "step_finish":
			var stepMsg OpenCodeStepFinish
			if err := json.Unmarshal([]byte(line), &stepMsg); err == nil {
				if lastResult == nil {
					lastResult = &OpenCodeJSONOutput{}
				}
				// Extract token usage information
				if stepMsg.Part.Tokens != nil {
					if lastResult.Usage == nil {
						lastResult.Usage = &OpenCodeUsage{}
					}
					// Use total tokens as input tokens, output tokens from the tokens structure
					if stepMsg.Part.Tokens.Total > 0 {
						// For opencode, we use total as a reference, but extract input/output separately
						lastResult.Usage.InputTokens = stepMsg.Part.Tokens.Input
						lastResult.Usage.OutputTokens = stepMsg.Part.Tokens.Output
					}
				}
				// Calculate duration if not provided
				elapsed := time.Since(startTime)
				lastResult.DurationAPIMS = int(elapsed.Milliseconds())
				inStep = false
				log.Debugf("opencode-json: received step_finish (suppressed from output)")
			} else {
				log.Debugf("opencode-json: failed to parse step_finish message: %v", err)
			}
		case "text":
			if !inStep {
				log.Debugf("opencode-json: received text message outside of step (suppressed)")
				continue
			}
			var textMsg OpenCodeText
			if err := json.Unmarshal([]byte(line), &textMsg); err == nil {
				printOpenCodeText(&textMsg, &resultBuilder)
			} else {
				log.Debugf("opencode-json: failed to parse text message: %v", err)
			}
		case "tool_use":
			if !inStep {
				log.Debugf("opencode-json: received tool_use message outside of step (suppressed)")
				continue
			}
			var toolMsg OpenCodeToolUse
			if err := json.Unmarshal([]byte(line), &toolMsg); err == nil {
				printOpenCodeToolUse(&toolMsg, &resultBuilder, maxToolOutputBytes)
			} else {
				log.Debugf("opencode-json: failed to parse tool_use message: %v", err)
			}
		default:
			// Unknown type, only display if in step
			if inStep {
				log.Debugf("opencode-json: unknown message type: %s", baseMsg.Type)
				resultBuilder.WriteString(line)
				resultBuilder.WriteString("\n")
				fmt.Println(line)
			} else {
				log.Debugf("opencode-json: unknown message type outside of step: %s (suppressed)", baseMsg.Type)
			}
		}
	}

	if err := scanner.Err(); err != nil {
		return []byte(resultBuilder.String()), lastResult, fmt.Errorf("failed to parse opencode JSONL: %w", err)
	}

	return []byte(resultBuilder.String()), lastResult, nil
}

// printOpenCodeText displays text message content.
func printOpenCodeText(msg *OpenCodeText, resultBuilder *strings.Builder) {
	if msg.Part.Text != "" {
		// Truncate text to 4KB for display
		displayText := truncateText(msg.Part.Text, maxDisplayBytes)
		// Print agent marker with robot emoji at the beginning of agent output
		fmt.Print("🤖 ")
		fmt.Println(displayText)
		resultBuilder.WriteString(msg.Part.Text)
	}
}

// printOpenCodeToolUse displays tool use message content.
func printOpenCodeToolUse(msg *OpenCodeToolUse, resultBuilder *strings.Builder, maxOutputBytes int) {
	if msg.Part.State == nil {
		return
	}

	// Display tool type and command
	toolType := msg.Part.Tool
	if toolType == "" {
		toolType = "unknown"
	}

	command := ""
	if msg.Part.State.Input.Command != "" {
		command = msg.Part.State.Input.Command
	} else if msg.Part.State.Input.FilePath != "" {
		command = msg.Part.State.Input.FilePath
	}

	// Print tool header
	if command != "" {
		fmt.Printf("🔧 %s: %s\n", toolType, command)
		resultBuilder.WriteString(fmt.Sprintf("%s: %s\n", toolType, command))
	} else {
		fmt.Printf("🔧 %s\n", toolType)
		resultBuilder.WriteString(fmt.Sprintf("%s\n", toolType))
	}

	// Display output (truncated to maxOutputBytes)
	if msg.Part.State.Output != "" {
		outputText := truncateText(msg.Part.State.Output, maxOutputBytes)
		fmt.Println(outputText)
		resultBuilder.WriteString(msg.Part.State.Output)
	}
}
