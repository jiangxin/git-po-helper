// Package util provides git-related utilities for po file operations.
package util

import (
	"fmt"
	"os/exec"
	"strings"

	"github.com/git-l10n/git-po-helper/repository"
	log "github.com/sirupsen/logrus"
)

// GetChangedPoFiles returns the list of changed po/XX.po files between two git versions.
// For commit mode (commit != ""): uses git diff-tree -r --name-only <baseCommit> <commit> -- po/
// For since/default mode: uses git diff -r --name-only <baseCommit> -- po/
// Returns only .po files (not .pot) under po/ directory.
func GetChangedPoFiles(commit, since string) ([]string, error) {
	workDir := repository.WorkDir()
	var baseCommit string
	var cmd *exec.Cmd

	if commit != "" {
		// Commit mode: compare parent of commit with commit
		revParseCmd := exec.Command("git", "rev-parse", commit+"^")
		revParseCmd.Dir = workDir
		output, err := revParseCmd.Output()
		if err != nil {
			baseCommit = "4b825dc642cb6eb9a060e54bf8d69288fbee4904" // Empty tree
		} else {
			baseCommit = strings.TrimSpace(string(output))
		}
		cmd = exec.Command("git", "diff-tree", "-r", "--name-only", baseCommit, commit, "--", PoDir)
		log.Debugf("getting changed po files: git diff-tree -r --name-only %s %s -- %s", baseCommit, commit, PoDir)
	} else if since != "" {
		// Since mode: compare since commit with working tree
		baseCommit = since
		cmd = exec.Command("git", "diff", "-r", "--name-only", baseCommit, "--", PoDir)
		log.Debugf("getting changed po files: git diff -r --name-only %s -- %s", baseCommit, PoDir)
	} else {
		// Default mode: compare HEAD with working tree
		baseCommit = "HEAD"
		cmd = exec.Command("git", "diff", "-r", "--name-only", baseCommit, "--", PoDir)
		log.Debugf("getting changed po files: git diff -r --name-only %s -- %s", baseCommit, PoDir)
	}

	cmd.Dir = workDir
	output, err := cmd.Output()
	if err != nil {
		return nil, fmt.Errorf("failed to get changed po files: %w", err)
	}

	// Filter to only .po files (not .pot)
	var poFiles []string
	for _, line := range strings.Split(strings.TrimSpace(string(output)), "\n") {
		line = strings.TrimSpace(line)
		if line == "" {
			continue
		}
		if strings.HasSuffix(line, ".po") {
			poFiles = append(poFiles, line)
		}
	}
	return poFiles, nil
}
