// Package util provides business logic for agent-test command.
package util

import (
	"fmt"
	"strings"

	"github.com/git-l10n/git-po-helper/config"
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
// It reuses RunAgentUpdatePot for each run and accumulates scores.
// Returns scores for each run, average score, and error.
func RunAgentTestUpdatePot(agentName string, runs int, cfg *config.AgentConfig) ([]RunResult, float64, error) {
	// Run the test multiple times
	results := make([]RunResult, runs)
	totalScore := 0

	for i := 0; i < runs; i++ {
		runNum := i + 1
		log.Infof("run %d/%d", runNum, runs)

		// Reuse RunAgentUpdatePot for each run
		agentResult, err := RunAgentUpdatePot(cfg, agentName)

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
// It reuses CmdAgentRunShowConfig from agent-run.
func CmdAgentTestShowConfig() error {
	return CmdAgentRunShowConfig()
}
