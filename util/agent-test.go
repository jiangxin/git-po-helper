// Package util provides business logic for agent-test command.
package util

import (
	"fmt"
	"path/filepath"
	"strings"

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
	cfg, err := config.LoadAgentConfig()
	if err != nil {
		return fmt.Errorf("failed to load agent configuration: %w", err)
	}

	// Determine number of runs
	if runs == 0 {
		if cfg.AgentTest.Runs != nil && *cfg.AgentTest.Runs > 0 {
			runs = *cfg.AgentTest.Runs
		} else {
			runs = 5 // Default
		}
	}

	log.Infof("starting agent-test update-pot with %d runs", runs)

	// Run the test
	results, averageScore, err := RunAgentTestUpdatePot(agentName, runs, cfg)
	if err != nil {
		return fmt.Errorf("agent-test failed: %w", err)
	}

	// Display results
	displayTestResults(results, averageScore, runs)

	return nil
}

// RunAgentTestUpdatePot runs the agent-test update-pot operation multiple times.
// Returns scores for each run, average score, and error.
func RunAgentTestUpdatePot(agentName string, runs int, cfg *config.AgentConfig) ([]RunResult, float64, error) {
	// Determine agent to use (same logic as agent-run)
	var selectedAgent config.Agent
	var agentKey string

	if agentName != "" {
		// Use specified agent
		agent, ok := cfg.Agents[agentName]
		if !ok {
			return nil, 0, fmt.Errorf("agent '%s' not found in configuration", agentName)
		}
		selectedAgent = agent
		agentKey = agentName
	} else {
		// Auto-select agent
		if len(cfg.Agents) == 0 {
			return nil, 0, fmt.Errorf("no agents configured")
		}
		if len(cfg.Agents) > 1 {
			agentList := make([]string, 0, len(cfg.Agents))
			for k := range cfg.Agents {
				agentList = append(agentList, k)
			}
			return nil, 0, fmt.Errorf("multiple agents configured (%s), please specify --agent", strings.Join(agentList, ", "))
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

	// Get prompt from configuration
	prompt := cfg.Prompt.UpdatePot
	if prompt == "" {
		return nil, 0, fmt.Errorf("prompt.update_pot is not configured")
	}

	// Replace placeholders in agent command
	agentCmd := make([]string, len(selectedAgent.Cmd))
	for i, arg := range selectedAgent.Cmd {
		agentCmd[i] = ReplacePlaceholders(arg, prompt, "", "")
	}

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
			log.Debugf("run %d: performing pre-validation", runNum)
			beforeCount, err := CountPotEntries(potFile)
			if err != nil {
				result.PreValidationError = fmt.Sprintf("failed to count entries: %v", err)
				log.Errorf("run %d: pre-validation failed: %s", runNum, result.PreValidationError)
				results[i] = result
				totalScore += 0 // Score = 0 for failure
				continue
			}

			result.BeforeCount = beforeCount
			if beforeCount != *cfg.AgentTest.PotEntriesBeforeUpdate {
				result.PreValidationError = fmt.Sprintf("entry count before update: expected %d, got %d",
					*cfg.AgentTest.PotEntriesBeforeUpdate, beforeCount)
				log.Errorf("run %d: pre-validation failed: %s", runNum, result.PreValidationError)
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
			log.Debugf("run %d: performing post-validation", runNum)
			afterCount, err := CountPotEntries(potFile)
			if err != nil {
				result.PostValidationError = fmt.Sprintf("failed to count entries: %v", err)
				log.Errorf("run %d: post-validation failed: %s", runNum, result.PostValidationError)
				result.Score = 0
				results[i] = result
				totalScore += 0
				continue
			}

			result.AfterCount = afterCount
			if afterCount != *cfg.AgentTest.PotEntriesAfterUpdate {
				result.PostValidationError = fmt.Sprintf("entry count after update: expected %d, got %d",
					*cfg.AgentTest.PotEntriesAfterUpdate, afterCount)
				log.Errorf("run %d: post-validation failed: %s", runNum, result.PostValidationError)
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
			if err := ValidatePotFile(potFile); err != nil {
				log.Warnf("run %d: POT file validation failed: %v", runNum, err)
				// Don't fail the run for syntax errors, but log it
			}
		}

		results[i] = result
		totalScore += result.Score
	}

	// Calculate average score
	averageScore := float64(totalScore) / float64(runs)

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
