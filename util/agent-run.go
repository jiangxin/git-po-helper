// Package util provides business logic for agent-run command.
package util

import (
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
	ReviewJSON     *ReviewJSONResult `json:"review_json,omitempty"`
	ReviewScore    int               `json:"review_score,omitempty"`
	ReviewJSONPath string            `json:"review_json_path,omitempty"`
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
		return 0, fmt.Errorf("invalid review result: total_entries must be greater than 0")
	}

	totalPossible := review.TotalEntries * 3
	totalScore := totalPossible

	for _, issue := range review.Issues {
		if issue.Score < 0 || issue.Score > 3 {
			return 0, fmt.Errorf("invalid issue score %d: must be between 0 and 3", issue.Score)
		}
		totalScore -= 3 - issue.Score
	}

	if totalScore < 0 {
		totalScore = 0
	}

	scorePercent := int(math.Round(float64(totalScore) * 100.0 / float64(totalPossible)))
	if scorePercent < 0 {
		scorePercent = 0
	} else if scorePercent > 100 {
		scorePercent = 100
	}

	return scorePercent, nil
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
func ValidatePoFile(potFile string) error {
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
		log.Debugf("running msgfmt --check on %s", potFile)
		cmd = exec.Command("msgfmt",
			"-o",
			os.DevNull,
			"--check",
			potFile)
	}
	cmd.Dir = repository.WorkDir()

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

// RunAgentUpdatePot executes a single agent-run update-pot operation.
// It performs pre-validation, executes the agent command, performs post-validation,
// and validates POT file syntax. Returns a result structure with detailed information.
func RunAgentUpdatePot(cfg *config.AgentConfig, agentName string) (*AgentRunResult, error) {
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

	// Pre-validation: Check entry count before update
	if cfg.AgentTest.PotEntriesBeforeUpdate != nil && *cfg.AgentTest.PotEntriesBeforeUpdate != 0 {
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

	// Post-validation: Check entry count after update
	if cfg.AgentTest.PotEntriesAfterUpdate != nil && *cfg.AgentTest.PotEntriesAfterUpdate != 0 {
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
func RunAgentUpdatePo(cfg *config.AgentConfig, agentName, poFile string) (*AgentRunResult, error) {
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
	workDir := repository.WorkDir()
	if poFile == "" {
		lang := cfg.DefaultLangCode
		if lang == "" {
			log.Errorf("default_lang_code is not configured in agent configuration")
			return result, fmt.Errorf("default_lang_code is not configured\nHint: Provide po/XX.po on the command line or set default_lang_code in git-po-helper.yaml")
		}
		poFile = filepath.Join(workDir, PoDir, fmt.Sprintf("%s.po", lang))
	} else if !filepath.IsAbs(poFile) {
		// Treat poFile as relative to repository root
		poFile = filepath.Join(workDir, poFile)
	}

	log.Debugf("PO file path: %s", poFile)

	// Pre-validation: Check entry count before update
	if cfg.AgentTest.PoEntriesBeforeUpdate != nil && *cfg.AgentTest.PoEntriesBeforeUpdate != 0 {
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

	// Post-validation: Check entry count after update
	if cfg.AgentTest.PoEntriesAfterUpdate != nil && *cfg.AgentTest.PoEntriesAfterUpdate != 0 {
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

	result, err := RunAgentUpdatePot(cfg, agentName)
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
	if cfg.AgentTest.PotEntriesAfterUpdate != nil && *cfg.AgentTest.PotEntriesAfterUpdate != 0 && !result.PostValidationPass {
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

	result, err := RunAgentUpdatePo(cfg, agentName, poFile)
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
	if cfg.AgentTest.PoEntriesAfterUpdate != nil && *cfg.AgentTest.PoEntriesAfterUpdate != 0 && !result.PostValidationPass {
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
func RunAgentTranslate(cfg *config.AgentConfig, agentName, poFile string) (*AgentRunResult, error) {
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
	workDir := repository.WorkDir()
	if poFile == "" {
		lang := cfg.DefaultLangCode
		if lang == "" {
			log.Errorf("default_lang_code is not configured in agent configuration")
			return result, fmt.Errorf("default_lang_code is not configured\nHint: Provide po/XX.po on the command line or set default_lang_code in git-po-helper.yaml")
		}
		poFile = filepath.Join(workDir, PoDir, fmt.Sprintf("%s.po", lang))
	} else if !filepath.IsAbs(poFile) {
		// Treat poFile as relative to repository root
		poFile = filepath.Join(workDir, poFile)
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

	// Get prompt for translate from configuration
	prompt := cfg.Prompt.Translate
	if prompt == "" {
		log.Error("prompt.translate is not configured")
		return result, fmt.Errorf("prompt.translate is not configured\nHint: Add 'prompt.translate' to git-po-helper.yaml")
	}
	log.Debugf("using translate prompt: %s", prompt)

	// Build agent command with placeholders replaced
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

	result, err := RunAgentTranslate(cfg, agentName, poFile)
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

// CmdAgentRunReview implements the agent-run review command logic.
// This is a stub implementation for Step 1. Full implementation will be
// completed in Step 5 according to the design document.
func CmdAgentRunReview(agentName, poFile, commit, since string) error {
	return fmt.Errorf("agent-run review is not yet implemented (Step 1 of implementation in progress)")
}
