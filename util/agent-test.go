// Package util provides business logic for agent-test command.
package util

import (
	"fmt"
	"os"
	"strings"

	"gopkg.in/yaml.v3"

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
	ExpectedBefore      *int
	ExpectedAfter       *int
}

// CmdAgentTestUpdatePot implements the agent-test update-pot command logic.
// It runs the agent-run update-pot operation multiple times and calculates an average score.
func CmdAgentTestUpdatePot(agentName string, runs int) error {
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

	// Run the test
	results, averageScore, err := RunAgentTestUpdatePot(agentName, runs, cfg)
	if err != nil {
		log.Errorf("agent-test execution failed: %v", err)
		return fmt.Errorf("agent-test failed: %w", err)
	}

	// Display results
	log.Debugf("displaying test results (average score: %.2f)", averageScore)
	displayTestResults(results, averageScore, runs)

	log.Infof("agent-test update-pot completed successfully (average score: %.2f/100)", averageScore)
	return nil
}

// RunAgentTestUpdatePot runs the agent-test update-pot operation multiple times.
// Returns scores for each run, average score, and error.
func RunAgentTestUpdatePot(agentName string, runs int, cfg *config.AgentConfig) ([]RunResult, float64, error) {
	// Determine agent to use
	selectedAgent, agentKey, err := SelectAgent(cfg, agentName)
	if err != nil {
		return nil, 0, err
	}

	log.Debugf("using agent: %s", agentKey)

	// Get POT file path
	potFile := GetPotFilePath()

	// Get prompt from configuration
	prompt, err := GetPrompt(cfg)
	if err != nil {
		return nil, 0, err
	}

	// Build agent command with placeholders replaced
	agentCmd := BuildAgentCommand(selectedAgent, prompt, "", "")

	// Run the test multiple times
	results := make([]RunResult, runs)
	totalScore := 0

	for i := 0; i < runs; i++ {
		runNum := i + 1
		log.Infof("run %d/%d", runNum, runs)

		result := RunResult{
			RunNumber:          runNum,
			Score:              0,
			PreValidationPass:  false,
			PostValidationPass: false,
			AgentExecuted:      false,
			AgentSuccess:       false,
			ExpectedBefore:     cfg.AgentTest.PotEntriesBeforeUpdate,
			ExpectedAfter:      cfg.AgentTest.PotEntriesAfterUpdate,
		}

		// Pre-validation: Check entry count before update
		if cfg.AgentTest.PotEntriesBeforeUpdate != nil && *cfg.AgentTest.PotEntriesBeforeUpdate != 0 {
			log.Debugf("run %d: performing pre-validation (expected: %d entries)", runNum, *cfg.AgentTest.PotEntriesBeforeUpdate)
			beforeCount, err := CountPotEntries(potFile)
			if err != nil {
				result.PreValidationError = fmt.Sprintf("failed to count entries: %v", err)
				log.Errorf("run %d: pre-validation failed - %s", runNum, result.PreValidationError)
				results[i] = result
				totalScore += 0 // Score = 0 for failure
				continue
			}

			result.BeforeCount = beforeCount
			if beforeCount != *cfg.AgentTest.PotEntriesBeforeUpdate {
				result.PreValidationError = fmt.Sprintf("entry count before update: expected %d, got %d",
					*cfg.AgentTest.PotEntriesBeforeUpdate, beforeCount)
				log.Errorf("run %d: pre-validation failed - %s", runNum, result.PreValidationError)
				results[i] = result
				totalScore += 0 // Score = 0 for failure
				continue        // Skip agent execution if pre-validation fails
			}

			result.PreValidationPass = true
			log.Debugf("run %d: pre-validation passed (%d entries)", runNum, beforeCount)
		} else {
			// No pre-validation configured, count entries for display purposes
			beforeCount, err := CountPotEntries(potFile)
			if err == nil {
				result.BeforeCount = beforeCount
			}
			result.PreValidationPass = true // Consider it passed if not configured
		}

		// Execute agent command (only if pre-validation passed or was disabled)
		workDir := repository.WorkDir()
		log.Debugf("run %d: executing agent command", runNum)
		stdout, stderr, err := ExecuteAgentCommand(agentCmd, workDir)
		result.AgentExecuted = true

		if err != nil {
			result.AgentSuccess = false
			result.AgentError = err.Error()
			if len(stderr) > 0 {
				result.AgentError += "\nstderr: " + string(stderr)
			}
			log.Errorf("run %d: agent command failed: %s", runNum, result.AgentError)
		} else {
			result.AgentSuccess = true
			log.Debugf("run %d: agent command completed successfully", runNum)
			if len(stdout) > 0 {
				log.Debugf("run %d: agent stdout: %s", runNum, string(stdout))
			}
		}

		// Post-validation: Check entry count after update
		if cfg.AgentTest.PotEntriesAfterUpdate != nil && *cfg.AgentTest.PotEntriesAfterUpdate != 0 {
			log.Debugf("run %d: performing post-validation (expected: %d entries)", runNum, *cfg.AgentTest.PotEntriesAfterUpdate)
			afterCount, err := CountPotEntries(potFile)
			if err != nil {
				result.PostValidationError = fmt.Sprintf("failed to count entries: %v", err)
				log.Errorf("run %d: post-validation failed - %s", runNum, result.PostValidationError)
				result.Score = 0
				results[i] = result
				totalScore += 0
				continue
			}

			result.AfterCount = afterCount
			if afterCount != *cfg.AgentTest.PotEntriesAfterUpdate {
				result.PostValidationError = fmt.Sprintf("entry count after update: expected %d, got %d",
					*cfg.AgentTest.PotEntriesAfterUpdate, afterCount)
				log.Errorf("run %d: post-validation failed - %s", runNum, result.PostValidationError)
				result.Score = 0
			} else {
				result.PostValidationPass = true
				result.Score = 100
				log.Debugf("run %d: post-validation passed (%d entries)", runNum, afterCount)
			}
		} else {
			// No post-validation configured, score based on agent exit code
			afterCount, err := CountPotEntries(potFile)
			if err == nil {
				result.AfterCount = afterCount
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
			log.Debugf("run %d: validating POT file syntax", runNum)
			if err := ValidatePoFile(potFile); err != nil {
				log.Warnf("run %d: POT file syntax validation failed: %v", runNum, err)
				// Don't fail the run for syntax errors, but log it
			} else {
				log.Debugf("run %d: POT file syntax validation passed", runNum)
			}
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
func displayTestResults(results []RunResult, averageScore float64, totalRuns int) {
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
	fmt.Println("=" + strings.Repeat("=", 70))
}

// CmdAgentTestShowConfig displays the current agent configuration in YAML format.
func CmdAgentTestShowConfig() error {
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
