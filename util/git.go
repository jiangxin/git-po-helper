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
	var rev1, rev2 string

	if commit != "" {
		rev1 = commit + "~"
		rev2 = commit
	} else if since != "" {
		rev1 = since
		rev2 = ""
	} else {
		rev1 = "HEAD"
		rev2 = "" // working tree
	}
	return GetChangedPoFilesRange(rev1, rev2)
}

func GetChangedPoFilesRange(rev1, rev2 string) ([]string, error) {
	var (
		cmd     *exec.Cmd
		workDir = repository.WorkDir()
	)

	if rev1 != "" && rev2 != "" {
		cmd = exec.Command("git", "diff-tree", "-r", "--name-only", rev1, rev2, "--", PoDir)
		log.Debugf("getting changed po files: git diff-tree -r --name-only %s %s -- %s", rev1, rev2, PoDir)
	} else if rev1 != "" && rev2 == "" {
		// Since mode: compare since commit with working tree
		cmd = exec.Command("git", "diff", "-r", "--name-only", rev1, "--", PoDir)
		log.Debugf("getting changed po files: git diff -r --name-only %s -- %s", rev1, PoDir)
	} else {
		// Default mode: compare HEAD with working tree
		return nil, fmt.Errorf("rev1 is nil for GetChangedPoFilesRange")
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
