// Package util provides business logic for agent-run command.
package util

import (
	"encoding/json"
	"fmt"
	"math"
	"os"
	"os/exec"
	"path/filepath"
	"strings"

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
	if review.TotalEntries <= 0 {
		log.Debugf("calculate score failed: total_entries=%d (must be > 0)", review.TotalEntries)
		return 0, fmt.Errorf("invalid review result: total_entries must be greater than 0")
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

	// Validate total_entries
	if review.TotalEntries <= 0 {
		log.Debugf("validation failed: total_entries=%d (must be > 0)", review.TotalEntries)
		return nil, fmt.Errorf("invalid review result: total_entries must be greater than 0, got %d", review.TotalEntries)
	}

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
	prompt, err := GetPrompt(cfg)
	if err != nil {
		return result, err
	}

	// Build agent command with placeholders replaced
	agentCmd := BuildAgentCommand(selectedAgent, prompt, "", "")

	// Execute agent command
	workDir := repository.WorkDir()
	log.Infof("executing agent command: %s", strings.Join(agentCmd, " "))
	stdout, stderr, err := ExecuteAgentCommand(agentCmd, workDir)
	result.AgentExecuted = true

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

	return result, nil
}

// RunAgentUpdatePo executes a single agent-run update-po operation.
// It performs pre-validation, executes the agent command, performs post-validation,
// and validates PO file syntax. Returns a result structure with detailed information.
// The agentTest parameter controls whether AgentTest configuration should be used.
// When agentTest is false (for agent-run), AgentTest configuration is ignored.
func RunAgentUpdatePo(cfg *config.AgentConfig, agentName, poFile string, agentTest bool) (*AgentRunResult, error) {
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
	prompt := cfg.Prompt.UpdatePo
	if prompt == "" {
		log.Error("prompt.update_po is not configured")
		return result, fmt.Errorf("prompt.update_po is not configured\nHint: Add 'prompt.update_po' to git-po-helper.yaml")
	}
	log.Debugf("using update-po prompt: %s", prompt)

	// Build agent command with placeholders replaced
	workDir := repository.WorkDir()
	sourcePath := poFile
	if rel, err := filepath.Rel(workDir, poFile); err == nil && rel != "" && rel != "." {
		sourcePath = filepath.ToSlash(rel)
	}
	agentCmd := BuildAgentCommand(selectedAgent, prompt, sourcePath, "")

	// Execute agent command
	log.Infof("executing agent command: %s", strings.Join(agentCmd, " "))
	stdout, stderr, err := ExecuteAgentCommand(agentCmd, workDir)
	result.AgentExecuted = true

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
	prompt := cfg.Prompt.Translate
	if prompt == "" {
		log.Error("prompt.translate is not configured")
		return result, fmt.Errorf("prompt.translate is not configured\nHint: Add 'prompt.translate' to git-po-helper.yaml")
	}
	log.Debugf("using translate prompt: %s", prompt)

	// Build agent command with placeholders replaced
	workDir := repository.WorkDir()
	sourcePath := poFile
	if rel, err := filepath.Rel(workDir, poFile); err == nil && rel != "" && rel != "." {
		sourcePath = filepath.ToSlash(rel)
	}
	agentCmd := BuildAgentCommand(selectedAgent, prompt, sourcePath, "")

	// Execute agent command
	log.Infof("executing agent command: %s", strings.Join(agentCmd, " "))
	stdout, stderr, err := ExecuteAgentCommand(agentCmd, workDir)
	result.AgentExecuted = true

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

	log.Infof("agent-run translate completed successfully")
	return nil
}

// PrepareReviewData prepares data for review by creating orig.po, new.po, and review-input.po files.
// It gets the original file from git, sorts both files by msgid, and extracts differences.
// Returns paths to orig.po, new.po, and review-input.po files.
func PrepareReviewData(poFile, commit, since string) (origPath, newPath, reviewInputPath string, err error) {
	workDir := repository.WorkDir()
	poFileName := filepath.Base(poFile)
	langCode := strings.TrimSuffix(poFileName, ".po")
	if langCode == "" || langCode == poFileName {
		return "", "", "", fmt.Errorf("invalid PO file path: %s (expected format: po/XX.po)", poFile)
	}

	poDir := filepath.Join(workDir, PoDir)
	origPath = filepath.Join(poDir, fmt.Sprintf("%s-orig.po", langCode))
	newPath = filepath.Join(poDir, fmt.Sprintf("%s-new.po", langCode))
	reviewInputPath = filepath.Join(poDir, fmt.Sprintf("%s-review-input.po", langCode))

	log.Debugf("preparing review data: orig=%s, new=%s, review-input=%s", origPath, newPath, reviewInputPath)

	// Determine the base commit for comparison and the new file source
	var baseCommit string
	var newFileSource string
	if commit != "" {
		// For commit mode: orig is parent commit, new is the specified commit
		cmd := exec.Command("git", "rev-parse", commit+"^")
		cmd.Dir = workDir
		output, err := cmd.Output()
		if err != nil {
			// If commit has no parent (root commit), use empty tree
			baseCommit = "4b825dc642cb6eb9a060e54bf8d69288fbee4904" // Empty tree
		} else {
			baseCommit = strings.TrimSpace(string(output))
		}
		newFileSource = commit
		log.Infof("commit mode: orig from %s, new from %s", baseCommit, commit)
	} else if since != "" {
		// Since mode: orig is since commit, new is current file
		baseCommit = since
		newFileSource = "" // Use current file
		log.Infof("since mode: orig from %s, new from current file", since)
	} else {
		// Default mode: orig is HEAD, new is current file
		baseCommit = "HEAD"
		newFileSource = "" // Use current file
		log.Infof("default mode: orig from HEAD, new from current file")
	}

	// Get original file from git
	log.Infof("getting original file from commit: %s", baseCommit)
	// Convert absolute path to relative path for git show command
	poFileRel, err := filepath.Rel(workDir, poFile)
	if err != nil {
		return "", "", "", fmt.Errorf("failed to convert PO file path to relative: %w", err)
	}
	// Normalize to use forward slashes (git uses forward slashes in paths)
	poFileRel = filepath.ToSlash(poFileRel)
	origFileRevision := FileRevision{
		Revision: baseCommit,
		File:     poFileRel,
	}
	if err := checkoutTmpfile(&origFileRevision); err != nil {
		// Check if error is because file doesn't exist in the commit
		if strings.Contains(err.Error(), "does not exist in") {
			// If file doesn't exist in that commit, create empty file
			log.Infof("file %s not found in commit %s, using empty file as original", poFileRel, baseCommit)
			if err := os.WriteFile(origPath, []byte{}, 0644); err != nil {
				return "", "", "", fmt.Errorf("failed to create empty orig file: %w", err)
			}
		} else {
			// For other errors, return them
			return "", "", "", fmt.Errorf("failed to get original file from commit %s: %w", baseCommit, err)
		}
	} else {
		// Copy tmpfile to orig.po
		origData, err := os.ReadFile(origFileRevision.Tmpfile)
		if err != nil {
			os.Remove(origFileRevision.Tmpfile)
			return "", "", "", fmt.Errorf("failed to read orig tmpfile: %w", err)
		}
		if err := os.WriteFile(origPath, origData, 0644); err != nil {
			os.Remove(origFileRevision.Tmpfile)
			return "", "", "", fmt.Errorf("failed to write orig file: %w", err)
		}
		os.Remove(origFileRevision.Tmpfile)
	}

	// Get new file (either from git commit or current file)
	if newFileSource != "" {
		// Get file from specified commit
		log.Debugf("getting new file from commit: %s", newFileSource)
		// Use the same relative path for git show command
		newFileRevision := FileRevision{
			Revision: newFileSource,
			File:     poFileRel,
		}
		if err := checkoutTmpfile(&newFileRevision); err != nil {
			return "", "", "", fmt.Errorf("failed to get new file from commit %s: %w", newFileSource, err)
		}
		newData, err := os.ReadFile(newFileRevision.Tmpfile)
		if err != nil {
			os.Remove(newFileRevision.Tmpfile)
			return "", "", "", fmt.Errorf("failed to read new tmpfile: %w", err)
		}
		if err := os.WriteFile(newPath, newData, 0644); err != nil {
			os.Remove(newFileRevision.Tmpfile)
			return "", "", "", fmt.Errorf("failed to write new file: %w", err)
		}
		os.Remove(newFileRevision.Tmpfile)
	} else {
		// Copy current file to new.po
		log.Debugf("copying current file to new.po")
		newData, err := os.ReadFile(poFile)
		if err != nil {
			return "", "", "", fmt.Errorf("failed to read current PO file: %w", err)
		}
		if err := os.WriteFile(newPath, newData, 0644); err != nil {
			return "", "", "", fmt.Errorf("failed to write new file: %w", err)
		}
	}

	// Sort both files by msgid using msgcat
	log.Debugf("sorting files by msgid")
	origSortedPath := filepath.Join(poDir, fmt.Sprintf("%s-orig-sorted.po", langCode))
	newSortedPath := filepath.Join(poDir, fmt.Sprintf("%s-new-sorted.po", langCode))
	defer func() {
		os.Remove(origSortedPath)
		os.Remove(newSortedPath)
	}()

	// Sort orig file
	cmd := exec.Command("msgcat", "--sort-output", origPath, "-o", origSortedPath)
	cmd.Dir = workDir
	if err := cmd.Run(); err != nil {
		// If msgcat fails (e.g., empty file), just copy the file
		log.Debugf("msgcat sort failed for orig, copying as-is: %v", err)
		if err := copyFile(origPath, origSortedPath); err != nil {
			return "", "", "", fmt.Errorf("failed to copy orig file: %w", err)
		}
	}

	// Sort new file
	cmd = exec.Command("msgcat", "--sort-output", newPath, "-o", newSortedPath)
	cmd.Dir = workDir
	if err := cmd.Run(); err != nil {
		return "", "", "", fmt.Errorf("failed to sort new file: %w", err)
	}

	// Extract differences: use msgcmp to find entries that are different or new
	// We'll use a simpler approach: extract entries from new that don't match orig
	log.Debugf("extracting differences to review-input.po")
	if err := extractReviewInput(origSortedPath, newSortedPath, reviewInputPath); err != nil {
		return "", "", "", fmt.Errorf("failed to extract review input: %w", err)
	}

	log.Infof("review data prepared: review-input=%s", reviewInputPath)
	return origPath, newPath, reviewInputPath, nil
}

// copyFile copies a file from src to dst.
func copyFile(src, dst string) error {
	data, err := os.ReadFile(src)
	if err != nil {
		return err
	}
	return os.WriteFile(dst, data, 0644)
}

// extractReviewInput extracts entries from new.po that are different or new compared to orig.po.
// It copies the header and first empty msgid entry from new.po, then adds all entries
// that are new or different in new.po.
func extractReviewInput(origPath, newPath, outputPath string) error {
	// Read both files
	origData, err := os.ReadFile(origPath)
	if err != nil {
		return fmt.Errorf("failed to read orig file: %w", err)
	}
	newData, err := os.ReadFile(newPath)
	if err != nil {
		return fmt.Errorf("failed to read new file: %w", err)
	}

	// Parse entries from both files
	origEntries, _, err := parsePoEntries(origData)
	if err != nil {
		return fmt.Errorf("failed to parse orig file: %w", err)
	}
	newEntries, newHeader, err := parsePoEntries(newData)
	if err != nil {
		return fmt.Errorf("failed to parse new file: %w", err)
	}

	// If orig file is empty, all entries in new file will be considered new
	// This handles the case where the file doesn't exist in HEAD
	if len(origData) == 0 {
		log.Debugf("orig file is empty, all entries in new file will be included in review-input")
	}

	// Create a map of orig entries by msgid for quick lookup
	origMap := make(map[string]*PoEntry)
	for _, entry := range origEntries {
		origMap[entry.MsgID] = entry
	}

	// Extract entries that are new or different
	var reviewEntries []*PoEntry
	for _, newEntry := range newEntries {
		origEntry, exists := origMap[newEntry.MsgID]
		if !exists {
			// New entry
			reviewEntries = append(reviewEntries, newEntry)
		} else if !entriesEqual(origEntry, newEntry) {
			// Different entry (msgid or msgstr changed)
			reviewEntries = append(reviewEntries, newEntry)
		}
		// If entry exists and is equal, skip it
	}

	// Write review-input.po with header and review entries
	return writeReviewInputPo(outputPath, newHeader, reviewEntries)
}

// PoEntry represents a single PO file entry.
type PoEntry struct {
	Comments     []string
	MsgID        string
	MsgStr       string
	MsgIDPlural  string
	MsgStrPlural []string
	RawLines     []string // Original lines for the entry
}

// parsePoEntries parses PO file entries and returns entries and header.
func parsePoEntries(data []byte) (entries []*PoEntry, header []string, err error) {
	lines := strings.Split(string(data), "\n")
	var currentEntry *PoEntry
	var inHeader = true
	var headerLines []string
	var entryLines []string
	var msgidValue strings.Builder
	var msgstrValue strings.Builder
	var msgidPluralValue strings.Builder
	var msgstrPluralValues []strings.Builder
	var inMsgid, inMsgstr, inMsgidPlural bool
	var currentPluralIndex int = -1

	for _, line := range lines {
		trimmed := strings.TrimSpace(line)

		// Check for header (empty msgid entry)
		if inHeader && strings.HasPrefix(trimmed, "msgid ") {
			value := strings.TrimPrefix(trimmed, "msgid ")
			value = strings.Trim(value, `"`)
			if value == "" {
				// This is the header entry
				inHeader = true
				headerLines = append(headerLines, line)
				entryLines = append(entryLines, line)
				// Continue to collect header
				continue
			}
		}

		// Check for header msgstr (empty msgstr after empty msgid)
		if inHeader && strings.HasPrefix(trimmed, "msgstr ") {
			value := strings.TrimPrefix(trimmed, "msgstr ")
			value = strings.Trim(value, `"`)
			if msgidValue.Len() == 0 && value == "" {
				// This is the header msgstr line
				headerLines = append(headerLines, line)
				// Continue collecting header (including continuation lines starting with ")
				// Header ends when we encounter an empty line or a new msgid entry
				continue
			}
		}

		// Collect header lines (including continuation lines after msgstr "")
		if inHeader {
			// Check if this is a continuation line of header msgstr (starts with ")
			// Only collect as header if we're still in header mode and haven't started parsing an entry
			// Also check that we're not in the middle of parsing a msgid or msgstr (which would indicate an entry)
			if strings.HasPrefix(trimmed, `"`) {
				// If we're already parsing an entry (currentEntry exists or inMsgid/inMsgstr is set),
				// this continuation line belongs to the entry, not the header
				if currentEntry != nil || inMsgid || inMsgstr || inMsgidPlural {
					// This is a continuation line of an entry, not header
					// Don't process it here, let it be handled by entry parsing logic below
				} else {
					// For header continuation lines, keep the quotes
					headerLines = append(headerLines, trimmed)
					continue
				}
			}
			// Check if this is an empty line - end of header
			if trimmed == "" {
				inHeader = false
				msgidValue.Reset()
				msgstrValue.Reset()
				continue
			}
			// Check if this is a new msgid entry - end of header
			if strings.HasPrefix(trimmed, "msgid ") {
				value := strings.TrimPrefix(trimmed, "msgid ")
				value = strings.Trim(value, `"`)
				if value != "" {
					// This is a real entry, not header
					inHeader = false
					msgidValue.Reset()
					msgstrValue.Reset()
					// Don't continue, let it be processed as a normal entry
				} else {
					// This is a duplicate empty msgid after header - this should not happen
					// in a valid PO file, but if it does, end the header and start a new entry
					inHeader = false
					msgidValue.Reset()
					msgstrValue.Reset()
					// Don't continue, let it be processed as a normal entry
				}
			} else {
				// Other header lines (comments, etc.)
				headerLines = append(headerLines, line)
				continue
			}
		}

		// Parse entry
		if strings.HasPrefix(trimmed, "#") {
			// Comment line
			if currentEntry == nil {
				currentEntry = &PoEntry{}
				entryLines = []string{}
			}
			currentEntry.Comments = append(currentEntry.Comments, line)
			entryLines = append(entryLines, line)
		} else if strings.HasPrefix(trimmed, "msgid ") {
			// Start of new entry
			// Save previous entry if we have one and it has content
			// (either msgid with continuation lines or msgstr)
			if currentEntry != nil && (msgidValue.Len() > 0 || msgstrValue.Len() > 0) {
				// Save previous entry
				currentEntry.MsgID = msgidValue.String()
				currentEntry.MsgStr = msgstrValue.String()
				currentEntry.RawLines = entryLines
				entries = append(entries, currentEntry)
			}
			// Start new entry (or continue existing entry if it only has comments)
			if currentEntry == nil {
				// Create a new entry
				currentEntry = &PoEntry{}
				entryLines = []string{}
			} else if msgidValue.Len() > 0 || msgstrValue.Len() > 0 {
				// Previous entry was saved, create new entry
				currentEntry = &PoEntry{}
				entryLines = []string{}
			}
			// If currentEntry has comments but no msgid/msgstr, keep it and continue
			// entryLines already contains the comments, so we don't reset it
			msgidValue.Reset()
			msgstrValue.Reset()
			msgidPluralValue.Reset()
			msgstrPluralValues = []strings.Builder{}
			inMsgid = true
			inMsgstr = false
			inMsgidPlural = false
			currentPluralIndex = -1

			value := strings.TrimPrefix(trimmed, "msgid ")
			value = strings.Trim(value, `"`)
			msgidValue.WriteString(value)
			entryLines = append(entryLines, line)
		} else if strings.HasPrefix(trimmed, "msgid_plural ") {
			inMsgid = false
			inMsgidPlural = true
			value := strings.TrimPrefix(trimmed, "msgid_plural ")
			value = strings.Trim(value, `"`)
			msgidPluralValue.WriteString(value)
			entryLines = append(entryLines, line)
		} else if strings.HasPrefix(trimmed, "msgstr[") {
			// Plural form
			inMsgid = false
			inMsgidPlural = false
			inMsgstr = true
			// Extract index
			idxStr := strings.TrimPrefix(trimmed, "msgstr[")
			idxStr = strings.Split(idxStr, "]")[0]
			var idx int
			fmt.Sscanf(idxStr, "%d", &idx)
			// Extend slice if needed
			for len(msgstrPluralValues) <= idx {
				msgstrPluralValues = append(msgstrPluralValues, strings.Builder{})
			}
			currentPluralIndex = idx
			value := strings.TrimPrefix(trimmed, fmt.Sprintf("msgstr[%d] ", idx))
			value = strings.Trim(value, `"`)
			msgstrPluralValues[idx].WriteString(value)
			entryLines = append(entryLines, line)
		} else if strings.HasPrefix(trimmed, "msgstr ") {
			inMsgid = false
			inMsgidPlural = false
			inMsgstr = true
			value := strings.TrimPrefix(trimmed, "msgstr ")
			value = strings.Trim(value, `"`)
			msgstrValue.WriteString(value)
			entryLines = append(entryLines, line)
		} else if strings.HasPrefix(trimmed, `"`) && (inMsgid || inMsgstr || inMsgidPlural) {
			// Continuation line
			value := strings.Trim(trimmed, `"`)
			if inMsgid {
				msgidValue.WriteString(value)
			} else if inMsgidPlural {
				msgidPluralValue.WriteString(value)
			} else if inMsgstr {
				if currentPluralIndex >= 0 {
					msgstrPluralValues[currentPluralIndex].WriteString(value)
				} else {
					msgstrValue.WriteString(value)
				}
			}
			entryLines = append(entryLines, line)
		} else if trimmed == "" {
			// Empty line - end of entry (only if we have a current entry)
			// For entries with msgid starting with empty string, we need to check
			// if we have collected any continuation lines (msgidValue.Len() > 0)
			// or if we have a complete entry with msgstr
			if currentEntry != nil && (msgidValue.Len() > 0 || msgstrValue.Len() > 0) {
				currentEntry.MsgID = msgidValue.String()
				currentEntry.MsgStr = msgstrValue.String()
				if msgidPluralValue.Len() > 0 {
					currentEntry.MsgIDPlural = msgidPluralValue.String()
					currentEntry.MsgStrPlural = make([]string, len(msgstrPluralValues))
					for i, b := range msgstrPluralValues {
						currentEntry.MsgStrPlural[i] = b.String()
					}
				}
				currentEntry.RawLines = entryLines
				entries = append(entries, currentEntry)
			}
			currentEntry = nil
			entryLines = []string{}
			msgidValue.Reset()
			msgstrValue.Reset()
			msgidPluralValue.Reset()
			msgstrPluralValues = []strings.Builder{}
			inMsgid = false
			inMsgstr = false
			inMsgidPlural = false
			currentPluralIndex = -1
		} else {
			// Other lines (continuation, etc.)
			if currentEntry != nil {
				entryLines = append(entryLines, line)
			} else if !inHeader {
				// If we're not in header and not in an entry, this might be a continuation
				// of a previous entry or a new entry starting
				entryLines = append(entryLines, line)
			}
		}
	}

	// Handle last entry
	// For entries with msgid starting with empty string, we need to check
	// if we have collected any continuation lines (msgidValue.Len() > 0)
	// or if we have a complete entry with msgstr
	if currentEntry != nil && (msgidValue.Len() > 0 || msgstrValue.Len() > 0) {
		currentEntry.MsgID = msgidValue.String()
		currentEntry.MsgStr = msgstrValue.String()
		if msgidPluralValue.Len() > 0 {
			currentEntry.MsgIDPlural = msgidPluralValue.String()
			currentEntry.MsgStrPlural = make([]string, len(msgstrPluralValues))
			for i, b := range msgstrPluralValues {
				currentEntry.MsgStrPlural[i] = b.String()
			}
		}
		currentEntry.RawLines = entryLines
		entries = append(entries, currentEntry)
	}

	return entries, headerLines, nil
}

// entriesEqual checks if two PO entries are equal (same msgid and msgstr).
func entriesEqual(e1, e2 *PoEntry) bool {
	if e1.MsgID != e2.MsgID {
		return false
	}
	if e1.MsgStr != e2.MsgStr {
		return false
	}
	if e1.MsgIDPlural != e2.MsgIDPlural {
		return false
	}
	if len(e1.MsgStrPlural) != len(e2.MsgStrPlural) {
		return false
	}
	for i := range e1.MsgStrPlural {
		if e1.MsgStrPlural[i] != e2.MsgStrPlural[i] {
			return false
		}
	}
	return true
}

// writeReviewInputPo writes the review input PO file with header and review entries.
func writeReviewInputPo(outputPath string, header []string, entries []*PoEntry) error {
	var content strings.Builder

	// Write header
	// Header structure:
	// - Comments (if any) - lines starting with #
	// - msgid "" - line starting with msgid
	// - msgstr "" - line starting with msgstr
	// - Continuation lines - already wrapped in quotes (preserved from parsePoEntries)
	for _, line := range header {
		content.WriteString(line)
		// Only add newline if the line doesn't already end with \n
		if !strings.HasSuffix(line, "\n") {
			content.WriteString("\n")
		}
	}

	// Add empty line after header
	content.WriteString("\n")

	// Write entries
	for _, entry := range entries {
		for _, line := range entry.RawLines {
			content.WriteString(line)
			content.WriteString("\n")
		}
		content.WriteString("\n")
	}

	return os.WriteFile(outputPath, []byte(content.String()), 0644)
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
		return result, fmt.Errorf("PO file does not exist: %s\nHint: Ensure the PO file exists before running review", poFile)
	}

	// Step 1: Prepare review data
	log.Infof("preparing review data")
	origPath, newPath, reviewInputPath, err := PrepareReviewData(poFile, commit, since)
	if err != nil {
		return result, fmt.Errorf("failed to prepare review data: %w", err)
	}
	defer func() {
		// Clean up temporary files
		os.Remove(origPath)
		os.Remove(newPath)
	}()

	// Step 2: Copy review-input.po to review-output.po
	workDir := repository.WorkDir()
	poFileName := filepath.Base(poFile)
	langCode := strings.TrimSuffix(poFileName, ".po")
	poDir := filepath.Join(workDir, PoDir)
	reviewOutputPath := filepath.Join(poDir, fmt.Sprintf("%s-review-output.po", langCode))
	reviewedPath := filepath.Join(poDir, fmt.Sprintf("%s-reviewed.po", langCode))

	log.Debugf("copying review-input.po to review-output.po")
	if err := copyFile(reviewInputPath, reviewOutputPath); err != nil {
		return result, fmt.Errorf("failed to copy review-input to review-output: %w", err)
	}

	// Step 3: Get prompt.review and execute agent
	prompt := cfg.Prompt.Review
	if prompt == "" {
		log.Error("prompt.review is not configured")
		return result, fmt.Errorf("prompt.review is not configured\nHint: Add 'prompt.review' to git-po-helper.yaml")
	}

	// Get relative path for source placeholder
	reviewOutputRelPath := filepath.Join(PoDir, fmt.Sprintf("%s-review-output.po", langCode))
	sourcePath := filepath.ToSlash(reviewOutputRelPath)

	log.Debugf("using review prompt: %s", prompt)
	log.Infof("reviewing file: %s", reviewOutputPath)

	// Build agent command with placeholders replaced
	agentCmd := BuildAgentCommand(selectedAgent, prompt, sourcePath, "")

	// Execute agent command
	log.Infof("executing agent command: %s", strings.Join(agentCmd, " "))
	stdout, stderr, err := ExecuteAgentCommand(agentCmd, workDir)
	result.AgentExecuted = true
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

	// Log output if verbose
	if len(stdout) > 0 {
		log.Debugf("agent command stdout: %s", string(stdout))
	}
	if len(stderr) > 0 {
		log.Debugf("agent command stderr: %s", string(stderr))
	}

	// Step 4: Merge review-output.po with new.po using msgcat --use-first
	log.Infof("merging review-output.po with new.po using msgcat")
	cmd := exec.Command("msgcat", "--use-first", reviewOutputPath, newPath, "-o", reviewedPath)
	cmd.Dir = workDir
	// Capture stderr to show error details
	var stderrBuf strings.Builder
	cmd.Stderr = &stderrBuf
	if err := cmd.Run(); err != nil {
		stderrStr := stderrBuf.String()
		if stderrStr != "" {
			log.Errorf("failed to merge files with msgcat: %v\nstderr: %s", err, stderrStr)
			return result, fmt.Errorf("failed to merge review-output.po with new.po: %w\nstderr: %s\nHint: Check that msgcat is available and files are valid", err, stderrStr)
		}
		log.Errorf("failed to merge files with msgcat: %v", err)
		return result, fmt.Errorf("failed to merge review-output.po with new.po: %w\nHint: Check that msgcat is available and files are valid", err)
	}
	log.Debugf("merged file saved to: %s", reviewedPath)

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
	result.ReviewedFilePath = reviewedPath

	log.Infof("review completed successfully (score: %d/100, total entries: %d, issues: %d, reviewed file: %s)",
		reviewScore, reviewJSON.TotalEntries, len(reviewJSON.Issues), reviewedPath)

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

	result, err := RunAgentReview(cfg, agentName, poFile, commit, since, false)
	if err != nil {
		return err
	}

	// For agent-run, we require agent execution to succeed
	if !result.AgentSuccess {
		return fmt.Errorf("agent execution failed: %s", result.AgentError)
	}

	// Display review results
	if result.ReviewJSON != nil {
		fmt.Printf("\nReview Results:\n")
		fmt.Printf("  Total entries: %d\n", result.ReviewJSON.TotalEntries)
		fmt.Printf("  Issues found: %d\n", len(result.ReviewJSON.Issues))
		fmt.Printf("  Review score: %d/100\n", result.ReviewScore)

		// Count issues by severity
		criticalCount := 0
		minorCount := 0
		perfectCount := 0
		for _, issue := range result.ReviewJSON.Issues {
			if issue.Score == 0 {
				criticalCount++
			} else if issue.Score == 2 {
				minorCount++
			} else if issue.Score == 3 {
				perfectCount++
			}
		}

		if len(result.ReviewJSON.Issues) > 0 {
			fmt.Printf("\n  Issue breakdown:\n")
			if criticalCount > 0 {
				fmt.Printf("    Critical (must fix): %d\n", criticalCount)
			}
			if minorCount > 0 {
				fmt.Printf("    Minor (needs adjustment): %d\n", minorCount)
			}
			if perfectCount > 0 {
				fmt.Printf("    Perfect entries: %d\n", perfectCount)
			}
		}

		if result.ReviewJSONPath != "" {
			fmt.Printf("\n  JSON saved to: %s\n", result.ReviewJSONPath)
		}
		if result.ReviewedFilePath != "" {
			fmt.Printf("  Reviewed file: %s\n", result.ReviewedFilePath)
		}
	}

	log.Infof("agent-run review completed successfully")
	return nil
}
