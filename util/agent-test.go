// Package util provides business logic for agent-test command.
package util

import (
	"bufio"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"time"

	"github.com/git-l10n/git-po-helper/config"
	"github.com/git-l10n/git-po-helper/repository"
	log "github.com/sirupsen/logrus"
)

// RunResult holds the result of a single test run.
type RunResult struct {
	RunNumber           int
	Score               int
	PreValidationPass   bool
	PostValidationPass  bool
	AgentExecuted       bool
	AgentSuccess        bool
	PreValidationError  string
	PostValidationError string
	AgentError          string
	BeforeCount         int
	AfterCount          int
	BeforeNewCount      int // For translate: new (untranslated) entries before
	AfterNewCount       int // For translate: new (untranslated) entries after
	BeforeFuzzyCount    int // For translate: fuzzy entries before
	AfterFuzzyCount     int // For translate: fuzzy entries after
	ExpectedBefore      *int
	ExpectedAfter       *int
}

// ConfirmAgentTestExecution displays a warning and requires user confirmation before proceeding.
// The user must explicitly type "yes" to continue, otherwise the function returns an error.
// This is used to prevent accidental data loss when agent-test commands modify po/ directory.
// If skipConfirmation is true, the confirmation prompt is skipped.
func ConfirmAgentTestExecution(skipConfirmation bool) error {
	if skipConfirmation {
		log.Debugf("skipping confirmation prompt (--dangerously-remove-po-directory flag set)")
		return nil
	}

	fmt.Fprintln(os.Stderr, "WARNING: This command will modify files under po/ and may cause data loss.")
	fmt.Fprint(os.Stderr, "Are you sure you want to continue? Type 'yes' to proceed: ")

	reader := bufio.NewReader(os.Stdin)
	answer, err := reader.ReadString('\n')
	if err != nil {
		return fmt.Errorf("failed to read user input: %w", err)
	}

	answer = strings.TrimSpace(strings.ToLower(answer))
	if answer != "yes" {
		return fmt.Errorf("operation cancelled by user")
	}

	return nil
}

// CleanPoDirectory restores the po/ directory to its state in HEAD using git restore.
// This is useful for agent-test operations to ensure a clean state before each test run.
// Returns an error if the git restore command fails.
func CleanPoDirectory(paths ...string) error {
	workDir := repository.WorkDir()

	// If no paths provided, use default "po/"
	targetPaths := paths
	if len(targetPaths) == 0 {
		targetPaths = []string{"po/"}
	}

	log.Debugf("cleaning paths using git restore (workDir: %s, paths: %v)", workDir, targetPaths)

	// Process each path individually to avoid failures on non-existent paths
	for _, path := range targetPaths {
		log.Debugf("restoring path: %s", path)

		// Build git restore command for this path
		args := []string{
			"restore",
			"--staged",
			"--worktree",
			"--source", "HEAD",
			"--",
			path,
		}

		cmd := exec.Command("git", args...)
		cmd.Dir = workDir

		// Capture stderr for error messages
		stderr, err := cmd.StderrPipe()
		if err != nil {
			log.Debugf("failed to create stderr pipe for git restore on path %s: %v", path, err)
			continue // Skip this path and continue with next
		}

		if err := cmd.Start(); err != nil {
			log.Debugf("failed to start git restore command for path %s: %v", path, err)
			continue // Skip this path and continue with next
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
			// Ignore errors for individual paths (path might not exist in repository)
			errorMsg := stderrOutput.String()
			if errorMsg == "" {
				errorMsg = err.Error()
			}
			log.Debugf("git restore failed for path %s (ignored): %s", path, errorMsg)
		} else {
			log.Debugf("path %s restored successfully", path)
		}
	}

	log.Debugf("all paths processed")

	// Clean untracked po/git.pot file that might not be in git repository
	// Only clean po/git.pot if default path "po/" is being used or explicitly specified
	shouldCleanPot := len(paths) == 0 || containsPath(paths, "po/") || containsPath(paths, "po/git.pot")
	if shouldCleanPot {
		log.Debugf("cleaning untracked po/git.pot file using git clean")
		cleanCmd := exec.Command("git",
			"clean",
			"-fx",
			"--",
			"po/git.pot")
		cleanCmd.Dir = workDir

		// Capture stderr for error messages
		cleanStderr, err := cleanCmd.StderrPipe()
		if err != nil {
			log.Warnf("failed to create stderr pipe for git clean: %v", err)
			// Continue even if we can't capture stderr
		} else {
			if err := cleanCmd.Start(); err != nil {
				log.Warnf("failed to start git clean command: %v", err)
				// Continue even if git clean fails
			} else {
				// Read stderr output
				var cleanStderrOutput strings.Builder
				buf := make([]byte, 1024)
				for {
					n, err := cleanStderr.Read(buf)
					if n > 0 {
						cleanStderrOutput.Write(buf[:n])
					}
					if err != nil {
						break
					}
				}

				if err := cleanCmd.Wait(); err != nil {
					// git clean may fail if there's nothing to clean, which is fine
					errorMsg := cleanStderrOutput.String()
					if errorMsg != "" {
						log.Debugf("git clean output: %s", errorMsg)
					}
					log.Debugf("git clean completed (exit code may be non-zero if nothing to clean)")
				} else {
					log.Debugf("untracked po/git.pot file cleaned successfully")
				}
			}
		}
	} else {
		log.Debugf("skipping po/git.pot cleanup (not in specified paths)")
	}

	log.Debugf("paths cleaned successfully")
	return nil
}

// containsPath checks if a path exists in the paths slice (exact match or prefix match).
func containsPath(paths []string, target string) bool {
	for _, p := range paths {
		if p == target || strings.HasPrefix(target, p) || strings.HasPrefix(p, target) {
			return true
		}
	}
	return false
}

// CmdAgentTestUpdatePot implements the agent-test update-pot command logic.
// It runs the agent-run update-pot operation multiple times and calculates an average score.
func CmdAgentTestUpdatePot(agentName string, runs int, skipConfirmation bool) error {
	// Require user confirmation before proceeding
	if err := ConfirmAgentTestExecution(skipConfirmation); err != nil {
		return err
	}

	// Load configuration
	log.Debugf("loading agent configuration")
	cfg, err := config.LoadAgentConfig()
	if err != nil {
		log.Errorf("failed to load agent configuration: %v", err)
		return fmt.Errorf("failed to load agent configuration: %w\nHint: Ensure git-po-helper.yaml exists in repository root or user home directory", err)
	}

	// Determine number of runs
	if runs == 0 {
		if cfg.AgentTest.Runs != nil && *cfg.AgentTest.Runs > 0 {
			runs = *cfg.AgentTest.Runs
			log.Debugf("using runs from configuration: %d", runs)
		} else {
			runs = 5 // Default
			log.Debugf("using default number of runs: %d", runs)
		}
	} else {
		log.Debugf("using runs from command line: %d", runs)
	}

	log.Infof("starting agent-test update-pot with %d runs", runs)

	startTime := time.Now()

	// Run the test
	results, averageScore, err := RunAgentTestUpdatePot(agentName, runs, cfg)
	if err != nil {
		log.Errorf("agent-test execution failed: %v", err)
		return fmt.Errorf("agent-test failed: %w", err)
	}

	elapsed := time.Since(startTime)

	// Display results
	log.Debugf("displaying test results (average score: %.2f)", averageScore)
	displayTestResults(results, averageScore, runs, elapsed)

	log.Infof("agent-test update-pot completed successfully (average score: %.2f/100)", averageScore)
	return nil
}

// RunAgentTestUpdatePot runs the agent-test update-pot operation multiple times.
// It reuses RunAgentUpdatePot for each run and accumulates scores.
// Returns scores for each run, average score, and error.
func RunAgentTestUpdatePot(agentName string, runs int, cfg *config.AgentConfig) ([]RunResult, float64, error) {
	// Run the test multiple times
	results := make([]RunResult, runs)
	totalScore := 0

	for i := 0; i < runs; i++ {
		runNum := i + 1
		log.Infof("run %d/%d", runNum, runs)

		// Clean po/ directory before each run to ensure a clean state
		if err := CleanPoDirectory("po/git.pot"); err != nil {
			log.Warnf("run %d: failed to clean po/ directory: %v", runNum, err)
			// Continue with the run even if cleanup fails, but log the warning
		}

		// Reuse RunAgentUpdatePot for each run
		agentResult, err := RunAgentUpdatePot(cfg, agentName, true)

		// Convert AgentRunResult to RunResult
		// agentResult is never nil (always returns a result structure)
		result := RunResult{
			RunNumber:           runNum,
			Score:               agentResult.Score,
			PreValidationPass:   agentResult.PreValidationPass,
			PostValidationPass:  agentResult.PostValidationPass,
			AgentExecuted:       agentResult.AgentExecuted,
			AgentSuccess:        agentResult.AgentSuccess,
			PreValidationError:  agentResult.PreValidationError,
			PostValidationError: agentResult.PostValidationError,
			AgentError:          agentResult.AgentError,
			BeforeCount:         agentResult.BeforeCount,
			AfterCount:          agentResult.AfterCount,
			ExpectedBefore:      cfg.AgentTest.PotEntriesBeforeUpdate,
			ExpectedAfter:       cfg.AgentTest.PotEntriesAfterUpdate,
		}

		// If there was an error, log it but continue (for agent-test, we want to collect all results)
		if err != nil {
			log.Debugf("run %d: agent-run returned error: %v", runNum, err)
			// Error details are already in the result structure
		}

		results[i] = result
		totalScore += result.Score
		log.Debugf("run %d: completed with score %d/100", runNum, result.Score)
	}

	// Calculate average score
	averageScore := float64(totalScore) / float64(runs)
	log.Infof("all runs completed. Total score: %d/%d, Average: %.2f/100", totalScore, runs*100, averageScore)

	return results, averageScore, nil
}

// CmdAgentTestUpdatePo implements the agent-test update-po command logic.
// It runs the agent-run update-po operation multiple times and calculates an average score.
func CmdAgentTestUpdatePo(agentName, poFile string, runs int, skipConfirmation bool) error {
	// Require user confirmation before proceeding
	if err := ConfirmAgentTestExecution(skipConfirmation); err != nil {
		return err
	}

	// Load configuration
	log.Debugf("loading agent configuration")
	cfg, err := config.LoadAgentConfig()
	if err != nil {
		log.Errorf("failed to load agent configuration: %v", err)
		return fmt.Errorf("failed to load agent configuration: %w\nHint: Ensure git-po-helper.yaml exists in repository root or user home directory", err)
	}

	// Determine number of runs
	if runs == 0 {
		if cfg.AgentTest.Runs != nil && *cfg.AgentTest.Runs > 0 {
			runs = *cfg.AgentTest.Runs
			log.Debugf("using runs from configuration: %d", runs)
		} else {
			runs = 5 // Default
			log.Debugf("using default number of runs: %d", runs)
		}
	} else {
		log.Debugf("using runs from command line: %d", runs)
	}

	log.Infof("starting agent-test update-po with %d runs", runs)

	startTime := time.Now()

	// Run the test
	results, averageScore, err := RunAgentTestUpdatePo(agentName, poFile, runs, cfg)
	if err != nil {
		log.Errorf("agent-test execution failed: %v", err)
		return fmt.Errorf("agent-test failed: %w", err)
	}

	elapsed := time.Since(startTime)

	// Display results
	log.Debugf("displaying test results (average score: %.2f)", averageScore)
	displayTestResults(results, averageScore, runs, elapsed)

	log.Infof("agent-test update-po completed successfully (average score: %.2f/100)", averageScore)
	return nil
}

// RunAgentTestUpdatePo runs the agent-test update-po operation multiple times.
// It reuses RunAgentUpdatePo for each run and accumulates scores.
// Returns scores for each run, average score, and error.
func RunAgentTestUpdatePo(agentName, poFile string, runs int, cfg *config.AgentConfig) ([]RunResult, float64, error) {
	// Run the test multiple times
	results := make([]RunResult, runs)
	totalScore := 0
	relPoFile, err := GetPoFileRelPath(cfg, poFile)
	if err != nil {
		log.Warnf("failed to get relative path of poFile: %v", err)
	}

	for i := 0; i < runs; i++ {
		runNum := i + 1
		log.Infof("run %d/%d", runNum, runs)

		// Clean po/ directory before each run to ensure a clean state
		if err := CleanPoDirectory(relPoFile, "po/git.pot"); err != nil {
			log.Warnf("run %d: failed to clean po/ directory: %v", runNum, err)
			// Continue with the run even if cleanup fails, but log the warning
		}

		// Reuse RunAgentUpdatePo for each run
		agentResult, err := RunAgentUpdatePo(cfg, agentName, poFile, true)

		// Convert AgentRunResult to RunResult
		// agentResult is never nil (always returns a result structure)
		result := RunResult{
			RunNumber:           runNum,
			Score:               agentResult.Score,
			PreValidationPass:   agentResult.PreValidationPass,
			PostValidationPass:  agentResult.PostValidationPass,
			AgentExecuted:       agentResult.AgentExecuted,
			AgentSuccess:        agentResult.AgentSuccess,
			PreValidationError:  agentResult.PreValidationError,
			PostValidationError: agentResult.PostValidationError,
			AgentError:          agentResult.AgentError,
			BeforeCount:         agentResult.BeforeCount,
			AfterCount:          agentResult.AfterCount,
			ExpectedBefore:      cfg.AgentTest.PoEntriesBeforeUpdate,
			ExpectedAfter:       cfg.AgentTest.PoEntriesAfterUpdate,
		}

		// If there was an error, log it but continue (for agent-test, we want to collect all results)
		if err != nil {
			log.Debugf("run %d: agent-run returned error: %v", runNum, err)
			// Error details are already in the result structure
		}

		results[i] = result
		totalScore += result.Score
		log.Debugf("run %d: completed with score %d/100", runNum, result.Score)
	}

	// Calculate average score
	averageScore := float64(totalScore) / float64(runs)
	log.Infof("all runs completed. Total score: %d/%d, Average: %.2f/100", totalScore, runs*100, averageScore)

	return results, averageScore, nil
}

// displayTestResults displays the test results in a readable format.
func displayTestResults(results []RunResult, averageScore float64, totalRuns int, elapsed time.Duration) {
	fmt.Println()
	fmt.Println("=" + strings.Repeat("=", 70))
	fmt.Println("Agent Test Results")
	fmt.Println("=" + strings.Repeat("=", 70))
	fmt.Println()

	successCount := 0
	failureCount := 0
	preValidationFailures := 0
	postValidationFailures := 0

	// Display individual run results
	for _, result := range results {
		status := "FAIL"
		if result.Score == 100 {
			status = "PASS"
			successCount++
		} else {
			failureCount++
		}

		fmt.Printf("Run %d: %s (Score: %d/100)\n", result.RunNumber, status, result.Score)

		// Show validation status
		if result.ExpectedBefore != nil && *result.ExpectedBefore != 0 {
			if result.PreValidationPass {
				fmt.Printf("  Pre-validation:  PASS (expected: %d, actual: %d)\n",
					*result.ExpectedBefore, result.BeforeCount)
			} else {
				fmt.Printf("  Pre-validation:  FAIL - %s\n", result.PreValidationError)
				preValidationFailures++
			}
		}

		if result.AgentExecuted {
			if result.AgentSuccess {
				fmt.Printf("  Agent execution: PASS\n")
			} else {
				fmt.Printf("  Agent execution: FAIL - %s\n", result.AgentError)
			}
		} else {
			fmt.Printf("  Agent execution: SKIPPED (pre-validation failed)\n")
		}

		if result.ExpectedAfter != nil && *result.ExpectedAfter != 0 {
			if result.PostValidationPass {
				fmt.Printf("  Post-validation: PASS (expected: %d, actual: %d)\n",
					*result.ExpectedAfter, result.AfterCount)
			} else {
				fmt.Printf("  Post-validation: FAIL - %s\n", result.PostValidationError)
				postValidationFailures++
			}
		} else if result.AgentExecuted {
			// Show entry counts even if validation is not configured
			fmt.Printf("  Entry count:     %d (before) -> %d (after)\n",
				result.BeforeCount, result.AfterCount)
		}

		fmt.Println()
	}

	// Display summary statistics
	fmt.Println("=" + strings.Repeat("=", 70))
	fmt.Println("Summary")
	fmt.Println("=" + strings.Repeat("=", 70))
	fmt.Printf("Total runs:        %d\n", totalRuns)
	fmt.Printf("Successful runs:   %d\n", successCount)
	fmt.Printf("Failed runs:       %d\n", failureCount)
	if preValidationFailures > 0 {
		fmt.Printf("Pre-validation failures: %d\n", preValidationFailures)
	}
	if postValidationFailures > 0 {
		fmt.Printf("Post-validation failures: %d\n", postValidationFailures)
	}
	fmt.Printf("Average score:     %.2f/100\n", averageScore)
	fmt.Printf("Execution time:    %s\n", elapsed.Round(time.Millisecond))
	fmt.Println("=" + strings.Repeat("=", 70))
}

// CmdAgentTestShowConfig displays the current agent configuration in YAML format.
// It reuses CmdAgentRunShowConfig from agent-run.
func CmdAgentTestShowConfig() error {
	return CmdAgentRunShowConfig()
}

// SaveTranslateResults saves the translation results to the output directory.
// It creates output/<agent-name>/<run-number>/ directory and copies the PO file
// and execution logs to preserve translation results for later review.
func SaveTranslateResults(agentName string, runNumber int, poFile string, stdout, stderr []byte) error {
	// Determine output directory path
	workDir := repository.WorkDir()
	outputDir := filepath.Join(workDir, "output", agentName, fmt.Sprintf("%d", runNumber))

	log.Debugf("saving translation results to %s", outputDir)

	// Create output directory if it doesn't exist
	if err := os.MkdirAll(outputDir, 0755); err != nil {
		return fmt.Errorf("failed to create output directory %s: %w", outputDir, err)
	}

	// Copy translated PO file to output directory
	poFileName := filepath.Base(poFile)
	destPoFile := filepath.Join(outputDir, poFileName)

	log.Debugf("copying %s to %s", poFile, destPoFile)

	// Read source PO file
	data, err := os.ReadFile(poFile)
	if err != nil {
		return fmt.Errorf("failed to read PO file %s: %w", poFile, err)
	}

	// Write to destination
	if err := os.WriteFile(destPoFile, data, 0644); err != nil {
		return fmt.Errorf("failed to write PO file to %s: %w", destPoFile, err)
	}

	// Save execution log (stdout + stderr)
	logFile := filepath.Join(outputDir, "translation.log")
	log.Debugf("saving execution log to %s", logFile)

	var logContent strings.Builder
	if len(stdout) > 0 {
		logContent.WriteString("=== STDOUT ===\n")
		logContent.Write(stdout)
		logContent.WriteString("\n")
	}
	if len(stderr) > 0 {
		logContent.WriteString("=== STDERR ===\n")
		logContent.Write(stderr)
		logContent.WriteString("\n")
	}

	if err := os.WriteFile(logFile, []byte(logContent.String()), 0644); err != nil {
		return fmt.Errorf("failed to write log file to %s: %w", logFile, err)
	}

	log.Infof("translation results saved to %s", outputDir)
	return nil
}

// SaveReviewResults saves review results to output directory for agent-test review.
// It creates the output directory structure, copies the PO file and JSON file,
// and saves the execution log. Files are overwritten if the directory exists.
// Returns error if any operation fails.
func SaveReviewResults(agentName string, runNumber int, poFile string, jsonFile string, stdout, stderr []byte) error {
	// Determine output directory path
	workDir := repository.WorkDir()
	outputDir := filepath.Join(workDir, "output", agentName, fmt.Sprintf("%d", runNumber))

	log.Debugf("saving review results to %s", outputDir)

	// Create output directory if it doesn't exist
	if err := os.MkdirAll(outputDir, 0755); err != nil {
		return fmt.Errorf("failed to create output directory %s: %w", outputDir, err)
	}

	// Copy PO file to output directory as XX-reviewed.po
	poFileName := filepath.Base(poFile)
	langCode := strings.TrimSuffix(poFileName, ".po")
	if langCode == "" || langCode == poFileName {
		return fmt.Errorf("invalid PO file path: %s (expected format: po/XX.po)", poFile)
	}
	destPoFile := filepath.Join(outputDir, fmt.Sprintf("%s-reviewed.po", langCode))

	log.Debugf("copying %s to %s", poFile, destPoFile)

	// Read source PO file
	poData, err := os.ReadFile(poFile)
	if err != nil {
		return fmt.Errorf("failed to read PO file %s: %w", poFile, err)
	}

	// Write to destination
	if err := os.WriteFile(destPoFile, poData, 0644); err != nil {
		return fmt.Errorf("failed to write PO file to %s: %w", destPoFile, err)
	}

	// Copy JSON file to output directory as XX-reviewed.json
	if jsonFile != "" {
		destJSONFile := filepath.Join(outputDir, fmt.Sprintf("%s-reviewed.json", langCode))

		log.Debugf("copying %s to %s", jsonFile, destJSONFile)

		// Read source JSON file
		jsonData, err := os.ReadFile(jsonFile)
		if err != nil {
			return fmt.Errorf("failed to read JSON file %s: %w", jsonFile, err)
		}

		// Write to destination
		if err := os.WriteFile(destJSONFile, jsonData, 0644); err != nil {
			return fmt.Errorf("failed to write JSON file to %s: %w", destJSONFile, err)
		}
	}

	// Save execution log (stdout + stderr) to review.log
	logFile := filepath.Join(outputDir, "review.log")
	log.Debugf("saving execution log to %s", logFile)

	var logContent strings.Builder
	if len(stdout) > 0 {
		logContent.WriteString("=== STDOUT ===\n")
		logContent.Write(stdout)
		logContent.WriteString("\n")
	}
	if len(stderr) > 0 {
		logContent.WriteString("=== STDERR ===\n")
		logContent.Write(stderr)
		logContent.WriteString("\n")
	}

	if err := os.WriteFile(logFile, []byte(logContent.String()), 0644); err != nil {
		return fmt.Errorf("failed to write log file to %s: %w", logFile, err)
	}

	log.Infof("review results saved to %s", outputDir)
	return nil
}

// CmdAgentTestTranslate implements the agent-test translate command logic.
// It runs the agent-run translate operation multiple times and calculates an average score.
func CmdAgentTestTranslate(agentName, poFile string, runs int, skipConfirmation bool) error {
	// Require user confirmation before proceeding
	if err := ConfirmAgentTestExecution(skipConfirmation); err != nil {
		return err
	}

	// Load configuration
	log.Debugf("loading agent configuration")
	cfg, err := config.LoadAgentConfig()
	if err != nil {
		log.Errorf("failed to load agent configuration: %v", err)
		return fmt.Errorf("failed to load agent configuration: %w\nHint: Ensure git-po-helper.yaml exists in repository root or user home directory", err)
	}

	// Determine number of runs
	if runs == 0 {
		if cfg.AgentTest.Runs != nil && *cfg.AgentTest.Runs > 0 {
			runs = *cfg.AgentTest.Runs
			log.Debugf("using runs from configuration: %d", runs)
		} else {
			runs = 5 // Default
			log.Debugf("using default number of runs: %d", runs)
		}
	} else {
		log.Debugf("using runs from command line: %d", runs)
	}

	log.Infof("starting agent-test translate with %d runs", runs)

	startTime := time.Now()

	// Run the test
	results, averageScore, err := RunAgentTestTranslate(agentName, poFile, runs, cfg)
	if err != nil {
		log.Errorf("agent-test execution failed: %v", err)
		return fmt.Errorf("agent-test failed: %w", err)
	}

	elapsed := time.Since(startTime)

	// Display results
	log.Debugf("displaying test results (average score: %.2f)", averageScore)
	displayTranslateTestResults(results, averageScore, runs, elapsed)

	log.Infof("agent-test translate completed successfully (average score: %.2f/100)", averageScore)
	return nil
}

// RunAgentTestTranslate runs the agent-test translate operation multiple times.
// It reuses RunAgentTranslate for each run and accumulates scores.
// Returns scores for each run, average score, and error.
func RunAgentTestTranslate(agentName, poFile string, runs int, cfg *config.AgentConfig) ([]RunResult, float64, error) {
	// Determine the agent to use (for saving results)
	selectedAgent, agentKey, err := SelectAgent(cfg, agentName)
	if err != nil {
		return nil, 0, err
	}
	_ = selectedAgent // Avoid unused variable warning

	// Determine PO file path
	poFile, err = GetPoFileAbsPath(cfg, poFile)
	if err != nil {
		return nil, 0, err
	}

	// Will clean poFile using relative path
	relPoFile, err := GetPoFileRelPath(cfg, poFile)
	if err != nil {
		log.Warnf("failed to get relative path of poFile: %v", err)
	}

	// Run the test multiple times
	results := make([]RunResult, runs)
	totalScore := 0
	for i := 0; i < runs; i++ {
		runNum := i + 1
		log.Infof("run %d/%d", runNum, runs)

		if err := CleanPoDirectory(relPoFile); err != nil {
			log.Warnf("run %d: failed to clean po/ directory: %v", runNum, err)
			// Continue with the run even if cleanup fails, but log the warning
		}

		// Reuse RunAgentTranslate for each run
		agentResult, err := RunAgentTranslate(cfg, agentName, poFile, true)

		// Convert AgentRunResult to RunResult
		// agentResult is never nil (always returns a result structure)
		result := RunResult{
			RunNumber:           runNum,
			Score:               agentResult.Score,
			PreValidationPass:   agentResult.PreValidationPass,
			PostValidationPass:  agentResult.PostValidationPass,
			AgentExecuted:       agentResult.AgentExecuted,
			AgentSuccess:        agentResult.AgentSuccess,
			PreValidationError:  agentResult.PreValidationError,
			PostValidationError: agentResult.PostValidationError,
			AgentError:          agentResult.AgentError,
			BeforeCount:         agentResult.BeforeCount,
			AfterCount:          agentResult.AfterCount,
			BeforeNewCount:      agentResult.BeforeNewCount,
			AfterNewCount:       agentResult.AfterNewCount,
			BeforeFuzzyCount:    agentResult.BeforeFuzzyCount,
			AfterFuzzyCount:     agentResult.AfterFuzzyCount,
			ExpectedBefore:      nil, // Not used for translate
			ExpectedAfter:       nil, // Not used for translate
		}

		// If there was an error, log it but continue (for agent-test, we want to collect all results)
		if err != nil {
			log.Debugf("run %d: agent-run returned error: %v", runNum, err)
			// Error details are already in the result structure
		}

		// Save translation results to output directory (ignore errors)
		if err := SaveTranslateResults(agentKey, runNum, poFile, nil, nil); err != nil {
			log.Warnf("run %d: failed to save translation results: %v", runNum, err)
			// Continue even if saving results fails
		}

		results[i] = result
		totalScore += result.Score
		log.Debugf("run %d: completed with score %d/100", runNum, result.Score)
	}

	// Calculate average score
	averageScore := float64(totalScore) / float64(runs)
	log.Infof("all runs completed. Total score: %d/%d, Average: %.2f/100", totalScore, runs*100, averageScore)

	return results, averageScore, nil
}

// RunAgentTestReview runs the agent-test review operation multiple times.
// It reuses RunAgentReview for each run, saves results to output directory,
// and accumulates scores. Returns scores for each run, average score, and error.
func RunAgentTestReview(cfg *config.AgentConfig, agentName, poFile string, runs int, commit, since string) ([]RunResult, float64, error) {
	// Determine the agent to use (for saving results)
	selectedAgent, agentKey, err := SelectAgent(cfg, agentName)
	if err != nil {
		return nil, 0, err
	}
	_ = selectedAgent // Avoid unused variable warning

	// Determine PO file path
	poFile, err = GetPoFileAbsPath(cfg, poFile)
	if err != nil {
		return nil, 0, err
	}

	// Will clean poFile using relative path
	relPoFile, err := GetPoFileRelPath(cfg, poFile)
	if err != nil {
		log.Warnf("failed to get relative path of poFile: %v", err)
	}

	// Run the test multiple times
	results := make([]RunResult, runs)
	totalScore := 0

	for i := 0; i < runs; i++ {
		runNum := i + 1
		log.Infof("run %d/%d", runNum, runs)

		if err := CleanPoDirectory(relPoFile); err != nil {
			log.Warnf("run %d: failed to clean po/ directory: %v", runNum, err)
			// Continue with the run even if cleanup fails, but log the warning
		}

		// Reuse RunAgentReview for each run
		agentResult, err := RunAgentReview(cfg, agentName, poFile, commit, since, true)

		// Convert AgentRunResult to RunResult
		// agentResult is never nil (always returns a result structure)
		result := RunResult{
			RunNumber:           runNum,
			PreValidationPass:   agentResult.PreValidationPass,
			PostValidationPass:  agentResult.PostValidationPass,
			AgentExecuted:       agentResult.AgentExecuted,
			AgentSuccess:        agentResult.AgentSuccess,
			PreValidationError:  agentResult.PreValidationError,
			PostValidationError: agentResult.PostValidationError,
			AgentError:          agentResult.AgentError,
			BeforeCount:         agentResult.BeforeCount,
			AfterCount:          agentResult.AfterCount,
			BeforeNewCount:      agentResult.BeforeNewCount,
			AfterNewCount:       agentResult.AfterNewCount,
			BeforeFuzzyCount:    agentResult.BeforeFuzzyCount,
			AfterFuzzyCount:     agentResult.AfterFuzzyCount,
			ExpectedBefore:      nil, // Not used for review
			ExpectedAfter:       nil, // Not used for review
		}

		// Calculate score from review JSON if available
		if agentResult.ReviewJSON != nil {
			// Score is already calculated in RunAgentReview and stored in ReviewScore
			result.Score = agentResult.ReviewScore
			log.Debugf("run %d: review score from JSON: %d (total_entries=%d, issues=%d)",
				runNum, agentResult.ReviewScore, agentResult.ReviewJSON.TotalEntries, len(agentResult.ReviewJSON.Issues))
		} else if agentResult.AgentSuccess {
			// If agent succeeded but no JSON, score is 0 (invalid output)
			log.Debugf("run %d: agent succeeded but no review JSON found, score=0", runNum)
			result.Score = 0
		} else {
			// If agent failed, score is 0
			log.Debugf("run %d: agent execution failed, score=0", runNum)
			result.Score = 0
		}

		// If there was an error, log it but continue (for agent-test, we want to collect all results)
		if err != nil {
			log.Debugf("run %d: agent-run returned error: %v", runNum, err)
			if agentResult.AgentError != "" {
				log.Debugf("run %d: agent error details: %s", runNum, agentResult.AgentError)
			}
			// Error details are already in the result structure
		}

		// Save review results to output directory (ignore errors)
		// Use reviewed.po file if available, otherwise use original poFile
		reviewedFile := agentResult.ReviewedFilePath
		if reviewedFile == "" {
			reviewedFile = poFile
		}
		if err := SaveReviewResults(agentKey, runNum, reviewedFile, agentResult.ReviewJSONPath, agentResult.AgentStdout, agentResult.AgentStderr); err != nil {
			log.Warnf("run %d: failed to save review results: %v", runNum, err)
			// Continue even if saving results fails
		}

		results[i] = result
		totalScore += result.Score
		log.Debugf("run %d: completed with score %d", runNum, result.Score)
	}

	// Calculate average score
	averageScore := float64(totalScore) / float64(runs)
	log.Infof("all runs completed. Total score: %d/%d, Average: %.2f", totalScore, runs, averageScore)

	return results, averageScore, nil
}

// displayTranslateTestResults displays the translation test results in a readable format.
func displayTranslateTestResults(results []RunResult, averageScore float64, totalRuns int, elapsed time.Duration) {
	fmt.Println()
	fmt.Println("=" + strings.Repeat("=", 70))
	fmt.Println("Agent Test Results (Translate)")
	fmt.Println("=" + strings.Repeat("=", 70))
	fmt.Println()

	successCount := 0
	failureCount := 0

	// Display individual run results
	for _, result := range results {
		status := "FAIL"
		if result.Score == 100 {
			status = "PASS"
			successCount++
		} else {
			failureCount++
		}

		fmt.Printf("Run %d: %s (Score: %d/100)\n", result.RunNumber, status, result.Score)

		// Show translation counts
		if result.AgentExecuted {
			fmt.Printf("  New entries:     %d (before) -> %d (after)\n",
				result.BeforeNewCount, result.AfterNewCount)
			fmt.Printf("  Fuzzy entries:   %d (before) -> %d (after)\n",
				result.BeforeFuzzyCount, result.AfterFuzzyCount)

			if result.AgentSuccess {
				fmt.Printf("  Agent execution: PASS\n")
			} else {
				fmt.Printf("  Agent execution: FAIL - %s\n", result.AgentError)
			}

			if result.PostValidationPass {
				fmt.Printf("  Validation:      PASS (all entries translated)\n")
			} else {
				fmt.Printf("  Validation:      FAIL - %s\n", result.PostValidationError)
			}
		} else {
			fmt.Printf("  Agent execution: SKIPPED (pre-validation failed)\n")
		}

		fmt.Println()
	}

	// Display summary statistics
	fmt.Println("=" + strings.Repeat("=", 70))
	fmt.Println("Summary")
	fmt.Println("=" + strings.Repeat("=", 70))
	fmt.Printf("Total runs:        %d\n", totalRuns)
	fmt.Printf("Successful runs:   %d\n", successCount)
	fmt.Printf("Failed runs:       %d\n", failureCount)
	fmt.Printf("Average score:     %.2f/100\n", averageScore)
	fmt.Printf("Execution time:    %s\n", elapsed.Round(time.Millisecond))
	fmt.Println("=" + strings.Repeat("=", 70))
}

// CmdAgentTestReview implements the agent-test review command logic.
// It runs the agent-run review operation multiple times and calculates an average score.
func CmdAgentTestReview(agentName, poFile string, runs int, skipConfirmation bool, commit, since string) error {
	// Require user confirmation before proceeding
	if err := ConfirmAgentTestExecution(skipConfirmation); err != nil {
		return err
	}

	// Load configuration
	log.Debugf("loading agent configuration")
	cfg, err := config.LoadAgentConfig()
	if err != nil {
		log.Errorf("failed to load agent configuration: %v", err)
		return fmt.Errorf("failed to load agent configuration: %w\nHint: Ensure git-po-helper.yaml exists in repository root or user home directory", err)
	}

	// Determine number of runs
	if runs == 0 {
		if cfg.AgentTest.Runs != nil && *cfg.AgentTest.Runs > 0 {
			runs = *cfg.AgentTest.Runs
			log.Debugf("using runs from configuration: %d", runs)
		} else {
			runs = 5 // Default
			log.Debugf("using default number of runs: %d", runs)
		}
	} else {
		log.Debugf("using runs from command line: %d", runs)
	}

	log.Infof("starting agent-test review with %d runs", runs)

	startTime := time.Now()

	// Run the test
	results, averageScore, err := RunAgentTestReview(cfg, agentName, poFile, runs, commit, since)
	if err != nil {
		log.Errorf("agent-test execution failed: %v", err)
		return fmt.Errorf("agent-test failed: %w", err)
	}

	elapsed := time.Since(startTime)

	// Display results
	log.Debugf("displaying test results (average score: %.2f)", averageScore)
	displayReviewTestResults(results, averageScore, runs, elapsed)

	log.Infof("agent-test review completed successfully (average score: %.2f/100)", averageScore)
	return nil
}

// displayReviewTestResults displays the review test results in a readable format.
func displayReviewTestResults(results []RunResult, averageScore float64, totalRuns int, elapsed time.Duration) {
	fmt.Println()
	fmt.Println("=" + strings.Repeat("=", 70))
	fmt.Println("Agent Test Results (Review)")
	fmt.Println("=" + strings.Repeat("=", 70))
	fmt.Println()

	successCount := 0
	failureCount := 0

	// Display individual run results
	for _, result := range results {
		status := "FAIL"
		if result.Score > 0 {
			status = "PASS"
			successCount++
		} else {
			failureCount++
		}

		fmt.Printf("Run %d: %s (Score: %d/100)\n", result.RunNumber, status, result.Score)

		// Show review details
		if result.AgentExecuted {
			if result.AgentSuccess {
				fmt.Printf("  Agent execution: PASS\n")
			} else {
				fmt.Printf("  Agent execution: FAIL - %s\n", result.AgentError)
			}

			if result.Score > 0 {
				fmt.Printf("  Review status:   PASS (valid JSON with score %d/100)\n", result.Score)
			} else {
				fmt.Printf("  Review status:   FAIL (no valid JSON or agent failed)\n")
			}
		} else {
			fmt.Printf("  Agent execution: SKIPPED\n")
		}

		fmt.Println()
	}

	// Display summary statistics
	fmt.Println("=" + strings.Repeat("=", 70))
	fmt.Println("Summary")
	fmt.Println("=" + strings.Repeat("=", 70))
	fmt.Printf("Total runs:        %d\n", totalRuns)
	fmt.Printf("Successful runs:   %d\n", successCount)
	fmt.Printf("Failed runs:       %d\n", failureCount)
	fmt.Printf("Average score:     %.2f/100\n", averageScore)
	fmt.Printf("Execution time:    %s\n", elapsed.Round(time.Millisecond))
	fmt.Println("=" + strings.Repeat("=", 70))
}
