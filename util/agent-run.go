// Package util provides business logic for agent-run command.
package util

import (
	"bufio"
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"math"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"time"

	"gopkg.in/yaml.v3"

	"github.com/git-l10n/git-po-helper/config"
	"github.com/git-l10n/git-po-helper/repository"
	log "github.com/sirupsen/logrus"
)

// AgentRunResult holds the result of a single agent-run execution.
type AgentRunResult struct {
	PreValidationPass     bool
	PostValidationPass    bool
	AgentExecuted         bool
	AgentSuccess          bool
	PreValidationError    string
	PostValidationError   string
	AgentError            string
	BeforeCount           int
	AfterCount            int
	BeforeNewCount        int // For translate: new (untranslated) entries before
	AfterNewCount         int // For translate: new (untranslated) entries after
	BeforeFuzzyCount      int // For translate: fuzzy entries before
	AfterFuzzyCount       int // For translate: fuzzy entries after
	SyntaxValidationPass  bool
	SyntaxValidationError string
	Score                 int // 0-100, calculated based on validations

	// Review-specific fields
	ReviewJSON       *ReviewJSONResult `json:"review_json,omitempty"`
	ReviewScore      int               `json:"review_score,omitempty"`
	ReviewJSONPath   string            `json:"review_json_path,omitempty"`
	ReviewedFilePath string            `json:"reviewed_file_path,omitempty"` // Final reviewed PO file path

	// Agent output (for saving logs in agent-test)
	AgentStdout []byte `json:"-"`
	AgentStderr []byte `json:"-"`

	// Agent diagnostics
	NumTurns      int           // Number of turns in the conversation
	ExecutionTime time.Duration // Execution time for this run
}

// ReviewIssue represents a single issue in a review JSON result.
type ReviewIssue struct {
	MsgID       string `json:"msgid"`
	MsgStr      string `json:"msgstr"`
	Score       int    `json:"score"`
	Description string `json:"description"`
	Suggestion  string `json:"suggestion"`
}

// ReviewJSONResult represents the overall review JSON format produced by an agent.
type ReviewJSONResult struct {
	TotalEntries int           `json:"total_entries"`
	Issues       []ReviewIssue `json:"issues"`
}

// CalculateReviewScore calculates a 0-100 score from a ReviewJSONResult.
// The scoring model treats each entry as having a maximum of 3 points.
// For each reported issue, the score is reduced by (3 - issue.Score).
// The final score is normalized to 0-100.
func CalculateReviewScore(review *ReviewJSONResult) (int, error) {
	// If total_entries is 0, we can't calculate a meaningful score
	// This might happen if the calculation hasn't been performed yet
	if review.TotalEntries <= 0 {
		// If there are no entries, and no issues, we can consider it as perfect
		if len(review.Issues) == 0 {
			log.Debugf("no entries and no issues, returning perfect score of 100")
			return 100, nil
		}
		// If there are issues but no entries, this is an inconsistent state
		log.Debugf("calculate score failed: total_entries=%d but has %d issues", review.TotalEntries, len(review.Issues))
		return 0, fmt.Errorf("invalid review result: total_entries must be greater than 0, got %d", review.TotalEntries)
	}

	totalPossible := review.TotalEntries * 3
	totalScore := totalPossible

	log.Debugf("calculating review score: total_entries=%d, total_possible=%d, issues_count=%d",
		review.TotalEntries, totalPossible, len(review.Issues))

	for i, issue := range review.Issues {
		if issue.Score < 0 || issue.Score > 3 {
			log.Debugf("calculate score failed: issue[%d].score=%d (must be 0-3)", i, issue.Score)
			return 0, fmt.Errorf("invalid issue score %d: must be between 0 and 3", issue.Score)
		}
		deduction := 3 - issue.Score
		totalScore -= deduction
		log.Debugf("issue[%d]: score=%d, deduction=%d, remaining=%d", i, issue.Score, deduction, totalScore)
	}

	if totalScore < 0 {
		log.Debugf("total score is negative (%d), clamping to 0", totalScore)
		totalScore = 0
	}

	scorePercent := int(math.Round(float64(totalScore) * 100.0 / float64(totalPossible)))
	if scorePercent < 0 {
		scorePercent = 0
	} else if scorePercent > 100 {
		scorePercent = 100
	}

	log.Debugf("review score calculated: %d/100 (total_score=%d, total_possible=%d)",
		scorePercent, totalScore, totalPossible)

	return scorePercent, nil
}

// ExtractJSONFromOutput extracts a JSON object from agent output.
// It searches for JSON object boundaries ({ and }) and handles cases where
// output contains other text before/after JSON.
// Returns the JSON bytes or an error if not found.
func ExtractJSONFromOutput(output []byte) ([]byte, error) {
	if len(output) == 0 {
		log.Debugf("agent output is empty, cannot extract JSON")
		return nil, fmt.Errorf("empty output, no JSON found")
	}

	log.Debugf("extracting JSON from agent output (length: %d bytes)", len(output))

	// Find the first '{' character
	startIdx := -1
	for i, b := range output {
		if b == '{' {
			startIdx = i
			break
		}
	}

	if startIdx == -1 {
		log.Debugf("no opening brace found in agent output")
		return nil, fmt.Errorf("no JSON object found in output (missing opening brace)")
	}

	log.Debugf("found JSON start at position %d", startIdx)

	// Find the matching closing '}' by counting braces
	braceCount := 0
	endIdx := -1
	for i := startIdx; i < len(output); i++ {
		if output[i] == '{' {
			braceCount++
		} else if output[i] == '}' {
			braceCount--
			if braceCount == 0 {
				endIdx = i
				break
			}
		}
	}

	if endIdx == -1 {
		log.Debugf("no matching closing brace found (unclosed JSON object)")
		return nil, fmt.Errorf("no complete JSON object found in output (missing closing brace)")
	}

	log.Debugf("found JSON end at position %d (extracted %d bytes)", endIdx, endIdx-startIdx+1)

	// Extract JSON bytes
	jsonBytes := output[startIdx : endIdx+1]
	return jsonBytes, nil
}

// ParseReviewJSON parses JSON output from agent and validates the structure.
// It validates that the JSON matches ReviewJSONResult structure and that
// all score values are in the valid range (0-3).
// Returns parsed result or error.
func ParseReviewJSON(jsonData []byte) (*ReviewJSONResult, error) {
	if len(jsonData) == 0 {
		log.Debugf("JSON data is empty")
		return nil, fmt.Errorf("empty JSON data")
	}

	log.Debugf("parsing JSON data (length: %d bytes)", len(jsonData))

	var review ReviewJSONResult
	if err := json.Unmarshal(jsonData, &review); err != nil {
		log.Debugf("JSON unmarshal failed: %v", err)
		return nil, fmt.Errorf("failed to parse JSON: %w", err)
	}

	log.Debugf("JSON parsed successfully: total_entries=%d, issues_count=%d", review.TotalEntries, len(review.Issues))

	// Note: We allow total_entries to be 0 here because it will be recalculated later
	// from the actual reviewInputPath file to ensure accuracy.
	// The validation of total_entries > 0 will happen after recalculation if needed.

	// Validate issues array
	if review.Issues == nil {
		// Issues can be an empty array, but not nil
		log.Debugf("issues array is nil, initializing as empty array")
		review.Issues = []ReviewIssue{}
	}

	// Validate each issue
	for i, issue := range review.Issues {
		// Validate score range
		if issue.Score < 0 || issue.Score > 3 {
			log.Debugf("validation failed: issue[%d].score=%d (must be 0-3)", i, issue.Score)
			return nil, fmt.Errorf("invalid issue score %d at index %d: must be between 0 and 3", issue.Score, i)
		}

		// Validate required fields are not empty (msgid and msgstr can be empty, but should be present)
		// Description and suggestion should not be empty for issues
		if issue.Description == "" {
			log.Debugf("validation failed: issue[%d].description is empty", i)
			return nil, fmt.Errorf("invalid issue at index %d: description is required", i)
		}

		log.Debugf("issue[%d]: msgid=%q, score=%d, description=%q", i, issue.MsgID, issue.Score, issue.Description)
	}

	log.Debugf("JSON validation passed: %d total entries, %d issues", review.TotalEntries, len(review.Issues))
	return &review, nil
}

// getRelativePath converts an absolute path to a path relative to the repository root.
// If conversion fails, returns the original absolute path as fallback.
func getRelativePath(absPath string) string {
	if absPath == "" {
		return ""
	}
	relPath, err := filepath.Rel(repository.WorkDir(), absPath)
	if err != nil {
		return absPath // fallback to absolute path
	}
	return relPath
}

// SaveReviewJSON saves review JSON result to file.
// It determines the output path from the PO file path:
// po/XX.po -> po/XX-reviewed.json (where XX is the language code).
// Creates directory if needed, writes JSON with proper formatting.
// Returns the file path or error.
func SaveReviewJSON(poFile string, review *ReviewJSONResult) (string, error) {
	if review == nil {
		return "", fmt.Errorf("review result is nil")
	}

	// Determine output file path from PO file path
	// Example: po/zh_CN.po -> po/zh_CN-reviewed.json
	poFileName := filepath.Base(poFile)
	langCode := strings.TrimSuffix(poFileName, ".po")
	if langCode == "" || langCode == poFileName {
		return "", fmt.Errorf("invalid PO file path: %s (expected format: po/XX.po)", poFile)
	}

	// Build output path: po/XX-reviewed.json
	workDir := repository.WorkDir()
	outputPath := filepath.Join(workDir, PoDir, fmt.Sprintf("%s-reviewed.json", langCode))

	log.Debugf("saving review JSON to %s", outputPath)

	// Create po/ directory if it doesn't exist
	poDir := filepath.Join(workDir, PoDir)
	if err := os.MkdirAll(poDir, 0755); err != nil {
		return "", fmt.Errorf("failed to create directory %s: %w", poDir, err)
	}

	// Marshal JSON with indentation for readability
	jsonData, err := json.MarshalIndent(review, "", "  ")
	if err != nil {
		return "", fmt.Errorf("failed to marshal JSON: %w", err)
	}

	// Add newline at end of file
	jsonData = append(jsonData, '\n')

	// Write JSON to file
	if err := os.WriteFile(outputPath, jsonData, 0644); err != nil {
		return "", fmt.Errorf("failed to write JSON file %s: %w", outputPath, err)
	}

	log.Infof("review JSON saved to %s", outputPath)
	return outputPath, nil
}

// ValidatePotEntryCount validates the entry count in a POT file.
// If expectedCount is nil or 0, validation is disabled and the function returns nil.
// Otherwise, it counts entries using CountPotEntries() and compares with expectedCount.
// Returns an error if counts don't match, nil if they match or validation is disabled.
// The stage parameter is used for error messages ("before update" or "after update").
// For "before update" stage, if the file doesn't exist, the entry count is treated as 0.
func ValidatePotEntryCount(potFile string, expectedCount *int, stage string) error {
	// If expectedCount is nil or 0, validation is disabled
	if expectedCount == nil || *expectedCount == 0 {
		return nil
	}

	// Check if file exists
	fileExists := Exist(potFile)
	var actualCount int
	var err error

	if !fileExists {
		// For "before update" stage, treat missing file as 0 entries
		if stage == "before update" {
			actualCount = 0
			log.Debugf("file %s does not exist, treating entry count as 0 for %s validation", potFile, stage)
		} else {
			// For "after update" stage, file should exist
			return fmt.Errorf("file does not exist %s: %s\nHint: The agent should have created the file", stage, potFile)
		}
	} else {
		// Count entries in POT file
		actualCount, err = CountPotEntries(potFile)
		if err != nil {
			return fmt.Errorf("failed to count entries %s in %s: %w", stage, potFile, err)
		}
	}

	// Compare with expected count
	if actualCount != *expectedCount {
		return fmt.Errorf("entry count %s: expected %d, got %d (file: %s)", stage, *expectedCount, actualCount, potFile)
	}

	log.Debugf("entry count %s validation passed: %d entries", stage, actualCount)
	return nil
}

// ValidatePoEntryCount validates the entry count in a PO file.
// If expectedCount is nil or 0, validation is disabled and the function returns nil.
// Otherwise, it counts entries using CountPoEntries() and compares with expectedCount.
// Returns an error if counts don't match, nil if they match or validation is disabled.
// The stage parameter is used for error messages ("before update" or "after update").
// For "before update" stage, if the file doesn't exist, the entry count is treated as 0.
func ValidatePoEntryCount(poFile string, expectedCount *int, stage string) error {
	// If expectedCount is nil or 0, validation is disabled
	if expectedCount == nil || *expectedCount == 0 {
		return nil
	}

	// Check if file exists
	fileExists := Exist(poFile)
	var actualCount int
	var err error

	if !fileExists {
		// For "before update" stage, treat missing file as 0 entries
		if stage == "before update" {
			actualCount = 0
			log.Debugf("file %s does not exist, treating entry count as 0 for %s validation", poFile, stage)
		} else {
			// For "after update" stage, file should exist
			return fmt.Errorf("file does not exist %s: %s\nHint: The agent should have created the file", stage, poFile)
		}
	} else {
		// Count entries in PO file
		actualCount, err = CountPoEntries(poFile)
		if err != nil {
			return fmt.Errorf("failed to count entries %s in %s: %w", stage, poFile, err)
		}
	}

	// Compare with expected count
	if actualCount != *expectedCount {
		return fmt.Errorf("entry count %s: expected %d, got %d (file: %s)", stage, *expectedCount, actualCount, poFile)
	}

	log.Debugf("entry count %s validation passed: %d entries", stage, actualCount)
	return nil
}

// ValidatePoFile validates POT/PO file syntax.
// For .pot files, it uses msgcat --use-first to validate (since POT files have placeholders in headers).
// For .po files, it uses msgfmt to validate.
// Returns an error if the file is invalid, nil if valid.
// If the file path is absolute, it doesn't require repository context.
// If the file path is relative, it uses repository.WorkDir() as the working directory.
func ValidatePoFile(potFile string) error {
	return validatePoFileInternal(potFile, false)
}

// ValidatePoFileFormat validates POT/PO file format syntax only (using --check-format for PO files).
// This is a more lenient check that doesn't require complete headers.
// For .pot files, it uses msgcat --use-first to validate.
// For .po files, it uses msgfmt --check-format to validate (only checks format, not completeness).
// Returns an error if the file format is invalid, nil if valid.
// If the file path is absolute, it doesn't require repository context.
// If the file path is relative, it uses repository.WorkDir() as the working directory.
func ValidatePoFileFormat(potFile string) error {
	return validatePoFileInternal(potFile, true)
}

// validatePoFileInternal is the internal implementation for PO/POT file validation.
// checkFormatOnly: if true, uses --check-format for PO files (more lenient, only checks format).
//
//	if false, uses --check for PO files (stricter, checks format and completeness).
func validatePoFileInternal(potFile string, checkFormatOnly bool) error {
	if !Exist(potFile) {
		return fmt.Errorf("POT file does not exist: %s\nHint: Ensure the file exists or run the agent to create it", potFile)
	}

	// Determine file extension to choose the appropriate validation tool
	ext := filepath.Ext(potFile)
	var cmd *exec.Cmd
	var toolName string

	if ext == ".pot" {
		// For POT files, use msgcat --use-first since POT files have placeholders in headers
		toolName = "msgcat"
		log.Debugf("running msgcat --use-first on %s", potFile)
		cmd = exec.Command("msgcat",
			"--use-first",
			potFile,
			"-o",
			os.DevNull)
	} else {
		// For PO files, use msgfmt
		toolName = "msgfmt"
		if checkFormatOnly {
			log.Debugf("running msgfmt --check-format on %s", potFile)
			cmd = exec.Command("msgfmt",
				"-o",
				os.DevNull,
				"--check-format",
				potFile)
		} else {
			log.Debugf("running msgfmt --check on %s", potFile)
			cmd = exec.Command("msgfmt",
				"-o",
				os.DevNull,
				"--check",
				potFile)
		}
	}

	// Only set working directory if file path is relative
	// For absolute paths, we don't need repository context
	if filepath.IsAbs(potFile) {
		// For absolute paths, use the directory containing the file as working directory
		cmd.Dir = filepath.Dir(potFile)
	} else {
		// For relative paths, use repository working directory
		cmd.Dir = repository.WorkDir()
	}

	// Capture stderr for error messages
	stderr, err := cmd.StderrPipe()
	if err != nil {
		return fmt.Errorf("failed to create stderr pipe for %s: %w", toolName, err)
	}

	if err := cmd.Start(); err != nil {
		return fmt.Errorf("failed to start %s command: %w\nHint: Ensure gettext tools (%s) are installed", toolName, err, toolName)
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
		return fmt.Errorf("file syntax validation failed: %s\nHint: Check the file syntax and fix any errors reported by %s", errorMsg, toolName)
	}

	log.Debugf("file validation passed: %s", potFile)
	return nil
}

// GetPoFileAbsPath determines the absolute path of a PO file.
// If poFile is empty, it uses cfg.DefaultLangCode to construct the path.
// If poFile is provided but not absolute, it's treated as relative to the repository root.
// Returns the absolute path and an error if default_lang_code is not configured when needed.
func GetPoFileAbsPath(cfg *config.AgentConfig, poFile string) (string, error) {
	workDir := repository.WorkDir()
	if poFile == "" {
		lang := cfg.DefaultLangCode
		if lang == "" {
			log.Errorf("default_lang_code is not configured in agent configuration")
			return "", fmt.Errorf("default_lang_code is not configured\nHint: Provide po/XX.po on the command line or set default_lang_code in git-po-helper.yaml")
		}
		poFile = filepath.Join(workDir, PoDir, fmt.Sprintf("%s.po", lang))
	} else if !filepath.IsAbs(poFile) {
		// Treat poFile as relative to repository root
		poFile = filepath.Join(workDir, poFile)
	}
	return poFile, nil
}

// GetPoFileRelPath determines the relative path of a PO file in "po/XX.po" format.
// If poFile is empty, it uses cfg.DefaultLangCode to construct the path.
// If poFile is an absolute path, it converts it to a relative path.
// If poFile is already a relative path, it normalizes it to "po/XX.po" format.
// Returns the relative path and an error if default_lang_code is not configured when needed.
func GetPoFileRelPath(cfg *config.AgentConfig, poFile string) (string, error) {
	workDir := repository.WorkDir()
	var absPath string
	var err error

	// First get the absolute path
	absPath, err = GetPoFileAbsPath(cfg, poFile)
	if err != nil {
		return "", err
	}

	// Convert absolute path to relative path
	relPath, err := filepath.Rel(workDir, absPath)
	if err != nil {
		log.Errorf("failed to convert absolute path to relative path: %v", err)
		return "", fmt.Errorf("failed to convert path to relative: %w", err)
	}

	// Normalize to use forward slashes (for consistency with "po/XX.po" format)
	relPath = filepath.ToSlash(relPath)

	return relPath, nil
}

// RunAgentUpdatePot executes a single agent-run update-pot operation.
// It performs pre-validation, executes the agent command, performs post-validation,
// and validates POT file syntax. Returns a result structure with detailed information.
// The agentTest parameter controls whether AgentTest configuration should be used.
// When agentTest is false (for agent-run), AgentTest configuration is ignored.
func RunAgentUpdatePot(cfg *config.AgentConfig, agentName string, agentTest bool) (*AgentRunResult, error) {
	startTime := time.Now()
	result := &AgentRunResult{
		Score: 0,
	}

	// Determine agent to use
	selectedAgent, agentKey, err := SelectAgent(cfg, agentName)
	if err != nil {
		result.AgentError = err.Error()
		return result, err
	}

	log.Debugf("using agent: %s", agentKey)

	// Get POT file path
	potFile := GetPotFilePath()
	log.Debugf("POT file path: %s", potFile)

	// Pre-validation: Check entry count before update (only for agent-test)
	if agentTest && cfg.AgentTest.PotEntriesBeforeUpdate != nil && *cfg.AgentTest.PotEntriesBeforeUpdate != 0 {
		log.Infof("performing pre-validation: checking entry count before update (expected: %d)", *cfg.AgentTest.PotEntriesBeforeUpdate)

		// Get before count for result
		if !Exist(potFile) {
			result.BeforeCount = 0
		} else {
			result.BeforeCount, _ = CountPotEntries(potFile)
		}

		if err := ValidatePotEntryCount(potFile, cfg.AgentTest.PotEntriesBeforeUpdate, "before update"); err != nil {
			log.Errorf("pre-validation failed: %v", err)
			result.PreValidationError = err.Error()
			return result, fmt.Errorf("pre-validation failed: %w\nHint: Ensure po/git.pot exists and has the expected number of entries", err)
		}
		result.PreValidationPass = true
		log.Infof("pre-validation passed")
	} else {
		// No pre-validation configured, count entries for display purposes
		if !Exist(potFile) {
			result.BeforeCount = 0
		} else {
			result.BeforeCount, _ = CountPotEntries(potFile)
		}
		result.PreValidationPass = true // Consider it passed if not configured
	}

	// Get prompt from configuration
	prompt, err := GetPrompt(cfg, "update-pot")
	if err != nil {
		return result, err
	}

	// Build agent command with placeholders replaced
	agentCmd := BuildAgentCommand(selectedAgent, prompt, "", "")

	// Determine output format
	outputFormat := selectedAgent.Output
	if outputFormat == "" {
		outputFormat = "default"
	}
	// Normalize output format (convert underscores to hyphens)
	outputFormat = normalizeOutputFormat(outputFormat)

	// Execute agent command
	workDir := repository.WorkDir()
	log.Infof("executing agent command: %s", strings.Join(agentCmd, " "))
	result.AgentExecuted = true

	var stdout []byte
	var stderr []byte
	var jsonResult *ClaudeJSONOutput
	var codexResult *CodexJSONOutput
	var opencodeResult *OpenCodeJSONOutput
	var geminiResult *GeminiJSONOutput

	// Detect agent type
	isCodex := len(agentCmd) > 0 && agentCmd[0] == "codex"
	isOpencode := len(agentCmd) > 0 && agentCmd[0] == "opencode"
	isGemini := len(agentCmd) > 0 && (agentCmd[0] == "gemini" || agentCmd[0] == "qwen")

	// Use streaming execution for json format (treated as stream-json)
	if outputFormat == "json" {
		stdoutReader, stderrBuf, cmdProcess, err := ExecuteAgentCommandStream(agentCmd, workDir)
		if err != nil {
			log.Errorf("agent command execution failed: %v", err)
			return result, fmt.Errorf("agent command failed: %w\nHint: Check that the agent command is correct and executable", err)
		}
		defer stdoutReader.Close()

		// Parse stream in real-time based on agent type
		if isCodex {
			parsedStdout, finalResult, err := ParseCodexJSONLRealtime(stdoutReader)
			if err != nil {
				log.Warnf("failed to parse codex JSONL: %v", err)
			}
			codexResult = finalResult
			stdout = parsedStdout
		} else if isOpencode {
			parsedStdout, finalResult, err := ParseOpenCodeJSONLRealtime(stdoutReader)
			if err != nil {
				log.Warnf("failed to parse opencode JSONL: %v", err)
			}
			opencodeResult = finalResult
			stdout = parsedStdout
		} else if isGemini {
			// Parsing stream-json for Gemini-CLI
			parsedStdout, finalResult, err := ParseGeminiJSONLRealtime(stdoutReader)
			if err != nil {
				log.Warnf("failed to parse gemini JSONL: %v", err)
			}
			geminiResult = finalResult
			stdout = parsedStdout
		} else {
			// Parsing stream-json for Claude Code
			parsedStdout, finalResult, err := ParseStreamJSONRealtime(stdoutReader)
			if err != nil {
				log.Warnf("failed to parse stream JSON: %v", err)
			}
			jsonResult = finalResult
			stdout = parsedStdout
		}

		// Wait for command to complete and get stderr
		waitErr := cmdProcess.Wait()
		stderr = stderrBuf.Bytes()

		if waitErr != nil {
			if len(stderr) > 0 {
				log.Debugf("agent command stderr: %s", string(stderr))
			}
			result.AgentError = fmt.Sprintf("agent command failed: %v (see logs for agent stderr output)", waitErr)
			log.Errorf("agent command execution failed: %v", waitErr)
			return result, fmt.Errorf("agent command failed: %w\nHint: Check that the agent command is correct and executable", waitErr)
		}
		result.AgentSuccess = true
		log.Infof("agent command completed successfully")
	} else {
		// Use regular execution for other formats
		var err error
		stdout, stderr, err = ExecuteAgentCommand(agentCmd, workDir)
		if err != nil {
			// Log stderr if available (debug level to avoid leaking sensitive details at normal verbosity)
			if len(stderr) > 0 {
				log.Debugf("agent command stderr: %s", string(stderr))
			}
			// Log stdout if available (might contain useful info even on error)
			if len(stdout) > 0 {
				log.Debugf("agent command stdout: %s", string(stdout))
			}

			// Store a summarized error message without embedding full stderr
			result.AgentError = fmt.Sprintf("agent command failed: %v (see logs for agent stderr output)", err)

			log.Errorf("agent command execution failed: %v", err)
			return result, fmt.Errorf("agent command failed: %w\nHint: Check that the agent command is correct and executable", err)
		}
		result.AgentSuccess = true
		log.Infof("agent command completed successfully")

		// Parse output based on agent output format (only for claude)
		if !isCodex && !isOpencode {
			parsedStdout, parsedResult, err := ParseAgentOutput(stdout, outputFormat)
			if err != nil {
				log.Warnf("failed to parse agent output: %v, using raw output", err)
				parsedStdout = stdout
			} else {
				stdout = parsedStdout
				jsonResult = parsedResult
			}
		}
	}

	// Print diagnostics if available
	if codexResult != nil {
		PrintAgentDiagnostics(codexResult)
		// Extract NumTurns from diagnostics
		if codexResult.NumTurns > 0 {
			result.NumTurns = codexResult.NumTurns
		}
	} else if opencodeResult != nil {
		PrintAgentDiagnostics(opencodeResult)
		// Extract NumTurns from diagnostics
		if opencodeResult.NumTurns > 0 {
			result.NumTurns = opencodeResult.NumTurns
		}
	} else if geminiResult != nil {
		PrintAgentDiagnostics(geminiResult)
		// Extract NumTurns from diagnostics
		if geminiResult.NumTurns > 0 {
			result.NumTurns = geminiResult.NumTurns
		}
	} else if jsonResult != nil {
		PrintAgentDiagnostics(jsonResult)
		// Extract NumTurns from diagnostics
		if jsonResult.NumTurns > 0 {
			result.NumTurns = jsonResult.NumTurns
		}
	}

	// Log output if verbose
	if len(stdout) > 0 {
		log.Debugf("agent command stdout: %s", string(stdout))
	}
	if len(stderr) > 0 {
		log.Debugf("agent command stderr: %s", string(stderr))
	}

	// Post-validation: Check entry count after update (only for agent-test)
	if agentTest && cfg.AgentTest.PotEntriesAfterUpdate != nil && *cfg.AgentTest.PotEntriesAfterUpdate != 0 {
		log.Infof("performing post-validation: checking entry count after update (expected: %d)", *cfg.AgentTest.PotEntriesAfterUpdate)

		// Get after count for result
		if Exist(potFile) {
			result.AfterCount, _ = CountPotEntries(potFile)
		}

		if err := ValidatePotEntryCount(potFile, cfg.AgentTest.PotEntriesAfterUpdate, "after update"); err != nil {
			log.Errorf("post-validation failed: %v", err)
			result.PostValidationError = err.Error()
			result.Score = 0
			return result, fmt.Errorf("post-validation failed: %w\nHint: The agent may not have updated the POT file correctly", err)
		}
		result.PostValidationPass = true
		result.Score = 100
		log.Infof("post-validation passed")
	} else {
		// No post-validation configured, score based on agent exit code
		if Exist(potFile) {
			result.AfterCount, _ = CountPotEntries(potFile)
		}
		if result.AgentSuccess {
			result.Score = 100
			result.PostValidationPass = true // Consider it passed if agent succeeded
		} else {
			result.Score = 0
		}
	}

	// Validate POT file syntax (only if agent succeeded)
	if result.AgentSuccess {
		log.Infof("validating file syntax: %s", potFile)
		if err := ValidatePoFile(potFile); err != nil {
			log.Errorf("file syntax validation failed: %v", err)
			result.SyntaxValidationError = err.Error()
			// Don't fail the run for syntax errors in agent-run, but log it
			// In agent-test, this might affect the score
		} else {
			result.SyntaxValidationPass = true
			log.Infof("file syntax validation passed")
		}
	}

	// Record execution time
	result.ExecutionTime = time.Since(startTime)

	return result, nil
}

// RunAgentUpdatePo executes a single agent-run update-po operation.
// It performs pre-validation, executes the agent command, performs post-validation,
// and validates PO file syntax. Returns a result structure with detailed information.
// The agentTest parameter controls whether AgentTest configuration should be used.
// When agentTest is false (for agent-run), AgentTest configuration is ignored.
func RunAgentUpdatePo(cfg *config.AgentConfig, agentName, poFile string, agentTest bool) (*AgentRunResult, error) {
	startTime := time.Now()
	result := &AgentRunResult{
		Score: 0,
	}

	// Determine agent to use
	selectedAgent, agentKey, err := SelectAgent(cfg, agentName)
	if err != nil {
		result.AgentError = err.Error()
		return result, err
	}

	log.Debugf("using agent: %s", agentKey)

	// Determine PO file path
	poFile, err = GetPoFileAbsPath(cfg, poFile)
	if err != nil {
		return result, err
	}

	log.Debugf("PO file path: %s", poFile)

	// Pre-validation: Check entry count before update (only for agent-test)
	if agentTest && cfg.AgentTest.PoEntriesBeforeUpdate != nil && *cfg.AgentTest.PoEntriesBeforeUpdate != 0 {
		log.Infof("performing pre-validation: checking PO entry count before update (expected: %d)", *cfg.AgentTest.PoEntriesBeforeUpdate)

		// Get before count for result
		if !Exist(poFile) {
			result.BeforeCount = 0
		} else {
			result.BeforeCount, _ = CountPoEntries(poFile)
		}

		if err := ValidatePoEntryCount(poFile, cfg.AgentTest.PoEntriesBeforeUpdate, "before update"); err != nil {
			log.Errorf("pre-validation failed: %v", err)
			result.PreValidationError = err.Error()
			return result, fmt.Errorf("pre-validation failed: %w\nHint: Ensure %s exists and has the expected number of entries", err, poFile)
		}
		result.PreValidationPass = true
		log.Infof("pre-validation passed")
	} else {
		// No pre-validation configured, count entries for display purposes
		if !Exist(poFile) {
			result.BeforeCount = 0
		} else {
			result.BeforeCount, _ = CountPoEntries(poFile)
		}
		result.PreValidationPass = true // Consider it passed if not configured
	}

	// Get prompt for update-po from configuration
	prompt, err := GetPrompt(cfg, "update-po")
	if err != nil {
		return result, err
	}

	// Build agent command with placeholders replaced
	workDir := repository.WorkDir()
	sourcePath := poFile
	if rel, err := filepath.Rel(workDir, poFile); err == nil && rel != "" && rel != "." {
		sourcePath = filepath.ToSlash(rel)
	}
	agentCmd := BuildAgentCommand(selectedAgent, prompt, sourcePath, "")

	// Determine output format
	outputFormat := selectedAgent.Output
	if outputFormat == "" {
		outputFormat = "default"
	}
	// Normalize output format (convert underscores to hyphens)
	outputFormat = normalizeOutputFormat(outputFormat)

	// Execute agent command
	log.Infof("executing agent command: %s", strings.Join(agentCmd, " "))
	result.AgentExecuted = true

	var stdout []byte
	var stderr []byte
	var jsonResult *ClaudeJSONOutput
	var codexResult *CodexJSONOutput
	var opencodeResult *OpenCodeJSONOutput
	var geminiResult *GeminiJSONOutput

	// Detect agent type
	isCodex := len(agentCmd) > 0 && agentCmd[0] == "codex"
	isOpencode := len(agentCmd) > 0 && agentCmd[0] == "opencode"
	isGemini := len(agentCmd) > 0 && (agentCmd[0] == "gemini" || agentCmd[0] == "qwen")

	// Use streaming execution for json format (treated as stream-json)
	if outputFormat == "json" {
		stdoutReader, stderrBuf, cmdProcess, err := ExecuteAgentCommandStream(agentCmd, workDir)
		if err != nil {
			log.Errorf("agent command execution failed: %v", err)
			return result, fmt.Errorf("agent command failed: %w\nHint: Check that the agent command is correct and executable", err)
		}
		defer stdoutReader.Close()

		// Parse stream in real-time based on agent type
		if isCodex {
			parsedStdout, finalResult, err := ParseCodexJSONLRealtime(stdoutReader)
			if err != nil {
				log.Warnf("failed to parse codex JSONL: %v", err)
			}
			codexResult = finalResult
			stdout = parsedStdout
		} else if isOpencode {
			parsedStdout, finalResult, err := ParseOpenCodeJSONLRealtime(stdoutReader)
			if err != nil {
				log.Warnf("failed to parse opencode JSONL: %v", err)
			}
			opencodeResult = finalResult
			stdout = parsedStdout
		} else if isGemini {
			// Parsing stream-json for Gemini-CLI
			parsedStdout, finalResult, err := ParseGeminiJSONLRealtime(stdoutReader)
			if err != nil {
				log.Warnf("failed to parse gemini JSONL: %v", err)
			}
			geminiResult = finalResult
			stdout = parsedStdout
		} else {
			// Parsing stream-json for Claude Code
			parsedStdout, finalResult, err := ParseStreamJSONRealtime(stdoutReader)
			if err != nil {
				log.Warnf("failed to parse stream JSON: %v", err)
			}
			jsonResult = finalResult
			stdout = parsedStdout
		}

		// Wait for command to complete and get stderr
		waitErr := cmdProcess.Wait()
		stderr = stderrBuf.Bytes()

		if waitErr != nil {
			if len(stderr) > 0 {
				log.Debugf("agent command stderr: %s", string(stderr))
			}
			result.AgentError = fmt.Sprintf("agent command failed: %v (see logs for agent stderr output)", waitErr)
			log.Errorf("agent command execution failed: %v", waitErr)
			return result, fmt.Errorf("agent command failed: %w\nHint: Check that the agent command is correct and executable", waitErr)
		}
		result.AgentSuccess = true
		log.Infof("agent command completed successfully")
	} else {
		// Use regular execution for other formats
		var err error
		stdout, stderr, err = ExecuteAgentCommand(agentCmd, workDir)
		if err != nil {
			// Log stderr if available (debug level to avoid leaking sensitive details at normal verbosity)
			if len(stderr) > 0 {
				log.Debugf("agent command stderr: %s", string(stderr))
			}
			// Log stdout if available (might contain useful info even on error)
			if len(stdout) > 0 {
				log.Debugf("agent command stdout: %s", string(stdout))
			}

			// Store a summarized error message without embedding full stderr
			result.AgentError = fmt.Sprintf("agent command failed: %v (see logs for agent stderr output)", err)

			log.Errorf("agent command execution failed: %v", err)
			return result, fmt.Errorf("agent command failed: %w\nHint: Check that the agent command is correct and executable", err)
		}
		result.AgentSuccess = true
		log.Infof("agent command completed successfully")

		// Parse output based on agent output format (only for claude)
		if !isCodex && !isOpencode {
			parsedStdout, parsedResult, err := ParseAgentOutput(stdout, outputFormat)
			if err != nil {
				log.Warnf("failed to parse agent output: %v, using raw output", err)
				parsedStdout = stdout
			} else {
				stdout = parsedStdout
				jsonResult = parsedResult
			}
		}
	}

	// Print diagnostics if available
	if codexResult != nil {
		PrintAgentDiagnostics(codexResult)
		// Extract NumTurns from diagnostics
		if codexResult.NumTurns > 0 {
			result.NumTurns = codexResult.NumTurns
		}
	} else if opencodeResult != nil {
		PrintAgentDiagnostics(opencodeResult)
		// Extract NumTurns from diagnostics
		if opencodeResult.NumTurns > 0 {
			result.NumTurns = opencodeResult.NumTurns
		}
	} else if geminiResult != nil {
		PrintAgentDiagnostics(geminiResult)
		// Extract NumTurns from diagnostics
		if geminiResult.NumTurns > 0 {
			result.NumTurns = geminiResult.NumTurns
		}
	} else if jsonResult != nil {
		PrintAgentDiagnostics(jsonResult)
		// Extract NumTurns from diagnostics
		if jsonResult.NumTurns > 0 {
			result.NumTurns = jsonResult.NumTurns
		}
	}

	// Log output if verbose
	if len(stdout) > 0 {
		log.Debugf("agent command stdout: %s", string(stdout))
	}
	if len(stderr) > 0 {
		log.Debugf("agent command stderr: %s", string(stderr))
	}

	// Post-validation: Check entry count after update (only for agent-test)
	if agentTest && cfg.AgentTest.PoEntriesAfterUpdate != nil && *cfg.AgentTest.PoEntriesAfterUpdate != 0 {
		log.Infof("performing post-validation: checking PO entry count after update (expected: %d)", *cfg.AgentTest.PoEntriesAfterUpdate)

		// Get after count for result
		if Exist(poFile) {
			result.AfterCount, _ = CountPoEntries(poFile)
		}

		if err := ValidatePoEntryCount(poFile, cfg.AgentTest.PoEntriesAfterUpdate, "after update"); err != nil {
			log.Errorf("post-validation failed: %v", err)
			result.PostValidationError = err.Error()
			result.Score = 0
			return result, fmt.Errorf("post-validation failed: %w\nHint: The agent may not have updated the PO file correctly", err)
		}
		result.PostValidationPass = true
		result.Score = 100
		log.Infof("post-validation passed")
	} else {
		// No post-validation configured, score based on agent exit code
		if Exist(poFile) {
			result.AfterCount, _ = CountPoEntries(poFile)
		}
		if result.AgentSuccess {
			result.Score = 100
			result.PostValidationPass = true // Consider it passed if agent succeeded
		} else {
			result.Score = 0
		}
	}

	// Validate PO file syntax (only if agent succeeded)
	if result.AgentSuccess {
		log.Infof("validating file syntax: %s", poFile)
		if err := ValidatePoFile(poFile); err != nil {
			log.Errorf("file syntax validation failed: %v", err)
			result.SyntaxValidationError = err.Error()
			// Don't fail the run for syntax errors in agent-run, but log it
			// In agent-test, this might affect the score
		} else {
			result.SyntaxValidationPass = true
			log.Infof("file syntax validation passed")
		}
	}

	// Record execution time
	result.ExecutionTime = time.Since(startTime)

	return result, nil
}

// CmdAgentRunUpdatePot implements the agent-run update-pot command logic.
// It loads configuration and calls RunAgentUpdatePot, then handles errors appropriately.
func CmdAgentRunUpdatePot(agentName string) error {
	// Load configuration
	log.Debugf("loading agent configuration")
	cfg, err := config.LoadAgentConfig()
	if err != nil {
		log.Errorf("failed to load agent configuration: %v", err)
		return fmt.Errorf("failed to load agent configuration: %w\nHint: Ensure git-po-helper.yaml exists in repository root or user home directory", err)
	}

	startTime := time.Now()

	result, err := RunAgentUpdatePot(cfg, agentName, false)
	if err != nil {
		return err
	}

	// For agent-run, we require all validations to pass
	if !result.PreValidationPass {
		return fmt.Errorf("pre-validation failed: %s", result.PreValidationError)
	}
	if !result.AgentSuccess {
		return fmt.Errorf("agent execution failed: %s", result.AgentError)
	}
	if !result.PostValidationPass {
		return fmt.Errorf("post-validation failed: %s", result.PostValidationError)
	}
	if result.SyntaxValidationError != "" {
		ext := filepath.Ext(GetPotFilePath())
		if ext == ".pot" {
			return fmt.Errorf("file validation failed: %s\nHint: Check the POT file syntax using 'msgcat --use-first <file> -o /dev/null'", result.SyntaxValidationError)
		}
		return fmt.Errorf("file validation failed: %s\nHint: Check the PO file syntax using 'msgfmt --check-format'", result.SyntaxValidationError)
	}

	elapsed := time.Since(startTime)
	fmt.Printf("\nSummary:\n")
	fmt.Printf("  Execution time: %s\n", elapsed.Round(time.Millisecond))

	log.Infof("agent-run update-pot completed successfully")
	return nil
}

// CmdAgentRunUpdatePo implements the agent-run update-po command logic.
// It loads configuration and calls RunAgentUpdatePo, then handles errors appropriately.
func CmdAgentRunUpdatePo(agentName, poFile string) error {
	// Load configuration
	log.Debugf("loading agent configuration")
	cfg, err := config.LoadAgentConfig()
	if err != nil {
		log.Errorf("failed to load agent configuration: %v", err)
		return fmt.Errorf("failed to load agent configuration: %w\nHint: Ensure git-po-helper.yaml exists in repository root or user home directory", err)
	}

	startTime := time.Now()

	result, err := RunAgentUpdatePo(cfg, agentName, poFile, false)
	if err != nil {
		return err
	}

	// For agent-run, we require all validations to pass
	if !result.PreValidationPass {
		return fmt.Errorf("pre-validation failed: %s", result.PreValidationError)
	}
	if !result.AgentSuccess {
		return fmt.Errorf("agent execution failed: %s", result.AgentError)
	}
	if !result.PostValidationPass {
		return fmt.Errorf("post-validation failed: %s", result.PostValidationError)
	}
	if result.SyntaxValidationError != "" {
		return fmt.Errorf("file validation failed: %s\nHint: Check the PO file syntax using 'msgfmt --check-format'", result.SyntaxValidationError)
	}

	elapsed := time.Since(startTime)
	fmt.Printf("\nSummary:\n")
	fmt.Printf("  Execution time: %s\n", elapsed.Round(time.Millisecond))

	log.Infof("agent-run update-po completed successfully")
	return nil
}

// CmdAgentRunShowConfig displays the current agent configuration in YAML format.
func CmdAgentRunShowConfig() error {
	// Load configuration
	log.Debugf("loading agent configuration")
	cfg, err := config.LoadAgentConfig()
	if err != nil {
		log.Errorf("failed to load agent configuration: %v", err)
		return fmt.Errorf("failed to load agent configuration: %w", err)
	}

	// Marshal configuration to YAML
	yamlData, err := yaml.Marshal(cfg)
	if err != nil {
		log.Errorf("failed to marshal configuration to YAML: %v", err)
		return fmt.Errorf("failed to marshal configuration to YAML: %w", err)
	}

	// Display the configuration
	fmt.Println("# Agent Configuration")
	fmt.Println("# This is the merged configuration from:")
	fmt.Println("# - User home directory: ~/.git-po-helper.yaml (lower priority)")
	fmt.Println("# - Repository root: <repo-root>/git-po-helper.yaml (higher priority)")
	fmt.Println()
	os.Stdout.Write(yamlData)

	return nil
}

// RunAgentTranslate executes a single agent-run translate operation.
// It performs pre-validation (count new/fuzzy entries), executes the agent command,
// performs post-validation (verify new=0 and fuzzy=0), and validates PO file syntax.
// Returns a result structure with detailed information.
// The agentTest parameter is provided for consistency, though this method
// does not use AgentTest configuration.
func RunAgentTranslate(cfg *config.AgentConfig, agentName, poFile string, agentTest bool) (*AgentRunResult, error) {
	startTime := time.Now()
	result := &AgentRunResult{
		Score: 0,
	}

	// Determine agent to use
	selectedAgent, agentKey, err := SelectAgent(cfg, agentName)
	if err != nil {
		result.AgentError = err.Error()
		return result, err
	}

	log.Debugf("using agent: %s", agentKey)

	// Determine PO file path
	poFile, err = GetPoFileAbsPath(cfg, poFile)
	if err != nil {
		return result, err
	}

	log.Debugf("PO file path: %s", poFile)

	// Check if PO file exists
	if !Exist(poFile) {
		log.Errorf("PO file does not exist: %s", poFile)
		return result, fmt.Errorf("PO file does not exist: %s\nHint: Ensure the PO file exists before running translate", poFile)
	}

	// Pre-validation: Count new and fuzzy entries before translation
	log.Infof("performing pre-validation: counting new and fuzzy entries")

	// Count new entries
	newCountBefore, err := CountNewEntries(poFile)
	if err != nil {
		log.Errorf("failed to count new entries: %v", err)
		return result, fmt.Errorf("failed to count new entries: %w", err)
	}
	result.BeforeNewCount = newCountBefore
	log.Infof("new (untranslated) entries before translation: %d", newCountBefore)

	// Count fuzzy entries
	fuzzyCountBefore, err := CountFuzzyEntries(poFile)
	if err != nil {
		log.Errorf("failed to count fuzzy entries: %v", err)
		return result, fmt.Errorf("failed to count fuzzy entries: %w", err)
	}
	result.BeforeFuzzyCount = fuzzyCountBefore
	log.Infof("fuzzy entries before translation: %d", fuzzyCountBefore)

	// Check if there's anything to translate
	if newCountBefore == 0 && fuzzyCountBefore == 0 {
		log.Infof("no new or fuzzy entries to translate, PO file is already complete")
		result.PreValidationPass = true
		result.PostValidationPass = true
		result.Score = 100
		return result, nil
	}

	result.PreValidationPass = true

	// We can extract new entries and fuzzy entries from the PO file using
	// "msgattrib --untranslated --only-fuzzy poFile", and saved to a
	// temporary file, then pass it to the agent as a source file.
	// This way, we can translate the new entries and fuzzy entries in one
	// round of translation. Later, we can use msgcat to merge the translations
	// back to the PO file like "msgcat --use-first new.po original.po -o merged.po".
	//
	// But we can document this in the po/README.md, and let the code agent
	// decide whether to use this feature.
	//
	// Now, load the simple prompt for translate the file.
	prompt, err := GetPrompt(cfg, "translate")
	if err != nil {
		return result, err
	}

	// Build agent command with placeholders replaced
	workDir := repository.WorkDir()
	sourcePath := poFile
	if rel, err := filepath.Rel(workDir, poFile); err == nil && rel != "" && rel != "." {
		sourcePath = filepath.ToSlash(rel)
	}
	agentCmd := BuildAgentCommand(selectedAgent, prompt, sourcePath, "")

	// Determine output format
	outputFormat := selectedAgent.Output
	if outputFormat == "" {
		outputFormat = "default"
	}
	// Normalize output format (convert underscores to hyphens)
	outputFormat = normalizeOutputFormat(outputFormat)

	// Execute agent command
	log.Infof("executing agent command: %s", strings.Join(agentCmd, " "))
	result.AgentExecuted = true

	var stdout []byte
	var stderr []byte
	var jsonResult *ClaudeJSONOutput
	var codexResult *CodexJSONOutput
	var opencodeResult *OpenCodeJSONOutput
	var geminiResult *GeminiJSONOutput

	// Detect agent type
	isCodex := len(agentCmd) > 0 && agentCmd[0] == "codex"
	isOpencode := len(agentCmd) > 0 && agentCmd[0] == "opencode"
	isGemini := len(agentCmd) > 0 && (agentCmd[0] == "gemini" || agentCmd[0] == "qwen")

	// Use streaming execution for json format (treated as stream-json)
	if outputFormat == "json" {
		stdoutReader, stderrBuf, cmdProcess, err := ExecuteAgentCommandStream(agentCmd, workDir)
		if err != nil {
			log.Errorf("agent command execution failed: %v", err)
			return result, fmt.Errorf("agent command failed: %w\nHint: Check that the agent command is correct and executable", err)
		}
		defer stdoutReader.Close()

		// Parse stream in real-time based on agent type
		if isCodex {
			parsedStdout, finalResult, err := ParseCodexJSONLRealtime(stdoutReader)
			if err != nil {
				log.Warnf("failed to parse codex JSONL: %v", err)
			}
			codexResult = finalResult
			stdout = parsedStdout
		} else if isOpencode {
			parsedStdout, finalResult, err := ParseOpenCodeJSONLRealtime(stdoutReader)
			if err != nil {
				log.Warnf("failed to parse opencode JSONL: %v", err)
			}
			opencodeResult = finalResult
			stdout = parsedStdout
		} else if isGemini {
			// Parsing stream-json for Gemini-CLI
			parsedStdout, finalResult, err := ParseGeminiJSONLRealtime(stdoutReader)
			if err != nil {
				log.Warnf("failed to parse gemini JSONL: %v", err)
			}
			geminiResult = finalResult
			stdout = parsedStdout
		} else {
			// Parsing stream-json for Claude Code
			parsedStdout, finalResult, err := ParseStreamJSONRealtime(stdoutReader)
			if err != nil {
				log.Warnf("failed to parse stream JSON: %v", err)
			}
			jsonResult = finalResult
			stdout = parsedStdout
		}

		// Wait for command to complete and get stderr
		waitErr := cmdProcess.Wait()
		stderr = stderrBuf.Bytes()

		if waitErr != nil {
			if len(stderr) > 0 {
				log.Debugf("agent command stderr: %s", string(stderr))
			}
			result.AgentError = fmt.Sprintf("agent command failed: %v (see logs for agent stderr output)", waitErr)
			log.Errorf("agent command execution failed: %v", waitErr)
			return result, fmt.Errorf("agent command failed: %w\nHint: Check that the agent command is correct and executable", waitErr)
		}
		result.AgentSuccess = true
		log.Infof("agent command completed successfully")
	} else {
		// Use regular execution for other formats
		var err error
		stdout, stderr, err = ExecuteAgentCommand(agentCmd, workDir)
		if err != nil {
			// Log stderr if available (debug level to avoid leaking sensitive details at normal verbosity)
			if len(stderr) > 0 {
				log.Debugf("agent command stderr: %s", string(stderr))
			}
			// Log stdout if available (might contain useful info even on error)
			if len(stdout) > 0 {
				log.Debugf("agent command stdout: %s", string(stdout))
			}

			// Store a summarized error message without embedding full stderr
			result.AgentError = fmt.Sprintf("agent command failed: %v (see logs for agent stderr output)", err)

			log.Errorf("agent command execution failed: %v", err)
			return result, fmt.Errorf("agent command failed: %w\nHint: Check that the agent command is correct and executable", err)
		}
		result.AgentSuccess = true
		log.Infof("agent command completed successfully")

		// Parse output based on agent output format (only for claude)
		if !isCodex && !isOpencode {
			parsedStdout, parsedResult, err := ParseAgentOutput(stdout, outputFormat)
			if err != nil {
				log.Warnf("failed to parse agent output: %v, using raw output", err)
				parsedStdout = stdout
			} else {
				stdout = parsedStdout
				jsonResult = parsedResult
			}
		}
	}

	// Print diagnostics if available
	if codexResult != nil {
		PrintAgentDiagnostics(codexResult)
		// Extract NumTurns from diagnostics
		if codexResult.NumTurns > 0 {
			result.NumTurns = codexResult.NumTurns
		}
	} else if opencodeResult != nil {
		PrintAgentDiagnostics(opencodeResult)
		// Extract NumTurns from diagnostics
		if opencodeResult.NumTurns > 0 {
			result.NumTurns = opencodeResult.NumTurns
		}
	} else if geminiResult != nil {
		PrintAgentDiagnostics(geminiResult)
		// Extract NumTurns from diagnostics
		if geminiResult.NumTurns > 0 {
			result.NumTurns = geminiResult.NumTurns
		}
	} else if jsonResult != nil {
		PrintAgentDiagnostics(jsonResult)
		// Extract NumTurns from diagnostics
		if jsonResult.NumTurns > 0 {
			result.NumTurns = jsonResult.NumTurns
		}
	}

	// Log output if verbose
	if len(stdout) > 0 {
		log.Debugf("agent command stdout: %s", string(stdout))
	}
	if len(stderr) > 0 {
		log.Debugf("agent command stderr: %s", string(stderr))
	}

	// Post-validation: Count new and fuzzy entries after translation
	log.Infof("performing post-validation: counting new and fuzzy entries")

	// Count new entries
	newCountAfter, err := CountNewEntries(poFile)
	if err != nil {
		log.Errorf("failed to count new entries after translation: %v", err)
		return result, fmt.Errorf("failed to count new entries after translation: %w", err)
	}
	result.AfterNewCount = newCountAfter
	log.Infof("new (untranslated) entries after translation: %d", newCountAfter)

	// Count fuzzy entries
	fuzzyCountAfter, err := CountFuzzyEntries(poFile)
	if err != nil {
		log.Errorf("failed to count fuzzy entries after translation: %v", err)
		return result, fmt.Errorf("failed to count fuzzy entries after translation: %w", err)
	}
	result.AfterFuzzyCount = fuzzyCountAfter
	log.Infof("fuzzy entries after translation: %d", fuzzyCountAfter)

	// Validate translation success: both new and fuzzy entries must be 0
	if newCountAfter != 0 || fuzzyCountAfter != 0 {
		log.Errorf("post-validation failed: translation incomplete (new: %d, fuzzy: %d)", newCountAfter, fuzzyCountAfter)
		result.PostValidationError = fmt.Sprintf("translation incomplete: %d new entries and %d fuzzy entries remaining", newCountAfter, fuzzyCountAfter)
		result.Score = 0
		return result, fmt.Errorf("post-validation failed: %s\nHint: The agent should translate all new entries and resolve all fuzzy entries", result.PostValidationError)
	}

	result.PostValidationPass = true
	result.Score = 100
	log.Infof("post-validation passed: all entries translated")

	// Validate PO file syntax (only if agent succeeded)
	if result.AgentSuccess {
		log.Infof("validating file syntax: %s", poFile)
		if err := ValidatePoFile(poFile); err != nil {
			log.Errorf("file syntax validation failed: %v", err)
			result.SyntaxValidationError = err.Error()
			// Don't fail the run for syntax errors in agent-run, but log it
			// In agent-test, this might affect the score
		} else {
			result.SyntaxValidationPass = true
			log.Infof("file syntax validation passed")
		}
	}

	// Record execution time
	result.ExecutionTime = time.Since(startTime)

	return result, nil
}

// CmdAgentRunTranslate implements the agent-run translate command logic.
// It loads configuration and calls RunAgentTranslate, then handles errors appropriately.
func CmdAgentRunTranslate(agentName, poFile string) error {
	// Load configuration
	log.Debugf("loading agent configuration")
	cfg, err := config.LoadAgentConfig()
	if err != nil {
		log.Errorf("failed to load agent configuration: %v", err)
		return fmt.Errorf("failed to load agent configuration: %w\nHint: Ensure git-po-helper.yaml exists in repository root or user home directory", err)
	}

	startTime := time.Now()

	result, err := RunAgentTranslate(cfg, agentName, poFile, false)
	if err != nil {
		return err
	}

	// For agent-run, we require all validations to pass
	if !result.PreValidationPass {
		return fmt.Errorf("pre-validation failed: %s", result.PreValidationError)
	}
	if !result.AgentSuccess {
		return fmt.Errorf("agent execution failed: %s", result.AgentError)
	}
	if !result.PostValidationPass {
		return fmt.Errorf("post-validation failed: %s", result.PostValidationError)
	}
	if result.SyntaxValidationError != "" {
		return fmt.Errorf("file validation failed: %s\nHint: Check the PO file syntax using 'msgfmt --check-format'", result.SyntaxValidationError)
	}

	elapsed := time.Since(startTime)
	fmt.Printf("\nSummary:\n")
	fmt.Printf("  Execution time: %s\n", elapsed.Round(time.Millisecond))

	log.Infof("agent-run translate completed successfully")
	return nil
}

// copyFile copies a file from src to dst.
func copyFile(src, dst string) error {
	data, err := os.ReadFile(src)
	if err != nil {
		return err
	}
	return os.WriteFile(dst, data, 0644)
}

// RunAgentReview executes a single agent-run review operation with the new workflow:
// 1. Prepare review data (orig.po, new.po, review-input.po)
// 2. Copy review-input.po to review-output.po
// 3. Execute agent to review and modify review-output.po
// 4. Merge review-output.po with new.po using msgcat
// 5. Parse JSON from agent output and calculate score
// Returns a result structure with detailed information.
// The agentTest parameter is provided for consistency, though this method
// does not use AgentTest configuration.
func RunAgentReview(cfg *config.AgentConfig, agentName, poFile, commit, since string, agentTest bool) (*AgentRunResult, error) {
	startTime := time.Now()
	result := &AgentRunResult{
		Score: 0,
	}

	// Determine agent to use
	selectedAgent, agentKey, err := SelectAgent(cfg, agentName)
	if err != nil {
		result.AgentError = err.Error()
		return result, err
	}

	log.Debugf("using agent: %s", agentKey)

	// Resolve poFile when not specified: use git diff to find changed po files
	changedPoFiles, err := GetChangedPoFiles(commit, since)
	if err != nil {
		return result, fmt.Errorf("failed to get changed po files: %w", err)
	}

	poFile, err = ResolvePoFile(poFile, changedPoFiles)
	if err != nil {
		return result, err
	}

	// Determine PO file path (convert to absolute)
	poFile, err = GetPoFileAbsPath(cfg, poFile)
	if err != nil {
		return result, err
	}

	log.Debugf("PO file path: %s", poFile)

	// Check if PO file exists
	if !Exist(poFile) {
		log.Errorf("PO file does not exist: %s", poFile)
		return result, fmt.Errorf("PO file does not exist: %s\nHint: Ensure the PO file exists before running review", poFile)
	}

	workDir := repository.WorkDir()
	poFileName := filepath.Base(poFile)
	langCode := strings.TrimSuffix(poFileName, ".po")
	poDir := filepath.Join(workDir, PoDir)

	// Step 1: Prepare review data
	log.Infof("preparing review data")
	reviewInputPath := filepath.Join(poDir, fmt.Sprintf("%s-review-input.po", langCode))
	if err := PrepareReviewData0(poFile, commit, since, reviewInputPath); err != nil {
		return result, fmt.Errorf("failed to prepare review data: %w", err)
	}

	// Step 2: Copy review-input.po to review-output.po
	reviewOutputPath := filepath.Join(poDir, fmt.Sprintf("%s-review-output.po", langCode))
	log.Debugf("copying review-input.po to review-output.po")
	if err := copyFile(reviewInputPath, reviewOutputPath); err != nil {
		return result, fmt.Errorf("failed to copy review-input to review-output: %w", err)
	}

	// Step 3: Get prompt.review and execute agent
	prompt, err := GetPrompt(cfg, "review")
	if err != nil {
		return result, err
	}

	// Get relative path for source placeholder
	reviewOutputRelPath := filepath.Join(PoDir, fmt.Sprintf("%s-review-output.po", langCode))
	sourcePath := filepath.ToSlash(reviewOutputRelPath)

	log.Debugf("using review prompt: %s", prompt)
	log.Infof("reviewing file: %s", reviewOutputPath)

	// Build agent command with placeholders replaced
	agentCmd := BuildAgentCommand(selectedAgent, prompt, sourcePath, "")

	// Determine output format
	outputFormat := selectedAgent.Output
	if outputFormat == "" {
		outputFormat = "default"
	}
	// Normalize output format (convert underscores to hyphens)
	outputFormat = normalizeOutputFormat(outputFormat)

	// Execute agent command
	log.Infof("executing agent command: %s", strings.Join(agentCmd, " "))
	result.AgentExecuted = true

	var stdout []byte
	var stderr []byte
	var jsonResult *ClaudeJSONOutput
	var codexResult *CodexJSONOutput
	var originalStdout []byte

	// Detect agent type
	isCodex := len(agentCmd) > 0 && agentCmd[0] == "codex"

	// Use streaming execution for json format (treated as stream-json)
	if outputFormat == "json" {
		stdoutReader, stderrBuf, cmdProcess, err := ExecuteAgentCommandStream(agentCmd, workDir)
		if err != nil {
			log.Errorf("agent command execution failed: %v", err)
			return result, fmt.Errorf("agent command failed: %w\nHint: Check that the agent command is correct and executable", err)
		}
		defer stdoutReader.Close()

		// For review, we need to capture the original stream for JSON extraction
		// Read all output into a buffer while also parsing in real-time
		var stdoutBuf bytes.Buffer
		teeReader := io.TeeReader(stdoutReader, &stdoutBuf)

		// Parse stream in real-time based on agent type
		if isCodex {
			parsedStdout, finalResult, err := ParseCodexJSONLRealtime(teeReader)
			if err != nil {
				log.Warnf("failed to parse codex JSONL: %v", err)
			}
			codexResult = finalResult
			stdout = parsedStdout
		} else {
			parsedStdout, finalResult, err := ParseStreamJSONRealtime(teeReader)
			if err != nil {
				log.Warnf("failed to parse stream JSON: %v", err)
			}
			jsonResult = finalResult
			stdout = parsedStdout
		}
		originalStdout = stdoutBuf.Bytes()

		// Wait for command to complete and get stderr
		waitErr := cmdProcess.Wait()
		stderr = stderrBuf.Bytes()

		if waitErr != nil {
			if len(stderr) > 0 {
				log.Debugf("agent command stderr: %s", string(stderr))
			}
			result.AgentError = fmt.Sprintf("agent command failed: %v (see logs for agent stderr output)", waitErr)
			log.Errorf("agent command execution failed: %v", waitErr)
			return result, fmt.Errorf("agent command failed: %w\nHint: Check that the agent command is correct and executable", waitErr)
		}
		result.AgentSuccess = true
		log.Infof("agent command completed successfully")
	} else {
		// Use regular execution for other formats
		var err error
		stdout, stderr, err = ExecuteAgentCommand(agentCmd, workDir)
		originalStdout = stdout
		result.AgentStdout = stdout
		result.AgentStderr = stderr

		if err != nil {
			// Log stderr if available
			if len(stderr) > 0 {
				log.Debugf("agent command stderr: %s", string(stderr))
			}
			// Log stdout if available
			if len(stdout) > 0 {
				log.Debugf("agent command stdout: %s", string(stdout))
			}

			result.AgentError = fmt.Sprintf("agent command failed: %v (see logs for agent stderr output)", err)
			log.Errorf("agent command execution failed: %v", err)
			return result, fmt.Errorf("agent command failed: %w\nHint: Check that the agent command is correct and executable", err)
		}
		result.AgentSuccess = true
		log.Infof("agent command completed successfully")

		// Parse output based on agent output format (only for claude)
		if !isCodex {
			parsedStdout, parsedResult, err := ParseAgentOutput(stdout, outputFormat)
			if err != nil {
				log.Warnf("failed to parse agent output: %v, using raw output", err)
				parsedStdout = stdout
			} else {
				stdout = parsedStdout
				jsonResult = parsedResult
			}
		}
	}

	// Print diagnostics if available
	if codexResult != nil {
		PrintAgentDiagnostics(codexResult)
		// Extract NumTurns from diagnostics
		if codexResult.NumTurns > 0 {
			result.NumTurns = codexResult.NumTurns
		}
	} else if jsonResult != nil {
		PrintAgentDiagnostics(jsonResult)
		// Extract NumTurns from diagnostics
		if jsonResult.NumTurns > 0 {
			result.NumTurns = jsonResult.NumTurns
		}
	}

	// For review, save the original stdout (before parsing) for JSON extraction
	result.AgentStdout = originalStdout
	if len(stderr) > 0 {
		result.AgentStderr = stderr
	}

	// Log output if verbose
	if len(stdout) > 0 {
		log.Debugf("agent command stdout: %s", string(stdout))
	}
	if len(stderr) > 0 {
		log.Debugf("agent command stderr: %s", string(stderr))
	}

	// Extract JSON from agent output
	log.Infof("extracting JSON from agent output")
	log.Debugf("agent stdout length: %d bytes", len(stdout))
	jsonBytes, err := ExtractJSONFromOutput(stdout)
	if err != nil {
		log.Errorf("failed to extract JSON from agent output: %v", err)
		previewLen := 500
		if len(stdout) < previewLen {
			previewLen = len(stdout)
		}
		if previewLen > 0 {
			log.Debugf("agent stdout (first %d chars): %s", previewLen, string(stdout[:previewLen]))
		}
		result.AgentError = fmt.Sprintf("failed to extract JSON: %v", err)
		return result, fmt.Errorf("failed to extract JSON from agent output: %w\nHint: Ensure the agent outputs valid JSON", err)
	}
	log.Debugf("extracted JSON length: %d bytes", len(jsonBytes))

	// Parse JSON
	log.Infof("parsing review JSON")
	reviewJSON, err := ParseReviewJSON(jsonBytes)
	if err != nil {
		log.Errorf("failed to parse review JSON: %v", err)
		previewLen := 500
		if len(jsonBytes) < previewLen {
			previewLen = len(jsonBytes)
		}
		if previewLen > 0 {
			log.Debugf("JSON data (first %d chars): %s", previewLen, string(jsonBytes[:previewLen]))
		}
		result.AgentError = fmt.Sprintf("failed to parse JSON: %v", err)
		return result, fmt.Errorf("failed to parse review JSON: %w\nHint: Check the JSON format matches ReviewJSONResult structure", err)
	}
	log.Debugf("parsed review JSON: total_entries=%d, issues=%d", reviewJSON.TotalEntries, len(reviewJSON.Issues))

	// Recalculate total_entries from reviewInputPath file to ensure accuracy
	totalEntries, err := countMsgidEntries(reviewInputPath)
	if err != nil {
		log.Errorf("failed to count msgid entries in review input file: %v", err)
	} else {
		if totalEntries > 0 {
			// Subtract 1 to account for the header entry
			totalEntries -= 1
		}
		log.Debugf("updating total_entries from %d to %d based on actual msgid count in %s", reviewJSON.TotalEntries, totalEntries, reviewInputPath)
		reviewJSON.TotalEntries = totalEntries
	}

	// Save JSON to file
	log.Infof("saving review JSON to file")
	jsonPath, err := SaveReviewJSON(poFile, reviewJSON)
	if err != nil {
		log.Errorf("failed to save review JSON: %v", err)
		log.Debugf("PO file path: %s", poFile)
		return result, fmt.Errorf("failed to save review JSON: %w", err)
	}
	result.ReviewJSON = reviewJSON
	result.ReviewJSONPath = jsonPath
	log.Debugf("review JSON saved to: %s", jsonPath)

	// Calculate review score
	log.Infof("calculating review score")
	reviewScore, err := CalculateReviewScore(reviewJSON)
	if err != nil {
		log.Errorf("failed to calculate review score: %v", err)
		log.Debugf("review JSON: total_entries=%d, issues=%d", reviewJSON.TotalEntries, len(reviewJSON.Issues))
		return result, fmt.Errorf("failed to calculate review score: %w", err)
	}
	result.ReviewScore = reviewScore
	result.Score = reviewScore
	result.ReviewedFilePath = reviewOutputPath

	log.Infof("review completed successfully (score: %d/100, total entries: %d, issues: %d, reviewed file: %s)",
		reviewScore, reviewJSON.TotalEntries, len(reviewJSON.Issues), reviewOutputPath)

	// Record execution time
	result.ExecutionTime = time.Since(startTime)

	return result, nil
}

// CmdAgentRunReview implements the agent-run review command logic.
// It loads configuration and calls RunAgentReview, then handles errors appropriately.
func CmdAgentRunReview(agentName, poFile, commit, since string) error {
	// Load configuration
	log.Debugf("loading agent configuration")
	cfg, err := config.LoadAgentConfig()
	if err != nil {
		log.Errorf("failed to load agent configuration: %v", err)
		return fmt.Errorf("failed to load agent configuration: %w\nHint: Ensure git-po-helper.yaml exists in repository root or user home directory", err)
	}

	startTime := time.Now()

	result, err := RunAgentReview(cfg, agentName, poFile, commit, since, false)
	if err != nil {
		log.Errorf("failed to run agent review: %v", err)
		return err
	}

	// For agent-run, we require agent execution to succeed
	if !result.AgentSuccess {
		log.Errorf("agent execution failed: %s", result.AgentError)
		return fmt.Errorf("agent execution failed: %s", result.AgentError)
	}

	elapsed := time.Since(startTime)

	// Display review results
	if result.ReviewJSON != nil {
		fmt.Printf("\nReview Results:\n")
		fmt.Printf("  Total entries: %d\n", result.ReviewJSON.TotalEntries)
		fmt.Printf("  Issues found: %d\n", len(result.ReviewJSON.Issues))
		fmt.Printf("  Review score: %d/100\n", result.ReviewScore)

		// Count issues by severity
		criticalCount := 0
		majorCount := 0
		minorCount := 0
		for _, issue := range result.ReviewJSON.Issues {
			switch issue.Score {
			case 0:
				criticalCount++
			case 1:
				majorCount++
			case 2:
				minorCount++
			}
		}

		fmt.Printf("\n  Issue breakdown:\n")
		if len(result.ReviewJSON.Issues) > 0 {
			if criticalCount > 0 {
				fmt.Printf("    Critical (must fix immediately): %d\n", criticalCount)
			}
			if majorCount > 0 {
				fmt.Printf("    Major (should fix): %d\n", majorCount)
			}
			if minorCount > 0 {
				fmt.Printf("    Minor (recommended to improve): %d\n", minorCount)
			}
		}
		fmt.Printf("    Perfect entries: %d\n",
			result.ReviewJSON.TotalEntries-criticalCount-minorCount)

		if result.ReviewJSONPath != "" {
			fmt.Printf("\n  JSON saved to: %s\n", getRelativePath(result.ReviewJSONPath))
		}
		if result.ReviewedFilePath != "" {
			fmt.Printf("  Reviewed file: %s\n", getRelativePath(result.ReviewedFilePath))
		}
	}

	fmt.Printf("\nSummary:\n")
	fmt.Printf("  Execution time: %s\n", elapsed.Round(time.Millisecond))

	log.Infof("agent-run review completed successfully")
	return nil
}

// countMsgidEntries counts the number of msgid entries in a PO file by counting lines that start with "msgid "
func countMsgidEntries(filePath string) (int, error) {
	file, err := os.Open(filePath)
	if err != nil {
		return 0, fmt.Errorf("failed to open file %s: %w", filePath, err)
	}
	defer file.Close()

	scanner := bufio.NewScanner(file)
	count := 0

	for scanner.Scan() {
		line := strings.TrimSpace(scanner.Text())
		if strings.HasPrefix(line, "msgid ") {
			count++
		}
	}

	if err := scanner.Err(); err != nil {
		return 0, fmt.Errorf("error reading file %s: %w", filePath, err)
	}

	return count, nil
}
