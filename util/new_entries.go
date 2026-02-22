// Package util provides new-entries command logic.
package util

import (
	"fmt"
	"path/filepath"

	"github.com/git-l10n/git-po-helper/repository"
	log "github.com/sirupsen/logrus"
)

// CmdNewEntries implements the new-entries command logic.
// It compares two PO file versions and outputs new/changed entries to stdout.
// Reuses PrepareReviewData (with output "-") and ResolvePoFile for po file selection.
func CmdNewEntries(poFile, commit, since string) error {
	workDir := repository.WorkDir()

	// Resolve poFile when not specified
	if poFile == "" {
		changedPoFiles, err := GetChangedPoFiles(commit, since)
		if err != nil {
			return fmt.Errorf("failed to get changed po files: %w", err)
		}

		poFile, err = ResolvePoFile(poFile, changedPoFiles)
		if err != nil {
			return err
		}
	}

	// Convert to absolute path
	if !filepath.IsAbs(poFile) {
		poFile = filepath.Join(workDir, poFile)
	}

	// Check if PO file exists
	if !Exist(poFile) {
		return fmt.Errorf("PO file does not exist: %s", poFile)
	}

	log.Debugf("outputting new entries for: %s", poFile)
	return PrepareReviewData(poFile, commit, since, "-")
}
