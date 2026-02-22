// Package util provides review-related utilities for agent-run.
package util

import (
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"

	"github.com/git-l10n/git-po-helper/repository"
	log "github.com/sirupsen/logrus"
)

// ParseCommitSince parses commit and since parameters into baseCommit and newFileSource.
// - commit: orig is parent of commit, new is the specified commit
// - since: orig is since commit, new is current file (empty string)
// - neither: orig is HEAD, new is current file (empty string)
func ParseCommitSince(workDir, commit, since string) (baseCommit, newFileSource string) {
	if commit != "" {
		cmd := exec.Command("git", "rev-parse", commit+"^")
		cmd.Dir = workDir
		output, err := cmd.Output()
		if err != nil {
			baseCommit = "4b825dc642cb6eb9a060e54bf8d69288fbee4904" // Empty tree for root commit
		} else {
			baseCommit = strings.TrimSpace(string(output))
		}
		log.Infof("commit mode: orig from %s, new from %s", baseCommit, commit)
		return baseCommit, commit
	}
	if since != "" {
		log.Infof("since mode: orig from %s, new from current file", since)
		return since, ""
	}
	log.Infof("default mode: orig from HEAD, new from current file")
	return "HEAD", ""
}

// PrepareReviewData0 prepares data for review by creating orig.po, new.po, and review-input.po files.
// It gets the original file from git, sorts both files by msgid, and extracts differences.
func PrepareReviewData0(poFile, commit, since, outputFile string) error {
	workDir := repository.WorkDir()
	srcCommit, targetCommit := ParseCommitSince(workDir, commit, since)
	return PrepareReviewData(srcCommit, poFile, targetCommit, poFile, outputFile)
}

func PrepareReviewData(oldCommit, oldFile, newCommit, newFile, outputFile string) error {
	var (
		err                    error
		workDir                = repository.WorkDir()
		relOldFile, relNewFile string
	)

	// Use temp files for orig and new; they are deleted when the function returns
	oldTmpFile, err := os.CreateTemp("", "review-old-*.po")
	if err != nil {
		return fmt.Errorf("failed to create temp old file: %w", err)
	}
	oldTmpFile.Close()
	defer func() {
		os.Remove(oldTmpFile.Name())
	}()

	newTmpFile, err := os.CreateTemp("", "review-new-*.po")
	if err != nil {
		return fmt.Errorf("failed to create temp new file: %w", err)
	}
	newTmpFile.Close()
	defer func() {
		os.Remove(newTmpFile.Name())
	}()

	log.Debugf("preparing review data: orig=%s, new=%s, review-input=%s",
		oldTmpFile.Name(), newTmpFile.Name(), outputFile)

	// Get original file from git
	log.Infof("getting old file from commit: %s", oldCommit)
	// Convert absolute path to relative path for git show command
	if filepath.IsAbs(oldFile) {
		relOldFile, err = filepath.Rel(workDir, oldFile)
		if err != nil {
			return fmt.Errorf("failed to convert PO file path to relative: %w", err)
		}
	} else {
		relOldFile = oldFile
	}
	// Normalize to use forward slashes (git uses forward slashes in paths)
	relOldFile = filepath.ToSlash(relOldFile)
	oldFileRevision := FileRevision{
		Revision: oldCommit,
		File:     relOldFile,
		Tmpfile:  oldTmpFile.Name(),
	}
	if err := checkoutTmpfile(&oldFileRevision); err != nil {
		// Check if error is because file doesn't exist in the commit
		if strings.Contains(err.Error(), "does not exist in") {
			// If file doesn't exist in that commit, create empty file
			log.Infof("file %s not found in commit %s, using empty file as original", relOldFile, oldCommit)
			if err := os.WriteFile(oldFileRevision.Tmpfile, []byte{}, 0644); err != nil {
				return fmt.Errorf("failed to create empty orig file: %w", err)
			}
		} else {
			// For other errors, return them
			return fmt.Errorf("failed to get original file from commit %s: %w", oldCommit, err)
		}
	}

	log.Infof("getting new file from commit: %s", newCommit)
	// Convert absolute path to relative path for git show command
	if filepath.IsAbs(newFile) {
		relNewFile, err = filepath.Rel(workDir, newFile)
		if err != nil {
			return fmt.Errorf("failed to convert PO file path to relative: %w", err)
		}
	} else {
		relNewFile = newFile
	}
	// Normalize to use forward slashes (git uses forward slashes in paths)
	relNewFile = filepath.ToSlash(relNewFile)
	newFileRevision := FileRevision{
		Revision: newCommit,
		File:     relNewFile,
		Tmpfile:  newTmpFile.Name(),
	}
	if err := checkoutTmpfile(&newFileRevision); err != nil {
		// Check if error is because file doesn't exist in the commit
		if strings.Contains(err.Error(), "does not exist in") {
			// If file doesn't exist in that commit, create empty file
			log.Infof("file %s not found in commit %s, using empty file as original", relNewFile, newCommit)
			if err := os.WriteFile(newFileRevision.Tmpfile, []byte{}, 0644); err != nil {
				return fmt.Errorf("failed to create empty new file: %w", err)
			}
		} else {
			// For other errors, return them
			return fmt.Errorf("failed to get new file from commit %s: %w", newCommit, err)
		}
	}

	// Extract differences: use msgcmp to find entries that are different or new
	// We'll use a simpler approach: extract entries from new that don't match orig
	log.Debugf("extracting differences to review-input.po")
	if err := extractReviewInput(oldFileRevision.Tmpfile, newFileRevision.Tmpfile, outputFile); err != nil {
		return fmt.Errorf("failed to extract review input: %w", err)
	}

	log.Infof("review data prepared: review-input=%s", outputFile)
	return nil
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
	origEntries, _, err := ParsePoEntries(origData)
	if err != nil {
		return fmt.Errorf("failed to parse orig file: %w", err)
	}
	newEntries, newHeader, err := ParsePoEntries(newData)
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
// When outputPath is "-" or "" and entries is empty, writes nothing (for new-entries command).
func writeReviewInputPo(outputPath string, header []string, entries []*PoEntry) error {
	if (outputPath == "-" || outputPath == "") && len(entries) == 0 {
		return nil
	}

	var content strings.Builder

	// Write header
	// Header structure:
	// - Comments (if any) - lines starting with #
	// - msgid "" - line starting with msgid
	// - msgstr "" - line starting with msgstr
	// - Continuation lines - already wrapped in quotes (preserved from parsePoEntries)
	if len(entries) > 0 {
		for _, line := range header {
			content.WriteString(line)
			// Only add newline if the line doesn't already end with \n
			if !strings.HasSuffix(line, "\n") {
				content.WriteString("\n")
			}
		}

		// Add empty line after header
		content.WriteString("\n")
	}

	// Write entries
	for _, entry := range entries {
		for _, line := range entry.RawLines {
			content.WriteString(line)
			content.WriteString("\n")
		}
		content.WriteString("\n")
	}

	data := []byte(content.String())
	if outputPath == "-" || outputPath == "" {
		_, err := os.Stdout.Write(data)
		return err
	}
	return os.WriteFile(outputPath, data, 0644)
}
